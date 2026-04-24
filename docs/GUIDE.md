# Hands-On Worksheet: Go Concurrency Patterns (`gpx-merge`)

Time: ~2.5 hours total
Goal: understand how a bounded worker pool built on `errgroup` provides concurrency, fail-fast error handling, cancellation, and deterministic output in a single primitive.

## 0. Code layout (5 min)

Key files and responsibilities:

- `internal/app/app.go` keeps orchestration in `Run(...)`.
- Per-file processing logic is in `internal/processor/process_file.go` (`processor.FileProcessor.Process`).
- Post-pipeline aggregation is in `internal/processor/aggregate.go` (`processor.AggregateResults`).
- `internal/pool/run.go` is the generic concurrency engine.

`app.Run` keeps the original discovery indexes for output order, then submits work to `pool.Run` in descending file-size order. `pool.Run` creates an `errgroup` via `errgroup.WithContext`, bounds parallelism with `g.SetLimit(workers)`, launches one `g.Go(...)` goroutine per scheduled file, and blocks on `g.Wait()`. Each goroutine calls `process(ctx, f)` and writes its result to `collected[f.Index]`. `g.Wait()` returns `(collected, firstErr)` — no jobs channel, no results channel, no closer goroutine, no output sort.

## 1. Prep (10 min)

Run:

```bash
go test ./...
```

Expected:

- Tests pass.
- You confirm the project is in a good starting state.

## 2. Map the concurrency graph (20 min)

Open `internal/pool/run.go` and `internal/app/app.go`, then identify:

- Where `errgroup.WithContext` creates the group and derived context.
- Where `g.SetLimit(workers)` bounds parallelism.
- Where `collected` is pre-allocated.
- Where `g.Go(...)` launches one goroutine per file.
- Where `g.Wait()` blocks until all goroutines finish and returns the first error.
- Where `Run(...)` wires in `fileProc.Process` (`app.go:67`).
- Where the `pool.Run` error is checked and the run aborts (`app.go:68–71`).
- Where `AggregateResults` is called (`app.go:72`).

Mini-task:

- Write a 6-line summary of control flow from `Run(...)` entry to return.

Expected:

- You can point to lines for errgroup setup, `collected` pre-allocation, `g.Go` fan-out, `g.Wait()`, `fileProc.Process` wiring, and `processor.AggregateResults(...)` call.

## 3. The errgroup worker pool (40 min)

`internal/pool/run.go` uses one pattern: a bounded worker pool via `errgroup`. Everything else (fan-out, error handling, cancellation, shutdown) is a consequence of how `errgroup` works — not separate patterns.

**Fan-out and direct index writes**

`g.SetLimit(workers)` + `g.Go(...)` distributes work across up to `workers` concurrent goroutines. The input work slice is size-descending, so larger files are started earlier while smaller files fill the pool around them. There is no fan-in: each goroutine writes directly to `collected[f.Index]`, a pre-allocated slice. This eliminates the results channel, closer goroutine, and send-or-cancel `select` entirely. It is safe because each file has a unique index and no two goroutines process the same file — no mutex needed.

**Fail-fast error handling**

When any goroutine returns an error, `errgroup` captures it and cancels the shared context. In-flight goroutines re-check `ctx.Err()` between the major `process` stages, during XML token reads inside `gpx.ParseFile`, and between segments inside `optimizeTrack`, so they stop without having to finish the rest of a large file. `g.Wait()` returns the first error once all goroutines have exited. `pool.Run` returns `(collected, err)`.

The same cancellation path applies to SIGINT/SIGTERM: `signal.NotifyContext` (`app.go:26`) cancels the root context, which propagates into the errgroup-derived context.

**Shutdown**

`g.Wait()` is the only shutdown mechanism. There is no jobs channel or results channel to close. Each goroutine processes one file and returns; `g.Wait()` unblocks when all have finished.

Run:

```bash
go test ./... -run TestRunDeterministicAcrossWorkerCounts
go test ./... -run TestRunFailsOnInvalidFile
```

Mini-tasks:

- Explain why outputs from `workers=1` and `workers=8` are byte-equal.
- Trace the error path end-to-end: `processor.FileProcessor.Process(...)` returns a `fileError{Path, Stage, Err}` → errgroup cancels ctx and captures the error → `g.Wait()` returns it → `app.Run` prints `"process files: <path>: <stage>: <err>"` to stderr and returns exit code 1. No output is written.
- Explain why `errgroup.WithContext` replaces `sync.WaitGroup` + `sync.Once` + `context.WithCancel`. (`errgroup` encapsulates all three in a single, well-tested API.)
- Explain why there is no risk of writing to a closed channel or reading from a closed channel in this design.

## 4. Determinism guarantee: output order after concurrency (15 min)

Goroutines finish in nondeterministic order depending on OS scheduling, file size, and parse time. Output order is still deterministic because each goroutine writes to `collected[f.Index]`, and `f.Index` was assigned sequentially from the lexical discovery list before any concurrency started. The work slice can be size-descending because `collected` is keyed by original index — no output sort needed.

Run:

```bash
go test ./... -run TestRunDeterministicOrder
```

Focus file: `internal/pool/order_test.go`

Note: the process closure uses `rand.New(rand.NewSource(int64(42 + f.Index)))` — each goroutine gets its own `*rand.Rand`. This avoids a data race: `math/rand.Rand` is not goroutine-safe, so sharing one instance would be flagged by `go test -race ./...`.

Mini-tasks:

- Explain why random goroutine timing does not affect final order.
- Explain what is aggregated in `processor.AggregateResults(...)` and why keeping aggregation separate from `Run(...)` improves maintainability. (`Run` is generic and knows nothing about GPX or tracks. Keeping domain logic in `AggregateResults` means the pool can be reused for any file type without modification.)

## 5. Parallel tests (10 min)

Scan:

```bash
rg -n "t\.Parallel\(" internal
```

Mini-task:

- Pick 2 tests and explain why parallel test execution is safe there.

Expected:

- You mention `t.TempDir()` and isolated test state. Each test constructs all its state locally with no shared mutable globals, so concurrent execution cannot cause interference.

## Checkpoint questions (self-test)

1. Why is writing to `collected[f.Index]` from multiple goroutines safe without a mutex?
2. Why is the output of `Run` already in deterministic order without a sort?
3. What happens to goroutines that are still running when one goroutine returns an error?
4. Why is there no risk of a `send on closed channel` panic in this design?
5. What would need to change if two files could share the same index?
6. Why does `errgroup.WithContext` replace `sync.WaitGroup` + `sync.Once` + `context.WithCancel`?

## Optional follow-up exercises

- Add `context.WithTimeout` support around `pool.Run`. Add a test that a timeout cancels in-flight goroutines and returns a deadline-exceeded error.

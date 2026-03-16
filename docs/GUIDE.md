# Hands-On Worksheet: Go Concurrency Patterns (`gpx-merge`)

Time: ~2.5 hours total
Goal: learn worker pool, fan-out, direct index writes, context-driven cancellation, graceful shutdown, and deterministic output guarantees.

## 0. Code layout (5 min)

Key files and responsibilities:

- `internal/app/app.go` keeps orchestration in `Run(...)`.
- Per-file processing logic is in `internal/processor/process_file.go` (`processor.FileProcessor.Process`).
- Post-pipeline aggregation is in `internal/processor/aggregate.go` (`processor.AggregateResults`).
- `internal/pool/run.go` is the generic concurrency engine.

The full data flow through `pool.Run` involves two roles. The `jobs` channel is pre-buffered to `len(files)` and filled synchronously before any goroutine starts. A pre-allocated `collected []Result` slice of length `len(files)` holds outputs. The **N worker** goroutines each range over `jobs`, call `process(ctx, f)`, and write the result directly to `collected[f.Index]`. The main goroutine calls `wg.Wait()` then returns `collected` — no results channel, no closer goroutine, no sort.

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

- Where jobs are pre-filled (lines 27–31 of `run.go`: synchronous buffer fill before workers start).
- Where `collected` is pre-allocated (line 33).
- Where workers start (lines 35–46).
- Where `wg.Wait()` blocks until all workers finish (line 48).
- Where `Run(...)` wires in `fileProc.Process` (`app.go:67`).
- Where `AggregateResults` is called (`app.go:68`).

Mini-task:

- Write a 6-line summary of control flow from `Run(...)` entry to return.

Expected:

- You can point to lines for jobs pre-fill, `collected` pre-allocation, worker start, `wg.Wait()`, `fileProc.Process` wiring, and `processor.AggregateResults(...)` call.

## 3. Pattern: worker pool + fan-out + direct index writes (20 min)

Fan-out means the pre-buffered `jobs` channel distributes work across many consumers (the workers). There is no fan-in: workers write results directly to `collected[f.Index]`, a pre-allocated slice, eliminating the results channel, closer goroutine, and send-or-cancel `select` entirely. This is safe because each file has a unique index and no two workers process the same file — no mutex needed. The main goroutine calls `wg.Wait()` and returns `collected` in deterministic order without any sort.

Run:

```bash
go test ./... -run TestRunDeterministicAcrossWorkerCounts
```

Focus file: `internal/app/app_integration_test.go`

Mini-task:

- Explain why outputs from workers=1 and workers=8 are byte-equal.

Expected:

- You mention that workers write to `collected[f.Index]` directly, so `collected` is already in deterministic order when `wg.Wait()` returns — no sort needed. Workers finishing in arbitrary order doesn't affect output order.

Follow-up: a classic fan-in design uses a results channel, a closer goroutine, and a send-or-cancel `select`. Explain why this design needs none of those three things.

## 4. Pattern: context cancellation propagation (20 min)

Cancellation propagates from parent to children through the shared `ctx` value. No explicit signal passing is needed — every goroutine that holds `ctx` can observe cancellation independently.

The three-step path:

1. The parent context is canceled — by `signal.NotifyContext` on SIGINT/SIGTERM (`app.go:26`), a timeout, or an explicit `cancel()` call.
2. Workers currently calling `process(ctx, f)` have their call return quickly (with a context error) because they hold the cancelled `ctx`. Workers then write the error result to `collected[f.Index]` and loop back to dequeue the next job, where `process` returns quickly again.
3. Once the `jobs` channel is drained, all workers return and `wg.Wait()` unblocks. `Run` returns `collected` with partial results — processed files have their real result, cancelled files have a context-error result.

Focus:

- App root signal context in `internal/app/app.go` (`signal.NotifyContext`).

Extra mini-task:

- Trace the error path end-to-end: `processor.FileProcessor.Process(...)` returns an error → worker wraps it in `Result{Err: err}` and sends it → `AggregateResults` detects `r.Err != nil` and appends to `errorsOut`, skipping the file's tracks → `report.PrintFailedFiles` reports it at the end. Other files continue processing unaffected.

## 5. Pattern: graceful shutdown ownership (20 min)

There is no results channel to close. Shutdown is simple:

- Workers range over `jobs` until it is exhausted (it was closed before workers started).
- Each worker calls `wg.Done()` via `defer wg.Done()` when its loop exits.
- The main goroutine calls `wg.Wait()`, which unblocks when all workers have called `wg.Done()`.
- `collected` is returned directly — no channel close, no closer goroutine, no collector loop.

Focus:

- `wg.Wait()` at line 48 in `internal/pool/run.go`.

Mini-task:

- Explain why there is no risk of writing to a closed channel or reading from a closed channel in this design.

Expected:

- Workers only write to a slice, never to a channel. No channel is open during worker execution that could be closed prematurely.

## 7. Determinism guarantee: output order after concurrency (15 min)

Workers finish in nondeterministic order depending on OS scheduling, file size, and parse time. Output order is still deterministic because each worker writes to `collected[f.Index]`, and `f.Index` was assigned sequentially from the input file list before any concurrency started. `collected` is always in the same order as the input `files` slice, regardless of worker scheduling — no sort needed.

Run:

```bash
go test ./... -run TestRunDeterministicOrder
```

Focus file: `internal/pool/order_test.go`

Note: the process closure uses `rand.New(rand.NewSource(int64(42 + f.Index)))` — each worker gets its own `*rand.Rand`. This avoids a data race: `math/rand.Rand` is not goroutine-safe, so sharing one instance across concurrent workers would be flagged by `go test -race ./...`.

Mini-task:

- Explain why random worker timing does not affect final order.

Expected:

- You mention that workers write to `collected[f.Index]` directly, so `collected` is already in deterministic order when `wg.Wait()` returns — no sort needed. Workers finishing in arbitrary order doesn't affect the final slice order.

Extra mini-task:

- Explain what is aggregated in `processor.AggregateResults(...)` and why keeping aggregation separate from `Run(...)` improves maintainability. (`Run` is generic and knows nothing about GPX or tracks. Keeping domain logic in `AggregateResults` means the pipeline can be reused for any file type without modification.)

## 8. Parallel tests as a separate pattern (10 min)

Scan:

```bash
rg -n "t\\.Parallel\\(" internal
```

Mini-task:

- Pick 2 tests and explain why parallel test execution is safe there.

Expected:

- You mention `t.TempDir()` and isolated test state. Each test constructs all its state locally with no shared mutable globals, so concurrent execution cannot cause interference.

## Checkpoint questions (self-test)

1. Why is writing to `collected[f.Index]` from multiple goroutines safe without a mutex?
2. Why can `cancel()` be called multiple times safely?
3. Why is the output of `Run` already in deterministic order without a sort?
4. What happens to files that are still in `jobs` when the context is canceled?
5. Why is there no risk of a `send on closed channel` panic in this design?
6. What would need to change if two files could share the same index?

## Optional follow-up exercises

- Add `context.WithTimeout` support around `pool.Run`. Add tests for timeout-driven cancellation and partial result behavior.

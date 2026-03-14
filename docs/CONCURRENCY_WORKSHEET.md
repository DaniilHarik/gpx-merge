# Hands-On Worksheet: Go Concurrency Patterns (`gpx-merge`)

Time: ~2.5 hours total
Goal: learn worker pool, fan-out/fan-in, send-or-cancel, context-driven cancellation, graceful shutdown, and deterministic output guarantees.

## 0. Code layout (5 min)

Key files and responsibilities:

- `internal/app/app.go` keeps orchestration in `Run(...)`.
- Per-file processing logic is in `internal/processor/process_file.go` (`processor.FileProcessor.Process`).
- Post-pipeline aggregation is in `internal/processor/aggregate.go` (`processor.AggregateResults`).
- `internal/pool/run.go` is the generic concurrency engine.

The full data flow through `pool.Run` involves four roles. The **feeder** goroutine ranges over the input `files` slice and sends each `File` into the unbuffered `jobs` channel, stopping early if the context is canceled. The **N worker** goroutines each range over `jobs`, call `process(ctx, f)`, and send the result into the buffered `results` channel — or exit on cancellation. The **closer** goroutine calls `wg.Wait()` to block until all workers exit, then closes `results`. The **collector** (the main goroutine) ranges over `results` until it is closed, then sorts the collected slice by `File.Index` and returns it.

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

- Where workers start (line 32–47 of `run.go`).
- Where jobs are produced (feeder goroutine, lines 49–58).
- Where results are consumed (collector loop, lines 65–68).
- Where the results channel is closed (closer goroutine, lines 60–63).
- Where `Run(...)` wires in `fileProc.Process` (`app.go:67`).
- Where `AggregateResults` is called (`app.go:68`).

Mini-task:

- Write a 6-line summary of control flow from `Run(...)` entry to return.

Expected:

- You can point to lines for worker start, feeder goroutine, closer goroutine, collector loop, `fileProc.Process` wiring, `processor.AggregateResults(...)` call, and final sort.

## 3. Pattern: worker pool + fan-out/fan-in (20 min)

Fan-out means one producer (the feeder) distributes work across many consumers (the workers) via a shared channel. Fan-in means those many workers all write into a single shared `results` channel, which is then drained by one collector. The `jobs` channel is unbuffered — the feeder blocks on each send until a worker is free, which naturally limits how far ahead the feeder can run. The `results` channel is buffered at `workers*2` so that workers are not immediately blocked by a slow collector.

Run:

```bash
go test ./... -run TestRunDeterministicAcrossWorkerCounts
```

Focus file: `internal/app/app_integration_test.go`

Mini-task:

- Explain why outputs from workers=1 and workers=8 are byte-equal.

Expected:

- You mention final sort by `File.Index` in `internal/pool/run.go`. Workers finish in arbitrary order, so the collector receives results in arrival order. The sort at lines 70–72 unconditionally restores `File.Index` order regardless of how many workers ran.

Follow-up: what would happen if `results` had a buffer size of 0? Trace the effect on the closer goroutine's `wg.Wait()`.

## 4. Pattern: send-or-cancel (`select` with send) (30 min)

After `process(ctx, f)` returns, the worker must either deliver the result or abort. It cannot do both, and it cannot block forever. The `select` at lines 40–44 of `run.go` expresses exactly this:

```go
select {
case results <- res:   // delivered: continue to next job
case <-ctx.Done():     // context canceled: drop result and exit
    return
}
```

Go picks whichever case is ready first. If both are ready simultaneously, Go picks one at random. This is not a check for "is the channel open" — that is a receive pattern (`v, ok := <-ch`). This is a non-blocking send with a cancellation escape hatch.

Mini-task:

- Temporarily add debug logs before and inside both `select` branches.
- Trigger cancellation by canceling a parent context (for example, run via `app.Run` with `context.WithCancel` and call `cancel()` during processing).

Expected:

- You see some workers publish results.
- After cancellation, some workers take the `<-ctx.Done()` branch and exit quickly.
- No deadlock.

Key question: why does this `select` not check whether `results` is closed? Because senders never close a channel — only the closer goroutine closes `results`, and it does so only after all workers have exited. At the moment a worker executes this `select`, the channel is guaranteed to still be open.

## 5. Pattern: context cancellation propagation (20 min)

Cancellation propagates from parent to children through the shared `ctx` value. No explicit signal passing is needed — every goroutine that holds `ctx` can observe cancellation independently.

The four-step path:

1. The parent context is canceled — by `signal.NotifyContext` on SIGINT/SIGTERM (`app.go:26`), a timeout, or an explicit `cancel()` call.
2. The feeder goroutine's `select` (`run.go:53–55`) unblocks on `ctx.Done()` and returns, closing the `jobs` channel via `defer close(jobs)`.
3. Workers drain any remaining items from `jobs`, then their `for f := range jobs` loop exits naturally. Any worker mid-flight after `process` returns takes the `ctx.Done()` branch of its send-or-cancel `select` and returns without sending.
4. Once all workers exit, `wg.Wait()` unblocks in the closer goroutine, which closes `results`. The collector's `for res := range results` loop exits, and `Run` returns whatever was collected before cancellation.

Focus:

- `select { case <-ctx.Done(): return ... }` in feeder goroutine in the same file.
- Worker send-or-cancel select in `internal/pool/run.go`.
- App root signal context in `internal/app/app.go` (`signal.NotifyContext`).

Extra mini-task:

- Trace the error path end-to-end: `processor.FileProcessor.Process(...)` returns an error → worker wraps it in `Result{Err: err}` and sends it → `AggregateResults` detects `r.Err != nil` and appends to `errorsOut`, skipping the file's tracks → `report.PrintFailedFiles` reports it at the end. Other files continue processing unaffected.

## 6. Pattern: graceful shutdown ownership (20 min)

`results` has exactly one owner for closing: the closer goroutine at lines 60–63 of `run.go`. This is the "single closer" rule. If any worker tried to close `results`, it would race with other workers that are still sending — causing a `send on closed channel` panic. Instead:

- Workers are senders only. They never close `results`.
- The closer goroutine waits for all workers to finish (`wg.Wait()`), then closes `results`. At that point no senders remain, so closing is safe.
- The collector ranges over `results` and exits when it is closed.

Focus:

- `wg.Wait()` then `close(results)` in `internal/pool/run.go`.

Mini-task:

- Explain why only the closer goroutine should close `results`.

Expected:

- "Single closer" prevents `send on closed channel` panic.
- Senders never close; they only send or exit.

## 7. Determinism guarantee: output order after concurrency (15 min)

Workers finish in nondeterministic order depending on OS scheduling, file size, and parse time. The collector receives results in arrival order. The `sort.Slice` at lines 70–72 of `run.go` then sorts by `File.Index`, which was assigned sequentially from the input file list before any concurrency started. This guarantees that `Run` always returns results in the same order as the input `files` slice, regardless of how many workers ran or how long each took.

Run:

```bash
go test ./... -run TestRunDeterministicOrder
```

Focus file: `internal/pool/order_test.go`

Note: `order_test.go` shares a single `*rand.Rand` across 4 concurrently-running goroutines (lines 19–23). `math/rand.Rand` is not goroutine-safe. Run `go test -race ./...` to see the data race. Fix: give each worker closure its own `rand.New(rand.NewSource(...))`.

Mini-task:

- Explain why random worker timing does not affect final order.

Expected:

- You mention that collection order is nondeterministic, but final `sort.Slice` restores index order.

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

1. Why is `select` with send not an "is channel open" check?
2. Why can `cancel()` be called multiple times safely?
3. Why does sorting happen after collection, not during worker processing?
4. Who owns closing `results`, and why?
5. What is the buffer size of `results` and what problem does it solve?
6. What happens to in-flight results when the context is canceled — are they delivered or dropped?

## Optional follow-up exercises

- Add `context.WithTimeout` support around `pool.Run`. Add tests for timeout-driven cancellation and partial result behavior.
- Fix the data race in `internal/pool/order_test.go` and confirm `go test -race ./...` passes clean.
- Change the `results` buffer size to 0 (unbuffered) and explain what deadlock you observe and why.

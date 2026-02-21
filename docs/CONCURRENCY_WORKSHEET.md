# Hands-On Worksheet: Go Concurrency Patterns (`gpx-merge`)

Time: ~2.5 hours total  
Goal: learn worker pool, fan-out/fan-in, send-or-cancel, context-driven cancellation, graceful shutdown, and deterministic output guarantees.

## 0. Code layout (5 min)

Key files and responsibilities:

- `internal/app/app.go` keeps orchestration in `Run(...)`.
- Per-file processing logic is in `internal/processor/process_file.go` (`processor.FileProcessor.Process`).
- Post-pipeline aggregation is in `internal/processor/aggregate.go` (`processor.AggregateResults`).
- `internal/pipeline/run.go` is the generic concurrency engine.

## 1. Prep (10 min)

Run:

```bash
go test ./...
```

Expected:

- Tests pass.
- You confirm the project is in a good starting state.

## 2. Map the concurrency graph (20 min)

Open `internal/pipeline/run.go` and `internal/app/app.go`, then identify:

- Where workers start.
- Where jobs are produced.
- Where results are consumed.
- Where results channel is closed.
- Where `Run(...)` wires in `fileProc.Process` from `processor.NewFileProcessor(...)`.
- Where `Run(...)` calls `processor.AggregateResults(...)`.

Mini-task:

- Write a 6-line summary of control flow from `Run(...)` entry to return.

Expected:

- You can point to lines for worker start, feeder goroutine, closer goroutine, collector loop, `fileProc.Process` wiring, `processor.AggregateResults(...)` call, and final sort.

## 3. Pattern: worker pool + fan-out/fan-in (20 min)

Run:

```bash
go test ./... -run TestRunDeterministicAcrossWorkerCounts
```

Focus file: `internal/app/app_integration_test.go`

Mini-task:

- Explain why outputs from workers=1 and workers=8 are byte-equal.

Expected:

- You mention final sort by `File.Index` in `internal/pipeline/run.go`.

## 4. Pattern: send-or-cancel (`select` with send) (30 min)

Focus snippet in `internal/pipeline/run.go`:

- `case results <- res`
- `case <-ctx.Done()`

Mini-task:

- Temporarily add debug logs before and inside both `select` branches.
- Trigger cancellation by canceling a parent context (for example, run via `app.Run` with `context.WithCancel` and call `cancel()` during processing).

Expected:

- You see some workers publish results.
- After cancellation, some workers take the `<-ctx.Done()` branch and exit quickly.
- No deadlock.

## 5. Pattern: context cancellation propagation (20 min)

Focus:

- `select { case <-ctx.Done(): return ... }` in feeder goroutine in the same file.
- Worker send-or-cancel select in `internal/pipeline/run.go`.
- App root signal context in `internal/app/app.go` (`signal.NotifyContext`).

Mini-task:

- Describe cancellation propagation path in 4 steps.

Expected:

1. Parent context is canceled (signal, timeout, or explicit cancel).
2. Feeder stops enqueueing.
3. Workers stop on `ctx.Done()` path.
4. Closer goroutine closes results after workers exit.

Extra mini-task:

- Trace one real error path end-to-end: `processor.FileProcessor.Process(...)` returns error -> worker sends `Result{Err: ...}` -> app reports file-level error while continuing other files.

## 6. Pattern: graceful shutdown ownership (20 min)

Focus:

- `wg.Wait()` then `close(results)` in `internal/pipeline/run.go`.

Mini-task:

- Explain why only the closer goroutine should close `results`.

Expected:

- “Single closer” prevents `send on closed channel` panic.
- Senders never close; they only send or exit.

## 7. Determinism guarantee: output order after concurrency (15 min)

Run:

```bash
go test ./... -run TestRunDeterministicOrder
```

Focus file: `internal/pipeline/order_test.go`

Mini-task:

- Explain why random worker timing does not affect final order.

Expected:

- You mention that collection order is nondeterministic, but final `sort.Slice` restores index order.

Extra mini-task:

- Explain what is aggregated in `processor.AggregateResults(...)` and why keeping aggregation separate from `Run(...)` improves maintainability.

## 8. Parallel tests as a separate pattern (10 min)

Scan:

```bash
rg -n "t\\.Parallel\\(" internal
```

Mini-task:

- Pick 2 tests and explain why parallel test execution is safe there.

Expected:

- You mention `t.TempDir()` and isolated test state.

## Checkpoint questions (self-test)

1. Why is `select` with send not an “is channel open” check?
2. Why can `cancel()` be called multiple times safely?
3. Why does sorting happen after collection, not during worker processing?
4. Who owns closing `results`, and why?

## Optional follow-up exercise

- Add `context.WithTimeout` support around `pipeline.Run`.
- Add tests for timeout-driven cancellation and partial result behavior.

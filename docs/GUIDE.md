# Concurrency reading exercises

Use the current code to answer these questions. Start with [the architecture overview](ARCHITECTURE.md) and run `go test ./...` from the repository root.

1. In [the worker pool](../internal/pool/run.go), find what limits concurrency, cancels work after an error, and waits for completion. What assumptions make concurrent result writes safe?
2. Follow cancellation from [the app](../internal/app/app.go) into [file processing](../internal/processor/process_file.go), [XML parsing](../internal/gpx/parse.go), and [ordinary-point parsing](../internal/gpx/parse_points.go). Where can work continue before noticing cancellation?
3. Read [the pool ordering test](../internal/pool/order_test.go) and [the app integration tests](../internal/app/app_integration_test.go). How do they check output order when scheduling changes?
4. Trace the lifetimes of source bytes, parsed and optimized tracks, and encoded XML fragments. Why does limiting workers not bound total memory independently of the dataset?
5. Inspect tests that call `t.Parallel()`. Which state does each test own, and what would make that state unsafe to share?
6. Follow encoded fragments from [file processing](../internal/processor/process_file.go) through [aggregation](../internal/processor/aggregate.go) to [the writer](../internal/gpx/write.go). Why can workers serialize concurrently while output order and the rule against writing after a file error remain unchanged?

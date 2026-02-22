# Architecture

## Runtime Summary

`gpx-merge` processes files in six stages:

1. Parse and validate CLI config (`internal/cli/config.go`)
2. Discover `.gpx` files recursively (`internal/discovery/discovery.go`)
3. Run concurrent per-file processing (`internal/pipeline/run.go` + `internal/processor/process_file.go`)
4. Aggregate successful tracks and statistics (`internal/processor/aggregate.go`)
5. Write merged GPX and human/JSON reports (`internal/gpx/write.go`, `internal/report/report.go`)
6. Optionally append run metrics CSV rows (`internal/report/metrics_csv.go`)

The app entrypoint is `internal/app/app.go`.

## Core Modules

- Parsing: `internal/gpx/parse.go`, `internal/gpx/types.go`
- Optimization: `internal/optimize/simplify.go`, `internal/optimize/round.go`
- Pipeline orchestration: `internal/pipeline/run.go`
- Per-file processing: `internal/processor/process_file.go`, `internal/processor/track_optimize.go`, `internal/processor/track_segments.go`
- Aggregation and totals: `internal/processor/aggregate.go`
- Writer: `internal/gpx/write.go`

## Architectural Pattern

The runtime now follows a **Functional Core, Imperative Shell** split:

- Imperative shell: `internal/app/app.go` performs side effects and orchestration (signals/context wiring, CLI parse, file discovery, filesystem writes, report emission, exit codes).
- Functional core: `internal/processor/*` and `internal/optimize/*` own deterministic data transforms (track optimization, segment ordering checks, warnings, and aggregation from `[]pipeline.Result`).
- Concurrency boundary: `internal/pipeline/run.go` is a reusable worker-pool engine that isolates goroutine/channel lifecycle from domain logic.

## Determinism + Concurrency

Concurrency improves throughput, but output order remains stable:

- Files are assigned deterministic indices during discovery.
- Workers process files in parallel.
- Results are sorted by file index before aggregation/output.

This guarantees reproducible merged ordering regardless of worker scheduling.

## Concurrency Patterns

`gpx-merge` uses a small set of explicit concurrency patterns in `internal/pipeline/run.go`:

- Bounded worker pool: `workers` controls fixed parallelism for per-file processing.
- Producer/consumer pipeline: feeder goroutine sends discovered files over `jobs` while workers consume.
- Fan-out: one input stream (`jobs`) is processed concurrently by `N` worker goroutines.
- Fan-in: workers publish `pipeline.Result` values into a shared `results` channel.
- Confinement by ownership: each worker keeps per-file processing state local to its goroutine and communicates only via channels; the collector goroutine exclusively owns and mutates the final `[]Result` buffer before sort.
- Coordinated shutdown: a closer goroutine waits on `sync.WaitGroup` and closes `results` exactly once.
- Cooperative cancellation: feeder and workers stop early when `ctx.Done()` is signaled.
- Deterministic completion barrier: collector buffers all results and sorts by `File.Index` before returning.

## Processing Sequence

```mermaid
sequenceDiagram
    autonumber
    actor CLI
    participant App as app.Run (shell)
    participant Pipe as pipeline.Run
    participant Feed as feeder goroutine
    participant W1 as worker (1..N)
    participant Proc as processor.FileProcessor.Process
    participant Parse as gpx.ParseFile
    participant SegSort as processor.sortTrackSegmentsByFirstTimestamp
    participant Opt as processor.optimizeTrack
    participant Measure as gpx.MeasureTracks
    participant Main as collector
    participant Close as closer goroutine
    participant Agg as processor.AggregateResults
    participant Write as gpx.WriteMerged or gpx.MeasureMerged
    participant Report as report.PrintSummary or report.WriteJSON
    participant Metrics as report.AppendMetricsCSV

    Note over CLI,Pipe: Setup
    CLI->>App: parse config + discover files
    App->>App: build processor.NewFileProcessor(cfg, optOpts)
    App->>Pipe: Run(ctx, files, workers, fileProc.Process)

    Note over Pipe,Close: Pipeline startup
    Pipe->>Feed: start feeder
    Pipe->>W1: start workers
    Pipe->>Close: start closer (wait for workers)

    Note over Feed,Main: Steady-state processing
    loop each input file
        Feed->>W1: jobs <- File
        W1->>Proc: Process(ctx, file)
        Proc->>Parse: parse tracks
        alt --sort-segments-by-time
            Proc->>SegSort: reorder segments by first timestamp
        end
        loop each track
            Proc->>Opt: simplify + distance in/out
        end
        Proc->>Measure: measure optimized bytes
        Proc-->>W1: filePayload | fileError
        W1->>Main: results <- Result
    end

    alt ctx canceled (SIGINT/SIGTERM/timeout)
        Pipe-->>Feed: stop enqueue
        Pipe-->>W1: stop work/send
    end

    Close->>Main: close(results) after wg.Wait()
    Main->>Main: collect + sort by File.Index
    Pipe-->>App: []Result (deterministic order)
    App->>Agg: aggregate per-file stats/warnings/errors
    Agg-->>App: totals + tracks + file stats
    App->>Write: write merged GPX (or measure in dry-run)
    App->>Report: print summary/errors/warnings (+ optional JSON)
    opt --metrics-csv set
        App->>Metrics: append row (started_at_utc, points_in, points_out, workers, duration_ms, mb_in, mb_out)
    end
    App-->>CLI: merged output + report
```

## Goroutine Hierarchy

```mermaid
flowchart TD
    A["app.Run caller goroutine"] --> B["pipeline.Run caller goroutine"]

    subgraph PR["inside pipeline.Run"]
        B --> C["jobs channel"]
        B --> D["results channel buffered workers x2"]
        B --> E["spawn feeder goroutine"]
        B --> F["spawn worker goroutines 1..N"]
        B --> G["spawn closer goroutine"]
        B --> H["collector loop in caller goroutine"]

        E --> I["enqueue files to jobs or stop on ctx done"]
        I --> C

        F --> J["read job call process build Result"]
        J --> K["send Result to results or exit on ctx done"]
        K --> D

        G --> L["wait for workers then close results"]
        L --> D

        H --> D
        H --> M["sort collected results by File.Index and return"]
    end
```

## Segment Discontinuity Handling

For adjacent segments in the same track, large endpoint gaps can create misleading straight connectors in some viewers.

- Detection: adjacent `<trkseg>` endpoint gaps are checked per source track.
- Warning threshold: a warning is emitted for gaps greater than `1000m`.
- Split behavior: `--split-track-gap` controls when a discontinuity is split into a new track.
  - Default is `1000m`.
  - `0` disables splitting.
- Optional ordering fix: `--sort-segments-by-time` reorders each track's segments by their first parseable timestamp before optimization.
  - If a track has segments without parseable timestamps, that track is left in original order.
- Terminology: `Track Name (part N)` means a new `<trk>` element, not a segment.
- Segment boundaries (`<trkseg>`) are preserved inside each output track part.

## Exit Code Model

- `0`: all files processed successfully
- `1`: one or more files failed
- `2`: configuration/usage/runtime setup error

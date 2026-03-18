# Architecture

## Runtime Summary

`gpx-merge` processes files in six stages:

1. Parse and validate CLI config (`internal/cli/config.go`)
2. Discover `.gpx` files recursively (`internal/discovery/discovery.go`)
3. Run concurrent per-file processing (`internal/pool/run.go` + `internal/processor/process_file.go`)
4. Aggregate successful tracks and statistics (`internal/processor/aggregate.go`)
5. Write merged GPX and human/JSON reports (`internal/gpx/write.go`, `internal/report/report.go`)
6. Optionally append run metrics CSV rows (`internal/report/metrics_csv.go`)

The app entrypoint is `internal/app/app.go`.

## Core Modules

- Parsing: `internal/gpx/parse.go`, `internal/gpx/types.go`
- Optimization: `internal/optimize/simplify.go`, `internal/optimize/round.go`
- Worker pool: `internal/pool/run.go`
- Per-file processing: `internal/processor/process_file.go`, `internal/processor/track_optimize.go`, `internal/processor/track_segments.go`
- Aggregation and totals: `internal/processor/aggregate.go`
- Writer: `internal/gpx/write.go`

## Architectural Pattern

The runtime now follows a **Functional Core, Imperative Shell** split:

- Imperative shell: `internal/app/app.go` performs side effects and orchestration (signals/context wiring, CLI parse, file discovery, filesystem writes, report emission, exit codes).
- Functional core: `internal/processor/*` and `internal/optimize/*` own deterministic data transforms (track optimization, segment ordering checks, warnings, and aggregation from `[]pool.Result`).
- Concurrency boundary: `internal/pool/run.go` is a reusable worker-pool engine that isolates goroutine/channel lifecycle from domain logic.

## Determinism + Concurrency

Concurrency improves throughput, but output order remains stable:

- Files are assigned deterministic indices during discovery.
- Goroutines process files in parallel, each writing to `collected[f.Index]`.
- `collected` is already in deterministic order when `g.Wait()` returns — no sort needed.

This guarantees reproducible merged ordering regardless of goroutine scheduling.

## Concurrency

`internal/pool/run.go` is a bounded worker pool implemented with `errgroup.WithContext` (`golang.org/x/sync/errgroup`):

- `g.SetLimit(workers)` bounds parallelism.
- One `g.Go(...)` goroutine is launched per file; each calls `process(ctx, f)` and writes to `collected[f.Index]` — no results channel, no sort needed.
- The first error cancels the shared context; `g.Wait()` returns it once all goroutines finish.
- `process` receives the errgroup-derived `ctx`, so cancellation (error or SIGINT) causes in-flight calls to return quickly.

Each GPX file is processed fully and independently (parse → sort → optimize → measure in one `Process` call), so a single worker pool is sufficient. A pipeline would only help if individual stages were bottlenecks worth overlapping across files.


## Processing Sequence

```
Setup
  CLI ──► app.Run: os.Args, stdout, stderr
  app.Run: parse config, discover files, build processor.NewFileProcessor(cfg, optOpts)
  app.Run ──► pool.Run: Run(ctx, files, workers, fileProc.Process)

Worker pool startup (inside pool.Run)
  pool.Run: create errgroup + derived ctx via errgroup.WithContext
  pool.Run: allocate collected []Result (len(files))
  pool.Run: g.SetLimit(workers) — bounds concurrency
  pool.Run: launch one g.Go(...) goroutine per file

Steady-state (each goroutine, one file)
  goroutine ──► Process(ctx, file)
    Process ──► gpx.ParseFile
    Process ──► sortTrackSegmentsByFirstTimestamp  [only with --sort-segments-by-time]
    Process ──► optimizeTrack (per track): simplify + distance in/out
    Process ──► gpx.MeasureTracks
  Process returns filePayload → goroutine: collected[file.Index] = Result
  Process returns error → errgroup cancels ctx; g.Wait() will return this error

On ctx cancel (SIGINT / SIGTERM / first file error)
  process(ctx, f) returns early with a context error
  remaining goroutines exit quickly; g.Wait() returns the first error

Shutdown
  pool.Run: g.Wait() — blocks until all goroutines finish
  pool.Run ──► app.Run: ([]Result, error)
  On error: app.Run prints to stderr and exits 1 (no output written)

Post-processing (success path only)
  app.Run ──► AggregateResults: aggregate stats / warnings
  app.Run ──► WriteMerged (or MeasureMerged in --dry-run)
  app.Run ──► PrintSummary / PrintWarnings (+ optional JSON)
  app.Run ──► AppendMetricsCSV  [only with --metrics-csv]
  app.Run ──► CLI: exit 0 | 2
```

## Goroutine Hierarchy

```
app.Run (caller goroutine)
└── pool.Run (caller goroutine)
    ├── errgroup.WithContext → g, ctx
    ├── g.SetLimit(workers)
    ├── allocate collected []Result (len(files))
    ├── g.Go per file → goroutine: process(ctx, f) ──► collected[f.Index] = Result
    └── g.Wait() → return (collected, firstErr)
```

## Input Validation

Coordinate bounds are validated during parsing in `internal/gpx/parse.go`, immediately before a point is appended to the segment buffer:

- `internal/cli/config.go` rejects `--output` paths inside `--input` before discovery begins. This prevents previously merged GPX output from being rediscovered as input on a later run.
- Latitude must be in `[-90, 90]`; values outside this range return an error of the form `invalid latitude <value>: must be in [-90, 90]`.
- Longitude must be in `[-180, 180]`; values outside this range return an error of the form `invalid longitude <value>: must be in [-180, 180]`.

Boundary values (`±90` lat, `±180` lon) are valid. An out-of-bounds point causes `ParseFile` to return immediately, which surfaces as a per-file error through the worker pool and is counted in the exit-code model below.

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
- `1`: any file failed during processing (run aborted, no output written)
- `2`: configuration/usage/runtime setup error

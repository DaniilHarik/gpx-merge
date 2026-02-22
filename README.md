# gpx-merge

`gpx-merge` is a concurrent Go CLI that discovers `.gpx` files, simplifies track geometry, and merges everything into a deterministic GPX 1.1 output.

## Why

Large GPX collections (from watches, bike computers, phones, etc.) are usually fragmented and oversized. This tool reduces point density, keeps useful structure, and produces a single output file that is easier to load and manage.

## Features

- Recursive GPX discovery under `--input`
- Douglas-Peucker simplification with hard max-error guard
- Deterministic output order even with concurrent workers
- Default worker pool size of `16`, chosen from local benchmarking on Apple MacBook M1 Pro (override with `--workers`)
- Optional preservation of `<time>` and `<ele>`
- Segment discontinuity warnings and optional track splitting
- Optional segment reordering by first timestamp (`--sort-segments-by-time`)
- Human-readable and JSON run reports
- Optional append-only CSV run metrics log (`--metrics-csv`)

## Internal Design

- `internal/app` is the orchestration shell (CLI lifecycle, I/O, reporting, exit codes).
- `internal/processor` and `internal/optimize` hold core transformation and aggregation logic.
- `internal/pipeline` provides the reusable worker-pool runtime used by the app shell.

## Requirements

- Go `1.25+`

## Build

```bash
go build -o gpx-merge ./cmd/gpx-merge
```

## Quick Start

```bash
./gpx-merge \
  --input ./data \
  --output ./out/merged_optimized.gpx
```

## Common Commands

Dry run (no output file):

```bash
./gpx-merge --input ./data --dry-run
```

Reorder segments by time before optimization:

```bash
./gpx-merge --input ./data --sort-segments-by-time
```

Preserve timestamps and elevation:

```bash
./gpx-merge --input ./data --keep-time --keep-ele
```

Write JSON report:

```bash
./gpx-merge --input ./data --json-report ./out/run.json
```

Append run metrics CSV:

```bash
./gpx-merge --input ./data --metrics-csv ./out/metrics.csv
```

## Documentation

- CLI flags, defaults, exit codes, and output details: [`docs/CLI_REFERENCE.md`](docs/CLI_REFERENCE.md)
- Pipeline and processing internals (with sequence diagram): [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)
- Design goals and behavior spec: [`SPEC.md`](SPEC.md)
- Concurrency notes: [`docs/CONCURRENCY_WORKSHEET.md`](docs/CONCURRENCY_WORKSHEET.md)

## Development

```bash
go test ./...
go run . --help
```

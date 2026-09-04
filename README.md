# gpx-merge

`gpx-merge` is a concurrent Go CLI that discovers `.gpx` files, simplifies track geometry, and merges everything into a deterministic GPX 1.1 output.

## Why

Large GPX collections (from watches, bike computers, phones, etc.) are usually fragmented and oversized. This tool reduces point density, keeps useful structure, and produces a single output file that is easier to load and manage.

## Features

- Recursive GPX discovery under `--input`
- Douglas-Peucker simplification with hard max-error guard
- Deterministic output order even with concurrent workers
- Size-aware worker scheduling that starts larger files first
- Default worker pool size of `8`, chosen from local benchmarking on an Apple M1 Pro (override with `--workers`)
- Optional preservation of `<time>` and `<ele>`
- Segment discontinuity warnings and optional track splitting
- Optional segment reordering by first timestamp (`--sort-segments-by-time`)
- Human-readable run reports
- Optional append-only CSV run metrics log (`--metrics-csv`)
- Validation that rejects `--output` paths inside `--input`

## Internal Design

- `internal/app` is the orchestration shell (CLI lifecycle, I/O, reporting, exit codes).
- `internal/processor` and `internal/optimize` hold core transformation and aggregation logic.
- `internal/pool` provides the reusable worker-pool runtime used by the app shell.

### XML token strategy

Parsing uses a 256 KiB buffered `encoding/xml.Decoder.RawToken` stream rather than decoding into XML mirror structs. Large GPX files can contain hundreds of thousands of `<trkpt>` elements, and struct decoding first builds an intermediate XML-shaped tree before converting it into the app's `Track`, `Segment`, and `Point` model. The parser writes directly into that domain model, checks element nesting itself, and skips elevation and timestamps when the selected options will discard them. It still checks for cancellation between token reads.

Writing still uses `xml.Encoder.EncodeToken` rather than hand-built strings. That keeps output escaping and XML correctness in the standard library while avoiding struct marshaling overhead on the hot GPX output path.

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

Append run metrics CSV:

```bash
./gpx-merge --input ./data --metrics-csv ./out/metrics.csv
```

The CSV is append-only and includes `started_at_utc,points_in,points_out,workers,duration_ms,mb_in,mb_out`.

`--output` must be outside `--input`. This prevents reruns from rediscovering a previously merged GPX as a fresh input file.

`run.sh` uses `GOGC=400` unless `GOGC` is already set. This trades memory for speed on the bundled dataset. Set `GOGC` or `GPX_MERGE_GOGC` to choose a different target.

## Documentation

- CLI flags, defaults, exit codes, and output details: [`docs/CLI_REFERENCE.md`](docs/CLI_REFERENCE.md)
- Pipeline and processing internals: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)
- Design goals and behavior spec: [`SPEC.md`](SPEC.md)
- Concurrency guide: [`docs/GUIDE.md`](docs/GUIDE.md)

## Performance

Benchmarked with a warm `go run` build cache on the local dataset (70 files, 180 MB, 967 500 points) using 8 workers on an Apple M1 Pro:

| Metric | Value |
|---|---|
| App elapsed time | ~590 ms |
| Files/s | ~118.64 files/s |
| Points/s | ~1 639 831 pts/s |
| Files scanned → processed | 70 → 70 |
| Points in → out | 967 500 → 250 431 (74.12% reduction) |
| Size in → out | 180.01 MB → 11.72 MB (93.49% reduction) |
| Distance in → out | 5836.77 km → 5820.79 km (0.27% reduction) |

## Development

```bash
go test ./...
go run . --help
```

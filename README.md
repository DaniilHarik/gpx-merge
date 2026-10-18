# gpx-merge

`gpx-merge` is a Go CLI for turning a local collection of dense GPX tracks into one smaller file for a GPX viewer. It simplifies geometry while preserving track and segment boundaries, with optional splitting at large gaps.

## Build and run

Use the Go version specified in [go.mod](go.mod) or newer.

```bash
go build -o gpx-merge ./cmd/gpx-merge
./gpx-merge --input ./data --output ./out/merged_optimized.gpx
./gpx-merge --help
```

Keep the output outside the input directory so later runs cannot discover the previous merge as another source track. Keep the originals too. This is a lossy conversion, and optional metadata is discarded unless supported by a retention flag.

## Common commands

Preview savings without writing the merged GPX:

```bash
./gpx-merge --input ./data --dry-run
```

Preserve timestamps and elevation on retained points:

```bash
./gpx-merge --input ./data --keep-time --keep-ele
```

Reorder segments by time and append run metrics:

```bash
./gpx-merge --input ./data --sort-segments-by-time --metrics-csv ./out/metrics.csv
```

Use [the CLI reference](docs/CLI_REFERENCE.md) to find current defaults and output formats.

[run.sh](run.sh) runs the local dataset with a higher garbage-collection target, trading memory for speed. Set `GOGC` or `GPX_MERGE_GOGC` to tune it for your machine. The script contains the defaults and other environment overrides.

## Development

```bash
go test ./...
```

### Performance regression test

The normal test suite includes a fixed end-to-end merge workload which must finish within five seconds. This intentionally conservative limit catches major regressions without depending on one machine's benchmark result. Use `GPX_MERGE_PERFORMANCE_BUDGET` to set an explicit Go duration for a CI runner or local performance gate:

```bash
GPX_MERGE_PERFORMANCE_BUDGET=2s go test ./internal/app -run TestRunCompletesWithinPerformanceBudget
```

## Historical performance

Commit `645ca3a` recorded these measurements on 2026-09-04 using eight workers on an Apple M1 Pro with a warm `go run` build cache. They describe that local dataset and build, not a current performance guarantee.

| Metric | Value |
|---|---|
| App elapsed time | ~590 ms |
| Files/s | ~118.64 files/s |
| Points/s | ~1 639 831 pts/s |
| Files scanned → processed | 70 → 70 |
| Points in → out | 967 500 → 250 431 (74.12% reduction) |
| Size in → out | 180.01 MB → 11.72 MB (93.49% reduction) |
| Distance in → out | 5836.77 km → 5820.79 km (0.27% reduction) |

## Documentation

- [Product requirements and limits](SPEC.md)
- [Architecture and decision records](docs/ARCHITECTURE.md)
- [CLI reference](docs/CLI_REFERENCE.md)
- [Concurrency reading exercises](docs/GUIDE.md)

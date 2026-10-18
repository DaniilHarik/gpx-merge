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

Check the local dataset against a time limit with [verify.sh](verify.sh):

```bash
./verify.sh
./verify.sh --help
```

This checks the app's reported elapsed time and compares the merged GPX with the local reference file. See the script's help for required file paths, threshold handling, and exit codes.

See [Performance](#performance) for the latest local benchmark.

The normal test suite includes a fixed end-to-end merge workload which must finish within five seconds. This intentionally conservative limit catches major regressions without depending on one machine's benchmark result. Use `GPX_MERGE_PERFORMANCE_BUDGET` to set an explicit Go duration for a CI runner or local performance gate:

```bash
GPX_MERGE_PERFORMANCE_BUDGET=2s go test ./internal/app -run TestRunCompletesWithinPerformanceBudget
```

## Performance

Measured on 2026-10-18 using the current working tree, an Apple M1 Pro, Go 1.26.1 on macOS arm64, eight workers, and `GOGC=400`. One warm-up run of `./verify.sh` took 182 ms, followed by 20 consecutive measured runs on the local `data/` collection. The warm-up is excluded from the table.

| Metric | Value |
|---|---|
| App elapsed time, median | 117 ms |
| App elapsed time, minimum–maximum | 103–191 ms |
| Runs below 200 ms | 20 / 20 |
| Throughput, median | 605.19 files/s; 8 246 835 points/s |
| Files scanned → processed | 71 → 71 |
| Points in → out | 967 500 → 250 431 (74.12% reduction) |
| Size in → out | 180.01 MB → 11.72 MB (93.49% reduction) |
| Distance in → out | 5836.77 km → 5820.79 km (0.27% reduction) |
| Output matches `out/Test.gpx` byte for byte | 20 / 20 |

Timings use the app's reported elapsed time, excluding Go compilation, process startup, and the reference-file comparison. Cache state and system load affect results; an earlier batch on this machine included a 272 ms outlier.

Before the optimization, repeated runs on the same local dataset took 606–608 ms. For older context, commit `645ca3a` recorded about 590 ms on 2026-09-04 with eight workers on an Apple M1 Pro and a warm build cache; that dataset snapshot contained 70 files.

## Documentation

- [Product requirements and limits](SPEC.md)
- [Architecture and decision records](docs/ARCHITECTURE.md)
- [CLI reference](docs/CLI_REFERENCE.md)
- [Concurrency reading exercises](docs/GUIDE.md)

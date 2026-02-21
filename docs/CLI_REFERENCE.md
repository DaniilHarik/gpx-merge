# CLI Reference

## Usage

```text
gpx-merge [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--input` | string | `./data` | Root folder containing GPX files |
| `--output` | string | `./out/merged_optimized.gpx` | Merged GPX output path |
| `--workers` | int | `runtime.NumCPU()` | Worker pool size |
| `--simplify` | float | `0.8` | Base simplification tolerance in meters |
| `--max-error` | float | `1.5` | Hard cap for allowed geometric deviation |
| `--precision` | int | `6` | Coordinate decimal precision |
| `--min-points` | int | `2` | Minimum points kept per segment |
| `--split-track-gap` | float | `1000` | Split a track when adjacent segment endpoint gaps exceed this many meters (`0` disables) |
| `--sort-segments-by-time` | bool | `false` | Reorder track segments by first timestamp before optimization |
| `--keep-time` | bool | `false` | Preserve `<time>` tags |
| `--keep-ele` | bool | `false` | Preserve `<ele>` tags |
| `--dry-run` | bool | `false` | Report projected savings without writing output |
| `--verbose` | bool | `false` | Print per-file optimization stats |
| `--include-run-metadata` | bool | `false` | Include generation stats in output metadata |
| `--json-report` | string | empty | Write machine-readable stats to a JSON file |

## Behavior Notes

- Discovery is recursive and case-insensitive for `.gpx` extension.
- File-level failures are reported; successful files are still merged.
- Output ordering is deterministic, even with concurrent workers.
- If `--sort-segments-by-time` is enabled and a track has non-parseable timestamps, that track remains in original order.
- Internal package split: orchestration is in `internal/app`, while per-file processing/aggregation are in `internal/processor`.

## Exit Codes

- `0`: all files succeeded
- `1`: one or more files failed
- `2`: configuration/usage/runtime setup error

## Human Report Format

The summary includes:

- Files scanned/processed/failed
- Worker count
- Points, size, and distance before/after optimization
- Elapsed time and throughput
- Failed file list (if any)
- Discontinuity warnings (if any)

## Example

```bash
./gpx-merge \
  --input ./data \
  --output ./out/merged_optimized.gpx \
  --workers 8 \
  --simplify 0.8 \
  --max-error 1.5 \
  --precision 6 \
  --split-track-gap 1000 \
  --sort-segments-by-time \
  --json-report ./out/run.json
```

# Product Spec: Concurrent GPX Optimizer + Merger CLI

## 1. Overview

Build a Go CLI that reads a large set of highly precise GPX tracks from a local `data/` folder, optimizes each track to reduce file size, and produces one merged GPX output where source track boundaries are preserved and may be split into multiple `<trk>` parts when large segment endpoint gaps are detected.

The tool must process inputs concurrently for speed while keeping output deterministic and compatible with common GPX viewers.
The conversion workflow must run in memory end-to-end (streaming/channels and in-memory structures), without writing intermediate GPX artifacts to disk.

## 2. Problem Statement

Large collections of high-precision GPX files can be slow to open and render in GPX viewers because they contain excessive point density and many files must be loaded individually.

We need one command that:

1. Mass-processes GPX inputs efficiently.
2. Reduces point count and serialized size without materially harming path shape.
3. Exports one merged GPX that loads quickly, while preserving track boundaries (with optional gap-based split into multiple `<trk>` parts per source track).

## 3. Goals

1. Read GPX files from `data/` (recursive).
2. Optimize each track for smaller size.
3. Merge all optimized tracks into one GPX file.
4. Keep each original track boundary in output, with optional splitting into multiple `<trk>` parts when adjacent segment gaps exceed `--split-track-gap`.
5. Use concurrency to speed up parse + optimize phases.
6. Produce deterministic output ordering.
7. Keep output broadly compatible with GPX 1.1 viewers.
8. Run conversion in memory (no intermediate temp files).

## 4. Non-Goals

1. Editing map-matching/routing logic.
2. Converting to non-GPX formats.
3. Reprojecting coordinates.
4. Building a GUI.
5. Lossless compression of all optional metadata fields.

## 5. Primary Users

1. Operators managing very large GPX collections.
2. Developers preparing GPX bundles for viewer performance.

## 6. CLI UX

Binary name: `gpx-merge`

Example:

```bash
gpx-merge \
  --input ./data \
  --output ./out/merged_optimized.gpx \
  --workers 8 \
  --simplify 1.5 \
  --max-error 3.0 \
  --precision 6
```

### Core flags

1. `--input <dir>`: root folder containing GPX files (default: `./data`)
2. `--output <file>`: merged GPX output path (default: `./out/merged_optimized.gpx`)

### Optional flags

1. `--workers <n>`: worker pool size (default: `16`)
2. `--simplify <meters>`: base simplification tolerance in meters
3. `--max-error <meters>`: hard cap for allowed geometric deviation
4. `--precision <digits>`: coordinate decimal precision in output (default: `6`)
5. `--min-points <n>`: minimum points to keep per segment (default: `2`)
6. `--split-track-gap <meters>`: split a track into multiple `<trk>` parts when adjacent segment endpoint gaps exceed this threshold (default: `1000`; `0` disables splitting)
7. `--sort-segments-by-time`: reorder each track's segments by first timestamp before optimization when all segments in that track have parseable times
8. `--keep-time`: preserve `<time>` tags (dropped by default)
9. `--keep-ele`: preserve `<ele>` tags (dropped by default)
10. `--dry-run`: report projected savings without writing output
11. `--verbose`: per-file optimization stats
12. `--include-run-metadata`: include generation stats in `<metadata>`
13. `--json-report <file>`: write machine-readable run report as JSON
14. `--metrics-csv <file>`: append one CSV row per completed run with points, workers, duration, and MB in/out

## 7. Functional Requirements

1. Discover all `*.gpx` files under `--input` recursively.
2. Parse GPX 1.1 input safely; skip malformed files with structured error reporting.
3. For each input track:
   - Preserve logical track identity (`<name>` when present, or filename fallback).
   - Preserve segment names/labels when present.
   - Optimize geometry per segment using line simplification.
   - Enforce `--min-points` so segments remain valid.
   - Drop nonessential metadata by default to reduce size; keep only geometry plus track/segment names unless explicitly retained by flags.
4. Write one merged GPX:
   - Single `<gpx>` root.
   - Preserve source track boundaries in output `<trk>` elements; when adjacent segment endpoint gaps exceed `--split-track-gap`, split into additional track parts (`Track Name (part N)`).
   - Track order deterministic (lexicographic path order by default).
5. Emit run summary:
   - Files scanned, files processed, failed files.
   - Active worker count.
   - Points in/out, bytes in/out, distance in/out, reduction percentages.
   - Integer totals use grouped formatting with spaces for readability (example: `967 002`).
   - When failures exist, print a `Failed files:` section with file path, stage, and reason.
   - Total elapsed time and throughput.

## 8. Optimization Strategy

Use a two-stage lossy optimization tuned for viewer performance:

1. Geometry simplification:
   - Algorithm: Douglas-Peucker (or Visvalingam as optional future strategy).
   - Distance metric: projected planar (equirectangular-style) point-to-segment distance in meters.
   - Controls: `--simplify` and `--max-error`.
2. Serialization compaction:
   - Round latitude/longitude to `--precision` decimals.
   - Remove nonessential whitespace in output XML.
   - Drop metadata fields by default (global, track, segment, and point-level) except track/segment names.
   - Allow selective retention only when explicitly requested (`--keep-time`, `--keep-ele`).

Quality guardrails:

1. Never emit invalid segments (<2 points).
2. Never exceed `--max-error`.
3. Preserve first and last point of every segment.

## 9. Concurrency Design

Pipeline architecture:

1. Discover phase:
   - Walks input tree, builds deterministic file list.
2. Worker pool (`--workers`):
   - Parse + optimize files in parallel.
3. Collector phase:
   - Collects all results, reorders by source index.
4. Aggregation + write phase:
   - Aggregates totals/tracks, then writes merged output sequentially.

Design constraints:

1. Output must be deterministic regardless of worker scheduling.
2. The `jobs` channel is pre-buffered to `len(files)` and filled synchronously before workers start. Workers write results directly to a pre-allocated `collected` slice at their file's index; no results channel. Full run results/tracks are available immediately after `wg.Wait()` returns.
3. No writer-stage backpressure during worker processing because writing starts after result collection.
4. Context cancellation support on fatal error or SIGINT.
5. No intermediate on-disk conversion artifacts; only final output is written.
6. Architectural split follows Functional Core, Imperative Shell:
   - `internal/app` owns imperative orchestration and side effects.
   - `internal/processor` and `internal/optimize` own transformation/aggregation core logic.
   - `internal/pool` owns worker-pool concurrency mechanics.

## 10. Output GPX Requirements

1. GPX version `1.1` and creator set to the CLI name/version.
2. Preserve source track boundaries as `<trk>` elements (no collapsing into one giant track); a single source track may produce multiple `<trk>` parts when gap-based splitting is enabled and thresholds are exceeded.
3. Preserve segment boundaries when available (`<trkseg>`).
4. Default payload should contain only fields required for rendering/performance, plus track and segment names.
5. Optional run metadata (disabled by default):
   - Include aggregate counts in `<metadata><desc>`.
   - Include generation timestamp.

## 11. Performance Targets

Targets on a modern laptop (baseline expectations):

1. At least 3x faster than single-threaded mode for large datasets.
2. At least 40% point reduction on dense precision traces (dataset-dependent).
3. Merged output opens perceptibly faster in common GPX viewers vs original multi-file set.
4. Peak memory scales with worker buffers plus accumulated successful results/tracks prior to final write.

## 12. Error Handling

1. Per-file errors do not stop the whole run by default.
2. Final exit code:
   - `0` if all files processed successfully.
   - `1` if one or more files failed.
   - `2` for configuration/usage errors and runtime setup/output/reporting errors.
3. Per-file processing errors are recorded and reported; processing continues for remaining files.

## 13. Observability

1. Human-readable summary by default.
2. Optional `--json-report <file>` for machine-readable stats:
   - Per-file point/byte/distance reductions and error info.
   - Run-level totals, timings, and distance metrics.
3. Optional `--metrics-csv <file>` for append-only run log rows:
   - Header: `started_at_utc,points_in,points_out,workers,duration_ms,mb_in,mb_out`
   - One row appended per completed run.
4. Human summary output shape:
   - `Files scanned`, `Files processed`, `Files failed`, `Workers`.
   - `Points`, `Bytes`, `Distance`, `Elapsed`, `Throughput`.
   - If failures occur, list each failed file with reason:
     - Example: `- file.gpx (optimize): segment has 1 points; expected at least 2`
   - If warnings occur, print a `Warnings:` section:
     - Segment discontinuity warnings for adjacent `<trkseg>` gaps over `1000m` (including split-threshold outcome).
     - Segment reorder warnings when `--sort-segments-by-time` changes one or more tracks.

## 14. Testing Requirements

1. Unit tests:
   - Simplification correctness and max-error enforcement.
   - Precision rounding behavior.
   - Deterministic ordering with concurrent workers.
2. Integration tests:
   - Mixed valid/invalid files.
   - Large synthetic dataset under `data/`.
3. Golden tests:
   - Stable XML output shape and compatibility checks.
4. Benchmark tests:
   - Throughput and memory profiles across worker counts.

## 15. Acceptance Criteria

1. Running CLI on `data/` produces one merged GPX at `--output` (or default: `./out/merged_optimized.gpx`).
2. Merged GPX contains `<trk>` elements corresponding to each successfully processed source track, with optional additional `(part N)` track splits when segment gaps exceed `--split-track-gap` (or no splits when set to `0`).
3. Size and point count are reduced relative to raw inputs for dense tracks.
4. Output is deterministic across repeated runs with same inputs/flags.
5. Tool processes files concurrently and reports measurable speedup vs `--workers 1`.
6. Result opens successfully in at least one target GPX viewer without schema errors.

## 16. Future Enhancements

1. Multiple simplification strategies (`--strategy dp|vw`).
2. Per-track adaptive tolerance based on local curvature.
3. Optional output splitting by region/date for huge merged datasets.
4. Embedded spatial index sidecar for faster viewer prefiltering.

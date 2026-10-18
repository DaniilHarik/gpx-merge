# ADR 0004: Read common GPX points faster and generate track XML once

## Status

Accepted and recorded on 2026-10-18.

## Context

The merge needed to pass the [local performance check](../../verify.sh) without changing its output. Profiling identified two costs:

- Reading XML was the main CPU cost and created many temporary objects in memory.
- The app generated the output XML twice: once to measure its size, then again to write the merged file.

## Decision

Use a small, dedicated fast reader for track points written in XML forms it recognizes. For example:

```xml
<trkpt lat="58.1" lon="24.2">
  <ele>10.5</ele>
  <time>2024-01-01T08:00:00Z</time>
</trkpt>
```

The fast reader extracts the coordinates and requested fields directly into a `Point`. For this block, it skips the standard XML decoder's work of turning tags and attributes into separate objects. This is what "common track-point XML" means here: XML forms supported by the fast reader.

Go's standard XML decoder still reads the surrounding GPX structure, including track names and segment boundaries. If the fast reader cannot fully recognize a point, the standard decoder reads that whole point. This can happen with valid but more complex XML, such as nested extension elements. The fast reader can resume at the next point it recognizes.

This keeps support for more complex input without making our reader implement every XML feature. The [point reader](../../internal/gpx/parse_points.go) and [compatibility tests](../../internal/gpx/parse_compat_test.go) define the exact supported forms and fallback behavior.

Each worker processes one input file at a time. It reads the file into memory, simplifies the tracks, and generates their output XML using Go's standard XML encoder. Keep that XML in memory so we can use the same bytes to measure its size and later write the merged file.

Wait until every input file succeeds. Then write the saved XML pieces in source order, surrounded by the opening and closing GPX tags. A dry run counts the same output bytes without writing a file.

## Alternatives considered

- Keep the previous approach. It holds less source data in memory, but was too slow for the local performance goal.
- Replace Go's XML decoder entirely. Our reader would then need to handle all supported XML syntax and reject invalid input correctly. That would be much more code to maintain and test.
- Write each worker's output straight to the destination file. This would need a new way to preserve source order and prevent a later input error from leaving a partial merge. See [ADR 0002](0002-abort-on-file-errors.md).

## Consequences

- The merge uses more memory to save processing time. Active workers hold source bytes and tracks. Completed results hold the generated XML as well as the simplified tracks.
- Files with many points that the fast reader recognizes benefit most. Files that need the standard decoder for more points may take longer, even with the same number of points.
- We must keep checking that both readers produce the same tracks and that reusing saved XML produces the same output file.
- Output order, simplification rules, and coordinate formatting stay the same. An input-processing error still prevents the output write from starting.

## Evidence

On 2026-10-18, the local benchmark used 71 files containing 967,500 points and 180.01 MB of GPX data. It ran on an Apple M1 Pro with Go 1.26.1, eight workers, and `GOGC=400`. The results below exclude one warm-up run and use the app's reported processing time, excluding compilation, process startup, and the output comparison.

| Outcome | Measured result |
|---|---|
| Processing time before the change | 606–608 ms in the recorded baseline runs |
| Processing time after the change | 117 ms median across 20 runs; 103–191 ms range |
| Improvement over the baseline | About 5.2 times faster, or 81% less processing time |
| Performance target | All 20 measured runs finished below 200 ms |
| Output preserved | All 20 runs produced `out/All.gpx` byte-for-byte identical to `out/Test.gpx` |
| Output size and point count | 11.72 MB and 250,431 points, unchanged from the baseline |

These numbers measure the combined optimization, including faster point reading, reuse of output XML, and reuse of geometry calculations. They do not isolate each optimization's contribution or quantify the increase in peak memory. An earlier batch included a 272 ms outlier, so cache state and system load can still cause the timing check to fail. See the [README benchmark](../../README.md#performance) for the recorded run conditions.

- [Reader compatibility tests](../../internal/gpx/parse_compat_test.go) compare the dedicated reader with Go's decoder, including complex and invalid XML.
- [Output tests](../../internal/gpx/write_parts_test.go) check that joining saved XML produces the same bytes as generating the whole output at once. They also check byte counts and write errors.
- [Application tests](../../internal/app/app_integration_test.go) check output order and behavior when an input file fails.

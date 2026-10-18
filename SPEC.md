# Product requirements

## Purpose and scope

Prepare large local GPX collections for viewing by reducing point density and combining files. The output should load more easily while preserving recognizable paths and source track identity.

This is a lossy track conversion tool. Map matching, routing, coordinate reprojection, other export formats, a GUI, and lossless metadata archival are outside its scope.

## Product constraints

- Preserve source track boundaries, segment boundaries, and available track and segment names. Large gaps between segments may split a source track into separate output tracks because some viewers draw misleading straight connectors.
- Preserve each segment's first and last points during simplification. Keep at least two points per output segment and respect the requested simplification error limit.
- With the same inputs and options, keep output deterministic across worker counts and completion order. Optional generation timestamps make metadata vary between runs.
- Process the conversion in memory without intermediate GPX files. Memory must accommodate accumulated output tracks as well as active workers.
- Abort on a file-processing error before writing the merged output. A successful subset is not a completed merge. See [ADR 0002](docs/adr/0002-abort-on-file-errors.md).
- Keep generated output outside the source collection to prevent later runs from ingesting it.
- Provide a preview of savings and a run summary so operators can judge whether the conversion is useful.

Exact flags, defaults, validation rules, and report formats live in the sources linked from the [CLI reference](docs/CLI_REFERENCE.md).

## Limits and acceptance

The simplification error limit applies before coordinate rounding. The exported coordinates can introduce additional error, so the limit is not an end-to-end accuracy guarantee.

File-processing errors prevent the output write from starting. A later write failure can leave a partial file, and a metrics-log failure can occur after the GPX has been written. Final output replacement is not atomic.

GPX 1.1 viewer compatibility remains an acceptance target. Check representative output in the intended viewer; XML serialization tests alone do not establish viewer compatibility or full schema validity.

The original performance targets are at least three times the speed of a single worker on large datasets and at least 40% point reduction on dense traces. These depend on the dataset and machine. Evaluate them with a fixed dataset and settings, alongside viewer loading time and memory use. The dated README measurements are historical evidence, not universal guarantees.

## Performance regression requirement

- A fixed end-to-end workload completes within the configured performance budget (default: 5 seconds; override with `GPX_MERGE_PERFORMANCE_BUDGET`).

See [the development instructions](README.md#performance-regression-test) to run this check.

## Possible future work

Earlier specifications listed alternative simplification algorithms, adaptive tolerance, region or date output splitting, and a spatial-index sidecar. These remain ideas, not supported features or delivery commitments.

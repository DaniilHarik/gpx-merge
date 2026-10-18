# ADR 0002: Abort the merge on a file error

## Status

Accepted. Implemented on 2026-03-17; recorded retrospectively on 2026-10-18.

## Context

The previous implementation continued after file errors and merged the successful files. Commit `b69cf13` changed the product requirement to abort on any file-processing error while simplifying concurrent error handling. The history does not explain a broader product rationale for rejecting partial-success output.

## Decision

Return the first file-processing error, cancel shared worker work, and wait for workers to finish. Start the merged output write only after every file succeeds. Use `errgroup.WithContext` to manage completion and error propagation together.

## Alternatives considered

- Continue after file errors and merge successful inputs. This was the previous behavior and is no longer supported.
- Abort the merge after a file error. This is the chosen behavior, coupled with shared cancellation through `errgroup`.

## Consequences

- One bad input prevents a new merged output. Operators must fix or remove that input before retrying.
- The error identifies a file and processing stage. A failed run does not produce a complete inventory of all bad files.
- Cancellation is cooperative. Other work stops when it next checks the context.
- This protects against partial-success merges after input-processing errors. It does not make the final filesystem write atomic. A write failure can leave partial output, and a later metrics failure can occur after GPX output exists.

## Evidence

Commit `b69cf13` changed the worker pool, application error path, specification, and integration tests together.

See [the application](../../internal/app/app.go), [the worker pool](../../internal/pool/run.go), and `TestRunFailsOnInvalidFile` in [the integration tests](../../internal/app/app_integration_test.go).

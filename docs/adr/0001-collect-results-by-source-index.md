# ADR 0001: Collect results by source index

## Status

Accepted. Implemented on 2026-03-16; recorded retrospectively on 2026-10-18.

## Context

The merge needs stable output order even when files finish processing in a different order. The previous pool sent results through a channel, collected them all, then sorted them by source index. It already retained the full result set before writing output, so the channel did not provide streaming output or a bound on total retained track data.

## Decision

Give each file a result slot based on its source index. Workers write to their own slots, and the app reads the results after all workers finish. This removes the result channel, its closer, and the final sort.

## Alternatives considered

- Collect results through a channel and sort afterward. This was the previous implementation, replaced by direct indexed writes.
- Collect results in their assigned slots. This is the chosen approach. It uses the output order already supplied by discovery.

## Consequences

- Each file must have a unique index within the result slice. This ownership is what makes concurrent writes safe.
- Scheduling can change without changing output order. Later work used this separation to start larger files first.
- The merge still retains all successful results before writing. A streaming writer would require a separate design for ordering, memory, and errors.

## Evidence

Commit `3c8a789` replaced the channel and sort with indexed writes. Commit `b78e927` had already documented accumulation before writing. Commit `a14b6ec` added size-based scheduling on 2026-04-24 while preserving source indexes.

See [the pool](../../internal/pool/run.go), [its ordering test](../../internal/pool/order_test.go), and [the app integration tests](../../internal/app/app_integration_test.go).

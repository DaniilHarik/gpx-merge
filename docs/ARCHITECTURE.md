# Architecture

Start with [app.Run](../internal/app/app.go) and follow its imports for the current implementation. Package tests describe edge cases. This document keeps the processing overview and the reasons behind decisions that are easy to lose in code.

## Processing sequence

```mermaid
sequenceDiagram
    participant CLI
    participant App
    participant Workers
    participant Output
    CLI->>App: Arguments and cancellation context
    App->>App: Validate configuration and discover inputs
    App->>Workers: Process files concurrently
    Workers->>Workers: Read each source into memory
    Workers->>Workers: Parse ordinary points directly, decode other XML with standard decoder
    Workers->>Workers: Optionally reorder, simplify and measure geometry
    Note over Workers: Reuse each segment's projection across simplification attempts
    Workers->>Workers: Encode track fragments and measure their byte lengths
    Workers-->>App: Ordered results and XML fragments, or first error after workers finish
    alt File processing failed
        App-->>CLI: Error, without starting the output write
    else All files succeeded
        App->>App: Aggregate statistics and fragments in source order
        App->>Output: Wrap fragments in merged GPX or measure for dry run
        opt Output step succeeded
            App-->>CLI: Summary and warnings
            App->>Output: Append metrics if requested
        end
    end
```

## Decision records

- [ADR 0001: Collect results by source index](adr/0001-collect-results-by-source-index.md) explains why the pool does not collect and sort results through a channel.
- [ADR 0002: Abort the merge on a file error](adr/0002-abort-on-file-errors.md) records the move away from partial-success output.
- [ADR 0003: Parse XML directly into tracks](adr/0003-parse-xml-directly-into-tracks.md) records the allocation trade-off and the parser's validation responsibility.
- [ADR 0004: Read common GPX points faster and generate track XML once](adr/0004-accelerate-point-parsing-and-reuse-encoded-tracks.md) explains why the faster merge keeps source data and generated XML in memory.

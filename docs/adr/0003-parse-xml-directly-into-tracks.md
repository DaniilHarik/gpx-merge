# ADR 0003: Parse XML directly into tracks

## Status

Accepted. Implemented on 2026-04-24 and refined on 2026-09-04; recorded retrospectively on 2026-10-18.

## Context

Large GPX files can contain hundreds of thousands of track points. The former parser decoded an XML-shaped struct tree, then copied it into the track model. The README added with the replacement records duplicate allocations and peak memory as reasons for parsing directly into tracks. It also records the value of checking cancellation between token reads.

## Decision

Read XML tokens into the track model without building an intermediate XML-shaped tree. The later refinement uses raw tokens and skips decoding optional point fields that the run will discard. Keep the standard XML encoder for output escaping.

## Alternatives considered

- Decode XML mirror structs, then convert them into tracks. This was the previous implementation, replaced to avoid the intermediate representation.
- Use `Decoder.Token` to build tracks directly. This was the first replacement.
- Use `Decoder.RawToken` with explicit nesting checks. This is the current approach; the code records avoiding namespace translation overhead as the reason for the refinement.

## Consequences

- Parsing avoids the intermediate tree, but the merge still retains tracks in memory.
- The parser now owns nesting checks that raw tokens do not provide. Malformed input and unknown nested elements need regression coverage.
- Timestamp parsing may still be needed for segment ordering even when timestamps will be omitted from output.
- New retained GPX fields require deliberate parser and model changes. Arbitrary metadata preservation remains outside the product's scope.

## Evidence

Commit `8d9b428` introduced direct token parsing and documented the allocation rationale. Commit `645ca3a` introduced raw tokens, selective field decoding, and additional nesting checks.

See [the parser](../../internal/gpx/parse.go), [parser tests](../../internal/gpx/parse_test.go), and [the writer](../../internal/gpx/write.go).

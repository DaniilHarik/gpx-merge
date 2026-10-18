# Workflow

When changing processing logic, update the sequence diagram in `docs/ARCHITECTURE.md`.

## Documentation ownership

Keep exact flags, defaults, algorithms, validation rules, and output schemas in code and tests. Link to their definitions instead of copying them into prose.

Update related documentation in the same change when its guidance becomes inaccurate:

- [README.md](README.md) owns setup, usage examples, and development instructions. Date benchmark observations and keep their machine and dataset context.
- [SPEC.md](SPEC.md) owns product intent, constraints, acceptance targets, and scope. Keep implementation walkthroughs out of it.
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) owns the processing sequence and links to decisions.
- [docs/CLI_REFERENCE.md](docs/CLI_REFERENCE.md) points to generated help and the code that defines CLI contracts.
- [docs/GUIDE.md](docs/GUIDE.md) contains reading exercises. Update links and questions when the concurrency design changes.

Before changing an architectural choice, read the relevant record in [docs/adr](docs/adr). Record new trade-offs there when their rationale cannot be recovered from code.

## ADR format

Use sequential filenames such as `docs/adr/0001-decision-title.md` and the heading `# ADR NNNN: Decision title`. Use these sections in order: `Status`, `Context`, `Decision`, `Alternatives considered`, `Consequences`, `Evidence`.

For retrospective records, verify implementation dates and commit references, state the recording date, and distinguish documented reasons from inference. Leave unknown motivations explicit. Follow [ADR 0001](docs/adr/0001-collect-results-by-source-index.md).

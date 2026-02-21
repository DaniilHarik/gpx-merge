# Workflow

When changing processing logic, update the sequence diagram in `docs/ARCHITECTURE.md`.

## Markdown update rules

Update these files whenever related behavior changes:

- `README.md`
  - Update for user-facing changes: project summary, feature list, requirements, quick-start commands, or documentation links.
- `docs/ARCHITECTURE.md`
  - Update for internal flow changes: pipeline stages, module responsibilities, concurrency model, and the processing sequence diagram.
- `docs/CLI_REFERENCE.md`
  - Update for CLI contract changes: added/removed/renamed flags, default values, behavior notes, exit codes, or report format.
- `SPEC.md`
  - Update when expected product behavior or guarantees change (not implementation details only).
- `docs/CONCURRENCY_WORKSHEET.md`
  - Update when concurrency strategy, worker behavior, bottlenecks, or performance assumptions are changed.

If a code change touches multiple areas (for example, new processing behavior exposed via a new flag), update all relevant markdown files in the same change.

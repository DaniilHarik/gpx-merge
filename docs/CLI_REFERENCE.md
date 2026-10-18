# CLI reference

Run the built binary to see flags and their current defaults:

```bash
./gpx-merge --help
```

Help comes from the same declarations used to parse arguments. The current CLI returns exit status `2` when printing help through its usage-error path.

For details beyond help, use these sources:

| Contract | Source |
|---|---|
| Flags, defaults, and validation | [CLI configuration](../internal/cli/config.go) |
| Exit codes and output timing | [Application entry point](../internal/app/app.go) |
| Human report formatting | [Report writer](../internal/report/report.go) |
| CSV columns, units, and append behavior | [Metrics writer](../internal/report/metrics_csv.go) |

See the [README](../README.md) for commands and the [product limits](../SPEC.md#limits-and-acceptance) before relying on output accuracy or failure handling.

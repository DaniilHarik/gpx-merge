#!/usr/bin/env bash
set -euo pipefail

INPUT_DIR="${INPUT_DIR:-./data}"
OUTPUT_FILE="${OUTPUT_FILE:-./out/All.gpx}"
GOGC="${GOGC:-${GPX_MERGE_GOGC:-400}}"

mkdir -p "$(dirname "$OUTPUT_FILE")"

GOGC="$GOGC" go run ./cmd/gpx-merge --input "$INPUT_DIR" --output "$OUTPUT_FILE" --metrics-csv ./out/metrics.csv "$@"

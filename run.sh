#!/usr/bin/env bash
set -euo pipefail

INPUT_DIR="${INPUT_DIR:-./data}"
OUTPUT_FILE="${OUTPUT_FILE:-./out/All.gpx}"

mkdir -p "$(dirname "$OUTPUT_FILE")"

go run ./cmd/gpx-merge --input "$INPUT_DIR" --output "$OUTPUT_FILE" --metrics-csv ./out/metrics.csv "$@"

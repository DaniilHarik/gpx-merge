#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: ./verify.sh [threshold_ms] [run.sh arguments...]

Run run.sh and require its reported Elapsed time to be below the threshold.
The default threshold is 650 milliseconds. Accepts 650 or 650ms.
Go compilation and startup time are excluded from the reported time.
Exit 0 on success, 1 when the threshold is met or exceeded or timing is
missing, and 2 for an invalid threshold. Errors from run.sh also fail.

Examples:
  ./verify.sh
  ./verify.sh 650ms
  ./verify.sh 800 --workers 4
EOF
}

if [[ ${1:-} == --help || ${1:-} == -h ]]; then
  usage
  exit 0
fi

threshold=650
if [[ $# -gt 0 && $1 != --* ]]; then
  threshold=${1%ms}
  shift
fi
if [[ ! $threshold =~ ^[0-9]+([.][0-9]+)?$ ]] ||
  ! LC_ALL=C awk -v threshold="$threshold" 'BEGIN { exit !(threshold > 0) }'; then
  printf 'Invalid threshold: %s. Use a positive number of milliseconds.\n' "$threshold" >&2
  exit 2
fi

cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
output=$(mktemp)
trap 'rm -f "$output"' EXIT

if ./run.sh "$@" | tee "$output"; then
  :
else
  status=$?
  printf 'FAIL: run.sh failed with exit status %s.\n' "$status" >&2
  exit "$status"
fi

# PrintSummary rounds to milliseconds but uses Go duration notation, including
# seconds, minutes, and hours: internal/report/report.go.
if elapsed_ms=$(LC_ALL=C awk '
  /^Elapsed: / && NF == 2 {
    duration = $2
    milliseconds = 0
    while (match(duration, /^[0-9]+(\.[0-9]+)?(ms|h|m|s)/)) {
      part = substr(duration, 1, RLENGTH)
      factor = part ~ /ms$/ ? 1 : part ~ /h$/ ? 3600000 : part ~ /m$/ ? 60000 : 1000
      milliseconds += (part + 0) * factor
      duration = substr(duration, RLENGTH + 1)
    }
    if (duration != "") exit 1
    count++
    elapsed = milliseconds
  }
  END {
    if (count != 1) exit 1
    printf "%.0f\n", elapsed
  }
' "$output"); then
  :
else
  printf 'FAIL: could not read a single valid Elapsed time from run.sh.\n' >&2
  exit 1
fi

if LC_ALL=C awk -v elapsed="$elapsed_ms" -v threshold="$threshold" 'BEGIN { exit !(elapsed < threshold) }'; then
  printf 'PASS: %sms is below %sms.\n' "$elapsed_ms" "$threshold"
else
  printf 'FAIL: %sms must be below %sms.\n' "$elapsed_ms" "$threshold" >&2
  exit 1
fi

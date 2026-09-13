#!/usr/bin/env bash
# End-to-end AutOps k6 bench: mode-switch → baseline → mode-switch → shaping → summary.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPTS="$ROOT/scripts/loadtest"
OUT_DIR="$ROOT/bench-results"
PROXY_URL="${PROXY_URL:-http://localhost:8080}"
TENANT="${TENANT_DOMAIN:-localhost}"
ADMIN_TOKEN="${ADMIN_TOKEN:-}"

if [[ -z "$ADMIN_TOKEN" ]]; then
  echo "error: ADMIN_TOKEN is not set (required for automatic mode switching)" >&2
  exit 1
fi

code="$(curl -s -o /dev/null -w '%{http_code}' --connect-timeout 2 \
  -H "Host: ${TENANT}" "${PROXY_URL}/" || true)"
if [[ -z "$code" || "$code" == "000" ]]; then
  echo "error: proxy not reachable at ${PROXY_URL} (is it running on :8080?)" >&2
  exit 1
fi
echo "proxy ok (${PROXY_URL}, http ${code})"

set_mode() {
  local mode="$1"
  echo "switching tenant '${TENANT}' → ${mode}"
  curl -sf -X POST "${PROXY_URL}/admin/mode" \
    -H "X-Admin-Token: ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"tenant\":\"${TENANT}\",\"mode\":\"${mode}\"}" >/dev/null
}

mkdir -p "$OUT_DIR"
TS="$(date +%Y%m%d-%H%M%S)"
BASELINE_JSON="$OUT_DIR/baseline-${TS}.json"
SHAPING_JSON="$OUT_DIR/shaping-${TS}.json"
SUITE_STATUS=0

run_k6() {
  local label="$1"
  local script="$2"
  local export_path="$3"
  echo "running ${label} …"
  # Threshold failures exit 99 but still write --summary-export; keep going so
  # both suites run and we can print a combined summary.
  set +e
  k6 run --summary-export="$export_path" "$script"
  local ec=$?
  set -e
  if [[ $ec -eq 0 ]]; then
    return 0
  fi
  if [[ $ec -eq 99 && -f "$export_path" ]]; then
    echo "warning: ${label} crossed k6 thresholds (continuing)" >&2
    SUITE_STATUS=99
    return 0
  fi
  echo "error: ${label} failed (exit ${ec})" >&2
  exit "$ec"
}

set_mode normal
run_k6 baseline.js "$SCRIPTS/baseline.js" "$BASELINE_JSON"

set_mode shaping
run_k6 spike-shaping.js "$SCRIPTS/spike-shaping.js" "$SHAPING_JSON"

# Leave the gateway in normal mode after the suite.
set_mode normal

python "$SCRIPTS/summarize_bench.py" \
  baseline "$BASELINE_JSON" \
  shaping "$SHAPING_JSON"

echo "raw results: $BASELINE_JSON"
echo "raw results: $SHAPING_JSON"
exit "$SUITE_STATUS"

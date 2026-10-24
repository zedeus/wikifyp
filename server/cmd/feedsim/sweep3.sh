#!/usr/bin/env bash
# sweep3.sh — automated parameter sweep with structured output.
# Usage: ./sweep3.sh [fast|full] [persona-filter]
set -euo pipefail
cd "$(dirname "$0")/../.."

MODE="${1:-fast}"
FILTER="${2:-}"

FAST_FLAG=""
if [ "$MODE" = "fast" ]; then
    FAST_FLAG="-fast"
fi

RUN_FLAG=""
if [ -n "$FILTER" ]; then
    RUN_FLAG="-run $FILTER"
fi

DSN="${WIKIFYP_DATABASE_URL:-postgres://postgres:wikifyp@127.0.0.1:5433/wikifyp?sslmode=disable}"
export WIKIFYP_DATABASE_URL="$DSN"

RESULTS="/tmp/feedsim-sweep3-$(date +%Y%m%d-%H%M%S).txt"

run() {
    local label="$1"; shift
    echo "=== $label ===" | tee -a "$RESULTS"
    go run ./cmd/feedsim -bench $FAST_FLAG $RUN_FLAG -engage-off "$@" 2>&1 | tee -a "$RESULTS"
    echo "" >> "$RESULTS"
}

echo "Results: $RESULTS"
echo "Mode: $MODE | Filter: ${FILTER:-all}"

# Baseline
run "baseline (sim=0.6 explore=0.20 decay=0.3-0.6 boost=1)" \
    -sim-penalty 0.6 -explore-base 0.20 -decay-min 0.3 -decay-max 0.6

# --- Cluster recency boost (serial/scattered focus) ---
run "boost=2" \
    -sim-penalty 0.6 -explore-base 0.20 -decay-min 0.3 -decay-max 0.6 -cluster-boost 2
run "boost=3" \
    -sim-penalty 0.6 -explore-base 0.20 -decay-min 0.3 -decay-max 0.6 -cluster-boost 3
run "boost=4" \
    -sim-penalty 0.6 -explore-base 0.20 -decay-min 0.3 -decay-max 0.6 -cluster-boost 4

# --- Boost + faster decay (help serial forget old interests) ---
run "boost=3 decay=0.2-0.4" \
    -sim-penalty 0.6 -explore-base 0.20 -decay-min 0.2 -decay-max 0.4 -cluster-boost 3
run "boost=3 decay=0.2-0.5" \
    -sim-penalty 0.6 -explore-base 0.20 -decay-min 0.2 -decay-max 0.5 -cluster-boost 3

# --- Higher explore for cold-start ---
run "boost=3 explore=0.25" \
    -sim-penalty 0.6 -explore-base 0.25 -decay-min 0.3 -decay-max 0.6 -cluster-boost 3
run "boost=3 exploremax=0.85" \
    -sim-penalty 0.6 -explore-base 0.20 -explore-max 0.85 -decay-min 0.3 -decay-max 0.6 -cluster-boost 3

# --- Combined: boost + decay + explore ---
run "boost=3 decay=0.2-0.5 explore=0.25" \
    -sim-penalty 0.6 -explore-base 0.25 -decay-min 0.2 -decay-max 0.5 -cluster-boost 3
run "boost=2 decay=0.3-0.5 explore=0.25" \
    -sim-penalty 0.6 -explore-base 0.25 -decay-min 0.3 -decay-max 0.5 -cluster-boost 2

# --- Lower sim for picky ---
run "boost=3 sim=0.5" \
    -sim-penalty 0.5 -explore-base 0.20 -decay-min 0.3 -decay-max 0.6 -cluster-boost 3

echo ""
echo "=== DONE ==="
echo "Results: $RESULTS"
echo ""
echo "--- Quick comparison (appeal %) ---"
grep -E '(^===|appeal)' "$RESULTS" | grep -v 'band:'

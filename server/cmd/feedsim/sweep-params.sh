#!/usr/bin/env bash
# sweep-params.sh — focused parameter sweep with engagement ON
# Tests combos that might interact with engagement's continuous signal
# Usage: ./sweep-params.sh
set -euo pipefail
cd "$(dirname "$0")/../.."

DSN="${WIKIFYP_DATABASE_URL:-postgres://postgres:wikifyp@127.0.0.1:5433/wikifyp?sslmode=disable}"
export WIKIFYP_DATABASE_URL="$DSN"

RESULTS="/tmp/feedsim-params-$(date +%Y%m%d-%H%M%S).txt"
BIN=$(mktemp -d)/feedsim
go build -o "$BIN" ./cmd/feedsim
trap "rm -f '$BIN'" EXIT

run() {
    local label="$1"; shift
    echo "=== $label ===" | tee -a "$RESULTS"
    "$BIN" -bench -fast -seeds 1,2,3 "$@" 2>&1 | grep -E '(band:)' | tee -a "$RESULTS"
    echo "" >> "$RESULTS"
}

echo "Results: $RESULTS"

# Baseline
run "engage ON (current defaults)"
run "engage OFF (binary)" -engage-off

# Decay floor with engagement
run "decay 0.2-0.5" -decay-min 0.2 -decay-max 0.5
run "decay 0.3-0.7" -decay-min 0.3 -decay-max 0.7
run "decay 0.4-0.7" -decay-min 0.4 -decay-max 0.7

# Similarity penalty with engagement
run "sim 0.5" -sim-penalty 0.5
run "sim 0.7" -sim-penalty 0.7
run "sim 0.8" -sim-penalty 0.8

# Explore share with engagement
run "explore 0.15" -explore-base 0.15
run "explore 0.25" -explore-base 0.25

# Cluster boost with engagement
run "boost 1 (no boost)" -cluster-boost 1
run "boost 3" -cluster-boost 3

# Best combos
run "sim 0.5 + decay 0.3-0.7" -sim-penalty 0.5 -decay-min 0.3 -decay-max 0.7
run "sim 0.7 + decay 0.2-0.5" -sim-penalty 0.7 -decay-min 0.2 -decay-max 0.5

echo ""
echo "=== DONE ==="
echo "Results: $RESULTS"

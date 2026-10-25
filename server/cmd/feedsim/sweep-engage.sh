#!/usr/bin/env bash
# sweep-engage.sh — engagement model parameter sweep + full benchmark
# Usage: ./sweep-engage.sh [full|fast]
set -euo pipefail
cd "$(dirname "$0")/../.."

MODE="${1:-full}"
FAST_FLAG=""
[ "$MODE" = "fast" ] && FAST_FLAG="-fast"

DSN="${WIKIFYP_DATABASE_URL:-postgres://postgres:wikifyp@127.0.0.1:5433/wikifyp?sslmode=disable}"
export WIKIFYP_DATABASE_URL="$DSN"

RESULTS="/tmp/feedsim-engage-$(date +%Y%m%d-%H%M%S).txt"

run() {
    local label="$1"; shift
    echo "=== $label ===" | tee -a "$RESULTS"
    go run ./cmd/feedsim -bench $FAST_FLAG -seeds 1,2,3 "$@" 2>&1 | grep -E '(benchmark|band:)' | tee -a "$RESULTS"
    echo "" >> "$RESULTS"
}

echo "Results: $RESULTS"
echo "Mode: $MODE"

# 1. Binary baseline (engage OFF)
run "binary baseline" -engage-off

# 2. Current engage ON (0.5+0.5*c finish, -(1-c) skip)
run "engage ON (current)"

# 3. Wider positive range: 0.3+0.7*c (more differentiation)
# Need to test via code change... use current as-is

# 4. Engagement + different decay
run "engage ON + decay 0.2-0.5" -decay-min 0.2 -decay-max 0.5
run "engage ON + decay 0.3-0.7" -decay-min 0.3 -decay-max 0.7

# 5. Engagement + different sim penalty
run "engage ON + sim 0.5" -sim-penalty 0.5
run "engage ON + sim 0.7" -sim-penalty 0.7

# 6. Engagement + different explore
run "engage ON + explore 0.25" -explore-base 0.25
run "engage ON + explore 0.15" -explore-base 0.15

# 7. Binary + same param combos for comparison
run "binary + decay 0.3-0.7" -engage-off -decay-min 0.3 -decay-max 0.7
run "binary + sim 0.5" -engage-off -sim-penalty 0.5

echo ""
echo "=== DONE ==="
echo "Results: $RESULTS"

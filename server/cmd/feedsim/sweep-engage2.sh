#!/usr/bin/env bash
# sweep-engage2.sh — engagement-specific parameter sweep
# Tests token saturation and decay interactions with the engagement model
# Usage: ./sweep-engage2.sh
set -euo pipefail
cd "$(dirname "$0")/../.."

DSN="${WIKIFYP_DATABASE_URL:-postgres://postgres:wikifyp@127.0.0.1:5433/wikifyp?sslmode=disable}"
export WIKIFYP_DATABASE_URL="$DSN"

RESULTS="/tmp/feedsim-engage2-$(date +%Y%m%d-%H%M%S).txt"
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
run "defaults (token-sat=500)"

# Token saturation: how many tokens for full engagement credit
run "token-sat 300" -token-sat 300
run "token-sat 750" -token-sat 750
run "token-sat 1000" -token-sat 1000

# Token saturation + decay combos
run "token-sat 300 + decay 0.3-0.7" -token-sat 300 -decay-min 0.3 -decay-max 0.7
run "token-sat 750 + decay 0.3-0.7" -token-sat 750 -decay-min 0.3 -decay-max 0.7

# Similarity penalty (the winning combo from prior sweeps)
run "sim 0.5 + token-sat 750" -sim-penalty 0.5 -token-sat 750
run "sim 0.7 + token-sat 750" -sim-penalty 0.7 -token-sat 750

echo ""
echo "=== DONE ==="
echo "Results: $RESULTS"

#!/bin/bash
# Follow-up sweep: test combinations of the top winners from sweep.sh.
# Winners: sim-penalty=0.5, explore=0.10-0.15, decay=0.7
set -e
cd "$(dirname "$0")/../.."
go build -o /tmp/feedsim-sweep ./cmd/feedsim
DB=${WIKIFYP_DATABASE_URL:?set WIKIFYP_DATABASE_URL}

clean() {
    PGPASSWORD=wikifyp psql -h 127.0.0.1 -p 5433 -U postgres -d wikifyp -q -c \
        "DELETE FROM reader_impressions WHERE reader_id LIKE 'sim-%'; DELETE FROM reader_article_state WHERE reader_id LIKE 'sim-%';" 2>/dev/null
}

run() {
    local label="$1"; shift
    clean
    printf "%-45s" "$label"
    result=$(/tmp/feedsim-sweep -bench -fast -seeds 1 "$@" 2>&1 | grep -oP 'appeal\s+\K[0-9.]+' | tr '\n' ' ')
    echo "$result"
}

echo "=== WINNER COMBINATIONS ==="
echo "Order: picky gundam mono juggler"
run "baseline"
run "sim=0.5"                                  -sim-penalty 0.5
run "explore=0.10"                             -explore-base 0.10
run "explore=0.15"                             -explore-base 0.15
run "decay=0.7"                                -decay-floor 0.7

echo ""
echo "=== PAIRS ==="
run "sim=0.5 + explore=0.10"                   -sim-penalty 0.5 -explore-base 0.10
run "sim=0.5 + explore=0.15"                   -sim-penalty 0.5 -explore-base 0.15
run "sim=0.5 + decay=0.7"                      -sim-penalty 0.5 -decay-floor 0.7
run "explore=0.10 + decay=0.7"                 -explore-base 0.10 -decay-floor 0.7
run "explore=0.15 + decay=0.7"                 -explore-base 0.15 -decay-floor 0.7

echo ""
echo "=== TRIPLES ==="
run "sim=0.5 + explore=0.10 + decay=0.7"       -sim-penalty 0.5 -explore-base 0.10 -decay-floor 0.7
run "sim=0.5 + explore=0.15 + decay=0.7"       -sim-penalty 0.5 -explore-base 0.15 -decay-floor 0.7
run "sim=0.5 + explore=0.10 + decay=0.5"       -sim-penalty 0.5 -explore-base 0.10 -decay-floor 0.5
run "sim=0.5 + explore=0.15 + decay=0.5"       -sim-penalty 0.5 -explore-base 0.15 -decay-floor 0.5

echo ""
echo "=== EXTREME COMBOS ==="
run "sim=0.3 + explore=0.10 + decay=0.7"       -sim-penalty 0.3 -explore-base 0.10 -decay-floor 0.7
run "sim=0.5 + explore=0.05 + decay=0.7"       -sim-penalty 0.5 -explore-base 0.05 -decay-floor 0.7
run "sim=0.5 + explore=0.15 + decay=0.8"       -sim-penalty 0.5 -explore-base 0.15 -decay-floor 0.8
run "sim=0.5+exp=0.15+dec=0.7+cap=0.7"         -sim-penalty 0.5 -explore-base 0.15 -decay-floor 0.7 -fresh-cap 0.7

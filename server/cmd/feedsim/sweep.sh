#!/bin/bash
# Comprehensive parameter sweep using fast mode (~50s per config).
# Tests engagement, impression, and feed mechanism parameters.
# Usage: WIKIFYP_DATABASE_URL=... ./sweep.sh
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
    printf "%-35s" "$label"
    result=$(/tmp/feedsim-sweep -bench -fast -seeds 1 "$@" 2>&1 | grep -oP 'appeal\s+\K[0-9.]+' | tr '\n' ' ')
    # Order: picky gundam mono juggler
    echo "$result"
}

echo "=== BASELINE (current defaults) ==="
echo "Order: picky gundam mono juggler"
run "baseline"

echo ""
echo "=== A. ENGAGEMENT THRESHOLD ==="
for t in 0.10 0.15 0.20 0.25 0.33; do
    run "thresh=$t" -engage-threshold "$t"
done

echo ""
echo "=== B. RECENCY DECAY FLOOR ==="
for d in 0.1 0.2 0.3 0.5 0.7; do
    run "decay=$d" -decay-floor "$d"
done

echo ""
echo "=== C. SIMILARITY PENALTY ==="
for s in 0.0 0.5 1.0 1.5 2.0; do
    run "sim-penalty=$s" -sim-penalty "$s"
done

echo ""
echo "=== D. EXPLORATION SHARE ==="
for e in 0.10 0.15 0.25 0.35; do
    run "explore=$e" -explore-base "$e"
done

echo ""
echo "=== E. SATIATION CAP ==="
for c in 0.3 0.5 0.7 1.0; do
    run "fresh-cap=$c" -fresh-cap "$c"
done

echo ""
echo "=== F. IMPRESSIONS OFF vs ON ==="
run "imps-off (warm=99999)" -imp-warmup 99999
run "imps-on (warm=5000)" -imp-warmup 5000
run "imps-aggressive (warm=2000)" -imp-warmup 2000

echo ""
echo "=== G. COMBINED EXPERIMENTS ==="
run "thresh=0.20+decay=0.1"         -engage-threshold 0.20 -decay-floor 0.1
run "thresh=0.20+decay=0.5"         -engage-threshold 0.20 -decay-floor 0.5
run "thresh=0.20+sim=0.5"           -engage-threshold 0.20 -sim-penalty 0.5
run "thresh=0.20+sim=2.0"           -engage-threshold 0.20 -sim-penalty 2.0
run "thresh=0.20+explore=0.15"      -engage-threshold 0.20 -explore-base 0.15
run "thresh=0.20+cap=0.3"           -engage-threshold 0.20 -fresh-cap 0.3
run "thresh=0.20+cap=0.7"           -engage-threshold 0.20 -fresh-cap 0.7
run "thresh=0.20+decay=0.5+sim=0.5" -engage-threshold 0.20 -decay-floor 0.5 -sim-penalty 0.5
run "thresh=0.20+decay=0.5+cap=0.7" -engage-threshold 0.20 -decay-floor 0.5 -fresh-cap 0.7
run "no-engagement (thresh=0.01)"   -engage-threshold 0.01
run "no-satiation (cap=99)"         -fresh-cap 99
run "no-MMR (sim=0)"                -sim-penalty 0
run "no-explore (base=0.01)"        -explore-base 0.01

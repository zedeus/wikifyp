# The feed against the recommender-systems literature

A cross-check of the feed engine (`server/internal/feed`) against published
work: what each mechanism is called elsewhere, where we deviate on purpose,
and which known techniques are worth trying next. Written at the end of the
feed's first build-out so later work starts from names instead of
rediscovery.

## What we built, by its literature name

| Ours | Known as | Notes |
|---|---|---|
| Link/category scoring with 1/sqrt(df) weights | Content-based filtering with TF-IDF-style term weighting | Classic. Our "terms" are categories; the square-root curve is the standard dampening choice. |
| Outgoing-link density with saturation | Normalized co-citation / hypergeometric lift | Ordering candidates by matches-per-link is exactly what the lift statistic orders by; the saturation cap is ours. |
| Nomination strength `min(1, 50/degree)` | Degree-penalized edge weighting (cf. Adamic–Adar's spirit) | Down-weighting high-degree nodes is standard graph-recommendation hygiene. |
| 3-hop category random walks | Personalized PageRank, Monte-Carlo approximated (cf. Pixie, Pinterest 2018) | Pixie also runs short seeded walks from user items at serve time and found the same thing we measured: degree-weighted steps drift to hubs. |
| Similarity penalty at selection | Maximal Marginal Relevance (Carbonell & Goldstein 1998) | Textbook MMR, greedy form. |
| Topical satiation with a cap | Submodular coverage / intent-aware diversification (xQuAD family) | Same goal — marginal value of another same-topic card decays — implemented as a hard per-feed gate instead of a decaying objective. |
| Interest-cluster rotation | Round-robin fairness over latent intents; closest to subprofile-aware diversification (Kaya & Bridge 2019) | Their "subprofiles" are our state clusters. Rotating turns across them is our variant of giving each subprofile list share. |
| Adaptive exploration share | Decaying epsilon-greedy (contextual bandit, population-level schedule) | Epsilon decays on corroborated evidence rather than time or impression count — a deliberate stand-in until impressions exist. |
| Featured pool for new readers | Editorial fallback for cold start | Standard practice; the wiki's own review process is the editor. |
| Recency-decayed interest counts | Time-decayed user profiles (standard in news recommendation) | Rank-based rather than wall-clock decay, so a slow reader isn't punished. |
| Day-seeded jitter | Deterministic dithering (common in industrial rankers) | Same motivation: stable within a session, fresh across days, no slot-machine feel. |

The overall shape — bounded candidate generation from several sources, one
scoring pass, then a greedy re-ranking stage where diversity, fairness and
exploration live — is the standard two-stage retrieve-then-rerank
architecture used by industrial feeds.

## Where we deviate on purpose

- **No learned model anywhere.** Scores are closed-form over links and
  categories. At our scale (one reader's bounded state, a corpus the size
  of Simple English Wikipedia) the measured failure modes were structural
  (turn allocation, pool composition), not estimation quality.
- **Fairness unit = connected component of the reader's reads**, computed
  per request, rather than offline-learned topics or embeddings. Cheap,
  explainable, and the same machinery drives satiation, the exploration
  ramp and diagnostics.
- **Exploration is slot-reserved, not score-mixed.** A guaranteed, evenly
  spaced share of the deck survives any score distribution, which matters
  when the reader swipes cards one at a time in served order.

## Known techniques worth trying, in order of expected value

1. **Impression discounting** (Lee et al., KDD 2014; "impression-aware
   recommender systems" is the survey term). Multiply a candidate's score
   by a decay in how often it was served without action. This is the
   missing signal behind our two measured open problems — the five-interest
   collapse and stale-interest residue — and the event pipeline should
   record impressions with exactly this consumer in mind. The same logs
   enable fatigue-aware variants later.
2. **Calibrated recommendations** (Steck, RecSys 2018). Make the served
   distribution over the reader's interests proportional to their history
   distribution, enforced greedily at selection with a KL term. Directly
   attacks the five-interest collapse *without* impressions: our cluster
   rotation aims at equal turns but loses balance when a broad interest
   fragments; calibration would bound each interest's share of the whole
   feed regardless of fragment count. The natural next selection
   experiment.
3. **Thompson sampling / UCB over interests.** Once impressions exist, the
   exploration schedule can become a real bandit (explore the interests
   whose value is uncertain, not a fixed share of the corpus).
4. **Serendipity-oriented evaluation** (unexpectedness x relevance). Our
   benchmark's latent-interest adoption metrics are a homegrown version;
   the literature's metrics could sharpen the lab if discovery regresses.

## Explicitly rejected after measurement

Links-only recommendation (degenerates to loops around popular pages — the
failure mode of every naive "related articles" feed), link-overlap and
candidate-co-ownership clustering (hub pages bridge unrelated interests),
turn share proportional to row counts (engagement compounds until one
interest owns the feed), and global unconditioned exploration quotas for
readers with no usable pattern. The server's git history and test suite
pin each of these with the numbers that killed them.

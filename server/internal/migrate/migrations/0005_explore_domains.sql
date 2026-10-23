-- Stratified exploration: the wiki's own category tree assigns every
-- content category a top-level branch (its domain) — on simplewiki the
-- children of Category:Articles: Geography, Nature, People, Science, … —
-- and every article the modal domain of its categories. Exploration then
-- opens each feed's doors on distinct domains instead of sampling the
-- corpus uniformly, where three mega-domains (People, Everyday life,
-- Geography) own 71% of the draws and Science surfaces once per ~16 feeds.
-- fetch-categories computes both; ingest maintains article_domains for new
-- articles. Empty tables mean "not fetched yet" and exploration falls back
-- to uniform sampling.
ALTER TABLE category_pages ADD COLUMN domain text;

CREATE TABLE article_domains (
    article_id bigint PRIMARY KEY REFERENCES articles (id),
    domain text NOT NULL
);

-- The exploration sampler's access path: a seeded random-id probe per
-- domain descends this index once per pick instead of scanning a domain's
-- partition.
CREATE INDEX article_domains_domain ON article_domains (domain, article_id);

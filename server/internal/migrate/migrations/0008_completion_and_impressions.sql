-- Completion percentage: how far through an article the reader got, derived
-- from the furthest token address in their events. Max-so-far, never
-- regresses. 0 = not started, 1.0 = every token seen.
ALTER TABLE reader_article_state ADD COLUMN completion real NOT NULL DEFAULT 0;

-- Impressions: which articles the feed served to each reader. One row per
-- (reader, page, served_at). The feed records an impression for every card
-- it returns; impression-normalized interest divides category weight by how
-- often the reader has seen articles in that category, so heavily-served
-- categories that never get clicked are downweighted.
CREATE TABLE reader_impressions (
    reader_id text NOT NULL,
    source text NOT NULL,
    page_id bigint NOT NULL,
    served_at timestamptz NOT NULL DEFAULT now()
);

-- The feed needs: how many times has this reader seen articles in each
-- category? That's a join from reader_impressions through latest_articles
-- to article_categories. Index the lookup path.
CREATE INDEX reader_impressions_reader ON reader_impressions (reader_id);
CREATE INDEX reader_impressions_reader_page ON reader_impressions (reader_id, source, page_id);

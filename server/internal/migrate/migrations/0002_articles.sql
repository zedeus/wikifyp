-- Articles and their sentences. One row per ingested revision: re-ingesting
-- a revision is a no-op (UNIQUE on source + revision_id) and a newer
-- revision of the same page becomes a new row, so stored sentence addresses
-- are never renumbered. Tokens are not stored: splitting is deterministic
-- and pinned by golden tests, so tokens are derived from sentence text.

CREATE TABLE articles (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source text NOT NULL,
    title text NOT NULL,
    url text NOT NULL,
    page_id bigint, -- wiki page ID; NULL when the source does not know it
    revision_id bigint NOT NULL,
    license text NOT NULL,
    attribution text NOT NULL,
    fetched_at date NOT NULL,
    ingested_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source, revision_id)
);

-- Token addresses are (article_id, sentence_idx, token_idx); sentence_idx is
-- assigned once at ingest and stable forever. heading_level 0 is prose;
-- 2 to 6 is a section heading of that depth, kept for article outlines.
CREATE TABLE sentences (
    article_id bigint NOT NULL REFERENCES articles (id) ON DELETE CASCADE,
    sentence_idx integer NOT NULL,
    heading_level smallint NOT NULL DEFAULT 0,
    text text NOT NULL,
    PRIMARY KEY (article_id, sentence_idx)
);

-- Outgoing links and visible categories, stored for feed ranking.
CREATE TABLE article_links (
    article_id bigint NOT NULL REFERENCES articles (id) ON DELETE CASCADE,
    target text NOT NULL,
    PRIMARY KEY (article_id, target)
);

CREATE TABLE article_categories (
    article_id bigint NOT NULL REFERENCES articles (id) ON DELETE CASCADE,
    category text NOT NULL,
    PRIMARY KEY (article_id, category)
);

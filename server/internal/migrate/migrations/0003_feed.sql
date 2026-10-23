-- Feed: per-reader article state and the version-grouping rule.
--
-- Every source now provides the wiki page ID (the three oldest fixtures were
-- backfilled), so page_id becomes required: it is the key that groups the
-- revisions of one page. A database holding pre-backfill NULL rows fails
-- here on purpose — only the three old fixtures could have produced them,
-- and re-ingesting their backfilled versions cannot repair a stored row
-- (same revision, ON CONFLICT DO NOTHING), so the operator must drop or fix
-- those rows deliberately rather than have a migration guess page IDs.
ALTER TABLE articles ALTER COLUMN page_id SET NOT NULL;

-- The one place that defines which revision of a page counts: the feed (and
-- anything else recommending articles) reads latest_articles, never articles,
-- so two revisions of one page can never both surface. Phrased as an
-- anti-join rather than DISTINCT ON so the planner can push the caller's
-- filters into the view and probe per row instead of materializing every
-- page's latest revision on each reference.
CREATE VIEW latest_articles AS
SELECT a.*
FROM articles a
WHERE NOT EXISTS (
    SELECT 1 FROM articles newer
    WHERE newer.source = a.source
      AND newer.page_id = a.page_id
      AND newer.revision_id > a.revision_id
);

-- What a reader did with a page. Keyed by page, not revision, so finishing
-- revision N also suppresses revision N+1. One state per page: a later
-- action replaces the earlier one (saving then finishing leaves finished).
CREATE TABLE reader_article_state (
    reader_id text NOT NULL,
    source text NOT NULL,
    page_id bigint NOT NULL,
    state text NOT NULL CHECK (state IN ('finished', 'saved', 'skipped')),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (reader_id, source, page_id)
);

-- Feed candidate lookups: articles linking to a finished title, and articles
-- sharing a category with the reader's interests.
CREATE INDEX article_links_target ON article_links (target);
CREATE INDEX article_categories_category ON article_categories (category);

-- The access path of latest_articles: without it every reference to the
-- view sorts the whole table.
CREATE INDEX articles_latest ON articles (source, page_id, revision_id DESC);

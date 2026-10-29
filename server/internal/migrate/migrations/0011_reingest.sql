-- Re-ingesting a revision whose stored text predates a pipeline fix.
--
-- Stored rows are immutable (addresses must round-trip forever), so a better
-- cleaner cannot edit them — but the fixtures pin their revisions, so the
-- cleaner's output for the *same* revision needs somewhere to live. A
-- re-ingest is a new article row for the same (source, revision_id) with a
-- higher ingest_seq; latest_articles prefers it, and rows already stored
-- (with every address pointing at them) are untouched. Plain ingest still
-- writes ingest_seq 0, so its idempotency per revision is unchanged.
ALTER TABLE articles ADD COLUMN ingest_seq smallint NOT NULL DEFAULT 0;
ALTER TABLE articles DROP CONSTRAINT articles_source_revision_id_key;
ALTER TABLE articles ADD UNIQUE (source, revision_id, ingest_seq);

-- The version-grouping rule gains the tiebreak: among rows of one page the
-- feed sees the highest (revision_id, ingest_seq). DROP + CREATE because a
-- view's column list cannot grow in place.
DROP VIEW latest_articles;
CREATE VIEW latest_articles AS
SELECT a.*
FROM articles a
WHERE NOT EXISTS (
    SELECT 1 FROM articles newer
    WHERE newer.source = a.source
      AND newer.page_id = a.page_id
      AND (newer.revision_id, newer.ingest_seq) > (a.revision_id, a.ingest_seq)
);

-- Keep the view's access path aligned with its ordering.
DROP INDEX articles_latest;
CREATE INDEX articles_latest ON articles (source, page_id, revision_id DESC, ingest_seq DESC);

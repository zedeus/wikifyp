-- Saved becomes a flag orthogonal to finished/skipped. As a value of
-- state, a later finish overwrote the bookmark (one state per page), so a
-- saved-articles list would silently lose everything the reader actually
-- read. The flag survives finishing; state keeps only what the reader DID
-- with the page (finished/skipped), and a bookmark with no state yet is a
-- row with NULL state and saved = true.
ALTER TABLE reader_article_state ADD COLUMN saved boolean NOT NULL DEFAULT false;
ALTER TABLE reader_article_state ALTER COLUMN state DROP NOT NULL;

-- Existing bookmarks move to the flag. Only rows that were never finished
-- or skipped can hold state = 'saved' (a later action overwrote it), so
-- nulling state loses nothing.
UPDATE reader_article_state SET saved = true, state = NULL WHERE state = 'saved';

-- Tighten the CHECK: 'saved' is no longer a state. Re-added under the name
-- the inline CHECK in 0003 generated.
ALTER TABLE reader_article_state DROP CONSTRAINT reader_article_state_state_check;
ALTER TABLE reader_article_state ADD CONSTRAINT reader_article_state_state_check
    CHECK (state IN ('finished', 'skipped'));

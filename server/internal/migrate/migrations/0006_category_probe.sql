-- Random walks step from an article to another member of one of its
-- categories; the member pick is a seeded random-id probe, one B-tree
-- descent per step, which needs the category index to carry article_id.
-- The composite strictly supersedes the single-column index (same prefix),
-- so the old one goes.
DROP INDEX article_categories_category;
CREATE INDEX article_categories_category ON article_categories (category, article_id);

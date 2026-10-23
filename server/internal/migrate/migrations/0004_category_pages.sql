-- Wiki category metadata, fetched from the wiki's own API by the
-- fetch-categories command. The wiki already separates content categories
-- from maintenance ones: tracking categories ("CS1 German-language sources",
-- "Pages needing to be simplified from July 2024") are marked hidden on
-- their category page, and most of the rest ("Articles with VIAF
-- identifiers") never had a category page created at all. So the rule is
-- the wiki's, not ours: a category counts for recommendations only when its
-- page exists and is not hidden. Ingest consults this table when storing
-- article categories; an empty table (no fetch yet for that source) keeps
-- everything, matching the old behavior.
CREATE TABLE category_pages (
    source text NOT NULL,
    category text NOT NULL,
    hidden boolean NOT NULL,
    PRIMARY KEY (source, category)
);

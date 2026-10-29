-- Lead image and short description: presentation metadata for feed cards
-- and article headers. All nullable — many pages have no free lead image
-- and some have no description. Metadata is not addressed text: refreshing
-- these columns on an already-stored revision is safe, unlike sentences,
-- which are immutable once stored.
ALTER TABLE articles ADD COLUMN image_url text;
ALTER TABLE articles ADD COLUMN image_width integer;
ALTER TABLE articles ADD COLUMN image_height integer;
ALTER TABLE articles ADD COLUMN description text;

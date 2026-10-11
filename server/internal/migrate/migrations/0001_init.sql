-- Bootstrap migration: proves the migration pipeline end to end.
-- Domain tables (articles, sentences, tokens, events) land with the
-- ingest work; nothing is created ahead of the issue that needs it.
SELECT 1;

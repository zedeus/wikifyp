-- Reading events: append-only, deduplicated by client UUID, partitioned by
-- month. There is no UPDATE or DELETE path; events are immutable once stored.
-- reader_id is added server-side on upload, not by the client.

CREATE TABLE reading_events (
    id text NOT NULL,
    reader_id text NOT NULL,
    schema_version integer NOT NULL,
    session_id text NOT NULL,
    type text NOT NULL,
    timestamp bigint NOT NULL,
    article_id bigint NOT NULL,
    sentence_idx integer NOT NULL,
    token_idx integer NOT NULL,
    wpm integer NOT NULL,
    landed_article_id bigint,
    landed_sentence_idx integer,
    landed_token_idx integer,
    distance integer,
    dwell_ms bigint,
    ingested_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

-- Default partition catches everything until month-specific ones exist.
-- The application or a cron creates monthly partitions ahead of time;
-- the default keeps inserts from failing if one is missed.
CREATE TABLE reading_events_default PARTITION OF reading_events DEFAULT;

-- Feed-state side effects need to look up which events a reader sent
-- for an article; the batch endpoint needs to skip already-stored IDs.
CREATE INDEX reading_events_reader_article ON reading_events (reader_id, article_id);

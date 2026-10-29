# Reading events (v1)

A reading event is a single thing that happened during a reading session:
the reader started playing, paused, scrolled back, asked the companion,
finished, skipped, saved or unsaved the article. Events are the raw material
for friction analysis and profile building.

## Rules

- **Append-only.** Events are never updated or deleted.
- **Versioned.** Every event carries `schema_version`. Consumers must reject
  versions they do not understand (the Go server returns a typed error for
  unknown versions; the reader only emits v1).
- **Deduplicated by `id`.** Each event has a client-generated UUID. The
  server accepts duplicates silently (idempotent upsert).
- **`reader_id` is server-side.** The reader emits events without a reader
  ID; the upload layer adds a pseudonymous reader ID on the server.

## Fields

| Field            | Type   | Required | Description |
|------------------|--------|----------|-------------|
| `id`             | string | yes | Client UUID (e.g. `UUID.randomUUID()`). Deduplication key. |
| `schema_version` | int    | yes | Always `1` for this version. |
| `session_id`     | string | yes | UUID for the reading session (one per article open). |
| `type`           | string | yes | One of: `play`, `pause`, `scroll_back`, `ask`, `finish`, `skip`, `save`, `unsave`. |
| `timestamp`      | long   | yes | Epoch milliseconds from the reader's clock. |
| `token`          | object | yes | Token address where the event occurred: `{article_id, sentence_idx, token_idx}`. |
| `wpm`            | int    | yes | Words per minute at the time of the event. |
| `landed_token`   | object | `scroll_back` only | Token address where the drag landed. |
| `distance`       | int    | `scroll_back` only | Number of tokens scrolled back (positive). |
| `dwell_ms`       | long   | `scroll_back` only | Milliseconds the reader stayed at the landed position. |

### Token address

```json
{
    "article_id": 42,
    "sentence_idx": 0,
    "token_idx": 3
}
```

`article_id` identifies a specific article revision (one row in the articles
table). `sentence_idx` and `token_idx` are zero-based. A token is one
whitespace-delimited chunk of the sentence's stored text (split on single
space), and its index is stable forever.

## Event types

- **`play`** — playback started (hold gesture passed the threshold) or
  resumed after a pause or drag.
- **`pause`** — the reader lifted their finger; playback stopped on the
  current word.
- **`scroll_back`** — the reader dragged to an earlier position. Includes
  the starting token (`token`), landing token (`landed_token`), distance in
  tokens and dwell time.
- **`ask`** — the reader double-tapped to ask the companion about the
  current word/sentence.
- **`finish`** — playback reached the end of the article.
- **`skip`** — the reader skipped the article without finishing.
- **`save`** — the reader saved the article to read later. The server shelves
  it: saved articles leave the feed but still feed the interest profile. The
  bookmark is a flag of its own, so a later finish or skip does not remove it.
- **`unsave`** — the reader removed the bookmark.

## Examples

Valid examples are in `fixtures/events/`:

- `play.json` — a play event at the start of an article
- `pause.json` — a pause mid-sentence
- `scroll_back.json` — a scroll back with landing, distance and dwell
- `ask.json` — a double-tap ask
- `finish.json` — article finished at a different WPM
- `skip.json` — article skipped early
- `save.json` — article saved for later
- `unsave.json` — the bookmark removed again

Invalid examples (for testing decoders):

- `invalid-missing-id.json` — missing the required `id` field
- `invalid-unknown-type.json` — unrecognized event type `"teleport"`
- `invalid-v0.json` — `schema_version` 0 (rejected by the server)

## Versioning

When the event format changes, `schema_version` increments. The rules:

1. New optional fields may be added without a version bump (unknown fields
   are ignored by decoders configured with `ignoreUnknownKeys`).
2. Removing a required field, changing a field's type, or changing the
   meaning of an existing field requires a version bump.
3. The Go server rejects events with unknown schema versions per-event
   without failing the rest of a batch upload.
4. The reader always emits the current version (1).

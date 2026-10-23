// Package event decodes, validates and stores reading events.
//
// Events are append-only and versioned. The server accepts them in
// batches, deduplicates by ID, and rejects unknown schema versions
// per-event without failing the rest of the batch.
package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// currentSchemaVersion is the only version the server accepts. The
// client's schema_version must equal this; unknown versions get a typed
// error so the batch handler can report them per-event.
const currentSchemaVersion = 1

// Event is a single reading event as the client sends it. Unknown JSON
// fields are tolerated (forward compat); unknown schema versions are
// rejected with ErrUnknownSchema.
type Event struct {
	ID            string        `json:"id"`
	SchemaVersion int           `json:"schema_version"`
	SessionID     string        `json:"session_id"`
	Type          string        `json:"type"`
	Timestamp     int64         `json:"timestamp"`
	Token         TokenAddress  `json:"token"`
	WPM           int           `json:"wpm"`
	LandedToken   *TokenAddress `json:"landed_token,omitempty"`
	Distance      *int          `json:"distance,omitempty"`
	DwellMs       *int64        `json:"dwell_ms,omitempty"`
}

// TokenAddress is a stable reference to one token in an article.
type TokenAddress struct {
	ArticleID   int64 `json:"article_id"`
	SentenceIdx int   `json:"sentence_idx"`
	TokenIdx    int   `json:"token_idx"`
}

// ErrUnknownSchema is returned when an event's schema_version is not
// one the server understands.
var ErrUnknownSchema = errors.New("unknown schema version")

// validTypes are the event types the server recognises.
var validTypes = map[string]bool{
	"play":        true,
	"pause":       true,
	"scroll_back": true,
	"ask":         true,
	"finish":      true,
	"skip":        true,
}

// Decode parses a single event from JSON, tolerating unknown fields but
// rejecting missing required fields and unknown schema versions.
func Decode(data []byte) (Event, error) {
	var e Event
	if err := json.Unmarshal(data, &e); err != nil {
		return Event{}, fmt.Errorf("event: decode: %w", err)
	}
	return e, e.validate()
}

// DecodeSlice parses a JSON array of events, returning per-event
// results. Each element is decoded independently: a bad event does not
// prevent valid siblings from being returned.
func DecodeSlice(data []byte) ([]Event, []error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, []error{fmt.Errorf("event: decode batch: %w", err)}
	}
	events := make([]Event, 0, len(raw))
	errs := make([]error, 0)
	for i, r := range raw {
		e, err := Decode(r)
		if err != nil {
			errs = append(errs, fmt.Errorf("event[%d]: %w", i, err))
			continue
		}
		events = append(events, e)
	}
	return events, errs
}

func (e Event) validate() error {
	if e.ID == "" {
		return errors.New("event: missing id")
	}
	if e.SchemaVersion != currentSchemaVersion {
		return fmt.Errorf("%w: %d", ErrUnknownSchema, e.SchemaVersion)
	}
	if e.SessionID == "" {
		return errors.New("event: missing session_id")
	}
	if !validTypes[e.Type] {
		return fmt.Errorf("event: unknown type %q", e.Type)
	}
	if e.Timestamp <= 0 {
		return errors.New("event: missing or invalid timestamp")
	}
	if e.WPM <= 0 {
		return errors.New("event: missing or invalid wpm")
	}
	if e.Token.ArticleID <= 0 {
		return errors.New("event: token.article_id must be positive")
	}
	if e.Token.SentenceIdx < 0 {
		return errors.New("event: token.sentence_idx must be non-negative")
	}
	if e.Token.TokenIdx < 0 {
		return errors.New("event: token.token_idx must be non-negative")
	}
	if e.Type == "scroll_back" {
		if e.LandedToken == nil {
			return errors.New("event: scroll_back requires landed_token")
		}
		if e.Distance == nil {
			return errors.New("event: scroll_back requires distance")
		}
		if e.DwellMs == nil {
			return errors.New("event: scroll_back requires dwell_ms")
		}
	}
	return nil
}

// IsFinishOrSkip reports whether the event type signals the reader is
// done with an article, requiring a feed state update.
func (e Event) IsFinishOrSkip() bool {
	return e.Type == "finish" || e.Type == "skip"
}

// FeedState returns the reader_article_state value for a finish or skip
// event: "finished" or "skipped". It panics for other types.
func (e Event) FeedState() string {
	switch e.Type {
	case "finish":
		return "finished"
	case "skip":
		return "skipped"
	default:
		panic(fmt.Sprintf("event: FeedState called on %q", e.Type))
	}
}

// TokenCount returns the number of tokens in a sentence by splitting
// on single spaces (the same derivation rule as the tokenizer; stored
// text uses single-space separators only).
func TokenCount(sentenceText string) int {
	if sentenceText == "" {
		return 0
	}
	return len(strings.Split(sentenceText, " "))
}

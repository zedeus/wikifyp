package event

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureDir() string {
	// Tests run with the working directory set to their package; the
	// fixtures live three levels up at the repo root.
	return filepath.Join("..", "..", "..", "fixtures", "events")
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir(), name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func TestDecodeValidFixtures(t *testing.T) {
	cases := []struct {
		file     string
		wantType string
	}{
		{"play.json", "play"},
		{"pause.json", "pause"},
		{"scroll_back.json", "scroll_back"},
		{"ask.json", "ask"},
		{"finish.json", "finish"},
		{"skip.json", "skip"},
		{"save.json", "save"},
		{"unsave.json", "unsave"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			e, err := Decode(readFixture(t, tc.file))
			if err != nil {
				t.Fatalf("Decode(%s): %v", tc.file, err)
			}
			if e.Type != tc.wantType {
				t.Errorf("type = %q, want %q", e.Type, tc.wantType)
			}
			if e.ID == "" {
				t.Error("id is empty")
			}
			if e.SchemaVersion != 1 {
				t.Errorf("schema_version = %d, want 1", e.SchemaVersion)
			}
			if e.Token.ArticleID != 42 {
				t.Errorf("token.article_id = %d, want 42", e.Token.ArticleID)
			}
		})
	}
}

func TestDecodeScrollBackFields(t *testing.T) {
	e, err := Decode(readFixture(t, "scroll_back.json"))
	if err != nil {
		t.Fatal(err)
	}
	if e.LandedToken == nil {
		t.Fatal("landed_token is nil")
	}
	if e.LandedToken.SentenceIdx != 0 || e.LandedToken.TokenIdx != 2 {
		t.Errorf("landed_token = %+v, want (0, 2)", e.LandedToken)
	}
	if e.Distance == nil || *e.Distance != 6 {
		t.Errorf("distance = %v, want 6", e.Distance)
	}
	if e.DwellMs == nil || *e.DwellMs != 1800 {
		t.Errorf("dwell_ms = %v, want 1800", e.DwellMs)
	}
}

func TestDecodeInvalidFixtures(t *testing.T) {
	cases := []struct {
		file    string
		wantErr string
	}{
		{"invalid-missing-id.json", "missing id"},
		{"invalid-unknown-type.json", "unknown type"},
		{"invalid-v0.json", "unknown schema version"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			_, err := Decode(readFixture(t, tc.file))
			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantErr)
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Errorf("error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func TestDecodeUnknownSchemaIsTyped(t *testing.T) {
	_, err := Decode(readFixture(t, "invalid-v0.json"))
	if !errors.Is(err, ErrUnknownSchema) {
		t.Errorf("error = %v, want errors.Is(err, ErrUnknownSchema)", err)
	}
}

func TestDecodeToleratesUnknownFields(t *testing.T) {
	data := `{
		"id": "test-1",
		"schema_version": 1,
		"session_id": "sess-1",
		"type": "play",
		"timestamp": 1000000,
		"token": {"article_id": 1, "sentence_idx": 0, "token_idx": 0},
		"wpm": 250,
		"future_field": "should be ignored"
	}`
	e, err := Decode([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.ID != "test-1" {
		t.Errorf("id = %q, want %q", e.ID, "test-1")
	}
}

func TestDecodeSliceMixedValidity(t *testing.T) {
	batch := `[
		{"id":"ok-1","schema_version":1,"session_id":"s","type":"play","timestamp":1000,"token":{"article_id":1,"sentence_idx":0,"token_idx":0},"wpm":250},
		{"schema_version":1,"session_id":"s","type":"play","timestamp":1000,"token":{"article_id":1,"sentence_idx":0,"token_idx":0},"wpm":250},
		{"id":"ok-2","schema_version":1,"session_id":"s","type":"pause","timestamp":1001,"token":{"article_id":1,"sentence_idx":0,"token_idx":3},"wpm":250}
	]`
	events, errs := DecodeSlice([]byte(batch))
	if len(events) != 2 {
		t.Errorf("got %d valid events, want 2", len(events))
	}
	if len(errs) != 1 {
		t.Errorf("got %d errors, want 1", len(errs))
	}
}

func TestFeedState(t *testing.T) {
	cases := []struct {
		file      string
		wantState string
		wantOK    bool
	}{
		{"finish.json", "finished", true},
		{"skip.json", "skipped", true},
		{"save.json", "", false},
		{"unsave.json", "", false},
		{"play.json", "", false},
		{"pause.json", "", false},
		{"scroll_back.json", "", false},
		{"ask.json", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			e, err := Decode(readFixture(t, tc.file))
			if err != nil {
				t.Fatal(err)
			}
			state, ok := e.FeedState()
			if state != tc.wantState || ok != tc.wantOK {
				t.Errorf("FeedState() = (%q, %v), want (%q, %v)", state, ok, tc.wantState, tc.wantOK)
			}
		})
	}
}

// TestSavedChange: save and unsave drive the saved flag; nothing else
// touches it.
func TestSavedChange(t *testing.T) {
	cases := []struct {
		file      string
		wantSaved bool
		wantOK    bool
	}{
		{"save.json", true, true},
		{"unsave.json", false, true},
		{"finish.json", false, false},
		{"skip.json", false, false},
		{"play.json", false, false},
		{"pause.json", false, false},
		{"scroll_back.json", false, false},
		{"ask.json", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			e, err := Decode(readFixture(t, tc.file))
			if err != nil {
				t.Fatal(err)
			}
			saved, ok := e.SavedChange()
			if saved != tc.wantSaved || ok != tc.wantOK {
				t.Errorf("SavedChange() = (%v, %v), want (%v, %v)", saved, ok, tc.wantSaved, tc.wantOK)
			}
		})
	}
}

func TestTokenCount(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"one two three four five", 5},
	}
	for _, tc := range cases {
		if got := TokenCount(tc.text); got != tc.want {
			t.Errorf("TokenCount(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}

package text

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSharedTokenFixture validates that the Go tokenization (Normalize then
// split on " ") agrees with the shared fixture at fixtures/tokens.json.
// The same fixture is tested on the Kotlin side in the :reader module, so
// the two platforms can never disagree on token addressing.
func TestSharedTokenFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "fixtures", "tokens.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var cases []struct {
		Comment string   `json:"comment"`
		Text    string   `json:"text"`
		Tokens  []string `json:"tokens"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("fixture is empty")
	}

	for _, tc := range cases {
		t.Run(tc.Comment, func(t *testing.T) {
			normalized := Normalize(tc.Text)
			got := strings.Split(normalized, " ")
			if len(got) != len(tc.Tokens) {
				t.Fatalf("token count: got %d, want %d\ngot:  %q\nwant: %q",
					len(got), len(tc.Tokens), got, tc.Tokens)
			}
			for i := range got {
				if got[i] != tc.Tokens[i] {
					t.Errorf("token[%d]: got %q, want %q", i, got[i], tc.Tokens[i])
				}
			}
		})
	}
}

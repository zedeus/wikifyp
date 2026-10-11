package text

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture reads a committed article fixture from the repo's fixtures/ tree.
func fixture(t testing.TB, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "fixtures", "articles", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(data)
}

// TestSplitWikipediaGroundTruth compares Split against hand-segmented ground
// truth for the opening sections of three large real Wikipedia articles
// (testdata/wikipedia/*.sentences.txt, one sentence per line, whitespace
// normalized). The ground truth was written by reading the text, not by
// running the tokenizer; disagreements are tokenizer defects unless a human
// decides the ground truth is wrong.
func TestSplitWikipediaGroundTruth(t *testing.T) {
	tests := []struct {
		fixture string
		lines   int // how many lines of the fixture the ground truth covers
		golden  string
	}{
		{fixture: "barack-obama.txt", lines: 10, golden: "obama.sentences.txt"},
		{fixture: "chemistry.txt", lines: 9, golden: "chemistry.sentences.txt"},
		{fixture: "computer.txt", lines: 10, golden: "computer.sentences.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			article := fixture(t, tt.fixture)
			input := strings.Join(strings.SplitN(article, "\n", tt.lines+1)[:tt.lines], "\n")

			data, err := os.ReadFile(filepath.Join("testdata", "wikipedia", tt.golden))
			if err != nil {
				t.Fatal(err)
			}
			want := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

			var got []string
			for _, s := range Split(input) {
				got = append(got, s.Text)
			}

			for i := 0; i < len(got) || i < len(want); i++ {
				switch {
				case i >= len(got):
					t.Errorf("missing sentence %d:\nwant %q", i, want[i])
				case i >= len(want):
					t.Errorf("extra sentence %d:\n got %q", i, got[i])
				case got[i] != want[i]:
					t.Errorf("sentence %d:\n got %q\nwant %q", i, got[i], want[i])
				}
				if t.Failed() && i > 6 {
					t.Fatal("stopping after first disagreements; later indices are shifted")
				}
			}
			checkInvariants(t, input)
		})
	}
}

// TestSplitWikipediaInvariants runs the structural invariants over the three
// full articles (~175 KB of real text): determinism, token rejoin, and no
// character loss.
func TestSplitWikipediaInvariants(t *testing.T) {
	for _, name := range []string{"barack-obama.txt", "chemistry.txt", "computer.txt"} {
		t.Run(name, func(t *testing.T) {
			checkInvariants(t, fixture(t, name))
		})
	}
}

func BenchmarkSplit(b *testing.B) {
	article := fixture(b, "barack-obama.txt")
	b.SetBytes(int64(len(article)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Split(article)
	}
}

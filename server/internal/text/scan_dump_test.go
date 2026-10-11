package text

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode"
)

// TestDumpAnomalies is a manual inspection aid, not an assertion. Run with
// WIKIFYP_SCAN=1 to print suspicious sentences from the full fixtures.
func TestDumpAnomalies(t *testing.T) {
	if os.Getenv("WIKIFYP_SCAN") == "" {
		t.Skip("set WIKIFYP_SCAN=1 to dump")
	}
	for _, f := range []string{"barack-obama.txt", "chemistry.txt", "computer.txt"} {
		ss := Split(fixture(t, f))
		fmt.Printf("== %s: %d sentences\n", f, len(ss))
		for i, s := range ss {
			r := []rune(s.Text)[0]
			prev := ""
			if i > 0 {
				prev = ss[i-1].Text
				if r := []rune(prev); len(r) > 70 {
					prev = "…" + string(r[len(r)-70:])
				}
			}
			switch {
			case unicode.IsLower(r):
				fmt.Printf("LOWER-START %d: %.80q  (prev: %q)\n", i, s.Text, prev)
			case len(s.Tokens) == 1 && !strings.HasPrefix(s.Text, "=="):
				fmt.Printf("ONE-TOKEN   %d: %q (prev: %q)\n", i, s.Text, prev)
			case strings.Contains(s.Text, "==") && !strings.HasPrefix(s.Text, "=="):
				fmt.Printf("MID-HEADING %d: %.100q\n", i, s.Text)
			case len(s.Tokens) > 120:
				fmt.Printf("HUGE        %d: %d tokens: %.90q\n", i, len(s.Tokens), s.Text)
			}
		}
	}
}

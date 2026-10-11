package text

import (
	"math/rand"
	"strings"
	"testing"
)

// TestSplitProperties feeds generated article-like text through Split and
// checks the invariants that must hold for any input: splitting twice gives
// identical sentences, tokens and addresses; rejoining a sentence's tokens
// with spaces gives back the sentence; and no non-whitespace character is
// lost or altered.
func TestSplitProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 500; i++ {
		checkInvariants(t, randomText(rng))
		if t.Failed() {
			t.Fatalf("failed on iteration %d", i)
		}
	}
}

// TestSplitPropertiesRawRunes throws unstructured rune soup at Split; the
// invariants must survive even non-language input.
func TestSplitPropertiesRawRunes(t *testing.T) {
	alphabet := []rune("ab .!?…\"'()[]\n\t1Ωé🙂…")
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 500; i++ {
		var b strings.Builder
		for n := rng.Intn(60); n > 0; n-- {
			b.WriteRune(alphabet[rng.Intn(len(alphabet))])
		}
		checkInvariants(t, b.String())
		if t.Failed() {
			t.Fatalf("failed on iteration %d", i)
		}
	}
}

var (
	randWords = []string{
		"the", "a", "reader", "sentence", "Wikipedia", "token", "pauses",
		"Dr.", "Mr.", "J.", "etc.", "e.g.", "i.e.", "No.", "vs.", "al.",
		"3.14", "1947", "100", "$4.50", "(aside)", `"quoted"`, "word,",
		"long-hyphenated-word", "it's", "…", "one",
	}
	randEnders = []string{".", "!", "?", "...", "…", ".\"", ".)", "?!"}
)

// randomText builds a few pseudo-sentences from vocabulary that includes the
// tricky cases: abbreviations, decimals, quotes, parentheses, ellipses.
func randomText(rng *rand.Rand) string {
	var b strings.Builder
	for s := 1 + rng.Intn(5); s > 0; s-- {
		for w := 1 + rng.Intn(8); w > 0; w-- {
			b.WriteString(randWords[rng.Intn(len(randWords))])
			if w > 1 {
				b.WriteByte(' ')
			}
		}
		b.WriteString(randEnders[rng.Intn(len(randEnders))])
		if s > 1 {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

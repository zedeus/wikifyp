package text

import (
	"reflect"
	"strings"
	"testing"
	"unicode"
)

// sentenceTexts flattens Split output for compact table tests.
func sentenceTexts(input string) []string {
	var out []string
	for _, s := range Split(input) {
		out = append(out, s.Text)
	}
	return out
}

func TestSplitSentences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "plain sentences",
			input: "The sky is blue. Water is wet! Is it raining?",
			want:  []string{"The sky is blue.", "Water is wet!", "Is it raining?"},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "whitespace only",
			input: " \n\t ",
			want:  nil,
		},
		{
			name:  "no terminator",
			input: "a fragment without an ending",
			want:  []string{"a fragment without an ending"},
		},
		{
			name:  "title abbreviation",
			input: "Dr. Smith arrived late. Mr. Jones had left.",
			want:  []string{"Dr. Smith arrived late.", "Mr. Jones had left."},
		},
		{
			name:  "abbreviation before capital",
			input: "See Fig. 3 in Dept. B for details.",
			want:  []string{"See Fig. 3 in Dept. B for details."},
		},
		{
			name:  "latin abbreviations",
			input: "Citrus fruit (e.g. oranges) is sour, i.e. acidic.",
			want:  []string{"Citrus fruit (e.g. oranges) is sour, i.e. acidic."},
		},
		{
			name:  "etc ends a sentence before a capital",
			input: "He packed shirts, socks, etc. The train left at nine.",
			want:  []string{"He packed shirts, socks, etc.", "The train left at nine."},
		},
		{
			name:  "etc runs on before lowercase",
			input: "Shirts, socks, etc. were packed.",
			want:  []string{"Shirts, socks, etc. were packed."},
		},
		{
			name:  "decimals",
			input: "Pi is about 3.14159 in value. The ratio is 1.5 to 2.",
			want:  []string{"Pi is about 3.14159 in value.", "The ratio is 1.5 to 2."},
		},
		{
			name:  "sentence ending in a number",
			input: "The war ended in 1945. Rebuilding began at once.",
			want:  []string{"The war ended in 1945.", "Rebuilding began at once."},
		},
		{
			name:  "initials",
			input: "J. R. R. Tolkien wrote novels. C. S. Lewis did too.",
			want:  []string{"J. R. R. Tolkien wrote novels.", "C. S. Lewis did too."},
		},
		{
			name:  "quoted speech",
			input: `He said, "Stop right there." Then he left.`,
			want:  []string{`He said, "Stop right there."`, "Then he left."},
		},
		{
			name:  "question inside quotes",
			input: `"Where is it?" She pointed north.`,
			want:  []string{`"Where is it?"`, "She pointed north."},
		},
		{
			name:  "quote continues in lowercase",
			input: `"Wait." he whispered.`,
			want:  []string{`"Wait." he whispered.`},
		},
		{
			name:  "parenthetical ending",
			input: "He left quietly (without a word). She stayed behind.",
			want:  []string{"He left quietly (without a word).", "She stayed behind."},
		},
		{
			name:  "full sentence in parentheses",
			input: "He left. (She stayed behind.) Nobody minded.",
			want:  []string{"He left.", "(She stayed behind.)", "Nobody minded."},
		},
		{
			name:  "ellipsis runs on",
			input: "He waited... then knocked again.",
			want:  []string{"He waited... then knocked again."},
		},
		{
			name:  "ellipsis ends a sentence",
			input: "The sound faded... Then came silence.",
			want:  []string{"The sound faded...", "Then came silence."},
		},
		{
			name:  "unicode ellipsis",
			input: "The sound faded… Then came silence.",
			want:  []string{"The sound faded…", "Then came silence."},
		},
		{
			name:  "interrobang run",
			input: "You did what?! Nobody told me.",
			want:  []string{"You did what?!", "Nobody told me."},
		},
		{
			name:  "numbered list with periods",
			input: "1. Break the eggs. 2. Whisk them well.",
			want:  []string{"1. Break the eggs.", "2. Whisk them well."},
		},
		{
			name:  "numbered list on separate lines",
			input: "1. First boil the water\n2. Then add the rice",
			want:  []string{"1. First boil the water", "2. Then add the rice"},
		},
		{
			// In extract-format text a newline is always a structural break
			// (paragraph, heading, list item, table row), never a soft wrap.
			name:  "each line stands alone",
			input: "The quick\nbrown fox\njumps. It lands.",
			want:  []string{"The quick", "brown fox", "jumps.", "It lands."},
		},
		{
			name:  "heading line stands alone",
			input: "He retired in 2017.\n\n\n== Early life ==\n\nHe was born in 1961.",
			want:  []string{"He retired in 2017.", "== Early life ==", "He was born in 1961."},
		},
		{
			name:  "unpunctuated list lines stand alone",
			input: "Memory comes in two varieties:\nrandom-access memory or RAM\nread-only memory or ROM",
			want:  []string{"Memory comes in two varieties:", "random-access memory or RAM", "read-only memory or ROM"},
		},
		{
			name:  "citation abbreviation before a number",
			input: "Encyclopædia Britannica. Vol. 5. pp. 23–24.",
			want:  []string{"Encyclopædia Britannica.", "Vol. 5. pp. 23–24."},
		},
		{
			name:  "unlisted abbreviation before a number",
			input: "The route is approx. 40,000 km long.",
			want:  []string{"The route is approx. 40,000 km long."},
		},
		{
			name:  "capitalized Vol before a non-digit",
			input: "See Vol. V of the series.",
			want:  []string{"See Vol. V of the series."},
		},
		{
			name:  "parenthesized list numbering",
			input: "(1. First item) follows.",
			want:  []string{"(1. First item) follows."},
		},
		{
			name:  "digits start a sentence after a question mark",
			input: "Was it worth it? 50 years later, nobody knew.",
			want:  []string{"Was it worth it?", "50 years later, nobody knew."},
		},
		{
			name:  "digits start a sentence after an exclamation mark",
			input: "It erupted! 57 villages were buried.",
			want:  []string{"It erupted!", "57 villages were buried."},
		},
		{
			name:  "number citation like No.",
			input: "See No. 7 for the full score.",
			want:  []string{"See No. 7 for the full score."},
		},
		{
			name:  "et al.",
			input: "The study by Chen et al. found no effect.",
			want:  []string{"The study by Chen et al. found no effect."},
		},
		{
			name:  "abbreviation after opening punctuation",
			input: "The man (Dr. Smith) left early. She followed.",
			want:  []string{"The man (Dr. Smith) left early.", "She followed."},
		},
		{
			name:  "quoted abbreviation",
			input: `"Mr. Smith is here." He nodded.`,
			want:  []string{`"Mr. Smith is here."`, "He nodded."},
		},
		{
			name:  "opening mark separated from the next word by a space",
			input: "He left. ( Her idea was better.) Nobody minded.",
			want:  []string{"He left.", "( Her idea was better.)", "Nobody minded."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sentenceTexts(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Split(%q)\n got %q\nwant %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitTokens(t *testing.T) {
	got := Split(`She paid $4.50 (half price!) for "The Hobbit".`)
	if len(got) != 1 {
		t.Fatalf("got %d sentences, want 1", len(got))
	}
	want := []string{"She", "paid", "$4.50", "(half", "price!)", "for", `"The`, `Hobbit".`}
	var texts []string
	for _, tok := range got[0].Tokens {
		texts = append(texts, tok.Text)
	}
	if !reflect.DeepEqual(texts, want) {
		t.Errorf("tokens = %q, want %q", texts, want)
	}
	for i, tok := range got[0].Tokens {
		if tok.Idx != i {
			t.Errorf("token %d has Idx %d", i, tok.Idx)
		}
	}
}

func TestSplitIndices(t *testing.T) {
	sentences := Split("One two. Three four five. Six.")
	if len(sentences) != 3 {
		t.Fatalf("got %d sentences, want 3", len(sentences))
	}
	for i, s := range sentences {
		if s.Idx != i {
			t.Errorf("sentence %d has Idx %d", i, s.Idx)
		}
	}
}

// hostile inputs must not panic and must obey the invariants checked by
// checkInvariants.
func TestSplitHostileInputs(t *testing.T) {
	inputs := []string{
		"",
		".",
		"...",
		"…",
		"?!?!",
		". . . .",
		"a.",
		".a",
		"\n\n\n",
		"1.",
		"1. ",
		"word",
		"🙂. 🙃!",
		"عربى. نص آخر.", // RTL text
		"e.g.",
		"((((never closed. Another one.",
		"\"",
		strings.Repeat("a", 10000) + ".",
		strings.Repeat("Very long. ", 500),
		"tab\tseparated\ttokens. second one.",
		"mixed   runs \t of whitespace. done.",
	}
	for _, in := range inputs {
		checkInvariants(t, in)
	}
}

// checkInvariants verifies the properties that must hold for any input:
// determinism, exact rejoining, and losing nothing but whitespace.
func checkInvariants(t *testing.T, input string) {
	t.Helper()
	first := Split(input)
	second := Split(input)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("Split(%q) is not deterministic", input)
	}
	var joined, original strings.Builder
	for _, s := range first {
		var texts []string
		for _, tok := range s.Tokens {
			texts = append(texts, tok.Text)
			joined.WriteString(tok.Text)
		}
		if rejoined := strings.Join(texts, " "); rejoined != s.Text {
			t.Errorf("Split(%q): rejoined tokens %q != sentence %q", input, rejoined, s.Text)
		}
	}
	for _, r := range input {
		if !unicode.IsSpace(r) {
			original.WriteRune(r)
		}
	}
	if joined.String() != original.String() {
		t.Errorf("Split(%q) lost or altered characters:\n got %q\nwant %q",
			input, joined.String(), original.String())
	}
}

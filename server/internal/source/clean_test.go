package source

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// rawExtract pulls the plain-text extract out of a recorded API response in
// testdata. The files are raw API samples saved by the ingest command's
// development, byte for byte.
func rawExtract(t testing.TB, file string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	var resp apiResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Query.Pages) == 0 || resp.Query.Pages[0].Extract == "" {
		t.Fatalf("%s: no extract", file)
	}
	return resp.Query.Pages[0].Extract
}

// The expectations below are ground truth authored by reading the raw
// samples, not by running Clean: each quoted string was derived by hand
// from the fragment lines visible in the raw extract.

func TestCleanExponentiation(t *testing.T) {
	cleaned := Clean(rawExtract(t, "exponentiation.json"))

	// Inline math spliced back into its sentence: the raw sample fragments
	// "x × y", "x y" and "x ⋅ y" across indented runs between the prose
	// pieces "multiplication, ", ", ", ", and " and ", interchangeably.".
	wantContains := []string{
		"This article uses the three common notations for multiplication, x × y, x y, and x ⋅ y, interchangeably.",
		// "b ↑ n" with its continuation " or b^n." inside the run.
		"Sometimes an up arrow or caret is used as the operator, e.g. b ↑ n or b^n.",
		// Two formulas with prose between them, all inside one indented run.
		"multiplied, f ∘ n denotes exponentiation with respect to function composition, whereas f n may denote exponentiation with respect to multiplication. That is,",
		// Section headings survive.
		"== Notation ==",
		"== History ==",
		// Last prose line of the last kept section.
		"For instance, in 1961 the School Mathematics Study Group developed the notation in connection with units used in the metric system.",
	}
	for _, want := range wantContains {
		if !strings.Contains(cleaned, want) {
			t.Errorf("cleaned text missing %q", want)
		}
	}
	for _, gone := range []string{`\displaystyle`, `\textstyle`, "== See also ==", "== Notes ==", "== References =="} {
		if strings.Contains(cleaned, gone) {
			t.Errorf("cleaned text still contains %q", gone)
		}
	}
	if !strings.HasPrefix(cleaned, "In mathematics, exponentiation, denoted bn, is an operation involving two numbers") {
		t.Errorf("first line altered: %q", firstLine(cleaned))
	}
}

func TestCleanPhotosynthesis(t *testing.T) {
	cleaned := Clean(rawExtract(t, "photosynthesis-1.json"))

	if !strings.HasPrefix(cleaned, "Photosynthesis is a system of biological processes") {
		t.Errorf("first line altered: %q", firstLine(cleaned))
	}
	// The sample's trailing apparatus is a chain of (empty) sections:
	// See also, Notes, References, Further reading with Books and Papers
	// subsections, External links. All of it goes, subsections included.
	for _, gone := range []string{"== See also ==", "== Further reading ==", "=== Books ===", "=== Papers ===", "== External links =="} {
		if strings.Contains(cleaned, gone) {
			t.Errorf("cleaned text still contains %q", gone)
		}
	}
	want := "since it is characterized by light-dependent oxygen consumption and the release of carbon dioxide."
	if !strings.HasSuffix(strings.TrimRight(cleaned, "\n"), want) {
		t.Errorf("cleaned text does not end with the last prose line; got %q", lastLine(cleaned))
	}
}

// TestCleanChemistryFixture runs Clean over a committed raw extract fixture:
// its References section carries a real bibliography (unlike the photosynthesis
// sample, where the apparatus sections are empty) and its math runs sit
// mid-paragraph with prose on both sides.
func TestCleanChemistryFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "fixtures", "articles", "chemistry.txt"))
	if err != nil {
		t.Fatal(err)
	}
	cleaned := Clean(string(data))

	wantContains := []string{
		// "e − E / k T" fragments plus the " – that is..." continuation
		// inside the run.
		"by the Boltzmann's population factor e − E / k T – that is the probability of a molecule",
		// "Δ G ≤ 0" with a "; if..." continuation line after the run:
		// closing punctuation splices without a space.
		"the Gibbs free energy is negative, Δ G ≤ 0; if it is equal to zero the chemical reaction is said to be at equilibrium.",
		"== Etymology ==",
		"=== Professional societies ===",
	}
	for _, want := range wantContains {
		if !strings.Contains(cleaned, want) {
			t.Errorf("cleaned text missing %q", want)
		}
	}
	for _, gone := range []string{`\displaystyle`, "== References ==", "Atkins, P.W., Overton", "== External links =="} {
		if strings.Contains(cleaned, gone) {
			t.Errorf("cleaned text still contains %q", gone)
		}
	}
}

func TestClean(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "plain text untouched", in: "One.\nTwo.\n", want: "One.\nTwo.\n"},
		{
			name: "trailing meta sections removed",
			in:   "Prose.\n\n== History ==\n\nMore prose.\n\n== See also ==\n\nA link list.\n\n== References ==\n\nSome citation.\n",
			want: "Prose.\n\n== History ==\n\nMore prose.\n\n",
		},
		{
			name: "meta section in the middle ends at the next same-level heading",
			in:   "== Notes ==\n\nNote text.\n\n== Legacy ==\n\nKept.\n",
			want: "== Legacy ==\n\nKept.\n",
		},
		{
			name: "meta subsections go with their section",
			in:   "Prose.\n\n== Further reading ==\n\n=== Books ===\n\nA book.\n\n=== Papers ===\n\nA paper.\n",
			want: "Prose.\n\n",
		},
		{
			name: "meta-named subsection inside a kept section",
			in:   "== Reception ==\n\nKept.\n\n=== Notes ===\n\nGone.\n\n== Legacy ==\n\nAlso kept.\n",
			want: "== Reception ==\n\nKept.\n\n== Legacy ==\n\nAlso kept.\n",
		},
		{
			name: "math run spliced into its sentence",
			// The line after a run is spliced too: a run can interrupt a
			// sentence mid-line, and the sentence splitter re-splits at
			// real sentence ends anyway.
			in:   "The value \n  \n    x\n    =\n    2\n  \n    {\\displaystyle x=2}\n  \n holds.\nNext paragraph.",
			want: "The value x = 2 holds. Next paragraph.",
		},
		{
			name: "continuation starting with punctuation",
			in:   "Both \n  \n    x\n  \n    {\\displaystyle x}\n  \n, say.",
			want: "Both x, say.",
		},
		{
			name: "textstyle is math too",
			in:   "A \n  \n    y\n  \n    {\\textstyle y}\n  \n term.",
			want: "A y term.",
		},
		{
			name: "standalone display equation keeps its own line",
			in:   "That is,\n\n  \n    E\n    =\n    m\n  \n    {\\displaystyle E=m}\n  \n\nNext.",
			want: "That is,\n\nE = m\n\nNext.",
		},
		{
			name: "heading after a run is not spliced",
			in:   "End \n  \n    z\n  \n    {\\displaystyle z}\n  \n== Next ==\n",
			want: "End z\n== Next ==\n",
		},
		{
			name: "run at the start of the text",
			in:   "  \n    a\n  \n    {\\displaystyle a}\n  \nfollows.",
			want: "a follows.",
		},
		{
			name: "run at the end of the text",
			in:   "Ends with \n  \n    b\n  \n    {\\displaystyle b}\n  ",
			want: "Ends with b",
		},
		{
			// A run holding nothing but the TeX line still interrupted one
			// paragraph: its neighbours are rejoined.
			name: "run holding only the TeX line glues its neighbours",
			in:   "Ohm's law \n    {\\displaystyle V=IR}\n  \n, for a sequence of values.",
			want: "Ohm's law, for a sequence of values.",
		},
		{
			name: "indented run without TeX is left alone",
			in:   "A poem:\n  roses are red\n  violets are blue\nDone.",
			want: "A poem:\n  roses are red\n  violets are blue\nDone.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Clean(tt.in); got != tt.want {
				t.Errorf("Clean(%q)\n got %q\nwant %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseHeading(t *testing.T) {
	tests := []struct {
		in    string
		level int
		title string
		ok    bool
	}{
		{"== History ==", 2, "History", true},
		{"=== Early life ===", 3, "Early life", true},
		{"====== Deep ======", 6, "Deep", true},
		{"==Tight==", 2, "Tight", true},
		{"= Top =", 0, "", false}, // level 1 never appears in extracts
		{"== Unbalanced =", 0, "", false},
		{"== ==", 0, "", false},
		{"======= Seven =======", 0, "", false},
		{"====", 0, "", false},
		{"Plain prose.", 0, "", false},
		{"x == y == z", 0, "", false},
		{"== a = b ==", 2, "a = b", true},
		{"", 0, "", false},
	}
	for _, tt := range tests {
		level, title, ok := ParseHeading(tt.in)
		if level != tt.level || title != tt.title || ok != tt.ok {
			t.Errorf("ParseHeading(%q) = %d, %q, %v; want %d, %q, %v",
				tt.in, level, title, ok, tt.level, tt.title, tt.ok)
		}
	}
}

func BenchmarkClean(b *testing.B) {
	extract := rawExtract(b, "exponentiation.json")
	b.SetBytes(int64(len(extract)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Clean(extract)
	}
}

// BenchmarkCleanProse watches Clean's cost on plain prose (BenchmarkClean's
// exponentiation sample is math-heavy, the pathological case for the bracket
// repairs). The orphan repairs run on every prose line of every ingested
// article, so this is the number that matters at dump scale.
func BenchmarkCleanProse(b *testing.B) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "fixtures", "articles", "barack-obama.txt"))
	if err != nil {
		b.Fatal(err)
	}
	text := string(data)
	b.SetBytes(int64(len(text)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Clean(text)
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func lastLine(s string) string {
	s = strings.TrimRight(s, "\n")
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// TestCleanSimplewikiSections checks the Simple English Wikipedia's names
// for the trailing apparatus sections ("Related pages" for see-also,
// "Other websites" for external links), which every real simplewiki
// article uses.
func TestCleanSimplewikiSections(t *testing.T) {
	cleaned := Clean(strings.Join([]string{
		"Chemistry is a science.",
		"== Related pages ==",
		"Physics",
		"== Other websites ==",
		"An external link",
		"== History ==",
		"It is old.",
	}, "\n"))
	for _, gone := range []string{"Related pages", "Physics", "Other websites", "An external link"} {
		if strings.Contains(cleaned, gone) {
			t.Errorf("cleaned text still contains %q", gone)
		}
	}
	for _, kept := range []string{"Chemistry is a science.", "== History ==", "It is old."} {
		if !strings.Contains(cleaned, kept) {
			t.Errorf("cleaned text lost %q", kept)
		}
	}
}

// TestCollapseOrphans pins the orphaned-punctuation repairs. Every case is
// hand-authored: artifact inputs and their expected repairs come from
// reading raw fixture and corpus text (the upstream extract renders some
// templates as nothing, leaving the punctuation that attached them), and the
// keep cases are real sentences whose punctuation is content, not residue.
func TestCollapseOrphans(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// Separator orphaned at a bracket opening (enwiki DNA lead,
		// simplewiki biography leads).
		{"leading semicolon", "Deoxyribonucleic acid (; DNA) is a polymer.", "Deoxyribonucleic acid (DNA) is a polymer."},
		{"leading comma", "The Duke of Rambouillet (, 2 January 1746 – 13 November 1749) died young.", "The Duke of Rambouillet (2 January 1746 – 13 November 1749) died young."},
		{"leading run", "Hatay State (; ; ), also known informally as the Republic of Hatay, was transitional.", "Hatay State, also known informally as the Republic of Hatay, was transitional."},
		{"opener glued to the word before", "beat out Jupiter(, Juno, and Minerva) in towns.", "beat out Jupiter(Juno, and Minerva) in towns."},
		// Separator orphaned before a bracket closing.
		{"trailing comma", "first wife (excluding Jiyong,) joined.", "first wife (excluding Jiyong) joined."},
		{"trailing comma before NBSP", "first wife (excluding Jiyong, ) joined.", "first wife (excluding Jiyong) joined."},
		{"trailing label colon", "90482 Orcus (symbol: ), originally 2004 DW.", "90482 Orcus (symbol), originally 2004 DW."},
		{"trailing mix", "Caen ( pronounced can (/kɑ̃/);) is a city in France.", "Caen ( pronounced can (/kɑ̃/)) is a city in France."},
		// Bracket pair left empty by a removed span.
		{"empty pair after a word", "Antarctica ( ) is Earth's southernmost continent.", "Antarctica is Earth's southernmost continent."},
		{"empty pair before punctuation", "Hatay State (; ), also known informally.", "Hatay State, also known informally."},
		// Doubled separators with the removed span between them.
		{"comma then semicolon", "Keiko Awaji (淡路 恵子, Awaji Keiko, ; 17 July 1933) was an actress.", "Keiko Awaji (淡路 恵子, Awaji Keiko; 17 July 1933) was an actress."},
		{"comma then comma", "the merger of towns—Saijō, , Hachihonmatsu.", "the merger of towns—Saijō, Hachihonmatsu."},
		{"semicolon then semicolon", "the empress abdicated; ; and the succession passed on.", "the empress abdicated; and the succession passed on."},
		{"colon then comma", "The Goths (Gothic: , Gutans) were an East Germanic tribe.", "The Goths (Gothic, Gutans) were an East Germanic tribe."},

		// Punctuation that is content stays, byte for byte.
		{"mention of a comma", "Lists use commas (,) between items.", "Lists use commas (,) between items."},
		{"mention of a colon", "This symbol is like a colon (:), but with triangles.", "This symbol is like a colon (:), but with triangles."},
		{"mention at the line end", "A comma looks like this: ,", "A comma looks like this: ,"},
		{"empty pair opening a line", "( ) are parentheses or curved brackets.", "( ) are parentheses or curved brackets."},
		{"album named with empty parens", "( ) (2002)", "( ) (2002)"},
		{"empty pair glued to code", "after ExitBootServices() is executed.", "after ExitBootServices() is executed."},
		{"symbol list with glued parens", "punctuation symbols other than +-*/()&=.,' were absent.", "punctuation symbols other than +-*/()&=.,' were absent."},
		{"IPv6 address", "known as 127.0.0.1 (IPv4) or (::1, IPv6).", "known as 127.0.0.1 (IPv4) or (::1, IPv6)."},
		{"emoticon", "Western Style: :-), :-(, ;-), :-O, (;D)", "Western Style: :-), :-(, ;-), :-O, (;D)"},
		{"editorial comma in brackets", "worold are for geaf[,] in the manuscript.", "worold are for geaf[,] in the manuscript."},
		{"spaced math", "the sum ( 1 3 + 1 5 ) + ( 1 5 + 1 7 ) diverges , slowly.", "the sum ( 1 3 + 1 5 ) + ( 1 5 + 1 7 ) diverges , slowly."},
		{"spaced number list", "the primes 2 , 3 , 5 , 7 , 11 , 13 , . . .", "the primes 2 , 3 , 5 , 7 , 11 , 13 , . . ."},
		{"mention of spaced comma in parens", "a comma ( , ), the end number.", "a comma ( , ), the end number."},
		{"unmatched opener", "C: <, {, [, (, ©, ¢, с", "C: <, {, [, (, ©, ¢, с"},
		{"fork bomb", ":(){ :|:& };:", ":(){ :|:& };:"},

		// Hostile shapes: cascades, nesting, splice landing on punctuation.
		{"two empty pairs", "a ( ) ( ) b", "a b"},
		{"empty pair before a period", "survived ( ).", "survived."},
		{"nested pair with inner orphan", "a (b (c,) d) e", "a (b (c) d) e"},
		{"nested separator orphan", "((; x))", "((x))"},
		{"tripled separators", "a, , , b", "a, b"},
		{"orphan in an unclosed pair stays", "trailing open (; and no close", "trailing open (; and no close"},
		{"square pair", "[; foo] bar", "[foo] bar"},
		{"separator run without a gap stays", "x (;;;;;;) y", "x (;;;;;;) y"},
		{"interleaved bracket types stay", "mixed ([ ) ] pairs", "mixed ([ ) ] pairs"},
		{"RTL content", "word (; نص) more", "word (نص) more"},
		{"doubled separator opening the line", ", ; 17 July 1933 was the date.", "; 17 July 1933 was the date."},
		{"line-opening double then a later double", ", ; a, , b", "; a, b"},
		{"empty pair at the line end", "The expedition survived ( )", "The expedition survived"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := collapseOrphans(tt.in); got != tt.want {
				t.Errorf("collapseOrphans(%q)\n got %q\nwant %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestCleanFixtureOrphans is the full-fixture pass: the expectations are
// ground truth authored by reading every fixture's raw text for orphaned
// punctuation (the two lead sentences below are the only genuine artifacts
// in the 26 committed articles), and the sweep then checks no fixture line
// keeps the artifact shapes the repairs target — a separator run glued to a
// bracket opening with content after it, or an empty pair interrupting
// prose.
func TestCleanFixtureOrphans(t *testing.T) {
	leads := map[string]string{
		"dna.txt":        "Deoxyribonucleic acid (DNA) is a polymer composed of two polynucleotide chains",
		"antarctica.txt": "Antarctica is Earth's southernmost and least-populated continent.",
	}
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "fixtures", "articles", "*.txt"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	sepAfterOpen := regexp.MustCompile(`[(\[][,;:]+\s`)
	emptyPair := regexp.MustCompile(`\pL\pZ+(\(\s*\)|\[\s*\])`)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		cleaned := Clean(string(data))
		if want := leads[filepath.Base(f)]; want != "" && !strings.HasPrefix(cleaned, want) {
			t.Errorf("%s: cleaned lead is %q, want prefix %q", filepath.Base(f), firstLine(cleaned), want)
		}
		for line := range strings.Lines(cleaned) {
			if indented(line) {
				continue
			}
			if m := sepAfterOpen.FindString(line); m != "" {
				t.Errorf("%s: separator orphaned at a bracket opening (%q) in %q", filepath.Base(f), m, firstLine(line))
			}
			if m := emptyPair.FindString(line); m != "" {
				t.Errorf("%s: empty bracket pair after a word (%q) in %q", filepath.Base(f), m, firstLine(line))
			}
		}
	}
	// The repairs must not reach into legitimate glued punctuation: the
	// exponentiation article's symbol list survives byte for byte.
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "fixtures", "articles", "exponentiation.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "symbols other than +-*/()&=.,' and so used ** for exponentiation"; !strings.Contains(Clean(string(data)), want) {
		t.Errorf("exponentiation symbol list no longer contains %q", want)
	}
}

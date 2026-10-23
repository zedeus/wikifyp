package source

import (
	"strings"
	"unicode/utf8"
)

// metaSections are the trailing apparatus sections stripped at ingest: lists
// of references and pointers, not article prose. This list is the single
// place that knows which sections those are.
var metaSections = map[string]bool{
	"references":      true,
	"notes":           true,
	"citations":       true,
	"sources":         true,
	"footnotes":       true,
	"works cited":     true,
	"bibliography":    true,
	"further reading": true,
	"external links":  true,
	"see also":        true,
	// The Simple English Wikipedia's names for "see also" and
	// "external links".
	"related pages":  true,
	"other websites": true,
}

// Clean strips what the extract format leaves behind in otherwise plain
// article text: reference apparatus sections (including their headings and
// subsections) and the fragmented rendering of math formulas. Section
// headings and prose are kept byte for byte.
//
// Math handling: the extract renders each formula as a run of indented
// lines (the formula's glyphs, roughly one per line, with any prose between
// adjacent formulas indented too) ending in the TeX source on a line like
// "{\displaystyle x^{2}}". Because the sentence splitter treats every
// newline as a structural break, those runs would otherwise shred the
// surrounding sentence. A newline in extract text separates paragraphs
// except around such a run, so Clean drops the TeX lines, rejoins the run's
// remaining fragments with spaces, and splices the result back into the
// line before and the line after the run. Indented runs without TeX are
// left untouched.
func Clean(text string) string {
	lines := strings.Split(stripSections(text), "\n")
	out := make([]string, 0, len(lines))
	// glue marks that the last output line was spliced around a math run
	// and the next plain line continues it.
	glue := false
	for i := 0; i < len(lines); {
		if !indented(lines[i]) {
			if glue && lines[i] != "" && !isHeadingLine(lines[i]) {
				out[len(out)-1] = splice(out[len(out)-1], lines[i])
			} else {
				out = append(out, lines[i])
			}
			glue = false
			i++
			continue
		}

		// Collect the maximal indented run and the formula text inside it.
		var fragments []string
		tex := false
		j := i
		for ; j < len(lines) && indented(lines[j]); j++ {
			t := strings.TrimSpace(lines[j])
			switch {
			case t == "":
			case strings.HasPrefix(t, `{\displaystyle`) || strings.HasPrefix(t, `{\textstyle`):
				tex = true
			default:
				fragments = append(fragments, t)
			}
		}
		if !tex {
			// Indented but not math: not this function's business.
			out = append(out, lines[i:j]...)
			glue = false
			i = j
			continue
		}
		math := strings.Join(fragments, " ")
		if n := len(out) - 1; n >= 0 && out[n] != "" && !isHeadingLine(out[n]) {
			if math != "" {
				out[n] = splice(out[n], math)
			}
			glue = true
		} else if math != "" {
			out = append(out, math)
			glue = true
		}
		i = j
	}
	return strings.Join(out, "\n")
}

// stripSections removes every meta section: from its heading up to the next
// heading of the same or a higher level, subsections included.
func stripSections(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	skipLevel := 0 // 0 = keeping; otherwise the level of the section being skipped
	for line := range strings.Lines(text) {
		if level, title, ok := ParseHeading(strings.TrimRight(line, "\n")); ok {
			if skipLevel > 0 && level <= skipLevel {
				skipLevel = 0
			}
			if skipLevel == 0 && metaSections[strings.ToLower(title)] {
				skipLevel = level
			}
		}
		if skipLevel == 0 {
			b.WriteString(line)
		}
	}
	return b.String()
}

// splice joins a continuation fragment onto a line, omitting the space when
// the fragment opens with punctuation that attaches to the preceding word
// (", interchangeably." must not become " , interchangeably."). This is
// typographic left-attachment — a wider class than the sentence-boundary
// closers the text package knows, so it lives here.
func splice(line, fragment string) string {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return line
	}
	line = strings.TrimRight(line, " \t")
	if r, _ := utf8.DecodeRuneInString(fragment); strings.ContainsRune(",.;:)]}!?…"+`"”’»`, r) {
		return line + fragment
	}
	return line + " " + fragment
}

// ParseHeading parses a MediaWiki extract heading line such as
// "== History ==" into its level (2-6) and title.
func ParseHeading(line string) (level int, title string, ok bool) {
	s := strings.TrimSpace(line)
	open := 0
	for open < len(s) && s[open] == '=' {
		open++
	}
	closing := 0
	for closing < len(s)-open && s[len(s)-1-closing] == '=' {
		closing++
	}
	if open != closing || open < 2 || open > 6 {
		return 0, "", false
	}
	title = strings.TrimSpace(s[open : len(s)-closing])
	if title == "" {
		return 0, "", false
	}
	return open, title, true
}

func isHeadingLine(line string) bool {
	_, _, ok := ParseHeading(line)
	return ok
}

// indented reports whether a line belongs to an extract math run: non-empty
// and starting with whitespace. Plain extract prose always starts at column
// zero.
func indented(line string) bool {
	return line != "" && (line[0] == ' ' || line[0] == '\t')
}

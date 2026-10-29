package source

import (
	"strings"
	"unicode"
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
//
// Clean also repairs punctuation orphaned by content the extract format
// already removed upstream (see collapseOrphans); indented lines are code or
// math fragments and are exempt.
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
	for i, line := range out {
		if !indented(line) {
			out[i] = collapseOrphans(line)
		}
	}
	return strings.Join(out, "\n")
}

// Orphaned punctuation: the extract format renders some templates
// (pronunciations, foreign scripts) as nothing at all, leaving behind the
// punctuation that attached them to their neighbors — "Deoxyribonucleic acid
// (; DNA)", "Antarctica ( ) is", "Awaji Keiko, ; 17 July". The artifact
// class is a separator or bracket pair with nothing left on one side; the
// repairs below detect that shape rather than enumerating templates.
//
// Every rule is biased by what real text showed (26 enwiki fixtures plus the
// full simplewiki corpus): punctuation mentions ("(,)", "(:)", "( ) are
// parentheses"), code ("round()", "(::1"), emoticons ("(;D)") and spaced
// math ("( x )", "2 , 3 , 5") must survive, so each repair requires the
// specific spacing shape a removed span leaves and nothing else. Known
// residue: an artifact glued on both sides ("(,TV drama)") stays, and a rare
// mention shaped exactly like an artifact ("(C:)") is cleaned; a false keep
// is cosmetic, a false clean costs one odd token, and the shapes repaired
// here outnumber the shapes mangled by two orders of magnitude in that
// corpus.

// separators is the punctuation that joins two pieces of content within a
// sentence — the single definition the orphan repairs share.
const separators = ",;:"

func isSeparator(r rune) bool {
	return strings.ContainsRune(separators, r)
}

// collapseOrphans repairs one line of prose: doubled separators collapse to
// the later one, and bracket pairs lose separators orphaned at their edges
// (dropping the pair entirely when nothing is left inside). Applied to
// fixpoint because one repair can expose another ("(; ; )" ends up removed).
// This runs on every prose line of every ingested article: a line with no
// separator or bracket returns immediately, and one with no matched bracket
// pair allocates only its rune slice; a line with pairs also pays the match
// table and a scan copy (BenchmarkClean and BenchmarkCleanProse watch the
// cost — ~12ns/byte of prose, minutes over a full dump).
func collapseOrphans(line string) string {
	if !strings.ContainsAny(line, separators+"([") {
		return line
	}
	runes := []rune(line)
	changed := false
	for {
		r, ch1 := collapseDoubledSeparators(runes)
		r, ch2 := repairBrackets(r)
		runes = r
		if !ch1 && !ch2 {
			break
		}
		changed = true
	}
	if !changed {
		return line
	}
	return string(runes)
}

// collapseDoubledSeparators drops a separator whose span to the next
// separator is empty ("Keiko, ; 17 July" → "Keiko; 17 July"): the removed
// content sat between them, and the later separator is the one that binds
// the clause that follows. The later separator must itself be followed by
// whitespace: a separator glued to what follows is content (":-(, ;-)" lists
// emoticons), and one ending the line is a mention ("A comma looks like
// this: ,"). Returns the input slice unchanged (no copy) when nothing
// collapses.
func collapseDoubledSeparators(runes []rune) ([]rune, bool) {
	var out []rune // allocated on the first drop; untouched lines copy nothing
	changed := false
	for i := 0; i < len(runes); i++ {
		if isSeparator(runes[i]) {
			j := i + 1
			for j < len(runes) && unicode.IsSpace(runes[j]) {
				j++
			}
			if j > i+1 && j+1 < len(runes) && isSeparator(runes[j]) &&
				unicode.IsSpace(runes[j+1]) {
				// Drop this separator and the space run; rescan from the
				// survivor, which may itself be doubled.
				if !changed {
					out = append(out, runes[:i]...)
					changed = true
				}
				i = j - 1
				continue
			}
		}
		if changed {
			out = append(out, runes[i])
		}
	}
	if !changed {
		return runes, false
	}
	return out, true
}

// repairBrackets walks the line's matched () and [] pairs, innermost first,
// and repairs each pair's interior: a separator run glued to the opener and
// followed by whitespace lost its left-hand content ("(; DNA)" → "(DNA)");
// trailing separators lost their right-hand content ("(excluding Jiyong,)" →
// "(excluding Jiyong)"); a pair left holding nothing after a word is removed
// outright and the line rejoined ("Antarctica ( ) is" → "Antarctica is").
// A pair whose only content is separators is a punctuation mention ("(,)")
// and is kept, as is an empty pair that does not follow a word ("( ) are
// parentheses", "round()"). Returns the input slice unchanged when no repair
// fires.
func repairBrackets(runes []rune) ([]rune, bool) {
	match := matchingCloses(runes)
	if match == nil {
		return runes, false
	}
	out, changed := repairRange(runes, match, 0, len(runes))
	if !changed {
		return runes, false
	}
	return out, true
}

// matchingCloses returns, for each opener, the index of its matching closer
// (same bracket type, nesting counted) or -1, in one pass — or nil when the
// line holds no opener at all.
func matchingCloses(runes []rune) []int {
	var match []int
	var parens, squares []int
	for i, r := range runes {
		switch r {
		case '(', '[':
			if match == nil {
				match = make([]int, len(runes))
				for k := range match {
					match[k] = -1
				}
			}
			if r == '(' {
				parens = append(parens, i)
			} else {
				squares = append(squares, i)
			}
		case ')':
			if n := len(parens); n > 0 {
				match[parens[n-1]] = i
				parens = parens[:n-1]
			}
		case ']':
			if n := len(squares); n > 0 {
				match[squares[n-1]] = i
				squares = squares[:n-1]
			}
		}
	}
	return match
}

// repairRange repairs the bracket pairs within runes[lo:hi), recursing into
// each pair's interior. match holds absolute closer indices; a match at or
// past hi belongs to an enclosing range and counts as unmatched here.
func repairRange(runes []rune, match []int, lo, hi int) ([]rune, bool) {
	out := make([]rune, 0, hi-lo)
	changed := false
	for i := lo; i < hi; i++ {
		open := runes[i]
		if open != '(' && open != '[' {
			out = append(out, open)
			continue
		}
		j := match[i]
		if j < 0 || j >= hi {
			out = append(out, open)
			continue
		}
		inner, ch := repairRange(runes, match, i+1, j)
		changed = changed || ch
		// Only a pair sitting in prose gets its interior repaired: an
		// opener glued to other punctuation is part of a symbol
		// (":-(, ;-)" pairs the first emoticon's mouth with the second's).
		if i == lo || unicode.IsSpace(runes[i-1]) || isWordRune(runes[i-1]) {
			inner, ch = stripLeadingSeparators(inner)
			changed = changed || ch
			inner, ch = trimTrailingSeparators(inner)
			changed = changed || ch
		}
		if blank(inner) && followsWord(out) {
			rest, _ := repairRange(runes, match, j+1, hi)
			return []rune(splice(string(out), string(rest))), true
		}
		out = append(out, open)
		out = append(out, inner...)
		out = append(out, runes[j])
		i = j
	}
	return out, changed
}

// stripLeadingSeparators removes separator runs at the start of a bracket
// pair's interior, but only when whitespace follows the run: that gap is
// where the removed content stood ("; DNA"). A run glued to what follows is
// content ("::1, IPv6", ";D") and a run against the closer is a mention
// (",") — both stay.
func stripLeadingSeparators(inner []rune) ([]rune, bool) {
	changed := false
	for {
		k := 0
		for k < len(inner) && isSeparator(inner[k]) {
			k++
		}
		if k == 0 {
			return inner, changed
		}
		m := k
		for m < len(inner) && unicode.IsSpace(inner[m]) {
			m++
		}
		if m == k {
			return inner, changed
		}
		inner = inner[m:]
		changed = true
	}
}

// trimTrailingSeparators removes a trailing mix of separators and whitespace
// from a bracket pair's interior ("excluding Jiyong,", "symbol: ") — the
// removed content stood before the closer. A tail of pure whitespace is
// spacing, not an orphan, and an interior that is nothing but separators and
// whitespace is a mention or an empty pair; both are left for other rules.
func trimTrailingSeparators(inner []rune) ([]rune, bool) {
	end := len(inner)
	seps := false
	for end > 0 {
		switch r := inner[end-1]; {
		case isSeparator(r):
			seps = true
		case !unicode.IsSpace(r):
			goto done
		}
		end--
	}
done:
	if !seps || end == 0 {
		return inner, false
	}
	return inner[:end], true
}

func blank(runes []rune) bool {
	for _, r := range runes {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// isWordRune reports a rune prose stands on — the shared definition of
// "a word was here" for the bracket repairs.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// followsWord reports whether the text ends in a word followed by
// whitespace: the shape left of a bracket pair that interrupted prose. An
// empty pair elsewhere — glued to a word like "round()", or opening a line
// like "( ) are parentheses" — is content.
func followsWord(out []rune) bool {
	i := len(out) - 1
	if i < 0 || !unicode.IsSpace(out[i]) {
		return false
	}
	for i >= 0 && unicode.IsSpace(out[i]) {
		i--
	}
	return i >= 0 && isWordRune(out[i])
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
	line = strings.TrimRight(line, " \t")
	if fragment == "" {
		return line
	}
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

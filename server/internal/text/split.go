// Package text splits article text into sentences and tokens with stable
// addresses.
//
// Tokens are the whitespace-delimited chunks of a sentence and keep their
// original characters, punctuation included. A sentence's Text is its tokens
// joined with single spaces, so rejoining tokens always reproduces the
// sentence and no non-whitespace character of the input is ever lost or
// altered. Splitting is deterministic: the same input always yields the same
// sentences, tokens and addresses.
package text

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Token is one whitespace-delimited chunk of a sentence.
type Token struct {
	Idx  int    // position within the sentence, stable forever
	Text string // original characters, untouched
}

// Sentence is one sentence of an article.
type Sentence struct {
	Idx    int    // position within the article, stable forever
	Text   string // tokens joined with single spaces
	Tokens []Token
}

// Split breaks text into sentences and tokens. Whitespace-only input yields
// no sentences.
func Split(text string) []Sentence {
	runes := []rune(text)
	var spans [][]rune
	start := 0

	for i := 0; i < len(runes); {
		r := runes[i]

		// A numbered list item starting on a new line begins a new sentence
		// even without a terminator before it ("…item one\n2. Item two").
		if r == '\n' && startsListItem(runes[i+1:]) && !allSpace(runes[start:i]) {
			spans = append(spans, runes[start:i])
			start = i + 1
			i++
			continue
		}

		if !isTerminator(r) {
			i++
			continue
		}

		// Absorb a run of terminators ("?!", "...") and closing marks.
		last := i
		for last+1 < len(runes) && isTerminator(runes[last+1]) {
			last++
		}
		after := last + 1
		for after < len(runes) && isCloser(runes[after]) {
			after++
		}

		if isBoundary(runes, start, i, last, after) {
			spans = append(spans, runes[start:after])
			start = after
		}
		i = after
	}
	if !allSpace(runes[start:]) {
		spans = append(spans, runes[start:])
	}

	sentences := make([]Sentence, 0, len(spans))
	for _, span := range spans {
		fields := strings.Fields(string(span))
		if len(fields) == 0 {
			continue
		}
		tokens := make([]Token, len(fields))
		for i, f := range fields {
			tokens[i] = Token{Idx: i, Text: f}
		}
		sentences = append(sentences, Sentence{
			Idx:    len(sentences),
			Text:   strings.Join(fields, " "),
			Tokens: tokens,
		})
	}
	return sentences
}

// allSpace reports whether the span holds nothing but whitespace.
func allSpace(runes []rune) bool {
	for _, r := range runes {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// abbreviations never end a sentence, with or without a following capital
// ("Dr. Smith", "No. 7"). Stored without the trailing dot. "etc." is left out
// on purpose: it often does end sentences, and the next-word case decides.
// Dotted forms like "e.g." and "i.e." need no entry: isInitials covers every
// word of dot-separated single letters.
var abbreviations = map[string]bool{
	"Mr": true, "Mrs": true, "Ms": true, "Dr": true, "Prof": true,
	"St": true, "Jr": true, "Sr": true, "Fig": true, "No": true,
	"vs": true, "cf": true, "ca": true, "approx": true, "al": true,
	"Inc": true, "Ltd": true, "Co": true, "Dept": true, "Univ": true,
	"ed": true, "eds": true, "vol": true, "pp": true, "p": true,
}

// isBoundary decides whether the terminator run runes[first..last] (followed
// by closing marks up to after) ends a sentence that began at start.
func isBoundary(runes []rune, start, first, last, after int) bool {
	// End of text always closes the sentence.
	if after >= len(runes) {
		return true
	}
	// "3.14", "e.g.", "U.S.A.": a terminator glued to the next character
	// never splits.
	if !unicode.IsSpace(runes[after]) {
		return false
	}

	if runes[first] == '.' && first == last {
		// Leading quotes or brackets are not part of the word: "(Dr." must
		// still be recognized as the abbreviation "Dr".
		word := strings.TrimLeftFunc(wordBefore(runes, start, first), isOpener)
		switch {
		case abbreviations[word]:
			return false
		case isInitials(word):
			// "J. R. R. Tolkien", "U.S" — single letters, dot-separated.
			return false
		case isListNumber(runes, start, first, word):
			// "1. First item" — the numbering dot is not an ending.
			return false
		}
	}

	// Look at how the next sentence would begin: skip whitespace and opening
	// marks (in any mix, "( Word" included) to the first substantive rune.
	k := after
	for k < len(runes) && (unicode.IsSpace(runes[k]) || isOpener(runes[k])) {
		k++
	}
	if k >= len(runes) {
		return true
	}
	next := runes[k]

	if isEllipsis(runes, first, last) {
		// "wait... five seconds" runs on; "It faded... Then silence." splits.
		return unicode.IsUpper(next)
	}
	// Sentences start with capitals or digits; a lowercase continuation
	// ("etc. and so on") runs on.
	return unicode.IsUpper(next) || unicode.IsDigit(next)
}

// wordBefore returns the non-space run immediately before position end,
// looking back no further than start.
func wordBefore(runes []rune, start, end int) string {
	i := end
	for i > start && !unicode.IsSpace(runes[i-1]) {
		i--
	}
	return string(runes[i:end])
}

// isInitials reports whether word is single letters separated by dots:
// "J", "U.S", "J.R.R".
func isInitials(word string) bool {
	if word == "" {
		return false
	}
	for _, part := range strings.Split(word, ".") {
		r := []rune(part)
		if len(r) != 1 || !unicode.IsLetter(r[0]) {
			return false
		}
	}
	return true
}

// isListNumber reports whether word is a bare number opening a sentence or a
// line, i.e. list numbering rather than a sentence ending in a number.
func isListNumber(runes []rune, start, first int, word string) bool {
	if word == "" {
		return false
	}
	for _, r := range word {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	// Position where the word begins.
	wordStart := first - utf8.RuneCountInString(word)
	for i := wordStart - 1; i >= start; i-- {
		if runes[i] == '\n' {
			return true
		}
		if !unicode.IsSpace(runes[i]) {
			return false
		}
	}
	return true // first word of the sentence
}

// startsListItem reports whether runes begin (after optional spaces) with
// digits followed by a dot, like "2." at the start of a line.
func startsListItem(runes []rune) bool {
	i := 0
	for i < len(runes) && (runes[i] == ' ' || runes[i] == '\t') {
		i++
	}
	digits := 0
	for i < len(runes) && unicode.IsDigit(runes[i]) {
		i++
		digits++
	}
	return digits > 0 && i < len(runes) && runes[i] == '.'
}

func isTerminator(r rune) bool {
	return r == '.' || r == '!' || r == '?' || r == '…'
}

func isEllipsis(runes []rune, first, last int) bool {
	if runes[first] == '…' {
		return true
	}
	dots := 0
	for i := first; i <= last; i++ {
		if runes[i] == '.' {
			dots++
		}
	}
	return dots >= 2
}

func isCloser(r rune) bool {
	switch r {
	case '"', '\'', ')', ']', '”', '’', '»':
		return true
	}
	return false
}

func isOpener(r rune) bool {
	switch r {
	case '"', '\'', '(', '[', '“', '‘', '«':
		return true
	}
	return false
}

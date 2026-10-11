package text

import (
	"fmt"
	"strconv"
	"strings"
)

// Addr is the stable address of one token: which article, which sentence in
// it, which token in that sentence. Addresses are never renumbered once
// stored, so everything downstream can point at exact words.
type Addr struct {
	ArticleID   string
	SentenceIdx int
	TokenIdx    int
}

// String encodes the address as "articleID:sentenceIdx:tokenIdx". Article IDs
// may themselves contain colons; ParseAddr splits from the right.
func (a Addr) String() string {
	return a.ArticleID + ":" + strconv.Itoa(a.SentenceIdx) + ":" + strconv.Itoa(a.TokenIdx)
}

// ParseAddr decodes the String encoding. Indices must be non-negative.
func ParseAddr(s string) (Addr, error) {
	rest, tokenPart, ok := cutLast(s)
	if !ok {
		return Addr{}, fmt.Errorf("text: address %q: want articleID:sentenceIdx:tokenIdx", s)
	}
	articleID, sentencePart, ok := cutLast(rest)
	if !ok {
		return Addr{}, fmt.Errorf("text: address %q: want articleID:sentenceIdx:tokenIdx", s)
	}
	sentenceIdx, err := parseIdx(sentencePart)
	if err != nil {
		return Addr{}, fmt.Errorf("text: address %q: sentence index: %w", s, err)
	}
	tokenIdx, err := parseIdx(tokenPart)
	if err != nil {
		return Addr{}, fmt.Errorf("text: address %q: token index: %w", s, err)
	}
	return Addr{ArticleID: articleID, SentenceIdx: sentenceIdx, TokenIdx: tokenIdx}, nil
}

// cutLast splits s around its last colon.
func cutLast(s string) (before, after string, ok bool) {
	i := strings.LastIndexByte(s, ':')
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+1:], true
}

func parseIdx(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, fmt.Errorf("negative index %d", n)
	}
	// Reject non-canonical spellings ("+1", "007"): addresses are compared
	// and deduplicated as strings, so each token has exactly one encoding.
	if strconv.Itoa(n) != s {
		return 0, fmt.Errorf("non-canonical index %q", s)
	}
	return n, nil
}

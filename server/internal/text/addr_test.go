package text

import (
	"math/rand"
	"testing"
	"testing/quick"
)

func TestAddrString(t *testing.T) {
	a := Addr{ArticleID: "earth", SentenceIdx: 3, TokenIdx: 14}
	if got, want := a.String(), "earth:3:14"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestParseAddr(t *testing.T) {
	tests := []struct {
		in      string
		want    Addr
		wantErr bool
	}{
		{in: "earth:3:14", want: Addr{"earth", 3, 14}},
		{in: "a:0:0", want: Addr{"a", 0, 0}},
		{in: "id:with:colons:7:2", want: Addr{"id:with:colons", 7, 2}},
		{in: ":1:2", want: Addr{"", 1, 2}},
		{in: "", wantErr: true},
		{in: "noindices", wantErr: true},
		{in: "only:1", wantErr: true},
		{in: "a:x:2", wantErr: true},
		{in: "a:1:y", wantErr: true},
		{in: "a:-1:2", wantErr: true},
		{in: "a:1:-2", wantErr: true},
		{in: "a:1:2 ", wantErr: true},
		{in: "a:01:2", wantErr: true},
		{in: "a:1:+2", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ParseAddr(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseAddr(%q) = %+v, want error", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseAddr(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseAddr(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

// Every address round-trips through its string encoding, whatever the
// article ID contains.
func TestAddrRoundTripProperty(t *testing.T) {
	roundTrips := func(articleID string, sentenceIdx, tokenIdx uint16) bool {
		a := Addr{ArticleID: articleID, SentenceIdx: int(sentenceIdx), TokenIdx: int(tokenIdx)}
		back, err := ParseAddr(a.String())
		return err == nil && back == a
	}
	cfg := &quick.Config{MaxCount: 2000, Rand: rand.New(rand.NewSource(1))}
	if err := quick.Check(roundTrips, cfg); err != nil {
		t.Error(err)
	}
}

// Addresses with colons and digits in the article ID are the adversarial
// case for right-to-left parsing.
func TestAddrRoundTripHostileIDs(t *testing.T) {
	ids := []string{"a:1", "1:2:3", ":", "::", "5", "-7", "a:-1", "…:🙂", "a 1:2"}
	for _, id := range ids {
		a := Addr{ArticleID: id, SentenceIdx: 9, TokenIdx: 4}
		back, err := ParseAddr(a.String())
		if err != nil {
			t.Errorf("ParseAddr(%q): %v", a.String(), err)
			continue
		}
		if back != a {
			t.Errorf("round trip of %+v gave %+v", a, back)
		}
	}
}

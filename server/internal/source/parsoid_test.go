package source

import (
	"slices"
	"strings"
	"testing"
)

func parse(t *testing.T, html string) *ParsedPage {
	t.Helper()
	p, err := ParseParsoid(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestParsoidThumbCaption: templates like {{Chess diagram}} render classic
// div-based thumbnails instead of <figure>; their captions are not article
// text. The markup is a trimmed verbatim fragment of the simplewiki "Chess"
// dump record (revision 10078171), which shipped its board as
// div.thumb > div.center > table + div.thumbcaption.
func TestParsoidThumbCaption(t *testing.T) {
	page := parse(t, `<html><body>
<p id="mwBQ">Chess is a board game for two players.</p>
<div class="thumb  tright
" about="#mwt5" typeof="mw:Transclusion" id="mwBg"><div class="center" style="line-height:130%;margin:0 auto;max-width:254px">
<table><tbody><tr><td>a</td><td>b</td></tr></tbody></table><div class="thumbcaption">A chess set in the <b>array</b>, or starting position
</div></div></div>
<p>It is played on a square board.</p>
</body></html>`)
	want := "Chess is a board game for two players.\nIt is played on a square board."
	if page.Text != want {
		t.Errorf("text = %q, want %q", page.Text, want)
	}
}

func TestParsoidHeadingsAndBlocks(t *testing.T) {
	page := parse(t, `<html><body>
<h1>Top</h1>
<p>Intro.</p>
<h2>History</h2>
<p>Old.</p>
<h3>Earlier</h3>
<ul><li>First item</li><li>Second <i>item</i></li></ul>
<h6>Deep</h6>
<blockquote>Quoted words.</blockquote>
</body></html>`)
	want := strings.Join([]string{
		"== Top ==", // h1 clamps to the minimum heading level ParseHeading accepts
		"Intro.",
		"== History ==",
		"Old.",
		"=== Earlier ===",
		"First item",
		"Second item",
		"====== Deep ======",
		"Quoted words.",
	}, "\n")
	if page.Text != want {
		t.Errorf("text = %q, want %q", page.Text, want)
	}
	// Every emitted line must survive the heading/indent conventions the
	// splitter pipeline relies on: no leading whitespace.
	for line := range strings.Lines(page.Text) {
		if trimmed := strings.TrimRight(line, "\n"); trimmed != strings.TrimLeft(trimmed, " \t") {
			t.Errorf("line starts with whitespace: %q", line)
		}
	}
}

// TestParsoidLinks: only plain wiki links count — Parsoid's rel attribute,
// not href patterns, decides. The title attribute wins; the href is the
// fallback; duplicates collapse; links inside skipped chrome don't count.
func TestParsoidLinks(t *testing.T) {
	page := parse(t, `<html><body>
<p>A <a rel="mw:WikiLink" href="./Idiom" title="Idiom">idiom</a> and a
<a rel="mw:WikiLink/Interwiki" href="https://simple.wiktionary.org/wiki/x" title="wikt:x">wiktionary word</a> and an
<a rel="mw:ExtLink" href="https://example.com">external site</a> and
<a rel="mw:WikiLink" href="./Bo%C3%B6tes_(constellation)#Stars">an href-only link</a> and
<a rel="mw:WikiLink" href="./Idiom" title="Idiom">the idiom again</a>.</p>
<div class="hatnote"><a rel="mw:WikiLink" href="./Hidden" title="Hidden">chrome link</a></div>
</body></html>`)
	want := []string{"Idiom", "Boötes (constellation)"}
	if !slices.Equal(page.Links, want) {
		t.Errorf("links = %q, want %q", page.Links, want)
	}
}

func TestParsoidRedirect(t *testing.T) {
	page := parse(t, `<html><body>
<link rel="mw:PageProp/redirect" href="./Aerosol_spray"/>
<p>REDIRECT</p>
</body></html>`)
	if !page.Redirect {
		t.Error("redirect page prop not detected")
	}
	if parse(t, `<html><body><p>Prose.</p></body></html>`).Redirect {
		t.Error("plain page reported as redirect")
	}
}

// TestParsoidMath: the MathML glyphs stay inline in the sentence; the TeX
// annotation is dropped. The markup is trimmed from the simplewiki
// "Cloverleaf quasar" record.
func TestParsoidMath(t *testing.T) {
	page := parse(t, `<html><body><p>at a redshift distance of <span class="mwe-math-element" typeof="mw:Extension/math"><span style="display: none;"><math alttext="{\displaystyle z=2.56}">
  <semantics>
    <mrow>
      <mstyle>
        <mi>z</mi>
        <mo>=</mo>
        <mn>2.56</mn>
      </mstyle>
    </mrow>
    <annotation encoding="application/x-tex">{\displaystyle z=2.56}</annotation>
  </semantics>
</math></span><img src="x.svg" alt="{\displaystyle z=2.56}"/></span></p></body></html>`)
	if want := "at a redshift distance of z = 2.56"; page.Text != want {
		t.Errorf("text = %q, want %q", page.Text, want)
	}
}

// TestParsoidTemplateError: a broken template renders its error message as
// an error-classed span inside the prose; the message (and its help link)
// is chrome. Trimmed verbatim from the simplewiki "1923 Great Kantō
// earthquake" record (revision 10056263), whose {{nihongo}} call is broken
// at the source.
func TestParsoidTemplateError(t *testing.T) {
	page := parse(t, `<html><body><p>The <span class="error" style="font-size:100%" typeof="mw:Transclusion">error: {{nihongo}}: Japanese or romaji text required (<a rel="mw:WikiLink" href="./Template:Nihongo" title="Template:Nihongo">help</a>)</span> was a <a rel="mw:WikiLink" href="./Japan" title="Japan">Japanese</a> natural disaster.</p></body></html>`)
	if want := "The was a Japanese natural disaster."; page.Text != want {
		t.Errorf("text = %q, want %q", page.Text, want)
	}
	if !slices.Equal(page.Links, []string{"Japan"}) {
		t.Errorf("links = %q, want only Japan (no Template:Nihongo)", page.Links)
	}
}

// TestParsoidGallery: gallery captions are figcaptions in all but markup.
// Trimmed from the same record's Gallery section.
func TestParsoidGallery(t *testing.T) {
	page := parse(t, `<html><body>
<h2 id="Gallery">Gallery</h2>
<ul class="gallery mw-gallery-traditional" typeof="mw:Extension/gallery">
<li class="gallerybox"><div class="thumb"><span typeof="mw:File"><a href="./File:X.jpg" title="Tokyo in flames"><img alt="Tokyo in flames"/></a></span></div><div class="gallerytext">Tokyo in flames</div></li>
<li class="gallerybox"><div class="gallerytext">A view of the destruction in Yokohama</div></li>
</ul>
<p>Prose after.</p>
</body></html>`)
	if want := "== Gallery ==\nProse after."; page.Text != want {
		t.Errorf("text = %q, want %q", page.Text, want)
	}
}

// TestParsoidReferenceList: <ref>-generated lists are dropped by their
// markup, not their section's heading — real dumps put them under headings
// like "Notes and references" or "Career statistics" that no heading list
// can enumerate (the leak hit 2,500 articles of the simplewiki corpus).
func TestParsoidReferenceList(t *testing.T) {
	page := parse(t, `<html><body>
<p>Prose.</p>
<h2 id="Notes_and_references">Notes and references</h2>
<div class="mw-references-wrap" typeof="mw:Extension/references">
<ol class="mw-references references">
<li id="cite_note-1"><span class="mw-cite-backlink"><a href="./X#cite_ref-1">↑</a></span> <span class="reference-text">Okell 1995, p. 3.</span></li>
</ol></div>
</body></html>`)
	if want := "Prose.\n== Notes and references =="; page.Text != want {
		t.Errorf("text = %q, want %q", page.Text, want)
	}
}

// TestParsoidCitationComment: CS1 cite templates append editor-facing error
// notes to otherwise fine citations; the citation stays, the note goes.
// Trimmed verbatim from the simplewiki "Anna Louise Strong" record, whose
// hand-written Works list uses {{cite book}} directly.
func TestParsoidCitationComment(t *testing.T) {
	page := parse(t, `<html><body><ul><li><cite id="CITEREF-1904" class="citation book cs1">- (1904). <i>Storm Songs and Fables</i>.</cite><span class="cs1-visible-error citation-comment"><code class="cs1-code">{{<a rel="mw:WikiLink" href="./Template:Cite_book" title="Template:Cite book">cite book</a>}}</code>: </span><span class="cs1-visible-error citation-comment"><code class="cs1-code"><span typeof="mw:Entity">|</span>author=</code> has numeric name (<a rel="mw:WikiLink" href="./Help:CS1_errors#numeric_names" title="Help:CS1 errors">help</a>)</span></li></ul></body></html>`)
	if want := "- (1904). Storm Songs and Fables."; page.Text != want {
		t.Errorf("text = %q, want %q", page.Text, want)
	}
	if len(page.Links) != 0 {
		t.Errorf("links = %q, want none (the note's template/help links are chrome)", page.Links)
	}
}

// TestParsoidAudioWidget: the pronunciation "listen" button is chrome, and
// the parens that wrapped it must not survive as an empty husk. Trimmed
// verbatim from the simplewiki "Yokohama" record (revision 10120220).
func TestParsoidAudioWidget(t *testing.T) {
	page := parse(t, `<html><body><p>Yokohama (pronounced [jokohama] <span style="font-size:85%">(<span class="unicode haudio"><span class="fn"><span typeof="mw:File"><a href="./File:Ja-Yokohama.ogg" title="About this sound"><img alt="audio speaker icon"/></a></span><a rel="mw:MediaLink" href="//upload.wikimedia.org/x.ogg" title="Ja-Yokohama.ogg">listen</a></span></span>)</span>) is a city.</p></body></html>`)
	if want := "Yokohama (pronounced [jokohama]) is a city."; page.Text != want {
		t.Errorf("text = %q, want %q", page.Text, want)
	}
}

// TestParsoidNestedHusk: a stripped widget that was the sole content of a
// parenthetical inside another parenthetical leaves nested husks; removal
// must reach a fixed point, not stop after the inner pair.
func TestParsoidNestedHusk(t *testing.T) {
	page := parse(t, `<html><body><p>Berlin ( <span style="font-size:85%">( <span class="unicode haudio"><a rel="mw:MediaLink" href="//upload.wikimedia.org/x.ogg">listen</a></span> )</span> ) is a city.</p></body></html>`)
	if want := "Berlin is a city."; page.Text != want {
		t.Errorf("text = %q, want %q", page.Text, want)
	}
}

// TestParsoidEmptyParensSurvivors: husk removal must not touch empty parens
// that are real text.
func TestParsoidEmptyParensSurvivors(t *testing.T) {
	page := parse(t, `<html><body>
<p>Methods like deposit() and withdraw() stay.</p>
<p>"( )" marks are sometimes called brackets.</p>
<ul><li>( ) (2002)</li></ul>
</body></html>`)
	want := "Methods like deposit() and withdraw() stay.\n\"( )\" marks are sometimes called brackets.\n( ) (2002)"
	if page.Text != want {
		t.Errorf("text = %q, want %q", page.Text, want)
	}
}

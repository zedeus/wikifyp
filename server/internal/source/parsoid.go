package source

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// ParsedPage is the readable content of one Parsoid-rendered wiki page, as
// produced by ParseParsoid.
type ParsedPage struct {
	// Text is extract-shaped: headings as "== Title ==" lines and one line
	// per block of prose, ready for the same Clean and Split every other
	// source goes through.
	Text string
	// Links are the page's outgoing wiki links in document order, without
	// duplicates. Targets are titles ("Atlantic slave trade"), including
	// links from apparatus sections that Clean later strips from the text:
	// a "Related pages" link is a real relation even though the list
	// itself is navigation, not prose.
	Links []string
	// Redirect reports that the page is a redirect, not an article.
	Redirect bool
}

// skipElements are subtrees that hold no readable article text: media,
// tables (infoboxes, navboxes, data tables), styling and page metadata.
var skipElements = map[atom.Atom]bool{
	atom.Table:      true,
	atom.Figure:     true,
	atom.Style:      true,
	atom.Script:     true,
	atom.Link:       true,
	atom.Meta:       true,
	atom.Title:      true, // the <head> title; the walk covers the whole document
	atom.Annotation: true, // MathML TeX source; the rendered glyphs are kept
}

// skipClasses mark elements that render as chrome, not article text:
// footnote markers ([1]), hatnotes ("See the main article:"), edit-section
// buttons, anything MediaWiki itself marks as non-content with the metadata
// class (stub notices, maintenance banners), and div-based thumbnails
// ("thumb" — templates like chess diagrams render these instead of the
// <figure> that plain images get; the caption is no more article text than
// a figcaption is).
var skipClasses = []string{"mw-ref", "hatnote", "mw-editsection", "metadata", "thumb",
	// MediaWiki renders template failures ("error: {{nihongo}}: … (help)",
	// harv/sfn reference errors) as error-classed spans inside the prose;
	// an error message is chrome, not article text.
	"error",
	// CS1 citation templates append editor-facing problem notes
	// ("{{cite book}}: |author= has numeric name (help)") to otherwise
	// fine citations, e.g. in a biography's hand-written Works list.
	"citation-comment",
	// Pronunciation audio widgets (the hAudio microformat): a speaker icon
	// and a "listen" media link, no article text.
	"haudio",
}

// skipTypeof are Parsoid extension outputs with no readable article text.
// Reference lists are dropped here by their markup rather than by their
// section's heading name (Clean's metaSections): on real dumps, <ref> lists
// render under arbitrary headings — "Notes and references", "Career
// statistics", the lead section — so the markup is the only reliable
// signal. Galleries are image grids; their captions are figcaptions in all
// but markup. Math (also an extension) is deliberately kept: its glyphs
// read inline.
var skipTypeof = []string{"mw:Extension/references", "mw:Extension/gallery"}

// ParseParsoid converts one Parsoid-rendered page (the HTML in a Wikimedia
// Enterprise dump record, or any action=parse output) into extract-shaped
// text plus its outgoing links.
func ParseParsoid(r io.Reader) (*ParsedPage, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse page HTML: %w", err)
	}
	p := &parser{seen: make(map[string]bool)}
	p.walk(doc)
	p.endBlock()
	return &ParsedPage{
		Text:     strings.Join(p.lines, "\n"),
		Links:    p.links,
		Redirect: p.redirect,
	}, nil
}

type parser struct {
	lines    []string
	block    strings.Builder // text of the block element being walked
	links    []string
	seen     map[string]bool // link targets already recorded
	redirect bool
}

// blockAtoms are elements whose text forms one line of output. Nested
// blocks (a list inside a list item) end the outer line first, which
// matches how extracts render nested lists: one line per item.
var blockAtoms = map[atom.Atom]bool{
	atom.P:          true,
	atom.Li:         true,
	atom.Dt:         true,
	atom.Dd:         true,
	atom.Blockquote: true,
	atom.Pre:        true,
	atom.Caption:    true,
}

func (p *parser) walk(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		p.block.WriteString(n.Data)
		return
	case html.ElementNode:
		// A redirect page carries <link rel="mw:PageProp/redirect">; the
		// element itself is skipped below like any other <link>.
		if n.DataAtom == atom.Link && strings.Contains(attr(n, "rel"), "mw:PageProp/redirect") {
			p.redirect = true
		}
		if skipElements[n.DataAtom] || hasSkipClass(attr(n, "class")) || hasSkipTypeof(attr(n, "typeof")) {
			return
		}
		if n.DataAtom == atom.A {
			p.recordLink(n)
		}
		if level := headingLevel(n.DataAtom); level > 0 {
			p.endBlock()
			p.walkChildren(n)
			if title := p.takeBlock(); title != "" {
				marker := strings.Repeat("=", level)
				p.lines = append(p.lines, marker+" "+title+" "+marker)
			}
			return
		}
		if blockAtoms[n.DataAtom] {
			p.endBlock()
			p.walkChildren(n)
			p.endBlock()
			return
		}
		if n.DataAtom == atom.Br {
			p.block.WriteString(" ")
		}
	}
	p.walkChildren(n)
}

func (p *parser) walkChildren(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		p.walk(c)
	}
}

// endBlock flushes the current block as one output line.
func (p *parser) endBlock() {
	if line := p.takeBlock(); line != "" {
		p.lines = append(p.lines, line)
	}
}

// emptyParens matches a whitespace-preceded empty parenthetical: the husk
// left behind when a skipped widget (a pronunciation audio button, a
// stripped inline image) was the parenthetical's whole content. Requiring
// the leading whitespace protects real text: "setup()" in code, a quoted
// "( )" in an article about punctuation, and the album titled "( )" at the
// start of a list line.
var emptyParens = regexp.MustCompile(`\s\(\s*\)`)

// takeBlock returns the current block's text with whitespace collapsed and
// stripped-widget husks removed, and resets the block.
func (p *parser) takeBlock() string {
	s := strings.Join(strings.Fields(p.block.String()), " ")
	// Removing a husk can expose an enclosing one (a stripped widget was
	// the sole content of a parenthetical inside another parenthetical),
	// so replace until nothing changes.
	for {
		t := emptyParens.ReplaceAllString(s, "")
		if t == s {
			break
		}
		s = t
	}
	p.block.Reset()
	return s
}

// recordLink records an anchor's target if it is a plain wiki link.
// Parsoid types its anchors: article links are rel="mw:WikiLink", while
// interwiki links ("mw:WikiLink/Interwiki"), external links ("mw:ExtLink")
// and media links carry other values, so the rel attribute — not pattern
// matching on the href — decides.
func (p *parser) recordLink(n *html.Node) {
	hasRel := false
	for _, rel := range strings.Fields(attr(n, "rel")) {
		if rel == "mw:WikiLink" {
			hasRel = true
		}
	}
	if !hasRel {
		return
	}
	// Prefer the title attribute (the canonical title, already unescaped);
	// fall back to decoding the "./Page_name" href.
	target := attr(n, "title")
	if target == "" {
		href := strings.TrimPrefix(attr(n, "href"), "./")
		href, _, _ = strings.Cut(href, "#")
		decoded, err := url.PathUnescape(href)
		if err != nil {
			decoded = href
		}
		target = strings.ReplaceAll(decoded, "_", " ")
	}
	if target != "" && !p.seen[target] {
		p.seen[target] = true
		p.links = append(p.links, target)
	}
}

func headingLevel(a atom.Atom) int {
	switch a {
	case atom.H1, atom.H2:
		return 2
	case atom.H3:
		return 3
	case atom.H4:
		return 4
	case atom.H5:
		return 5
	case atom.H6:
		return 6
	}
	return 0
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func hasSkipClass(class string) bool {
	for _, c := range strings.Fields(class) {
		for _, skip := range skipClasses {
			if c == skip {
				return true
			}
		}
	}
	return false
}

func hasSkipTypeof(typeof string) bool {
	for _, t := range strings.Fields(typeof) {
		for _, skip := range skipTypeof {
			if t == skip {
				return true
			}
		}
	}
	return false
}

package devui

// pages holds every page template. Each page includes the shared head and
// foot; the CSS is deliberately tiny — this is an inspection tool.
const pages = `
{{define "head"}}<!doctype html>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>wikifyp dev · {{.}}</title>
<style>
 body { font: 15px/1.5 system-ui, sans-serif; margin: 1.5rem auto; max-width: 70rem; padding: 0 1rem; color: #222; }
 a { color: #1a5fb4; text-decoration: none; } a:hover { text-decoration: underline; }
 h1, h2, h3 { line-height: 1.2; } h1 a { color: inherit; }
 table { border-collapse: collapse; } td, th { padding: .2rem .7rem .2rem 0; text-align: left; vertical-align: top; }
 th { border-bottom: 1px solid #ccc; }
 .muted { color: #777; } .mono { font-family: ui-monospace, monospace; font-size: 13px; }
 .idx { color: #999; font-family: ui-monospace, monospace; font-size: 12px; min-width: 2.5rem; display: inline-block; }
 .h2x { font-weight: 600; } .badge { background: #e8e4da; border-radius: 4px; padding: 0 .4rem; font-size: 12px; }
 .tok { background: #eef3fa; border: 1px solid #d5e0f0; border-radius: 4px; padding: 0 .25rem; margin: 0 .1rem;
        display: inline-block; font-family: ui-monospace, monospace; font-size: 13px; }
 .tok sub { color: #96a5bd; }
 .cols { display: flex; gap: 2rem; flex-wrap: wrap; } .cols > div { flex: 1 1 20rem; min-width: 0; }
 pre { background: #f6f4ef; padding: .8rem; border-radius: 6px; white-space: pre-wrap; overflow-wrap: anywhere; font-size: 13px; }
 form.inline { display: inline; } input[type=text], textarea { font: inherit; padding: .3rem .5rem; border: 1px solid #bbb; border-radius: 5px; }
 textarea { width: 100%; font-family: ui-monospace, monospace; font-size: 13px; }
 button { font: inherit; padding: .25rem .7rem; border: 1px solid #bbb; border-radius: 5px; background: #f3f1ec; cursor: pointer; }
 button:hover { background: #e9e6df; }
 nav { margin-bottom: 1.2rem; } nav a { margin-right: 1rem; }
 ul.plain { list-style: none; padding: 0; columns: 3 16rem; } ul.plain li { break-inside: avoid; }
 .gone { color: #a33; } .state { background: #fdf2d0; border-radius: 4px; padding: 0 .4rem; }
</style>
<nav><a href="/dev"><b>wikifyp dev</b></a> <a href="/dev/feed">feed simulator</a> <a href="/dev/pipeline">pipeline playground</a>
<form class="inline" action="/dev/search"><input type="text" name="q" placeholder="search titles…" value=""></form></nav>
{{end}}

{{define "home"}}{{template "head" "home"}}
<h1>Corpus</h1>
<table>
<tr><th>source</th><th>pages (latest)</th><th>stored revisions</th></tr>
{{range .Stats}}<tr><td>{{.Source}}</td><td>{{.Articles}}</td><td>{{.Revisions}}</td></tr>{{end}}
</table>
<p class="muted">{{.Sentences}} sentences · {{.Links}} links · {{.Cats}} category rows
{{if .FixturesDir}} · fixtures at <span class="mono">{{.FixturesDir}}</span>{{else}} · fixtures directory not found (raw views hidden){{end}}</p>
<h2>Largest categories</h2>
<ul class="plain">
{{range .TopCategories}}<li><a href="/dev/category/{{pathescape .Category}}">{{.Category}}</a> <span class="muted">{{.Count}}</span></li>{{end}}
</ul>
{{end}}

{{define "hits"}}
<table>
<tr><th>article</th><th>source</th><th>revision</th><th>sentences</th></tr>
{{range .}}<tr><td><a href="/dev/article/{{.ID}}">{{.Title}}</a></td><td>{{.Source}}</td>
<td class="mono">{{.RevisionID}}</td><td>{{.Sentences}}</td></tr>{{end}}
</table>
{{end}}

{{define "search"}}{{template "head" "search"}}
<h1>Search</h1>
<form action="/dev/search"><input type="text" name="q" value="{{.Query}}" autofocus> <button>Search</button></form>
{{if .Query}}{{if .Hits}}{{template "hits" .Hits}}{{else}}<p>No stored article title contains “{{.Query}}”.</p>{{end}}{{end}}
{{end}}

{{define "statebuttons"}}
<form class="inline" method="post" action="/dev/state">
<input type="hidden" name="reader" value="{{.Reader}}">
<input type="hidden" name="source" value="{{.Source}}">
<input type="hidden" name="page_id" value="{{.PageID}}">
<input type="hidden" name="back" value="{{.Back}}">
<button name="state" value="finished">finished</button>
<button name="state" value="saved">saved</button>
<button name="state" value="unsaved">unsaved</button>
<button name="state" value="skipped">skipped</button>
<button name="state" value="clear">clear</button>
</form>
{{end}}

{{define "article"}}{{template "head" .A.Title}}
<h1>{{.A.Title}} <span class="badge">{{.A.Source}}</span></h1>
<p class="muted mono">article {{.A.ID}} · page {{.A.PageID}} · revision {{.A.RevisionID}} ·
{{len .Sentences}} sentences · {{.Words}} words · {{.A.License}} · fetched {{.A.Fetched}} ·
<a href="{{.A.URL}}">on the wiki</a>{{if .HasFixture}} · <a href="/dev/article/{{.A.ID}}/raw">raw fixture vs stored</a>{{end}}</p>
<p class="muted">{{.A.Attribution}}</p>
<p>Reader <span class="mono">{{.Reader}}</span>{{if .State}} has this page <span class="state">{{.State}}</span>{{end}} —
{{template "statebuttons" .StateForm}}</p>
<p><a href="/dev/article/{{.A.ID}}{{if .ShowTokens}}{{else}}?tokens=1{{end}}">{{if .ShowTokens}}hide tokens{{else}}show tokens{{end}}</a></p>
<div class="cols">
<div style="flex: 2 1 30rem">
<h2>Sentences</h2>
{{$show := .ShowTokens}}{{$aid := .A.ID}}
{{range .Sentences}}
<p><span class="idx">{{.Idx}}</span>{{if .Heading}}<span class="badge">h{{.Heading}}</span> <span class="h2x">{{.Text}}</span>{{else}}{{if $show}}{{range $i, $t := .Tokens}}<span class="tok">{{$t}}<sub>{{$i}}</sub></span>{{end}}{{else}}{{.Text}}{{end}}{{end}}</p>
{{end}}
</div>
<div>
<h2>Links out <span class="muted">({{len .LinksOut}})</span></h2>
<ul class="plain">
{{range .LinksOut}}<li>{{if .ArticleID}}<a href="/dev/article/{{.ArticleID}}">{{.Target}}</a>{{else}}<span class="gone" title="not in the corpus">{{.Target}}</span>{{end}}</li>{{end}}
</ul>
<h2>Links in <span class="muted">({{len .LinksIn}}{{if .LinksInMore}}+{{end}})</span></h2>
<ul class="plain">
{{range .LinksIn}}<li><a href="/dev/article/{{.ID}}">{{.Title}}</a></li>{{end}}
</ul>
<h2>Categories</h2>
<ul class="plain">
{{range .Categories}}<li><a href="/dev/category/{{pathescape .}}">{{.}}</a></li>{{end}}
</ul>
</div>
</div>
{{end}}

{{define "raw"}}{{template "head" (printf "%s raw" .A.Title)}}
<h1><a href="/dev/article/{{.A.ID}}">{{.A.Title}}</a> — raw fixture vs cleaned</h1>
<p class="muted">Fixture revision {{.Fixture.RevisionID}}{{if not .SameRev}} — <b>the stored article is revision {{.A.RevisionID}}</b>, so the two sides are different revisions of the page{{end}}.</p>
<div class="cols">
<div><h2>Raw extract (fixture .txt)</h2><pre>{{.Fixture.Text}}</pre></div>
<div><h2>After Clean</h2><pre>{{.Cleaned}}</pre></div>
</div>
{{end}}

{{define "category"}}{{template "head" .Name}}
<h1>Category: {{.Name}}</h1>
{{if .Hits}}{{template "hits" .Hits}}{{else}}<p>No stored articles carry this category.</p>{{end}}
{{end}}

{{define "feed"}}{{template "head" "feed"}}
<h1>Feed simulator</h1>
<form action="/dev/feed"><label>Reader <input type="text" name="reader" value="{{.Reader}}"></label> <button>Load</button></form>
<p class="muted">Mark pages finished / saved / skipped from their article pages (search above), then reload. A reader with no state gets featured articles.</p>
{{if .State}}
<h2>State</h2>
<table><tr><th>page</th><th>source</th><th>state</th><th></th></tr>
{{$reader := .Reader}}
{{range .State}}<tr>
<td>{{if .ArticleID}}<a href="/dev/article/{{.ArticleID}}">{{.Title}}</a>{{else}}{{.Title}}{{end}}</td>
<td>{{.Source}}</td><td><span class="state">{{.State}}</span></td>
<td><form class="inline" method="post" action="/dev/state">
<input type="hidden" name="reader" value="{{$reader}}"><input type="hidden" name="source" value="{{.Source}}">
<input type="hidden" name="page_id" value="{{.PageID}}"><input type="hidden" name="back" value="/dev/feed?reader={{$reader}}">
<button name="state" value="clear">clear</button></form></td>
</tr>{{end}}</table>
{{end}}
<h2>Feed (what /v1/feed serves)</h2>
<table><tr><th>#</th><th>article</th><th>source</th><th>excerpt</th><th>words</th><th></th></tr>
{{range $i, $c := .Cards}}<tr><td class="muted">{{$i}}</td>
<td><a href="/dev/article/{{$c.ArticleID}}">{{$c.Title}}</a></td><td>{{$c.Source}}</td>
<td class="muted">{{$c.Excerpt}}</td><td>{{$c.Words}}</td>
<td>{{if $c.Exploration}}<span class="badge" title="exploration slot: picked from outside the interest neighbourhood">explore</span>{{end}}</td></tr>{{end}}
</table>
{{if .Interest}}
<h2>Interest weights <span class="muted">(1/df per category the state touches; the heaviest 25)</span></h2>
<ul class="plain">
{{range .Interest}}<li><a href="/dev/category/{{pathescape .Category}}">{{.Category}}</a> <span class="muted mono">{{printf "%.4f" .Weight}}</span></li>{{end}}
</ul>
{{end}}
{{if .Ranked}}
<h2>Ranking’s working <span class="muted">(relevance-scored candidate pool before diverse selection, top 100)</span></h2>
<table><tr><th>score</th><th>candidate</th><th>source</th><th>links</th><th>categories</th></tr>
{{range .Ranked}}<tr><td class="mono">{{printf "%.2f" .Score}}</td>
<td><a href="/dev/article/{{.ArticleID}}">{{.Title}}</a></td><td>{{.Source}}</td>
<td class="muted">{{len .Links}}</td><td class="muted">{{len .Categories}}</td></tr>{{end}}
</table>
{{end}}
{{end}}

{{define "pipeline"}}{{template "head" "pipeline"}}
<h1>Pipeline playground</h1>
<p class="muted">Paste extract-shaped text (what the MediaWiki API returns, headings as <span class="mono">== Title ==</span> lines)
or a dump record’s Parsoid HTML, and see exactly what ingest would store: Clean’s output, the sentence rows with their
stable indices, and each sentence’s derived tokens.</p>
<form method="post" action="/dev/pipeline">
<p><label><input type="radio" name="mode" value="extract" {{if eq .Mode "extract"}}checked{{end}}> extract text</label>
   <label><input type="radio" name="mode" value="parsoid" {{if eq .Mode "parsoid"}}checked{{end}}> Parsoid HTML</label>
   <label><input type="checkbox" name="tokens" value="1" {{if .ShowTokens}}checked{{end}}> show tokens</label></p>
<textarea name="input" rows="12" placeholder="Paste text or HTML…">{{.Input}}</textarea>
<p><button>Run</button></p>
</form>
{{if .Error}}<p class="gone">{{.Error}}</p>{{end}}
{{if .Ran}}
{{if eq .Mode "parsoid"}}
<h2>Parsoid → extract-shaped text{{if .Redirect}} <span class="badge">redirect page</span>{{end}}</h2>
<pre>{{.Extracted}}</pre>
{{if .Links}}<p class="muted">Links: {{range .Links}}<span class="badge">{{.}}</span> {{end}}</p>{{end}}
{{end}}
<h2>After Clean</h2>
<pre>{{.Cleaned}}</pre>
<h2>Sentence rows ({{len .Rows}})</h2>
{{$show := .ShowTokens}}
{{range .Rows}}
<p><span class="idx">{{.Idx}}</span>{{if .Heading}}<span class="badge">h{{.Heading}}</span> <span class="h2x">{{.Text}}</span>{{else}}{{if $show}}{{range $i, $t := tokens .Text}}<span class="tok">{{$t}}<sub>{{$i}}</sub></span>{{end}}{{else}}{{.Text}}{{end}}{{end}}</p>
{{end}}
{{end}}
{{end}}
`

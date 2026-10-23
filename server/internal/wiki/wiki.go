// Package wiki holds the handful of names the wikis themselves define and
// several parts of the system must agree on. It is a leaf package so that
// ingest, commands and the feed can all consult the same single copy.
package wiki

// FeaturedCategories names, per source, the categories a wiki's community
// uses to mark its reviewed best work. The feed seeds a brand-new reader
// from them and tops exploration up with them. The wikis flag these
// categories HIDDEN, like maintenance plumbing — so the junk filtering
// that trusts the hidden flag (ingest's category filter and
// fetch-categories' prune) must exempt them, or the featured pool is
// silently emptied: the dev corpus lost every featured marker for six
// rounds and a zero-history reader got an empty feed.
var FeaturedCategories = map[string][]string{
	"simplewiki": {"Very good articles", "Good articles"},
	"wikipedia":  {"Featured articles", "Good articles"},
}

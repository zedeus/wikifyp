Raw MediaWiki API responses, recorded verbatim during development with the
same query the ingest client sends (see mediawiki.go). They are the "raw API
samples" the cleaning and client tests run against — never edit them by hand;
re-record instead.

- photosynthesis-1/2/3.json — one continuation chain (links span three pages)
- exponentiation.json — math-heavy extract (307 displaystyle formulas)
- simplewiki-chemistry.json — Simple English Wikipedia response shape
- missing-page.json — response for a nonexistent title

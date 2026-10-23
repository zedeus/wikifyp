# Dump fixtures

`simplewiki-sample.json.tar.gz` holds six unmodified records from the
Wikimedia Enterprise HTML dump of the Simple English Wikipedia
(`simplewiki-NS0-20250320-ENTERPRISE-HTML.json.tar.gz`, run 2025-03-20,
https://dumps.wikimedia.org/other/enterprise_html/runs/20250320/), split
across two NDJSON members the way the real dump is. Each record carries its
own revision ID, license and attribution metadata. Article text is
CC BY-SA, attributed to the Simple English Wikipedia article named in the
record.

The records were chosen to pin dump-ingestion behavior:

- **Fossil word** — headings, a list, an empty reference section and a stub
  notice; `fossil-word.expected.txt` is the hand-written expected output of
  the Parsoid HTML conversion, authored by reading the record's HTML.
- **Cloverleaf quasar** — inline math and red links;
  `cloverleaf-quasar.expected.txt` as above.
- **Microeconomics** — ordinary short article with sections.
- **Red Hot Chili Peppers** — a large article in `Category:Very good
  articles`, simplewiki's featured marker (used by feed tests).
- **History of Canada** — a three-character stub ("Far").
- **Spraycan** — a malformed redirect (external-link syntax) that MediaWiki
  renders as a one-line "REDIRECT" page rather than treating as a redirect.

The expected `.txt` files are ground truth, never regenerated from code; if
conversion output changes, re-read the HTML and decide which side is wrong.

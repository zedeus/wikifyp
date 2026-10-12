package com.wikifyp.reader

/**
 * Derives tokens from sentence text by splitting on single spaces.
 *
 * This mirrors the server's derivation rule: stored text is normalized
 * (whitespace fields joined by single spaces via [normalize]), and tokens
 * are that text split on `" "`. The shared fixture at `fixtures/tokens.json`
 * pins agreement between this implementation and the Go server.
 */
fun tokenize(normalizedText: String): List<String> =
    if (normalizedText.isBlank()) emptyList() else normalizedText.split(" ")

/**
 * Regex matching the same whitespace characters as Go's `unicode.IsSpace`:
 * ASCII whitespace (`\s`), NEL (U+0085), NBSP (U+00A0), and all Unicode
 * separator categories (Zs, Zl, Zp). Compiled once and reused.
 */
private val WHITESPACE = Regex("[\\s\\u0085\\u00A0\\p{Z}]+")

/**
 * Normalizes text to its stored form: whitespace fields joined by single
 * spaces. This is the Kotlin equivalent of the Go `text.Normalize` function
 * (which uses `strings.Fields`). The whitespace definition matches Go's
 * `unicode.IsSpace` so token addresses agree across platforms.
 */
fun normalize(text: String): String = text.split(WHITESPACE).filter { it.isNotEmpty() }.joinToString(" ")

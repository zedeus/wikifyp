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
 * Normalizes text to its stored form: whitespace fields joined by single
 * spaces. This is the Kotlin equivalent of the Go `text.Normalize` function.
 */
fun normalize(text: String): String = text.trim().split(Regex("\\s+")).joinToString(" ")

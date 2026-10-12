package com.wikifyp.reader

/**
 * Calculates the display delay for a token based on WPM and word properties.
 *
 * The base delay is `60_000 / wpm` ms. Modifiers stack additively:
 * - Long words (>8 chars): +20% per char beyond 8
 * - Contains a digit: +30%
 * - Mid-sentence punctuation (ends with `,` `;` `:`): +40%
 * - Sentence-ending punctuation (ends with `.` `!` `?`): +60%
 * - Last token in the sentence: +40%
 */
fun delayMs(
    tokenText: String,
    isLastInSentence: Boolean,
    wpm: Int,
): Long {
    require(wpm > 0) { "WPM must be positive" }
    val baseMs = 60_000.0 / wpm

    var multiplier = 1.0

    // Long words
    val len = tokenText.length
    if (len > 8) {
        multiplier += 0.2 * (len - 8)
    }

    // Numbers
    if (tokenText.any { it.isDigit() }) {
        multiplier += 0.3
    }

    // Trailing punctuation
    val last = tokenText.lastOrNull()
    when (last) {
        ',', ';', ':' -> multiplier += 0.4
        '.', '!', '?' -> multiplier += 0.6
    }

    // Sentence end
    if (isLastInSentence) {
        multiplier += 0.4
    }

    return (baseMs * multiplier).toLong()
}

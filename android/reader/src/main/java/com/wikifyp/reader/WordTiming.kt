package com.wikifyp.reader

import kotlin.math.sqrt

/**
 * Calculates the display delay for a token in RSVP playback.
 *
 * The formula is informed by eye-tracking research and existing RSVP
 * implementations (Spritz, speedread, grunion-ai/rsvp, RSVP-Reader):
 *
 * ```
 * delay = baseMs × lengthFactor × punctuationFactor × digitFactor × sentenceEndFactor
 * ```
 *
 * Factors are **multiplicative** (each scales the result independently):
 *
 * - **Length** — concave (√) curve anchored so 5-letter words = 1.0×.
 *   Short function words (1–2 letters) display faster (~0.7–0.8×),
 *   reflecting that they are often skipped in natural reading; long words
 *   plateau around 1.5× because each extra character adds less processing
 *   time than the last (diminishing returns). Letter count excludes
 *   punctuation — "mat." is 3 letters, not 4.
 *
 * - **Punctuation** — sentence-ending marks (`.` `!` `?` `…`) get 1.5×;
 *   clause boundaries (`,` `;` `:`) get 1.3×. Handles trailing
 *   quotes/brackets so `word."` and `word.)` are recognised.
 *
 * - **Digits** — tokens containing a digit get 1.3×; numbers are denser
 *   than prose and require more processing.
 *
 * - **Sentence end** — the last token in a sentence gets 1.3× for the
 *   "wrap-up effect": readers need a structural pause to integrate the
 *   clause before moving on. In natural reading this shows up as ~20–30 ms
 *   of extra fixation time on a ~250 ms base; in RSVP, where there is no
 *   parafoveal preview, the pause needs to be proportionally larger.
 *
 * Sources:
 * - Rayner (1998): average fixation ~250 ms; length and frequency both
 *   affect duration independently.
 * - grunion-ai/rsvp: multiplicative 1.6×/1.3×/1.4×/1.2× for punctuation,
 *   clause, digits, long words.
 * - pasky/speedread: 3× sentence-end, 2× comma, √length scaling.
 * - RSVP-Reader (Edwin580): concave curve 1→0.75, 5→1.0, 12→1.25,
 *   cap 1.6; weights normalised per book.
 * - Wrap-up effect research: inflated fixations at clause/sentence
 *   boundaries across languages.
 */
fun delayMs(
    tokenText: String,
    isLastInSentence: Boolean,
    wpm: Int,
): Long {
    require(wpm > 0) { "WPM must be positive" }
    val baseMs = 60_000.0 / wpm

    val length = lengthFactor(tokenText)
    val punctuation = punctuationFactor(tokenText)
    val digit = if (tokenText.any { it.isDigit() }) 1.3 else 1.0
    val sentenceEnd = if (isLastInSentence) 1.3 else 1.0

    return (baseMs * length * punctuation * digit * sentenceEnd).toLong()
}

/**
 * Concave (√) length curve anchored so a 5-letter word = 1.0×.
 *
 * ```
 * factor = 0.5 + 0.5 × √(letterCount / 5)
 * ```
 *
 * | Letters | Factor |
 * |---------|--------|
 * | 1       |  0.72  |
 * | 3       |  0.89  |
 * | 5       |  1.00  |
 * | 8       |  1.13  |
 * | 12      |  1.27  |
 * | 20      |  1.50  |
 *
 * Letter count excludes punctuation so "mat." counts as 3.
 */
internal fun lengthFactor(tokenText: String): Double {
    val letters = tokenText.count { it.isLetterOrDigit() }
    if (letters <= 0) return 0.7
    return (0.5 + 0.5 * sqrt(letters.toDouble() / 5.0)).coerceAtMost(1.5)
}

/** Sentence-ending punctuation, optionally followed by closing quotes/brackets. */
private val SENTENCE_END = Regex("[.!?…][\"')\\]]*$")

/** Clause-boundary punctuation, optionally followed by closing quotes/brackets. */
private val CLAUSE_BOUNDARY = Regex("[,;:][\"')\\]]*$")

/**
 * Punctuation factor from the token's trailing characters.
 *
 * - Sentence end (`.` `!` `?` `…`): 1.5×
 * - Clause boundary (`,` `;` `:`): 1.3×
 * - Neither: 1.0×
 */
internal fun punctuationFactor(tokenText: String): Double =
    when {
        SENTENCE_END.containsMatchIn(tokenText) -> 1.5
        CLAUSE_BOUNDARY.containsMatchIn(tokenText) -> 1.3
        else -> 1.0
    }

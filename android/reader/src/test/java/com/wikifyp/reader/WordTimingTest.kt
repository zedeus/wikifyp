package com.wikifyp.reader

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import kotlin.math.sqrt

/**
 * Tests for the word timing formula. All factors are multiplicative:
 * `delay = base × length × punctuation × digit × sentenceEnd`.
 */
class WordTimingTest {
    /** At 250 WPM, base delay is 240ms. */
    private val wpm = 250
    private val baseMs = 60_000.0 / wpm // 240.0

    // --- Length curve ---

    @Test
    fun shortWordGetsBelowBaseline() {
        // "a" — 1 letter → ~0.72×
        val factor = lengthFactor("a")
        assertTrue("1-letter word should be below 1.0", factor < 1.0)
        assertClose(0.72, factor, 0.01)
    }

    @Test
    fun threeLetterWord() {
        // "the" — 3 letters → ~0.89×
        assertClose(0.89, lengthFactor("the"), 0.01)
    }

    @Test
    fun fiveLetterWordIsBaseline() {
        // "about" — 5 letters = 1.0× (anchor point)
        assertClose(1.0, lengthFactor("about"), 0.001)
    }

    @Test
    fun eightLetterWord() {
        // "absolute" — 8 letters → ~1.13×
        assertClose(1.13, lengthFactor("absolute"), 0.01)
    }

    @Test
    fun twelvePlusLetterWord() {
        // "internationally" — 15 letters → capped near 1.5×
        val factor = lengthFactor("internationally")
        assertTrue("long word should be above 1.2", factor > 1.2)
        assertTrue("long word should be at most 1.5", factor <= 1.5)
    }

    @Test
    fun lengthExcludesPunctuation() {
        // "mat." — 3 letters (not 4 chars)
        assertClose(lengthFactor("mat"), lengthFactor("mat."), 0.001)
    }

    @Test
    fun lengthCountsDigits() {
        // "3.14" — 3 letter-or-digit chars
        assertClose(lengthFactor("the"), lengthFactor("3.14"), 0.001)
    }

    // --- Punctuation ---

    @Test
    fun sentenceEndingPeriod() {
        assertClose(1.5, punctuationFactor("mat."), 0.001)
    }

    @Test
    fun sentenceEndingExclamation() {
        assertClose(1.5, punctuationFactor("Yes!"), 0.001)
    }

    @Test
    fun sentenceEndingQuestion() {
        assertClose(1.5, punctuationFactor("what?"), 0.001)
    }

    @Test
    fun sentenceEndingWithTrailingQuote() {
        // `word."` — period followed by closing quote
        assertClose(1.5, punctuationFactor("word.\""), 0.001)
    }

    @Test
    fun sentenceEndingWithTrailingBracket() {
        assertClose(1.5, punctuationFactor("word.)"), 0.001)
    }

    @Test
    fun clauseBoundaryComma() {
        assertClose(1.3, punctuationFactor("wait,"), 0.001)
    }

    @Test
    fun clauseBoundarySemicolon() {
        assertClose(1.3, punctuationFactor("however;"), 0.001)
    }

    @Test
    fun clauseBoundaryColon() {
        assertClose(1.3, punctuationFactor("follows:"), 0.001)
    }

    @Test
    fun noPunctuation() {
        assertClose(1.0, punctuationFactor("word"), 0.001)
    }

    // --- Digits ---

    @Test
    fun numberTokenGetsDigitFactor() {
        val withDigit = delayMs("42", isLastInSentence = false, wpm = wpm)
        val withoutDigit = delayMs("ab", isLastInSentence = false, wpm = wpm)
        // Digit factor is 1.3×; length factors are equal (both 2 letters)
        assertClose(1.3, withDigit.toDouble() / withoutDigit.toDouble(), 0.05)
    }

    // --- Sentence end ---

    @Test
    fun sentenceEndFactorApplies() {
        val atEnd = delayMs("runs", isLastInSentence = true, wpm = wpm)
        val notEnd = delayMs("runs", isLastInSentence = false, wpm = wpm)
        assertClose(1.3, atEnd.toDouble() / notEnd.toDouble(), 0.05)
    }

    // --- Multiplicative stacking ---

    @Test
    fun sentenceEndWithPeriodStacksMultiplicatively() {
        // "mat." — period (1.5) × sentence end (1.3) = 1.95×
        // compared to "mat" with no punctuation and not at sentence end
        val full = delayMs("mat.", isLastInSentence = true, wpm = wpm)
        val plain = delayMs("mat", isLastInSentence = false, wpm = wpm)
        val ratio = full.toDouble() / plain.toDouble()
        // 1.5 × 1.3 = 1.95
        assertClose(1.95, ratio, 0.1)
    }

    @Test
    fun longNumberAtSentenceEnd() {
        // "3.14159265." — 10 digits/letters, period, digit, sentence end
        // length(10) ≈ 1.21, period 1.5, digit 1.3, sentence end 1.3
        val delay = delayMs("3.14159265.", isLastInSentence = true, wpm = wpm)
        // Expected: 240 × 1.21 × 1.5 × 1.3 × 1.3 ≈ 735
        assertTrue("complex token should be well above base", delay > 600)
        assertTrue("complex token should not be absurdly long", delay < 900)
    }

    // --- WPM changes ---

    @Test
    fun wpmChangeScalesLinearly() {
        val at250 = delayMs("about", isLastInSentence = false, wpm = 250)
        val at500 = delayMs("about", isLastInSentence = false, wpm = 500)
        // 5-letter word, no modifiers: pure base scaling
        assertEquals(240L, at250) // 60000/250 × 1.0
        assertEquals(120L, at500) // 60000/500 × 1.0
    }

    @Test
    fun shortWordAtHighWpm() {
        // "a" at 300 WPM: base 200ms × 0.72 ≈ 144ms
        val delay = delayMs("a", isLastInSentence = false, wpm = 300)
        assertTrue("short word at high WPM should be under 200ms", delay < 200)
        assertTrue("short word at high WPM should be above 100ms", delay > 100)
    }

    @Test(expected = IllegalArgumentException::class)
    fun zeroWpmThrows() {
        delayMs("word", isLastInSentence = false, wpm = 0)
    }

    // --- Short words are faster than long words ---

    @Test
    fun shortFunctionWordFasterThanContentWord() {
        val theDelay = delayMs("the", isLastInSentence = false, wpm = wpm)
        val aboutDelay = delayMs("about", isLastInSentence = false, wpm = wpm)
        val beautifulDelay = delayMs("beautiful", isLastInSentence = false, wpm = wpm)

        assertTrue("'the' < 'about'", theDelay < aboutDelay)
        assertTrue("'about' < 'beautiful'", aboutDelay < beautifulDelay)
    }

    // --- Helpers ---

    private fun assertClose(
        expected: Double,
        actual: Double,
        tolerance: Double,
    ) {
        assertEquals(
            "expected ≈$expected ± $tolerance, got $actual",
            expected,
            actual,
            tolerance,
        )
    }
}

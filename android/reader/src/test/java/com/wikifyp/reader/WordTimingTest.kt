package com.wikifyp.reader

import org.junit.Assert.assertEquals
import org.junit.Test

/** Tests for the word timing formula. */
class WordTimingTest {
    /** At 250 WPM, base delay is 240ms. */
    private val wpm = 250
    private val baseMs = 60_000L / wpm // 240

    @Test
    fun shortPlainWord() {
        // "cat" — no modifiers, 1.0×
        assertEquals(baseMs, delayMs("cat", isLastInSentence = false, wpm = wpm))
    }

    @Test
    fun longWord() {
        // "beautiful" — 9 chars, 1 beyond 8 → +20% → 1.2×
        val expected = (baseMs * 1.2).toLong()
        assertEquals(expected, delayMs("beautiful", isLastInSentence = false, wpm = wpm))
    }

    @Test
    fun veryLongWord() {
        // "internationally" — 15 chars, 7 beyond 8 → +140% → 2.4×
        val expected = (baseMs * 2.4).toLong()
        assertEquals(expected, delayMs("internationally", isLastInSentence = false, wpm = wpm))
    }

    @Test
    fun numberToken() {
        // "42" — contains digit → +30% → 1.3×
        val expected = (baseMs * 1.3).toLong()
        assertEquals(expected, delayMs("42", isLastInSentence = false, wpm = wpm))
    }

    @Test
    fun yearWithComma() {
        // "1905," — digit +30%, comma +40% → 1.7×
        val expected = (baseMs * 1.7).toLong()
        assertEquals(expected, delayMs("1905,", isLastInSentence = false, wpm = wpm))
    }

    @Test
    fun midSentenceComma() {
        // "wait," — comma → +40% → 1.4×
        val expected = (baseMs * 1.4).toLong()
        assertEquals(expected, delayMs("wait,", isLastInSentence = false, wpm = wpm))
    }

    @Test
    fun semicolonPause() {
        // "however;" — semicolon → +40%, 8 chars exactly → no long-word bonus → 1.4×
        val expected = (baseMs * 1.4).toLong()
        assertEquals(expected, delayMs("however;", isLastInSentence = false, wpm = wpm))
    }

    @Test
    fun sentenceEndingPeriod() {
        // "mat." — period → +60% → 1.6×
        val expected = (baseMs * 1.6).toLong()
        assertEquals(expected, delayMs("mat.", isLastInSentence = false, wpm = wpm))
    }

    @Test
    fun exclamationMark() {
        // "Yes!" — exclamation → +60% → 1.6×
        val expected = (baseMs * 1.6).toLong()
        assertEquals(expected, delayMs("Yes!", isLastInSentence = false, wpm = wpm))
    }

    @Test
    fun questionMark() {
        // "what?" — question → +60% → 1.6×
        val expected = (baseMs * 1.6).toLong()
        assertEquals(expected, delayMs("what?", isLastInSentence = false, wpm = wpm))
    }

    @Test
    fun lastTokenInSentence() {
        // "mat." — period +60%, last in sentence +40% → 2.0×
        val expected = (baseMs * 2.0).toLong()
        assertEquals(expected, delayMs("mat.", isLastInSentence = true, wpm = wpm))
    }

    @Test
    fun lastTokenPlainWord() {
        // "runs" — last in sentence +40% only → 1.4×
        val expected = (baseMs * 1.4).toLong()
        assertEquals(expected, delayMs("runs", isLastInSentence = true, wpm = wpm))
    }

    @Test
    fun combinedLongAndDigit() {
        // "3.14159265" — 10 chars, 2 beyond 8 → +40% long, digit → +30% → 1.7×
        val expected = (baseMs * 1.7).toLong()
        assertEquals(expected, delayMs("3.14159265", isLastInSentence = false, wpm = wpm))
    }

    @Test
    fun combinedAllModifiers() {
        // "beautiful." — 10 chars, 2 beyond 8 → +40%, period → +60%, last → +40% → 2.4×
        val expected = (baseMs * 2.4).toLong()
        assertEquals(expected, delayMs("beautiful.", isLastInSentence = true, wpm = wpm))
    }

    @Test
    fun wpmChange() {
        // At 300 WPM, base = 200ms. "cat" plain → 200ms
        assertEquals(200L, delayMs("cat", isLastInSentence = false, wpm = 300))

        // At 150 WPM, base = 400ms. "cat" plain → 400ms
        assertEquals(400L, delayMs("cat", isLastInSentence = false, wpm = 150))
    }

    @Test
    fun wpmAffectsModifiedTokens() {
        // At 300 WPM, base = 200ms. "mat." period +60% → 320ms
        val expected = (200 * 1.6).toLong()
        assertEquals(expected, delayMs("mat.", isLastInSentence = false, wpm = 300))
    }

    @Test(expected = IllegalArgumentException::class)
    fun zeroWpmThrows() {
        delayMs("word", isLastInSentence = false, wpm = 0)
    }
}

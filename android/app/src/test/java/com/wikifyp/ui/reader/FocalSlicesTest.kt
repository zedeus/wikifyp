package com.wikifyp.ui.reader

import org.junit.Assert.assertEquals
import org.junit.Test

/** Slicing the focal code point out of hostile tokens. */
class FocalSlicesTest {
    @Test
    fun ordinaryWordSplitsAroundFocalLetter() {
        // "quick": focal index 1 → before "q", focal "u", after "ick".
        assertEquals(Triple("q", "u", "ick"), focalSlices("quick"))
    }

    @Test
    fun singleLetterIsAllFocal() {
        assertEquals(Triple("", "I", ""), focalSlices("I"))
    }

    @Test
    fun emptyTokenGivesEmptySlices() {
        assertEquals(Triple("", "", ""), focalSlices(""))
    }

    @Test
    fun emojiTokenKeepsTheSurrogatePairWhole() {
        // A DNA emoji is one code point in two UTF-16 units; the focal
        // slice must carry both or the glyph renders as mojibake.
        assertEquals(Triple("", "🧬", ""), focalSlices("🧬"))
    }

    @Test
    fun surrogateMidTokenNeverSplits() {
        // Whatever position the focal lands on, the slices never cut a
        // surrogate pair in half.
        for (word in listOf("(🧬)", "a🧬b", "🧬🧬", "x🧬")) {
            val (b, f, a) = focalSlices(word)
            assertEquals(word, b + f + a)
            for (s in listOf(b, f, a)) {
                if (s.isNotEmpty()) {
                    check(!s.first().isLowSurrogate()) { "$word split a pair: '$s'" }
                    check(!s.last().isHighSurrogate()) { "$word split a pair: '$s'" }
                }
            }
        }
    }

    @Test
    fun rejoinedSlicesGiveBackTheToken() {
        for (word in listOf("a", "ab", "word.", "(hello)", "🧬", "naïve", "עברית", "12,345")) {
            val (b, f, a) = focalSlices(word)
            assertEquals(word, b + f + a)
        }
    }
}

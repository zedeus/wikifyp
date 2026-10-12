package com.wikifyp.ui.reader

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Exhaustive test of the context-text computation. Checks every position
 * in a 500-token article to verify the above and below context always
 * shifts when the position changes, never crashes, and never shows
 * tokens that overlap with the focal word.
 */
class ContextTextTest {
    // Use padding so "tok001" never substring-matches "tok010" etc.
    private val tokens = (0 until 500).map { "tok${it.toString().padStart(3, '0')}" }

    @Test
    fun aboveContextChangesAtEveryPosition() {
        // For positions 4+ (where above text is non-empty), every single
        // position change should produce different above text.
        var unchanged = 0
        var prev = ""
        for (pos in 0 until tokens.size) {
            val text = aboveContextText(pos, tokens)
            if (pos >= 4 && prev.isNotEmpty() && text.isNotEmpty()) {
                if (text == prev) unchanged++
            }
            prev = text
        }
        assertEquals(
            "Above context should change at every position (found $unchanged unchanged)",
            0,
            unchanged,
        )
    }

    @Test
    fun belowContextChangesAtEveryPosition() {
        var unchanged = 0
        var prev = ""
        for (pos in 0 until tokens.size) {
            val text = belowContextText(pos, tokens)
            if (pos < tokens.size - 4 && prev.isNotEmpty() && text.isNotEmpty()) {
                if (text == prev) unchanged++
            }
            prev = text
        }
        assertEquals(
            "Below context should change at every position (found $unchanged unchanged)",
            0,
            unchanged,
        )
    }

    @Test
    fun noCrashAtBoundaries() {
        // Position 0, 1, 2, last, past-end — none should crash.
        for (pos in listOf(0, 1, 2, 3, tokens.size - 1, tokens.size, tokens.size + 10)) {
            aboveContextText(pos, tokens)
            belowContextText(pos, tokens)
        }
        // Empty token list.
        aboveContextText(0, emptyList())
        belowContextText(0, emptyList())
    }

    @Test
    fun aboveNeverContainsFocalWord() {
        for (pos in 0 until tokens.size) {
            val above = aboveContextText(pos, tokens)
            val focal = tokens[pos]
            // The above window ends at pos-2, so the focal word (at pos)
            // should never appear.
            if (above.isNotEmpty()) {
                assertTrue(
                    "Above at pos=$pos should not contain focal '$focal': $above",
                    !above.contains(focal) || focal.length <= 4,
                    // Short words like "word0" could match "word0" in "word01" etc.
                    // The unique naming (word0..word499) avoids this.
                )
            }
        }
    }

    @Test
    fun belowNeverContainsFocalWord() {
        for (pos in 0 until tokens.size) {
            val below = belowContextText(pos, tokens)
            val focal = tokens[pos]
            if (below.isNotEmpty()) {
                assertTrue(
                    "Below at pos=$pos should not contain focal '$focal': $below",
                    !below.contains(focal),
                )
            }
        }
    }

    @Test
    fun aboveEmptyForFirstFewPositions() {
        assertEquals("", aboveContextText(0, tokens))
        assertEquals("", aboveContextText(1, tokens))
        assertEquals("", aboveContextText(2, tokens))
    }

    @Test
    fun belowEmptyForLastFewPositions() {
        assertEquals("", belowContextText(tokens.size - 1, tokens))
        assertEquals("", belowContextText(tokens.size - 2, tokens))
        assertEquals("", belowContextText(tokens.size - 3, tokens))
    }

    @Test
    fun windowSizeNeverExceedsTwelve() {
        for (pos in 0 until tokens.size) {
            val above = aboveContextText(pos, tokens)
            val below = belowContextText(pos, tokens)
            if (above.isNotEmpty()) {
                val count = above.split(" ").size
                assertTrue("Above at pos=$pos has $count words (max 12)", count <= 12)
            }
            if (below.isNotEmpty()) {
                val count = below.split(" ").size
                assertTrue("Below at pos=$pos has $count words (max 12)", count <= 12)
            }
        }
    }
}

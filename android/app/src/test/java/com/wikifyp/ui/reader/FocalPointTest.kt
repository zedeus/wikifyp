package com.wikifyp.ui.reader

import org.junit.Assert.assertEquals
import org.junit.Test

class FocalPointTest {
    @Test
    fun emptyAndSingleCharReturnZero() {
        assertEquals(0, focalIndex(""))
        assertEquals(0, focalIndex("I"))
    }

    /** Standard ORP tiers, tested on clean words. */
    @Test
    fun orpTiers() {
        // length 2–5 → index 1
        assertEquals(1, focalIndex("is"))
        assertEquals(1, focalIndex("the"))
        assertEquals(1, focalIndex("word"))
        assertEquals(1, focalIndex("hello"))

        // length 6–9 → index 2
        assertEquals(2, focalIndex("jumped"))
        assertEquals(2, focalIndex("reading"))
        assertEquals(2, focalIndex("xxxxxxxx"))
        assertEquals(2, focalIndex("xxxxxxxxx"))

        // length 10–13 → index 3
        assertEquals(3, focalIndex("philosophy"))
        assertEquals(3, focalIndex("xxxxxxxxxxxx"))
        assertEquals(3, focalIndex("xxxxxxxxxxxxx"))

        // length 14+ → index 4
        assertEquals(4, focalIndex("xxxxxxxxxxxxxx"))
        assertEquals(4, focalIndex("acknowledgement"))
    }

    /** Punctuation is stripped for length calculation. */
    @Test
    fun punctuationIgnored() {
        // "mat." has 3 letters → index 1, same as "mat"
        assertEquals(focalIndex("mat"), focalIndex("mat."))
        // "word," has 4 letters → index 1
        assertEquals(1, focalIndex("word,"))
    }

    /** focalPosition maps back to the original string correctly. */
    @Test
    fun focalPositionSkipsPunctuation() {
        // "(hello" → ORP on "hello" (5 letters, index 1) → 'e' at original position 2
        assertEquals(2, focalPosition("(hello"))
        // "hello" → index 1 → 'e' at position 1
        assertEquals(1, focalPosition("hello"))
        // Simple words: position == index for clean words
        assertEquals(0, focalPosition("I"))
        assertEquals(1, focalPosition("is"))
    }

    @Test
    fun veryLongWordsCappedAtFour() {
        assertEquals(4, focalIndex("a".repeat(50)))
    }
}

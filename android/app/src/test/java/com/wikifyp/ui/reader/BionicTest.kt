package com.wikifyp.ui.reader

import org.junit.Assert.assertEquals
import org.junit.Test

class BionicTest {
    @Test
    fun shortWordsGetOneLetter() {
        assertEquals(1, bionicPrefixLength("a"))
        assertEquals(1, bionicPrefixLength("to"))
        assertEquals(1, bionicPrefixLength("the"))
    }

    @Test
    fun mediumWordsGetAboutFortyPercent() {
        assertEquals(2, bionicPrefixLength("quick")) // 5 × 0.4 = 2
        assertEquals(3, bionicPrefixLength("reading")) // 7 × 0.4 = 2.8 → 3
    }

    @Test
    fun longWordsScale() {
        assertEquals(5, bionicPrefixLength("extraordinary")) // 13 × 0.4 = 5.2 → 5
    }

    @Test
    fun tokensWithoutLettersGetNoPrefix() {
        assertEquals(0, bionicPrefixLength("42"))
        assertEquals(0, bionicPrefixLength("—"))
        assertEquals(0, bionicPrefixLength("3.14"))
    }

    @Test
    fun punctuatedWordStillGetsAPrefix() {
        assertEquals(2, bionicPrefixLength("(word")) // 5 chars × 0.4 = 2
    }
}

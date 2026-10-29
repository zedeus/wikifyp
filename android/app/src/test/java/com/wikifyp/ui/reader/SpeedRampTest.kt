package com.wikifyp.ui.reader

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class SpeedRampTest {
    @Test
    fun firstPlayStartsAtSixtyPercent() {
        assertEquals(0.6, rampScale(0, resuming = false), 1e-9)
    }

    @Test
    fun firstPlayReachesFullSpeedAtThirtyWords() {
        assertEquals(1.0, rampScale(30, resuming = false), 1e-9)
        assertEquals(1.0, rampScale(100, resuming = false), 1e-9)
    }

    @Test
    fun firstPlayRampIsMonotonic() {
        var last = 0.0
        for (w in 0..35) {
            val s = rampScale(w, resuming = false)
            assertTrue("scale at $w must not decrease", s >= last)
            last = s
        }
    }

    @Test
    fun resumeStartsHigherAndRampsShorter() {
        assertEquals(0.8, rampScale(0, resuming = true), 1e-9)
        assertEquals(1.0, rampScale(10, resuming = true), 1e-9)
        assertTrue(rampScale(5, resuming = true) > rampScale(5, resuming = false))
    }

    @Test
    fun negativeWordsClampToStart() {
        assertEquals(0.6, rampScale(-3, resuming = false), 1e-9)
    }
}

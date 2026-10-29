package com.wikifyp.reader

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * FakeClock must model real time for rescheduling chains: an action fires
 * with nowMs at its own deadline, so work it schedules relative to "now"
 * lands where production would put it. A clock that jumped to the target
 * first stalled the engine's word-advance loop after one step whenever a
 * test advanced across several words at once.
 */
class FakeClockTest {
    @Test
    fun rescheduledChainFiresAtEachDeadlineAcrossOneBigAdvance() {
        val clock = FakeClock()
        val fireTimes = mutableListOf<Long>()

        fun tick() {
            fireTimes.add(clock.nowMs())
            if (fireTimes.size < 5) clock.schedule(100) { tick() }
        }
        clock.schedule(100) { tick() }

        clock.advanceBy(10_000)

        assertEquals(listOf(100L, 200L, 300L, 400L, 500L), fireTimes)
        assertEquals(10_000L, clock.nowMs())
    }

    @Test
    fun actionsBeyondTheTargetStayPending() {
        val clock = FakeClock()
        var fired = 0
        clock.schedule(100) { fired++ }
        clock.schedule(300) { fired++ }

        clock.advanceBy(200)
        assertEquals(1, fired)

        clock.advanceBy(100)
        assertEquals(2, fired)
    }
}

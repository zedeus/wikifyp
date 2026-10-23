package com.wikifyp.ui.feed

import com.wikifyp.data.FakeFeedRepository
import com.wikifyp.data.FeedResult
import com.wikifyp.data.testCard
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Test
import java.io.IOException

class FeedStateTest {
    @Test
    fun cardsBecomeContent() =
        runTest {
            val state = loadFeed(FakeFeedRepository(onFeed = { FeedResult(listOf(testCard())) }))
            assertEquals(FeedState.Content(listOf(testCard()), offline = false), state)
        }

    @Test
    fun offlineResultKeepsItsFlag() =
        runTest {
            val state = loadFeed(FakeFeedRepository(onFeed = { FeedResult(listOf(testCard()), offline = true) }))
            assertEquals(FeedState.Content(listOf(testCard()), offline = true), state)
        }

    @Test
    fun noCardsIsEmpty() =
        runTest {
            assertEquals(FeedState.Empty, loadFeed(FakeFeedRepository(onFeed = { FeedResult(emptyList()) })))
        }

    @Test
    fun failureIsError() =
        runTest {
            assertEquals(FeedState.Error, loadFeed(FakeFeedRepository(onFeed = { throw IOException("no network") })))
        }

    @Test
    fun readingTimeRoundsUpAndFloorsAtOneMinute() {
        assertEquals("1 min", readingTime(0))
        assertEquals("1 min", readingTime(250))
        assertEquals("2 min", readingTime(251))
        assertEquals("5 min", readingTime(1200))
    }
}

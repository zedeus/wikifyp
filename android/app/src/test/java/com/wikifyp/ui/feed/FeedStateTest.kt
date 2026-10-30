package com.wikifyp.ui.feed

import com.wikifyp.data.FakeFeedRepository
import com.wikifyp.data.FeedResult
import com.wikifyp.data.testCard
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.IOException

class FeedStateTest {
    @Test
    fun cardsBecomeContent() =
        runTest {
            val state = loadFeed(FakeFeedRepository(onFeed = { _ -> FeedResult(listOf(testCard())) }))
            assertEquals(FeedState.Content(listOf(testCard()), offline = false), state)
        }

    @Test
    fun offlineResultKeepsItsFlag() =
        runTest {
            val state = loadFeed(FakeFeedRepository(onFeed = { _ -> FeedResult(listOf(testCard()), offline = true) }))
            assertEquals(FeedState.Content(listOf(testCard()), offline = true), state)
        }

    @Test
    fun noCardsIsEmpty() =
        runTest {
            assertEquals(FeedState.Empty, loadFeed(FakeFeedRepository(onFeed = { _ -> FeedResult(emptyList()) })))
        }

    @Test
    fun failureIsError() =
        runTest {
            assertEquals(
                FeedState.Error,
                loadFeed(
                    FakeFeedRepository(onFeed = { _ ->
                        throw IOException("no network")
                    }),
                ),
            )
        }

    @Test
    fun emptyPageWithExcludesIsNotEmptyState() =
        runTest {
            // When we have excludes but get no results, it means the end of
            // pagination, not an empty feed.
            val state =
                loadFeed(
                    FakeFeedRepository(onFeed = { _ -> FeedResult(emptyList()) }),
                    exclude = setOf(1L, 2L),
                )
            assertTrue("expected Content with canLoadMore=false", state is FeedState.Content)
            assertFalse((state as FeedState.Content).canLoadMore)
        }

    @Test
    fun excludeIsPassedToRepository() =
        runTest {
            var receivedExclude: Set<Long> = emptySet()
            loadFeed(
                FakeFeedRepository(onFeed = { exclude ->
                    receivedExclude = exclude
                    FeedResult(listOf(testCard()))
                }),
                exclude = setOf(10L, 20L),
            )
            assertEquals(setOf(10L, 20L), receivedExclude)
        }

    @Test
    fun readingTimeRoundsUpAndFloorsAtOneMinute() {
        assertEquals("1 min", readingTime(0))
        assertEquals("1 min", readingTime(250))
        assertEquals("2 min", readingTime(251))
        assertEquals("5 min", readingTime(1200))
    }
}

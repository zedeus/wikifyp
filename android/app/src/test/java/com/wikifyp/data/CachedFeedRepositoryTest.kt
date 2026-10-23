package com.wikifyp.data

import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.IOException

class CachedFeedRepositoryTest {
    @get:Rule val tmp = TemporaryFolder()

    private fun repo(remote: FakeFeedRepository) = CachedFeedRepository(remote, FeedCache(tmp.root))

    @Test
    fun feedIsCachedAndServedWhenTheNetworkFails() =
        runTest {
            val remote = FakeFeedRepository(onFeed = { FeedResult(listOf(testCard())) })
            val repo = repo(remote)

            val fresh = repo.feed()
            assertFalse(fresh.offline)
            assertEquals(listOf(testCard()), fresh.cards)

            remote.onFeed = { throw IOException("airplane mode") }
            val cached = repo.feed()
            assertTrue(cached.offline)
            assertEquals(fresh.cards, cached.cards)
        }

    @Test
    fun feedFailureWithoutCachePropagates() =
        runTest {
            val repo = repo(FakeFeedRepository(onFeed = { throw IOException("airplane mode") }))
            try {
                repo.feed()
                fail("feed() succeeded with no network and no cache")
            } catch (expected: IOException) {
            }
        }

    @Test
    fun articleIsCachedAndServedWhenTheNetworkFails() =
        runTest {
            val remote = FakeFeedRepository(onArticle = { testArticle(it) })
            val repo = repo(remote)

            assertEquals(testArticle(7), repo.article(7))

            remote.onArticle = { throw IOException("airplane mode") }
            assertEquals(testArticle(7), repo.article(7))
            try {
                repo.article(8)
                fail("article(8) succeeded with no network and no cache")
            } catch (expected: IOException) {
            }
        }

    @Test
    fun prefetchFillsTheCacheAndSwallowsFailures() =
        runTest {
            val remote =
                FakeFeedRepository(onArticle = { id ->
                    if (id == 2L) throw IOException("flaky") else testArticle(id)
                })
            val repo = repo(remote)

            repo.prefetch(listOf(testCard(id = 1), testCard(id = 2), testCard(id = 3)))

            remote.onArticle = { throw IOException("airplane mode") }
            assertEquals(testArticle(1), repo.article(1))
            assertEquals(testArticle(3), repo.article(3))
            try {
                repo.article(2)
                fail("article(2) was served without ever being cached")
            } catch (expected: IOException) {
            }
        }

    @Test
    fun prefetchSkipsArticlesAlreadyCached() =
        runTest {
            var fetches = 0
            val remote =
                FakeFeedRepository(onArticle = { id ->
                    fetches++
                    testArticle(id)
                })
            val repo = repo(remote)

            repo.article(1)
            repo.prefetch(listOf(testCard(id = 1), testCard(id = 2)))
            assertEquals(2, fetches)
        }

    @Test
    fun corruptCacheFilesAreIgnored() =
        runTest {
            val cache = FeedCache(tmp.root)
            tmp.newFile("feed.json").writeText("{not json")
            assertNull(cache.loadFeed())
            assertNull(cache.loadArticle(1))
        }

    @Test
    fun emptyRemoteFeedDoesNotClobberTheCache() =
        runTest {
            val remote = FakeFeedRepository(onFeed = { FeedResult(listOf(testCard())) })
            val repo = repo(remote)
            repo.feed()

            remote.onFeed = { FeedResult(emptyList()) }
            assertTrue(repo.feed().cards.isEmpty())

            remote.onFeed = { throw IOException("airplane mode") }
            assertEquals(listOf(testCard()), repo.feed().cards)
        }
}

package com.wikifyp.data

import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.json.Json
import java.io.File

/**
 * Disk cache of the last feed and of article text, so both stay readable
 * without network. Files live under [dir] (the app's cache or files dir);
 * everything is best-effort — a cache that fails to write must never break
 * a successful fetch.
 */
class FeedCache(
    private val dir: File,
) {
    private val feedFile get() = File(dir, "feed.json")

    private fun articleFile(id: Long) = File(dir, "articles/$id.json")

    suspend fun saveFeed(cards: List<FeedCard>) =
        write(feedFile, json.encodeToString(ListSerializer(FeedCard.serializer()), cards))

    suspend fun loadFeed(): List<FeedCard>? =
        read(feedFile)?.let {
            runCatching { json.decodeFromString<List<FeedCard>>(it) }.getOrNull()
        }

    suspend fun saveArticle(article: Article) =
        write(articleFile(article.id), json.encodeToString(Article.serializer(), article))

    suspend fun loadArticle(id: Long): Article? =
        read(articleFile(id))?.let {
            runCatching { json.decodeFromString<Article>(it) }.getOrNull()
        }

    suspend fun hasArticle(id: Long): Boolean = withContext(Dispatchers.IO) { articleFile(id).exists() }

    private suspend fun write(
        file: File,
        content: String,
    ) = withContext(Dispatchers.IO) {
        runCatching {
            file.parentFile?.mkdirs()
            // Write-then-rename so a crash mid-write never leaves a torn file.
            val tmp = File(file.parentFile, file.name + ".tmp")
            tmp.writeText(content)
            tmp.renameTo(file)
        }
    }

    private suspend fun read(file: File): String? =
        withContext(Dispatchers.IO) {
            runCatching { file.takeIf { it.exists() }?.readText() }.getOrNull()
        }

    private companion object {
        val json = Json { ignoreUnknownKeys = true }
    }
}

/**
 * A [FeedRepository] that keeps the last successful feed and every fetched
 * article on disk, and serves them when the network fails: the feed comes
 * back flagged offline, an article comes back as if fetched. Only when both
 * the network and the cache come up empty does the failure propagate.
 */
class CachedFeedRepository(
    private val remote: FeedRepository,
    private val cache: FeedCache,
) : FeedRepository {
    override suspend fun feed(): FeedResult =
        try {
            val result = remote.feed()
            // An empty feed is an anomaly (a fresh or misconfigured server),
            // not something worth losing the last good feed over.
            if (result.cards.isNotEmpty()) {
                cache.saveFeed(result.cards)
            }
            result
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            val cached = cache.loadFeed() ?: throw e
            FeedResult(cached, offline = true)
        }

    override suspend fun article(id: Long): Article =
        try {
            remote.article(id).also { cache.saveArticle(it) }
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            cache.loadArticle(id) ?: throw e
        }

    /**
     * Downloads any of [cards]'s articles not yet cached, so the last feed
     * stays readable offline. Failures are silently left for the next try.
     */
    override suspend fun prefetch(cards: List<FeedCard>) {
        for (card in cards) {
            if (!cache.hasArticle(card.id)) {
                try {
                    article(card.id)
                } catch (e: CancellationException) {
                    // runCatching here would swallow cancellation and keep
                    // the loop downloading in a cancelled scope.
                    throw e
                } catch (_: Exception) {
                    // Left for the next prefetch attempt.
                }
            }
        }
    }
}

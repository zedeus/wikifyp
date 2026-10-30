package com.wikifyp.data

import android.content.Context
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json

/**
 * The bundled fixture articles served as a feed: the app's offline default
 * until a server is configured. Cards are derived from the article text the
 * same way the server derives them (first prose sentence as the excerpt,
 * words = tokens of every sentence).
 */
class FixtureFeedRepository(
    context: Context,
) : FeedRepository {
    private val appContext = context.applicationContext
    private val mutex = Mutex()
    private var loaded: List<Article>? = null

    /** Reads and decodes the bundled articles once, off the main thread. */
    private suspend fun articles(): List<Article> =
        mutex.withLock {
            loaded ?: withContext(Dispatchers.IO) {
                val assets = appContext.assets
                (assets.list("articles") ?: emptyArray())
                    .filter { it.endsWith(".json") }
                    .sorted()
                    .map { filename ->
                        val text = assets.open("articles/$filename").bufferedReader().readText()
                        json.decodeFromString<Article>(text)
                    }
            }.also { loaded = it }
        }

    override suspend fun feed(exclude: Set<Long>): FeedResult =
        FeedResult(articles().map { it.toFeedCard("fixture") }.filter { it.id !in exclude })

    override suspend fun article(id: Long): Article =
        articles().firstOrNull { it.id == id }
            ?: throw NoSuchElementException("no bundled article $id")

    private companion object {
        val json = Json { ignoreUnknownKeys = true }
    }
}

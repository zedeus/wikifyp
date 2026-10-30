package com.wikifyp.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder

/**
 * The server's feed API. [baseUrl] is the server root without a trailing
 * slash (e.g. "http://10.0.2.2:8080"); [readerId] is this install's
 * pseudonymous reader ID.
 */
class HttpFeedRepository(
    private val baseUrl: String,
    private val readerId: String,
) : FeedRepository {
    @Serializable
    private data class FeedResponse(
        val articles: List<FeedCard>,
    )

    override suspend fun feed(exclude: Set<Long>): FeedResult {
        val encoded = URLEncoder.encode(readerId, Charsets.UTF_8.name())
        val url =
            buildString {
                append("$baseUrl/v1/feed?reader=$encoded")
                if (exclude.isNotEmpty()) {
                    append("&exclude=")
                    append(exclude.joinToString(","))
                }
            }
        val body = get(url)
        return FeedResult(json.decodeFromString<FeedResponse>(body).articles)
    }

    override suspend fun article(id: Long): Article = json.decodeFromString(get("$baseUrl/v1/articles/$id"))

    private suspend fun get(url: String): String =
        withContext(Dispatchers.IO) {
            val connection = URL(url).openConnection() as HttpURLConnection
            try {
                connection.connectTimeout = TIMEOUT_MS
                connection.readTimeout = TIMEOUT_MS
                if (connection.responseCode != HttpURLConnection.HTTP_OK) {
                    throw IOException("GET $url: HTTP ${connection.responseCode}")
                }
                connection.inputStream.bufferedReader().readText()
            } finally {
                connection.disconnect()
            }
        }

    private companion object {
        const val TIMEOUT_MS = 10_000
        val json = Json { ignoreUnknownKeys = true }
    }
}

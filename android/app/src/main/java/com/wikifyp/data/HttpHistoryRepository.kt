package com.wikifyp.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder

/**
 * Server-backed history, stats and interests. Mirrors the HTTP feed
 * repository's conventions: same base URL, same reader ID, same timeout.
 */
class HttpHistoryRepository(
    private val baseUrl: String,
    private val readerId: String,
) : HistoryRepository {
    override suspend fun history(
        cursor: String,
        search: String,
        date: String,
    ): HistoryPage {
        val encoded = URLEncoder.encode(readerId, Charsets.UTF_8.name())
        val url =
            buildString {
                append("$baseUrl/v1/history?reader=$encoded&limit=50")
                if (cursor.isNotEmpty()) append("&cursor=${URLEncoder.encode(cursor, Charsets.UTF_8.name())}")
                if (search.isNotEmpty()) append("&search=${URLEncoder.encode(search, Charsets.UTF_8.name())}")
                if (date.isNotEmpty()) append("&date=${URLEncoder.encode(date, Charsets.UTF_8.name())}")
            }
        return json.decodeFromString(get(url))
    }

    override suspend fun stats(): ReadingStats {
        val encoded = URLEncoder.encode(readerId, Charsets.UTF_8.name())
        return json.decodeFromString(get("$baseUrl/v1/stats?reader=$encoded"))
    }

    override suspend fun interests(): ReaderInterests {
        val encoded = URLEncoder.encode(readerId, Charsets.UTF_8.name())
        return json.decodeFromString(get("$baseUrl/v1/interests?reader=$encoded"))
    }

    override suspend fun deletePages(pages: List<PageRef>): DeleteResult {
        val encoded = URLEncoder.encode(readerId, Charsets.UTF_8.name())
        val body = json.encodeToString(pages)
        return json.decodeFromString(delete("$baseUrl/v1/history?reader=$encoded", body))
    }

    override suspend fun deleteAll(): DeleteResult {
        val encoded = URLEncoder.encode(readerId, Charsets.UTF_8.name())
        return json.decodeFromString(delete("$baseUrl/v1/history?reader=$encoded&all=1", null))
    }

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

    private suspend fun delete(
        url: String,
        body: String?,
    ): String =
        withContext(Dispatchers.IO) {
            val connection = URL(url).openConnection() as HttpURLConnection
            try {
                connection.requestMethod = "DELETE"
                connection.connectTimeout = TIMEOUT_MS
                connection.readTimeout = TIMEOUT_MS
                if (body != null) {
                    connection.doOutput = true
                    connection.setRequestProperty("Content-Type", "application/json")
                    connection.outputStream.bufferedWriter().use { it.write(body) }
                }
                if (connection.responseCode != HttpURLConnection.HTTP_OK) {
                    throw IOException("DELETE $url: HTTP ${connection.responseCode}")
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

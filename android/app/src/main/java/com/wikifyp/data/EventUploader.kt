package com.wikifyp.data

import android.util.Log
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder

/**
 * Drains the [EventQueue] by posting batches to `POST /v1/events`.
 *
 * The uploader is a suspend function that runs in a loop: it peeks a
 * batch from the queue, posts it to the server, and removes successfully
 * accepted events. On failure it backs off exponentially. On an empty
 * queue it polls at a calm interval.
 *
 * Pass an empty [baseUrl] to disable uploading (fixture mode).
 */
class EventUploader(
    private val queue: EventQueue,
    private val baseUrl: String,
    private val readerId: String,
) {
    /**
     * Runs the upload loop until cancelled. Call from a lifecycle-aware
     * coroutine scope.
     */
    suspend fun run() {
        if (baseUrl.isEmpty()) return
        var backoffMs = INITIAL_BACKOFF_MS
        while (true) {
            val batch = queue.peek(BATCH_SIZE)
            if (batch.isEmpty()) {
                delay(POLL_INTERVAL_MS)
                backoffMs = INITIAL_BACKOFF_MS
                continue
            }
            try {
                val ok = postEvents(batch)
                if (ok) {
                    // The server processed every event in the batch (ok,
                    // duplicate or rejected). Rejected events won't succeed
                    // on retry, so all are removed.
                    queue.remove(batch.map { it.id })
                    backoffMs = INITIAL_BACKOFF_MS
                } else {
                    delay(backoffMs)
                    backoffMs = (backoffMs * 2).coerceAtMost(MAX_BACKOFF_MS)
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                Log.w(TAG, "upload failed, backing off", e)
                delay(backoffMs)
                backoffMs = (backoffMs * 2).coerceAtMost(MAX_BACKOFF_MS)
            }
        }
    }

    /**
     * Posts a batch of events to the server. Returns true if the server
     * processed the batch (HTTP 200), false otherwise.
     */
    private suspend fun postEvents(batch: List<QueuedEvent>): Boolean =
        withContext(Dispatchers.IO) {
            val encoded = URLEncoder.encode(readerId, Charsets.UTF_8.name())
            val url = URL("$baseUrl/v1/events?reader=$encoded")
            val connection = url.openConnection() as HttpURLConnection
            try {
                connection.requestMethod = "POST"
                connection.doOutput = true
                connection.connectTimeout = TIMEOUT_MS
                connection.readTimeout = TIMEOUT_MS
                connection.setRequestProperty("Content-Type", "application/json; charset=utf-8")

                // Build the request body: {"events": [<raw json>, ...]}
                val body =
                    buildString {
                        append("""{"events":[""")
                        batch.forEachIndexed { i, event ->
                            if (i > 0) append(',')
                            append(event.json)
                        }
                        append("]}")
                    }

                connection.outputStream.bufferedWriter().use { it.write(body) }
                connection.responseCode == HttpURLConnection.HTTP_OK
            } finally {
                connection.disconnect()
            }
        }

    companion object {
        private const val TAG = "EventUploader"
        const val BATCH_SIZE = 50
        const val POLL_INTERVAL_MS = 30_000L
        const val INITIAL_BACKOFF_MS = 1_000L
        const val MAX_BACKOFF_MS = 300_000L // 5 minutes
        const val TIMEOUT_MS = 10_000
    }
}

package com.wikifyp.reader

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

/**
 * A single reading event emitted by the reader engine. Events are
 * append-only, versioned (`schema_version`) and deduplicated by [id]
 * (a client UUID). The `reader_id` is added by the server on upload,
 * not by the reader.
 */
@Serializable
data class ReadingEvent(
    val id: String,
    @SerialName("schema_version") val schemaVersion: Int = SCHEMA_VERSION,
    @SerialName("session_id") val sessionId: String,
    val type: EventType,
    val timestamp: Long,
    val token: TokenAddress,
    val wpm: Int,
    // scroll_back only:
    @SerialName("landed_token") val landedToken: TokenAddress? = null,
    val distance: Int? = null,
    @SerialName("dwell_ms") val dwellMs: Long? = null,
) {
    companion object {
        const val SCHEMA_VERSION = 1
    }
}

@Serializable
enum class EventType {
    @SerialName("play")
    PLAY,

    @SerialName("pause")
    PAUSE,

    @SerialName("scroll_back")
    SCROLL_BACK,

    @SerialName("ask")
    ASK,

    @SerialName("finish")
    FINISH,

    @SerialName("skip")
    SKIP,
}

/** JSON codec for reading events; lenient on unknown keys for forward compat. */
val eventJson: Json =
    Json {
        encodeDefaults = true
        ignoreUnknownKeys = true
        prettyPrint = true
    }

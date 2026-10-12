package com.wikifyp.reader

import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.fail
import org.junit.Test

/** Tests that ReadingEvent serialization matches the fixture examples. */
class ReadingEventTest {
    private fun fixtureJson(name: String): String =
        javaClass.classLoader!!
            .getResourceAsStream("events/$name")
            ?.bufferedReader()
            ?.readText()
            ?: error("fixture events/$name not on test classpath")

    // --- Valid fixtures deserialize correctly ---

    @Test
    fun playFixtureDeserializes() {
        val event = eventJson.decodeFromString<ReadingEvent>(fixtureJson("play.json"))
        assertEquals("a1b2c3d4-e5f6-7890-abcd-ef1234567890", event.id)
        assertEquals(1, event.schemaVersion)
        assertEquals(EventType.PLAY, event.type)
        assertEquals(TokenAddress(42, 0, 0), event.token)
        assertEquals(250, event.wpm)
        assertNull(event.landedToken)
        assertNull(event.distance)
        assertNull(event.dwellMs)
    }

    @Test
    fun pauseFixtureDeserializes() {
        val event = eventJson.decodeFromString<ReadingEvent>(fixtureJson("pause.json"))
        assertEquals(EventType.PAUSE, event.type)
        assertEquals(TokenAddress(42, 0, 4), event.token)
    }

    @Test
    fun scrollBackFixtureDeserializes() {
        val event = eventJson.decodeFromString<ReadingEvent>(fixtureJson("scroll_back.json"))
        assertEquals(EventType.SCROLL_BACK, event.type)
        assertEquals(TokenAddress(42, 1, 3), event.token)
        assertNotNull(event.landedToken)
        assertEquals(TokenAddress(42, 0, 2), event.landedToken)
        assertEquals(6, event.distance)
        assertEquals(1800L, event.dwellMs)
    }

    @Test
    fun askFixtureDeserializes() {
        val event = eventJson.decodeFromString<ReadingEvent>(fixtureJson("ask.json"))
        assertEquals(EventType.ASK, event.type)
    }

    @Test
    fun finishFixtureDeserializes() {
        val event = eventJson.decodeFromString<ReadingEvent>(fixtureJson("finish.json"))
        assertEquals(EventType.FINISH, event.type)
        assertEquals(280, event.wpm)
    }

    @Test
    fun skipFixtureDeserializes() {
        val event = eventJson.decodeFromString<ReadingEvent>(fixtureJson("skip.json"))
        assertEquals(EventType.SKIP, event.type)
    }

    // --- Round-trip: serialize then deserialize ---

    @Test
    fun roundTripPlayEvent() {
        val event =
            ReadingEvent(
                id = "test-id",
                sessionId = "test-session",
                type = EventType.PLAY,
                timestamp = 1000L,
                token = TokenAddress(1, 0, 0),
                wpm = 250,
            )
        val json = eventJson.encodeToString(ReadingEvent.serializer(), event)
        val decoded = eventJson.decodeFromString<ReadingEvent>(json)
        assertEquals(event, decoded)
    }

    @Test
    fun roundTripScrollBackEvent() {
        val event =
            ReadingEvent(
                id = "test-id-2",
                sessionId = "test-session",
                type = EventType.SCROLL_BACK,
                timestamp = 2000L,
                token = TokenAddress(1, 2, 5),
                wpm = 300,
                landedToken = TokenAddress(1, 1, 3),
                distance = 7,
                dwellMs = 1500L,
            )
        val json = eventJson.encodeToString(ReadingEvent.serializer(), event)
        val decoded = eventJson.decodeFromString<ReadingEvent>(json)
        assertEquals(event, decoded)
    }

    // --- Serialized JSON uses snake_case field names ---

    @Test
    fun serializedFieldNamesAreSnakeCase() {
        val event =
            ReadingEvent(
                id = "test",
                sessionId = "sess",
                type = EventType.PLAY,
                timestamp = 0,
                token = TokenAddress(1, 0, 0),
                wpm = 250,
            )
        val obj =
            eventJson.decodeFromString<JsonObject>(
                eventJson.encodeToString(ReadingEvent.serializer(), event),
            )
        assertNotNull("schema_version", obj["schema_version"])
        assertNotNull("session_id", obj["session_id"])
        assertEquals(1, obj["schema_version"]!!.jsonPrimitive.int)
    }

    // --- Invalid fixtures ---

    @Test
    fun missingIdFieldFails() {
        try {
            eventJson.decodeFromString<ReadingEvent>(fixtureJson("invalid-missing-id.json"))
            fail("expected deserialization to fail for missing id")
        } catch (_: SerializationException) {
            // expected
        }
    }

    @Test
    fun unknownEventTypeFails() {
        try {
            eventJson.decodeFromString<ReadingEvent>(fixtureJson("invalid-unknown-type.json"))
            fail("expected deserialization to fail for unknown type")
        } catch (_: SerializationException) {
            // expected
        }
    }

    @Test
    fun schemaVersionZeroDeserializesButIsDetectable() {
        // v0 is syntactically valid JSON — the reader can deserialize it,
        // but consumers should reject schema versions they don't understand.
        // The Go server (#7) will reject unknown versions; the reader emits
        // only v1.
        val event = eventJson.decodeFromString<ReadingEvent>(fixtureJson("invalid-v0.json"))
        assertEquals(0, event.schemaVersion)
    }
}

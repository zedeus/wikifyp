package com.wikifyp.data

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import com.wikifyp.reader.EventType
import com.wikifyp.reader.ReadingEvent
import com.wikifyp.reader.TokenAddress
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class EventQueueTest {
    private lateinit var queue: EventQueue

    @Before
    fun setUp() {
        val context = ApplicationProvider.getApplicationContext<Context>()
        // Delete any leftover database from a previous run.
        context.deleteDatabase("event_queue.db")
        queue = EventQueue(context)
    }

    @After
    fun tearDown() {
        queue.close()
    }

    private fun event(
        id: String,
        type: EventType = EventType.PLAY,
    ): ReadingEvent =
        ReadingEvent(
            id = id,
            sessionId = "test-session",
            type = type,
            timestamp = System.currentTimeMillis(),
            token = TokenAddress(articleId = 1, sentenceIdx = 0, tokenIdx = 0),
            wpm = 250,
        )

    @Test
    fun `enqueue and peek returns event`() {
        queue.enqueue(event("e1"))
        val peeked = queue.peek()
        assertEquals(1, peeked.size)
        assertEquals("e1", peeked[0].id)
    }

    @Test
    fun `peek respects FIFO order`() {
        queue.enqueue(event("e1"))
        queue.enqueue(event("e2"))
        queue.enqueue(event("e3"))
        val peeked = queue.peek()
        assertEquals(listOf("e1", "e2", "e3"), peeked.map { it.id })
    }

    @Test
    fun `remove deletes events`() {
        queue.enqueue(event("e1"))
        queue.enqueue(event("e2"))
        queue.remove(listOf("e1"))
        val peeked = queue.peek()
        assertEquals(1, peeked.size)
        assertEquals("e2", peeked[0].id)
    }

    @Test
    fun `duplicate enqueue is idempotent`() {
        queue.enqueue(event("e1"))
        queue.enqueue(event("e1"))
        assertEquals(1, queue.count())
    }

    @Test
    fun `count reflects queue size`() {
        assertEquals(0, queue.count())
        queue.enqueue(event("e1"))
        queue.enqueue(event("e2"))
        assertEquals(2, queue.count())
        queue.remove(listOf("e1"))
        assertEquals(1, queue.count())
    }

    @Test
    fun `peek limit is respected`() {
        repeat(10) { i -> queue.enqueue(event("e$i")) }
        assertEquals(3, queue.peek(limit = 3).size)
    }

    @Test
    fun `events survive reopen`() {
        queue.enqueue(event("e1"))
        queue.enqueue(event("e2"))
        queue.close()

        // Reopen the queue — events should still be there.
        val context = ApplicationProvider.getApplicationContext<Context>()
        queue = EventQueue(context)
        assertEquals(2, queue.count())
        assertEquals(listOf("e1", "e2"), queue.peek().map { it.id })
    }

    @Test
    fun `json roundtrips through the queue`() {
        val original = event("rt-1", EventType.PAUSE)
        queue.enqueue(original)
        val peeked = queue.peek()
        // The JSON should be valid and deserialize back.
        val decoded =
            com.wikifyp.reader.eventJson.decodeFromString(
                ReadingEvent.serializer(),
                peeked[0].json,
            )
        assertEquals(original.id, decoded.id)
        assertEquals(original.type, decoded.type)
        assertEquals(original.wpm, decoded.wpm)
    }
}

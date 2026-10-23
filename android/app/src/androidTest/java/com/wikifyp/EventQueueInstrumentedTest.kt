package com.wikifyp

import android.content.Context
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.wikifyp.data.EventQueue
import com.wikifyp.reader.EventType
import com.wikifyp.reader.ReadingEvent
import com.wikifyp.reader.TokenAddress
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Emulator test: events written to the queue survive reopening the
 * database (simulating a restart). This runs on a real device/emulator
 * to exercise the actual SQLite implementation, not Robolectric's.
 */
@RunWith(AndroidJUnit4::class)
class EventQueueInstrumentedTest {
    private lateinit var context: Context
    private lateinit var queue: EventQueue

    @Before
    fun setUp() {
        context = InstrumentationRegistry.getInstrumentation().targetContext
        context.deleteDatabase("event_queue_test.db")
        queue = EventQueue(context, "event_queue_test.db")
    }

    @After
    fun tearDown() {
        queue.close()
        context.deleteDatabase("event_queue_test.db")
    }

    private fun event(id: String): ReadingEvent =
        ReadingEvent(
            id = id,
            sessionId = "test-session",
            type = EventType.PLAY,
            timestamp = System.currentTimeMillis(),
            token = TokenAddress(articleId = 1, sentenceIdx = 0, tokenIdx = 0),
            wpm = 250,
        )

    @Test
    fun eventsSurviveReopen() {
        queue.enqueue(event("survive-1"))
        queue.enqueue(event("survive-2"))
        assertEquals(2, queue.count())

        // Close and reopen — simulates a restart.
        queue.close()
        queue = EventQueue(context, "event_queue_test.db")

        assertEquals(2, queue.count())
        val peeked = queue.peek()
        assertEquals(listOf("survive-1", "survive-2"), peeked.map { it.id })
    }
}

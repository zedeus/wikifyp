package com.wikifyp.ui.reader

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertTextEquals
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performTouchInput
import com.wikifyp.reader.EventType
import com.wikifyp.reader.FakeClock
import com.wikifyp.reader.ReadingEvent
import com.wikifyp.reader.SentenceTokens
import com.wikifyp.theme.WikifypTheme
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/**
 * Compose integration tests that exercise the reader UI with a controlled
 * clock. Each test creates a [ReaderScreen] with a [FakeClock] and verifies
 * gestures, state transitions, and emitted events.
 */
@RunWith(RobolectricTestRunner::class)
class ReaderScreenGestureTest {
    @get:Rule val compose = createComposeRule()

    private val sentences =
        listOf(
            SentenceTokens(listOf("The", "quick", "brown")),
            SentenceTokens(listOf("fox", "jumps")),
        )

    private fun setup(
        clock: FakeClock = FakeClock(),
        events: MutableList<ReadingEvent> = mutableListOf(),
    ): Pair<FakeClock, MutableList<ReadingEvent>> {
        compose.setContent {
            WikifypTheme {
                ReaderScreen(
                    articleTitle = "Test",
                    sentences = sentences,
                    articleId = 42L,
                    clock = clock,
                    onEvent = { events.add(it) },
                    initialWpm = 250,
                )
            }
        }
        return clock to events
    }

    @Test
    fun tapDoesNotAdvance() {
        val (clock, events) = setup()

        // A quick tap (under 200ms hold threshold) should not start playback.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        // Don't advance past hold threshold.
        clock.advanceBy(100)
        compose.waitForIdle()

        compose.onNodeWithTag("readerSurface").performTouchInput {
            up()
        }
        compose.waitForIdle()

        // No play event should have been emitted.
        assertTrue(
            "Tap should not emit play event",
            events.none { it.type == EventType.PLAY },
        )
        // Idle hint should still be visible (or the first word if loaded).
        // Since no playback started, the word hasn't changed from initial state.
    }

    @Test
    fun holdStartsPlayback() {
        val (clock, events) = setup()

        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        // Advance past hold threshold (200ms).
        clock.advanceBy(151)
        compose.waitForIdle()

        // A PLAY event should have been emitted.
        assertTrue(
            "Hold should emit play event",
            events.any { it.type == EventType.PLAY },
        )

        // The current word should now be displayed.
        compose.onNodeWithTag("currentWord").assertIsDisplayed()
    }

    @Test
    fun liftPausesPlayback() {
        val (clock, events) = setup()

        // Hold to start playing.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        clock.advanceBy(151)
        compose.waitForIdle()

        // Lift to pause.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            up()
        }
        compose.waitForIdle()

        assertTrue(
            "Lift should emit pause event",
            events.any { it.type == EventType.PAUSE },
        )
    }

    @Test
    fun doubleTapEmitsAskEvent() {
        val (clock, events) = setup()

        // First hold and lift to get into paused state.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        clock.advanceBy(151)
        compose.waitForIdle()
        compose.onNodeWithTag("readerSurface").performTouchInput {
            up()
        }
        compose.waitForIdle()

        // Double-tap: two quick taps within 300ms.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        clock.advanceBy(50)
        compose.onNodeWithTag("readerSurface").performTouchInput {
            up()
        }
        compose.waitForIdle()

        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        clock.advanceBy(50)
        compose.onNodeWithTag("readerSurface").performTouchInput {
            up()
        }
        compose.waitForIdle()

        assertTrue(
            "Double-tap should emit ask event",
            events.any { it.type == EventType.ASK },
        )
    }

    @Test
    fun dragBackScrollsAndResumeFromDragEnd() {
        val (clock, events) = setup()

        // Hold to start playing.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        clock.advanceBy(151)
        compose.waitForIdle()

        // Let a few words advance (250 wpm → ~240ms per word base).
        clock.advanceBy(1000)
        compose.waitForIdle()

        // Lift to pause.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            up()
        }
        compose.waitForIdle()

        val pauseEvents = events.filter { it.type == EventType.PAUSE }
        assertTrue("Should have paused", pauseEvents.isNotEmpty())

        // Clear events to track the drag.
        events.clear()

        // Start a drag: touch down, move left significantly.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
            // Drag left to scroll back.
            moveBy(Offset(-200f, 0f))
        }
        compose.waitForIdle()

        // Lift after drag — should emit scroll_back event.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            up()
        }
        compose.waitForIdle()

        val scrollBacks = events.filter { it.type == EventType.SCROLL_BACK }
        assertTrue(
            "Drag should emit scroll_back event",
            scrollBacks.isNotEmpty(),
        )

        // Now hold to resume from drag end.
        events.clear()
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        clock.advanceBy(151)
        compose.waitForIdle()

        assertTrue(
            "Hold after drag should resume playback",
            events.any { it.type == EventType.PLAY },
        )
    }

    @Test
    fun emittedEventsHaveCorrectFields() {
        val (clock, events) = setup()

        // Hold to start playing.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        clock.advanceBy(151)
        compose.waitForIdle()

        val playEvent = events.first { it.type == EventType.PLAY }
        assertEquals("Article ID should match", 42L, playEvent.token.articleId)
        assertEquals("Schema version should be 1", 1, playEvent.schemaVersion)
        assertEquals("WPM should match", 250, playEvent.wpm)
        assertTrue("Event ID should not be empty", playEvent.id.isNotEmpty())
        assertTrue(
            "Session ID should not be empty",
            playEvent.sessionId.isNotEmpty(),
        )
    }

    @Test
    fun playAndPauseEventsShareSessionId() {
        val (clock, events) = setup()

        // Play.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        clock.advanceBy(151)
        compose.waitForIdle()

        // Pause.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            up()
        }
        compose.waitForIdle()

        val play = events.first { it.type == EventType.PLAY }
        val pause = events.first { it.type == EventType.PAUSE }
        assertEquals(
            "Play and pause should share session ID",
            play.sessionId,
            pause.sessionId,
        )
    }
}

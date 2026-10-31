package com.wikifyp.reader

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test

/**
 * Clock-controlled tests for the reader engine. Every test uses a [FakeClock]
 * so time only advances explicitly, and a sequential ID generator so event
 * IDs are deterministic.
 */
class ReaderEngineTest {
    private lateinit var clock: FakeClock
    private val events = mutableListOf<ReadingEvent>()
    private var nextId = 0

    private lateinit var engine: ReaderEngine

    /** Two short sentences for most tests. */
    private val twoSentences =
        listOf(
            SentenceTokens(listOf("The", "cat", "sat.")),
            SentenceTokens(listOf("Dogs", "run.")),
        )

    @Before
    fun setUp() {
        clock = FakeClock()
        events.clear()
        nextId = 0
        engine =
            ReaderEngine(
                clock = clock,
                listener = { events.add(it) },
                idGenerator = { "evt-${nextId++}" },
            )
        engine.load(articleId = 42, sentences = twoSentences)
    }

    // --- Tap does not advance ---

    @Test
    fun tapInIdleDoesNotAdvance() {
        engine.onTouchDown()
        // Lift before hold threshold — this is a tap
        clock.advanceBy(100)
        engine.onTouchUp()

        assertEquals(ReaderState.IDLE, engine.state)
        assertEquals(0, events.size)
        assertEquals("The", engine.currentToken())
    }

    @Test
    fun tapWhilePausedDoesNotAdvance() {
        // First hold to start, then lift to pause
        holdAndPlay()
        engine.onTouchUp()
        assertEquals(ReaderState.PAUSED, engine.state)
        val posAfterPause = engine.currentPosition()
        events.clear()

        // Tap while paused
        engine.onTouchDown()
        clock.advanceBy(100)
        engine.onTouchUp()

        assertEquals(ReaderState.PAUSED, engine.state)
        assertEquals(posAfterPause, engine.currentPosition())
        assertEquals(0, events.size)
    }

    // --- Hold starts playback ---

    @Test
    fun holdInIdleStartsPlaying() {
        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)

        assertEquals(ReaderState.PLAYING, engine.state)
        assertEquals(1, events.count { it.type == EventType.PLAY })
    }

    @Test
    fun holdEmitsPlayEventWithCorrectToken() {
        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)

        val play = events.first { it.type == EventType.PLAY }
        assertEquals(TokenAddress(42, 0, 0), play.token)
        assertEquals(250, play.wpm)
    }

    @Test
    fun holdAfterPauseResumesPlaying() {
        holdAndPlay()
        engine.onTouchUp() // pause
        events.clear()

        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)

        assertEquals(ReaderState.PLAYING, engine.state)
        assertEquals(1, events.count { it.type == EventType.PLAY })
    }

    // --- Lift pauses on current word ---

    @Test
    fun liftWhilePlayingPausesOnCurrentWord() {
        holdAndPlay()
        val tokenBefore = engine.currentToken()

        engine.onTouchUp()

        assertEquals(ReaderState.PAUSED, engine.state)
        assertEquals(tokenBefore, engine.currentToken())
        assertEquals(1, events.count { it.type == EventType.PAUSE })
    }

    @Test
    fun pauseEventHasCurrentTokenAddress() {
        holdAndPlay()
        engine.onTouchUp()

        val pause = events.first { it.type == EventType.PAUSE }
        assertEquals(TokenAddress(42, 0, 0), pause.token)
    }

    // --- Playback advances through tokens ---

    @Test
    fun playbackAdvancesThroughTokens() {
        holdAndPlay()

        // Advance past first token ("The")
        val delay = delayMs("The", isLastInSentence = false, wpm = 250)
        clock.advanceBy(delay)

        assertEquals("cat", engine.currentToken())
    }

    @Test
    fun playbackCrossesSentenceBoundary() {
        holdAndPlay()

        // Advance through "The", "cat", "sat." (sentence 0)
        advanceTokens("The", false)
        advanceTokens("cat", false)
        advanceTokens("sat.", true) // last in sentence

        // Now on sentence 1, token 0
        assertEquals("Dogs", engine.currentToken())
    }

    @Test
    fun playbackFinishesArticle() {
        holdAndPlay()

        // Advance through all 5 tokens
        advanceTokens("The", false)
        advanceTokens("cat", false)
        advanceTokens("sat.", true)
        advanceTokens("Dogs", false)
        advanceTokens("run.", true)

        assertEquals(ReaderState.IDLE, engine.state)
        assertEquals(1, events.count { it.type == EventType.FINISH })
    }

    // --- Drag / scrub ---

    @Test
    fun dragFromPausedEntersScrubbing() {
        holdAndPlay()
        engine.onTouchUp() // pause
        events.clear()

        engine.onDragStart()
        assertEquals(ReaderState.SCRUBBING, engine.state)
    }

    @Test
    fun dragMoveChangesPosition() {
        holdAndPlay()
        // Advance to token 2 ("sat.")
        advanceTokens("The", false)
        advanceTokens("cat", false)
        engine.onTouchUp() // pause at "sat."
        events.clear()

        engine.onDragStart()
        engine.onDragMove(-2) // back 2 tokens

        assertEquals("The", engine.currentToken())
    }

    @Test
    fun dragMoveClampsToBounds() {
        holdAndPlay()
        engine.onTouchUp() // pause at "The"
        engine.onDragStart()

        // Try to drag before start
        engine.onDragMove(-10)
        assertEquals("The", engine.currentToken())
        assertEquals(0, engine.currentPosition())
    }

    @Test
    fun liftAfterDragEmitsScrollBack() {
        holdAndPlay()
        advanceTokens("The", false)
        advanceTokens("cat", false)
        // Now at "sat." (position 2)
        engine.onTouchUp() // pause
        events.clear()

        engine.onDragStart()
        clock.advanceBy(500) // dwell time
        engine.onDragMove(-2) // back to "The"
        engine.onTouchUp()

        assertEquals(ReaderState.PAUSED, engine.state)
        val scrollBack = events.first { it.type == EventType.SCROLL_BACK }
        assertEquals(TokenAddress(42, 0, 2), scrollBack.token) // drag started at
        assertEquals(TokenAddress(42, 0, 0), scrollBack.landedToken) // landed at
        assertEquals(2, scrollBack.distance)
        assertEquals(500L, scrollBack.dwellMs)
    }

    @Test
    fun holdAfterDragResumesFromDragPosition() {
        holdAndPlay()
        advanceTokens("The", false)
        advanceTokens("cat", false)
        engine.onTouchUp() // pause at "sat."
        events.clear()

        engine.onDragStart()
        engine.onDragMove(-1) // back to "cat"
        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)

        assertEquals(ReaderState.PLAYING, engine.state)
        assertEquals("cat", engine.currentToken())
        // Should have both scroll_back and play events
        assertEquals(1, events.count { it.type == EventType.SCROLL_BACK })
        assertEquals(1, events.count { it.type == EventType.PLAY })
    }

    // --- Double-tap ---

    @Test
    fun doubleTapEmitsAskEvent() {
        holdAndPlay()
        engine.onDoubleTap()

        val ask = events.first { it.type == EventType.ASK }
        assertEquals(TokenAddress(42, 0, 0), ask.token)
    }

    @Test
    fun doubleTapInIdleDoesNothing() {
        engine.onDoubleTap()
        assertEquals(0, events.size)
    }

    // --- Skip ---

    @Test
    fun skipEmitsSkipEvent() {
        holdAndPlay()
        engine.skip()

        assertEquals(ReaderState.IDLE, engine.state)
        assertEquals(1, events.count { it.type == EventType.SKIP })
    }

    // --- Session ID ---

    @Test
    fun allEventsInOnePlaybackShareSessionId() {
        holdAndPlay()
        engine.onTouchUp() // pause
        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS) // resume

        val sessionIds = events.map { it.sessionId }.toSet()
        assertEquals("all events should share one session", 1, sessionIds.size)
    }

    // --- WPM ---

    @Test
    fun changingWpmAffectsSubsequentEvents() {
        engine.wpm = 300
        holdAndPlay()

        val play = events.first { it.type == EventType.PLAY }
        assertEquals(300, play.wpm)
    }

    // --- Empty content ---

    @Test
    fun noContentLoadedReturnsNullToken() {
        val empty = ReaderEngine(clock, { events.add(it) })
        assertNull(empty.currentToken())
    }

    // --- Helpers ---

    /** Hold past threshold to start playing. */
    private fun holdAndPlay() {
        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)
    }

    // --- Resume position ---

    @Test
    fun loadWithStartPositionResumesAtToken() {
        // Position 3 = "Dogs" (first sentence has 3 tokens: The, cat, sat.)
        engine.load(articleId = 42, sentences = twoSentences, startPosition = 3)
        assertEquals(ReaderState.IDLE, engine.state)
        assertEquals("Dogs", engine.currentToken())
        assertEquals(3, engine.currentPosition())
    }

    @Test
    fun loadWithStartPositionClampsToEnd() {
        // Total tokens = 5; position 99 clamps to 4 ("run.")
        engine.load(articleId = 42, sentences = twoSentences, startPosition = 99)
        assertEquals("run.", engine.currentToken())
        assertEquals(4, engine.currentPosition())
    }

    @Test
    fun loadWithStartPositionZeroStartsAtBeginning() {
        engine.load(articleId = 42, sentences = twoSentences, startPosition = 0)
        assertEquals("The", engine.currentToken())
        assertEquals(0, engine.currentPosition())
    }

    @Test
    fun resumedEngineStartsPlayingFromResumePosition() {
        engine.load(articleId = 42, sentences = twoSentences, startPosition = 3)
        assertEquals("Dogs", engine.currentToken())
        // Hold to play.
        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS + 1)
        assertEquals(ReaderState.PLAYING, engine.state)
        // The play event's token should be at the resume position.
        val playEvent = events.firstOrNull { it.type == EventType.PLAY }
        assertEquals(1, playEvent?.token?.sentenceIdx) // sentence 1
        assertEquals(0, playEvent?.token?.tokenIdx) // token 0 = "Dogs"
    }

    /** Advance the clock by the delay for one token. */
    private fun advanceTokens(
        tokenText: String,
        isLastInSentence: Boolean,
    ) {
        val delay = delayMs(tokenText, isLastInSentence, engine.wpm)
        clock.advanceBy(delay)
    }
}

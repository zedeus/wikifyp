package com.wikifyp.reader

import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test

/**
 * Tests for [ReaderEngine.seekTo] (browse-mode word jumps) and the
 * injected speed-scale policy (the UI layer's ramp).
 */
class ReaderEngineSeekTest {
    private lateinit var clock: FakeClock
    private val events = mutableListOf<ReadingEvent>()
    private var nextId = 0

    private lateinit var engine: ReaderEngine

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

    private fun holdAndPlay() {
        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)
    }

    private fun startScrubbing() {
        holdAndPlay()
        engine.onTouchUp() // pause
        engine.onDragStart()
        assertEquals(ReaderState.SCRUBBING, engine.state)
    }

    // --- save ---

    @Test
    fun saveEmitsEventAtCurrentPositionWithoutChangingState() {
        startScrubbing()
        engine.seekTo(3) // "Dogs"
        engine.onTouchUp() // pause there
        events.clear()

        engine.save()

        assertEquals(ReaderState.PAUSED, engine.state)
        assertEquals(1, events.size)
        assertEquals(EventType.SAVE, events[0].type)
        assertEquals(TokenAddress(42, 1, 0), events[0].token)
    }

    @Test
    fun saveWorksFromIdleBeforeAnyPlay() {
        engine.save()
        assertEquals(ReaderState.IDLE, engine.state)
        assertEquals(listOf(EventType.SAVE), events.map { it.type })
        assertEquals(TokenAddress(42, 0, 0), events[0].token)
    }

    @Test
    fun saveWithNoContentEmitsNothing() {
        engine.load(articleId = 7, sentences = emptyList())
        engine.save()
        assertEquals(0, events.size)
    }

    @Test
    fun unsaveEmitsEventWithoutChangingState() {
        engine.unsave()
        assertEquals(ReaderState.IDLE, engine.state)
        assertEquals(listOf(EventType.UNSAVE), events.map { it.type })
    }

    // --- jumpTo ---

    @Test
    fun jumpToLandsPausedAndReportsScrollBack() {
        holdAndPlay()
        engine.onTouchUp() // pause at 0
        events.clear()

        engine.jumpTo(3)

        assertEquals(ReaderState.PAUSED, engine.state)
        assertEquals(3, engine.currentPosition())
        assertEquals(listOf(EventType.SCROLL_BACK), events.map { it.type })
        assertEquals(TokenAddress(42, 1, 0), events[0].landedToken)
    }

    @Test
    fun jumpToSamePositionEmitsNothing() {
        holdAndPlay()
        engine.onTouchUp()
        events.clear()

        engine.jumpTo(engine.currentPosition())

        assertEquals(ReaderState.PAUSED, engine.state)
        assertEquals(0, events.size)
    }

    // --- seekTo ---

    @Test
    fun seekToMovesPositionWhileScrubbing() {
        startScrubbing()
        engine.seekTo(3)
        assertEquals(3, engine.currentPosition())
        assertEquals("Dogs", engine.currentToken())
    }

    @Test
    fun seekToClampsBelowZero() {
        startScrubbing()
        engine.seekTo(-5)
        assertEquals(0, engine.currentPosition())
    }

    @Test
    fun seekToClampsPastEnd() {
        startScrubbing()
        engine.seekTo(99)
        assertEquals(4, engine.currentPosition())
        assertEquals("run.", engine.currentToken())
    }

    @Test
    fun seekToIgnoredWhenNotScrubbing() {
        holdAndPlay()
        engine.onTouchUp() // paused
        val pos = engine.currentPosition()
        engine.seekTo(3)
        assertEquals(pos, engine.currentPosition())
    }

    @Test
    fun seekThenLiftEmitsScrollBackWithLandedToken() {
        startScrubbing()
        events.clear()
        engine.seekTo(3)
        clock.advanceBy(500)
        engine.onTouchUp()

        val back = events.single { it.type == EventType.SCROLL_BACK }
        // Scrub started at position 0 (sentence 0, token 0); landed on
        // flat index 3 (sentence 1, token 0).
        assertEquals(TokenAddress(42, 0, 0), back.token)
        assertEquals(TokenAddress(42, 1, 0), back.landedToken)
        assertEquals(3, back.distance)
        assertEquals(500L, back.dwellMs)
        assertEquals(ReaderState.PAUSED, engine.state)
    }

    @Test
    fun seekBackToStartThenLiftEmitsNoEvent() {
        startScrubbing()
        events.clear()
        engine.seekTo(2)
        engine.seekTo(engine.currentPosition() - 2) // back where scrub began
        engine.onTouchUp()
        assertEquals(0, events.size)
    }

    @Test
    fun holdWhileScrubbingFromIdleStartsPlayingAtSeekTarget() {
        engine.onDragStart() // browse opened before the first hold
        engine.seekTo(3)
        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)

        assertEquals(ReaderState.PLAYING, engine.state)
        assertEquals("Dogs", engine.currentToken())
        assertEquals(1, events.count { it.type == EventType.SCROLL_BACK })
    }

    // --- speed scale policy ---

    /** An engine whose injected policy records its inputs. */
    private fun scaledEngine(
        scale: Double,
        calls: MutableList<Pair<Int, Boolean>> = mutableListOf(),
    ): ReaderEngine =
        ReaderEngine(
            clock = clock,
            listener = { events.add(it) },
            idGenerator = { "evt-${nextId++}" },
            speedScaleFor = { words, resuming ->
                calls.add(words to resuming)
                scale
            },
        ).also { it.load(articleId = 42, sentences = twoSentences) }

    @Test
    fun scaleBelowOneSlowsAdvance() {
        val normalDelay = delayMs("The", isLastInSentence = false, wpm = 250)
        val eng = scaledEngine(0.5)
        eng.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)

        // At half speed the word shows for twice the normal delay.
        clock.advanceBy(normalDelay)
        assertEquals("The", eng.currentToken())
        clock.advanceBy(normalDelay)
        assertEquals("cat", eng.currentToken())
    }

    @Test
    fun defaultPolicyKeepsNormalDelay() {
        val normalDelay = delayMs("The", isLastInSentence = false, wpm = 250)
        holdAndPlay()
        clock.advanceBy(normalDelay)
        assertEquals("cat", engine.currentToken())
    }

    @Test
    fun policySeesWordsSincePlayAndResume() {
        val calls = mutableListOf<Pair<Int, Boolean>>()
        val eng = scaledEngine(1.0, calls)

        // First play: two words, then pause.
        eng.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)
        clock.advanceBy(delayMs("The", false, 250))
        eng.onTouchUp()
        assertEquals(listOf(0 to false, 1 to false), calls)

        // Resume: the policy is told this play is a resume, counting
        // words from the resume point.
        calls.clear()
        eng.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)
        assertEquals(listOf(0 to true), calls)
    }

    @Test
    fun reloadResetsResumeFlag() {
        val calls = mutableListOf<Pair<Int, Boolean>>()
        val eng = scaledEngine(1.0, calls)
        eng.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)
        eng.onTouchUp()

        eng.load(articleId = 43, sentences = twoSentences)
        calls.clear()
        eng.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)
        assertEquals(listOf(0 to false), calls)
    }

    @Test
    fun eventsKeepUserWpmUnderScale() {
        val eng = scaledEngine(0.6)
        eng.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)
        val play = events.single { it.type == EventType.PLAY }
        assertEquals(250, play.wpm)
    }
}

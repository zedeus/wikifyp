package com.wikifyp.reader

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/**
 * Edge cases and hostile inputs for the reader engine: empty articles,
 * one-word sentences, rapid gestures, boundary conditions.
 */
class ReaderEngineEdgeCaseTest {
    private lateinit var clock: FakeClock
    private val events = mutableListOf<ReadingEvent>()
    private var nextId = 0
    private lateinit var engine: ReaderEngine

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
    }

    // --- Empty and minimal content ---

    @Test
    fun emptyArticleHoldStaysIdle() {
        engine.load(articleId = 1, sentences = emptyList())
        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)

        assertEquals(ReaderState.IDLE, engine.state)
        assertEquals(0, events.size)
    }

    @Test
    fun singleTokenArticleFinishesImmediately() {
        engine.load(articleId = 1, sentences = listOf(SentenceTokens(listOf("Hello"))))
        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)

        assertEquals(ReaderState.PLAYING, engine.state)
        // Advance past the one token
        val delay = delayMs("Hello", isLastInSentence = true, wpm = 250)
        clock.advanceBy(delay)

        assertEquals(ReaderState.IDLE, engine.state)
        assertEquals(1, events.count { it.type == EventType.FINISH })
    }

    @Test
    fun hasFinishedTracksFinishAndResetsOnLoad() {
        engine.load(articleId = 1, sentences = listOf(SentenceTokens(listOf("Hello"))))
        assertEquals(false, engine.hasFinished)
        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)
        clock.advanceBy(delayMs("Hello", isLastInSentence = true, wpm = 250))

        // Finished: the flag is set and survives a post-finish browse
        // (scrub), so the UI can tell a finished read from an abandoned
        // one and suppress the skip event on exit.
        assertEquals(true, engine.hasFinished)
        engine.onDragStart()
        engine.onTouchUp()
        assertEquals(ReaderState.PAUSED, engine.state)
        assertEquals(true, engine.hasFinished)

        // A new load starts over.
        engine.load(articleId = 2, sentences = listOf(SentenceTokens(listOf("World"))))
        assertEquals(false, engine.hasFinished)
    }

    @Test
    fun oneWordSentences() {
        engine.load(
            articleId = 1,
            sentences =
                listOf(
                    SentenceTokens(listOf("One.")),
                    SentenceTokens(listOf("Two.")),
                    SentenceTokens(listOf("Three.")),
                ),
        )
        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)

        // Advance through all three one-word sentences
        for (word in listOf("One.", "Two.", "Three.")) {
            val delay = delayMs(word, isLastInSentence = true, wpm = 250)
            clock.advanceBy(delay)
        }

        assertEquals(ReaderState.IDLE, engine.state)
        assertEquals(1, events.count { it.type == EventType.FINISH })
    }

    // --- Rapid gestures ---

    @Test
    fun rapidTapTapDoesNothing() {
        engine.load(articleId = 1, sentences = listOf(SentenceTokens(listOf("A", "B"))))
        // Rapid tap-tap (both under hold threshold)
        engine.onTouchDown()
        clock.advanceBy(50)
        engine.onTouchUp()
        engine.onTouchDown()
        clock.advanceBy(50)
        engine.onTouchUp()

        assertEquals(ReaderState.IDLE, engine.state)
        assertEquals(0, events.size)
    }

    @Test
    fun liftBeforeHoldThresholdIsTap() {
        engine.load(articleId = 1, sentences = listOf(SentenceTokens(listOf("Word"))))
        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS - 1)
        engine.onTouchUp()

        assertEquals(ReaderState.IDLE, engine.state)
        assertEquals(0, events.size)
    }

    @Test
    fun holdExactlyAtThreshold() {
        engine.load(articleId = 1, sentences = listOf(SentenceTokens(listOf("Word"))))
        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)

        assertEquals(ReaderState.PLAYING, engine.state)
    }

    // --- Drag edge cases ---

    @Test
    fun dragPastStartClamps() {
        engine.load(articleId = 1, sentences = listOf(SentenceTokens(listOf("A", "B", "C"))))
        holdAndPlay()
        engine.onTouchUp()
        engine.onDragStart()
        engine.onDragMove(-100)

        assertEquals(0, engine.currentPosition())
        assertEquals("A", engine.currentToken())
    }

    @Test
    fun dragPastEndClamps() {
        engine.load(articleId = 1, sentences = listOf(SentenceTokens(listOf("A", "B", "C"))))
        holdAndPlay()
        engine.onTouchUp()
        engine.onDragStart()
        engine.onDragMove(100)

        assertEquals(2, engine.currentPosition())
        assertEquals("C", engine.currentToken())
    }

    @Test
    fun dragInIdleScrubsFromStart() {
        // Dragging an unstarted article scrubs from the beginning: the
        // browse overlay opens on a drag and must be able to seek before
        // the first hold.
        engine.load(articleId = 1, sentences = listOf(SentenceTokens(listOf("A", "B"))))
        engine.onDragStart()
        assertEquals(ReaderState.SCRUBBING, engine.state)
        engine.onDragMove(1)
        assertEquals("B", engine.currentToken())
        engine.onTouchUp()
        assertEquals(ReaderState.PAUSED, engine.state)
        // Scrubbing alone is a preview, not a play.
        assertEquals(false, engine.hasPlayed)
    }

    @Test
    fun doubleTapWhilePlayingEmitsAsk() {
        engine.load(articleId = 1, sentences = listOf(SentenceTokens(listOf("A", "B"))))
        holdAndPlay()
        engine.onDoubleTap()

        assertEquals(1, events.count { it.type == EventType.ASK })
        // Still playing
        assertEquals(ReaderState.PLAYING, engine.state)
    }

    // --- Very long tokens ---

    @Test
    fun veryLongTokenGetsLargeDelay() {
        val longWord = "a".repeat(50) // 50 letters
        val shortWord = "a".repeat(5) // 5 letters (baseline)
        val longDelay = delayMs(longWord, isLastInSentence = false, wpm = 250)
        val shortDelay = delayMs(shortWord, isLastInSentence = false, wpm = 250)
        // Length factor is capped at 1.5×, so long word ≈ 1.5 × short word
        assertTrue("50-letter word should be longer than 5-letter", longDelay > shortDelay)
        assertTrue("length factor caps at 1.5×", longDelay <= shortDelay * 2)
    }

    // --- Emoji and unicode ---

    @Test
    fun emojiToken() {
        engine.load(articleId = 1, sentences = listOf(SentenceTokens(listOf("Hello", "🌍"))))
        holdAndPlay()

        val delay = delayMs("Hello", isLastInSentence = false, wpm = 250)
        clock.advanceBy(delay)
        assertEquals("🌍", engine.currentToken())
    }

    // --- Load resets state ---

    @Test
    fun loadResetsState() {
        engine.load(articleId = 1, sentences = listOf(SentenceTokens(listOf("A", "B"))))
        holdAndPlay()
        // Advance one token
        clock.advanceBy(delayMs("A", isLastInSentence = false, wpm = 250))
        assertEquals("B", engine.currentToken())

        // Load new content — should reset
        engine.load(articleId = 2, sentences = listOf(SentenceTokens(listOf("X", "Y"))))
        assertEquals(ReaderState.IDLE, engine.state)
        assertEquals("X", engine.currentToken())
        assertEquals(0, engine.currentPosition())
    }

    // --- Skip from various states ---

    @Test
    fun skipFromPaused() {
        engine.load(articleId = 1, sentences = listOf(SentenceTokens(listOf("A"))))
        holdAndPlay()
        engine.onTouchUp()
        events.clear()

        engine.skip()
        assertEquals(ReaderState.IDLE, engine.state)
        assertEquals(1, events.count { it.type == EventType.SKIP })
    }

    @Test
    fun skipFromPlaying() {
        engine.load(articleId = 1, sentences = listOf(SentenceTokens(listOf("A", "B"))))
        holdAndPlay()
        events.clear()

        engine.skip()
        assertEquals(ReaderState.IDLE, engine.state)
        assertEquals(1, events.count { it.type == EventType.SKIP })
    }

    // --- Helpers ---

    private fun holdAndPlay() {
        engine.onTouchDown()
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS)
    }
}

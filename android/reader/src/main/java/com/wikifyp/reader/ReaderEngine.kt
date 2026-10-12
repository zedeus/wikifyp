package com.wikifyp.reader

import java.util.UUID

/**
 * State of the reader engine.
 *
 * Transitions are driven by gesture signals (hold, lift, tap, drag,
 * double-tap) through the engine's public API.
 */
enum class ReaderState {
    IDLE,
    PLAYING,
    PAUSED,
    SCRUBBING,
}

/** Callback for reading events emitted by the engine. */
fun interface EventListener {
    fun onEvent(event: ReadingEvent)
}

/**
 * The reader engine: a state machine that accepts gesture signals, advances
 * through tokens on a timer, and emits [ReadingEvent]s.
 *
 * The engine owns no threads. Playback is driven by [ReaderClock.schedule];
 * tests inject a [FakeClock] and advance time manually.
 *
 * @param clock injectable clock for timing and scheduling
 * @param listener receives every emitted event
 * @param idGenerator produces event IDs; defaults to random UUIDs
 */
class ReaderEngine(
    private val clock: ReaderClock,
    private val listener: EventListener,
    private val idGenerator: () -> String = { UUID.randomUUID().toString() },
) {
    var state: ReaderState = ReaderState.IDLE
        private set

    var wpm: Int = 250
        set(value) {
            require(value > 0) { "WPM must be positive" }
            field = value
        }

    /** The tokens loaded for playback, grouped by sentence. */
    private var sentences: List<SentenceTokens> = emptyList()

    /** Flat index into all tokens across sentences. */
    private var position: Int = 0

    /** Total number of tokens across all sentences. */
    private val totalTokens: Int get() = sentences.sumOf { it.tokens.size }

    /** Session UUID, created on first play. */
    private var sessionId: String? = null

    /** Article ID for token addresses. */
    private var articleId: Long = 0

    /** Pending advance callback. */
    private var pendingAdvance: Cancellable? = null

    /** Pending hold-threshold callback. */
    private var pendingHoldCheck: Cancellable? = null

    /** Drag start position (flat index) and timestamp, for scroll_back events. */
    private var dragStartPosition: Int = 0
    private var dragStartMs: Long = 0

    /**
     * Load content for playback. Resets to the beginning.
     */
    fun load(
        articleId: Long,
        sentences: List<SentenceTokens>,
    ) {
        cancelPending()
        this.articleId = articleId
        this.sentences = sentences
        this.position = 0
        this.state = ReaderState.IDLE
        this.sessionId = null
    }

    /**
     * Signal that the user touched down. After [HOLD_THRESHOLD_MS], if no
     * lift has occurred, playback begins.
     */
    fun onTouchDown() {
        when (state) {
            ReaderState.IDLE, ReaderState.PAUSED -> {
                // Schedule hold check — if still down after threshold, start playing
                pendingHoldCheck =
                    clock.schedule(HOLD_THRESHOLD_MS) {
                        startPlaying()
                    }
            }

            ReaderState.SCRUBBING -> {
                // Hold in scrubbing: resume from drag position
                pendingHoldCheck =
                    clock.schedule(HOLD_THRESHOLD_MS) {
                        emitScrollBack()
                        startPlaying()
                    }
            }

            ReaderState.PLAYING -> {
                // Already playing — ignore
            }
        }
    }

    /**
     * Signal that the user lifted their finger.
     */
    fun onTouchUp() {
        // Cancel any pending hold check — if it hasn't fired, this was a tap
        pendingHoldCheck?.cancel()
        pendingHoldCheck = null

        when (state) {
            ReaderState.PLAYING -> {
                // Lift while playing: pause on current word
                cancelPending()
                state = ReaderState.PAUSED
                emitEvent(EventType.PAUSE)
            }

            ReaderState.SCRUBBING -> {
                // Lift while scrubbing: emit scroll_back event, go to paused
                emitScrollBack()
                state = ReaderState.PAUSED
            }

            ReaderState.IDLE, ReaderState.PAUSED -> {
                // Lift in idle/paused without hold threshold met = tap = nothing
            }
        }
    }

    /**
     * Signal that the user started dragging (while paused or playing).
     */
    fun onDragStart() {
        pendingHoldCheck?.cancel()
        pendingHoldCheck = null
        cancelPending()

        if (state == ReaderState.PLAYING || state == ReaderState.PAUSED) {
            dragStartPosition = position
            dragStartMs = clock.nowMs()
            state = ReaderState.SCRUBBING
        }
    }

    /**
     * Signal that the user dragged by [tokenOffset] tokens (negative = back).
     * Clamps to valid range.
     */
    fun onDragMove(tokenOffset: Int) {
        if (state != ReaderState.SCRUBBING) return
        position = (position + tokenOffset).coerceIn(0, (totalTokens - 1).coerceAtLeast(0))
    }

    /**
     * Signal a double-tap (ask event).
     */
    fun onDoubleTap() {
        if (state == ReaderState.IDLE) return
        if (sentences.isEmpty()) return
        emitEvent(EventType.ASK)
    }

    /**
     * Skip the current article.
     */
    fun skip() {
        if (sentences.isEmpty()) return
        cancelPending()
        ensureSession()
        emitEvent(EventType.SKIP)
        state = ReaderState.IDLE
    }

    /**
     * Returns the current token text, or null if no content is loaded.
     */
    fun currentToken(): String? {
        val (sentIdx, tokIdx) = flatToAddress(position) ?: return null
        return sentences.getOrNull(sentIdx)?.tokens?.getOrNull(tokIdx)
    }

    /** Current position as a flat token index. */
    fun currentPosition(): Int = position

    // --- internals ---

    private fun startPlaying() {
        if (sentences.isEmpty()) return
        ensureSession()
        state = ReaderState.PLAYING
        emitEvent(EventType.PLAY)
        scheduleNextAdvance()
    }

    private fun scheduleNextAdvance() {
        val (sentIdx, tokIdx) = flatToAddress(position) ?: return
        val sentence = sentences[sentIdx]
        val tokenText = sentence.tokens[tokIdx]
        val isLast = tokIdx == sentence.tokens.size - 1
        val delay = delayMs(tokenText, isLast, wpm)

        pendingAdvance =
            clock.schedule(delay) {
                advanceToNext()
            }
    }

    private fun advanceToNext() {
        if (state != ReaderState.PLAYING) return

        val next = position + 1
        if (next >= totalTokens) {
            // Finished the article
            emitEvent(EventType.FINISH)
            state = ReaderState.IDLE
            return
        }

        position = next
        scheduleNextAdvance()
    }

    private fun ensureSession() {
        if (sessionId == null) {
            sessionId = idGenerator()
        }
    }

    private fun emitEvent(type: EventType) {
        val (sentIdx, tokIdx) = flatToAddress(position) ?: return
        listener.onEvent(
            ReadingEvent(
                id = idGenerator(),
                sessionId = sessionId!!,
                type = type,
                timestamp = clock.nowMs(),
                token = TokenAddress(articleId, sentIdx, tokIdx),
                wpm = wpm,
            ),
        )
    }

    private fun emitScrollBack() {
        val (sentIdx, tokIdx) = flatToAddress(position) ?: return
        val (startSentIdx, startTokIdx) = flatToAddress(dragStartPosition) ?: return
        val distance = dragStartPosition - position
        val dwellMs = clock.nowMs() - dragStartMs

        listener.onEvent(
            ReadingEvent(
                id = idGenerator(),
                sessionId = sessionId!!,
                type = EventType.SCROLL_BACK,
                timestamp = clock.nowMs(),
                token = TokenAddress(articleId, startSentIdx, startTokIdx),
                wpm = wpm,
                landedToken = TokenAddress(articleId, sentIdx, tokIdx),
                distance = distance,
                dwellMs = dwellMs,
            ),
        )
    }

    /**
     * Converts a flat token index to (sentenceIdx, tokenIdx), or null if
     * out of range.
     */
    private fun flatToAddress(flat: Int): Pair<Int, Int>? {
        var remaining = flat
        for ((sentIdx, sentence) in sentences.withIndex()) {
            if (remaining < sentence.tokens.size) {
                return sentIdx to remaining
            }
            remaining -= sentence.tokens.size
        }
        return null
    }

    private fun cancelPending() {
        pendingAdvance?.cancel()
        pendingAdvance = null
    }

    companion object {
        /** Milliseconds a touch must be held before it counts as a hold. */
        const val HOLD_THRESHOLD_MS = 200L
    }
}

/**
 * A sentence's tokens for playback. The engine does not need the full
 * sentence text, only the token strings and their count per sentence.
 */
data class SentenceTokens(
    val tokens: List<String>,
)

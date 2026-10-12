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
    /** Called after every state change (position, state, WPM). UI layers
     *  use this to sync observable state instead of polling. */
    private val onStateChanged: (() -> Unit)? = null,
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

    /** Total number of tokens across all sentences, cached on [load]. */
    private var totalTokens: Int = 0

    /** Session UUID, created on each [load]. */
    private var sessionId: String = idGenerator()

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
        this.totalTokens = sentences.sumOf { it.tokens.size }
        this.position = 0
        this.state = ReaderState.IDLE
        this.sessionId = idGenerator()
        notifyChanged()
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
        notifyChanged()
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
        notifyChanged()
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
        notifyChanged()
    }

    /**
     * Signal that the user dragged by [tokenOffset] tokens (negative = back).
     * Clamps to valid range.
     */
    fun onDragMove(tokenOffset: Int) {
        if (state != ReaderState.SCRUBBING) return
        position = (position + tokenOffset).coerceIn(0, (totalTokens - 1).coerceAtLeast(0))
        notifyChanged()
    }

    /**
     * Signal a double-tap (ask event).
     */
    fun onDoubleTap() {
        if (state == ReaderState.IDLE) return
        if (sentences.isEmpty()) return
        emitEvent(EventType.ASK)
        notifyChanged()
    }

    /**
     * Skip the current article.
     */
    fun skip() {
        if (sentences.isEmpty()) return
        cancelPending()
        emitEvent(EventType.SKIP)
        state = ReaderState.IDLE
        notifyChanged()
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

    /** Whether the current token belongs to a heading sentence. */
    fun currentIsHeading(): Boolean {
        val (sentIdx, _) = flatToAddress(position) ?: return false
        return sentences.getOrNull(sentIdx)?.isHeading == true
    }

    // --- internals ---

    private fun startPlaying() {
        if (totalTokens == 0) return
        state = ReaderState.PLAYING
        emitEvent(EventType.PLAY)
        notifyChanged()
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
            notifyChanged()
            return
        }

        position = next
        notifyChanged()
        scheduleNextAdvance()
    }

    private fun emitEvent(type: EventType) {
        val (sentIdx, tokIdx) = flatToAddress(position) ?: return
        listener.onEvent(
            ReadingEvent(
                id = idGenerator(),
                sessionId = sessionId,
                type = type,
                timestamp = clock.nowMs(),
                token = TokenAddress(articleId, sentIdx, tokIdx),
                wpm = wpm,
            ),
        )
    }

    private fun emitScrollBack() {
        if (position == dragStartPosition) return // no movement, no event
        val (sentIdx, tokIdx) = flatToAddress(position) ?: return
        val (startSentIdx, startTokIdx) = flatToAddress(dragStartPosition) ?: return
        val distance = kotlin.math.abs(dragStartPosition - position)
        val dwellMs = clock.nowMs() - dragStartMs

        listener.onEvent(
            ReadingEvent(
                id = idGenerator(),
                sessionId = sessionId,
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

    private fun notifyChanged() {
        onStateChanged?.invoke()
    }

    private fun cancelPending() {
        pendingAdvance?.cancel()
        pendingAdvance = null
        pendingHoldCheck?.cancel()
        pendingHoldCheck = null
    }

    companion object {
        /** Milliseconds a touch must be held before it counts as a hold. */
        const val HOLD_THRESHOLD_MS = 200L
    }
}

/**
 * A sentence's tokens for playback. The engine does not need the full
 * sentence text, only the token strings and their count per sentence.
 *
 * @param isHeading true for section headings; the UI may display these
 *   in a distinct colour so the reader knows what kind of text they're in.
 */
data class SentenceTokens(
    val tokens: List<String>,
    val isHeading: Boolean = false,
)

package com.wikifyp.reader

/**
 * Injectable clock for the reader engine. Tests supply a controllable
 * implementation; production uses real time.
 */
interface ReaderClock {
    /** Current time in milliseconds (epoch or monotonic — only deltas matter). */
    fun nowMs(): Long

    /**
     * Schedules [action] to run after [delayMs] milliseconds.
     * Returns a handle that can cancel the pending action.
     */
    fun schedule(
        delayMs: Long,
        action: () -> Unit,
    ): Cancellable
}

/** A scheduled action that can be cancelled. */
fun interface Cancellable {
    fun cancel()
}

/**
 * Test clock with manual time control. Time only advances when [advanceBy]
 * or [advanceTo] is called, and scheduled actions fire synchronously in
 * order when their deadline is reached.
 */
class FakeClock(
    private var timeMs: Long = 0L,
) : ReaderClock {
    private data class Scheduled(
        val deadlineMs: Long,
        val action: () -> Unit,
    )

    private val pending = mutableListOf<Scheduled>()

    override fun nowMs(): Long = timeMs

    override fun schedule(
        delayMs: Long,
        action: () -> Unit,
    ): Cancellable {
        val entry = Scheduled(timeMs + delayMs, action)
        pending.add(entry)
        return Cancellable { pending.remove(entry) }
    }

    /** Advances time by [ms] milliseconds, firing any due actions in order. */
    fun advanceBy(ms: Long) {
        advanceTo(timeMs + ms)
    }

    /** Advances time to [targetMs], firing any due actions in order. */
    fun advanceTo(targetMs: Long) {
        require(targetMs >= timeMs) { "cannot go backwards" }
        timeMs = targetMs
        fireDue()
    }

    private fun fireDue() {
        // Fire in deadline order; actions may schedule new work.
        while (true) {
            val next =
                pending
                    .filter { it.deadlineMs <= timeMs }
                    .minByOrNull { it.deadlineMs } ?: break
            pending.remove(next)
            next.action()
        }
    }
}

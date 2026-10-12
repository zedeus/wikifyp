package com.wikifyp.ui.reader

import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import com.wikifyp.reader.Cancellable
import com.wikifyp.reader.ReaderClock

/**
 * Production [ReaderClock] backed by [SystemClock.elapsedRealtime] and
 * [Handler.postDelayed]. Runs callbacks on the main thread.
 */
class HandlerClock : ReaderClock {
    private val handler = Handler(Looper.getMainLooper())

    override fun nowMs(): Long = SystemClock.elapsedRealtime()

    override fun schedule(
        delayMs: Long,
        action: () -> Unit,
    ): Cancellable {
        val runnable = Runnable { action() }
        handler.postDelayed(runnable, delayMs)
        return Cancellable { handler.removeCallbacks(runnable) }
    }
}

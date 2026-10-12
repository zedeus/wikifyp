package com.wikifyp.theme

import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.Easing

// Motion tokens. Calm means short distances and gentle easing: nothing on
// screen should move faster than the reader's eye or keep moving after it
// has arrived.
object Motion {
    /** State changes the reader barely notices: selection, press feedback. */
    const val QUICK_MS = 120

    /** The default for most transitions: fades, ghost text in and out. */
    const val STANDARD_MS = 240

    /** Large or spatial changes: a screen settling, a card expanding. */
    const val SLOW_MS = 400

    /** Starts moving immediately, lands softly. The default. */
    val Settle: Easing = CubicBezierEasing(0.2f, 0f, 0f, 1f)

    /** For things leaving the screen: accelerate out, no lingering. */
    val Leave: Easing = CubicBezierEasing(0.3f, 0f, 1f, 1f)
}

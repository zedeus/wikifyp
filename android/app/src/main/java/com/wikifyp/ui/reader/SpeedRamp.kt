package com.wikifyp.ui.reader

/**
 * Speed-ramp policy: ease into reading speed instead of starting at full
 * WPM, so the eye has time to settle (grunion-ai and speeedy both do
 * this; see the RSVP research notes).
 *
 * The first play of an article ramps from 60% over ~30 words. A resume
 * after a pause ramps from 80% over ~10 words — the reader was just here,
 * they only need a beat to re-settle.
 *
 * Injected into the engine as its speed-scale policy; emitted events keep
 * the reader's chosen WPM.
 */
fun rampScale(
    wordsSincePlay: Int,
    resuming: Boolean,
): Double {
    val start = if (resuming) RESUME_START else FIRST_START
    val words = if (resuming) RESUME_WORDS else FIRST_WORDS
    val t = (wordsSincePlay.toDouble() / words).coerceIn(0.0, 1.0)
    return start + (1.0 - start) * t
}

private const val FIRST_START = 0.6
private const val FIRST_WORDS = 30
private const val RESUME_START = 0.8
private const val RESUME_WORDS = 10

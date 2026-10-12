package com.wikifyp.ui.reader

/**
 * Computes the focal-letter index (ORP — Optimal Recognition Point) for
 * RSVP display. The current word is positioned so this letter sits at the
 * screen centre.
 *
 * Uses the standard Spritz-style tier table on the letter/digit count
 * (punctuation stripped), matching speeedy and other mature implementations:
 *
 * | Clean length | Focal index |
 * |--------------|-------------|
 * | 0–1          | 0           |
 * | 2–5          | 1           |
 * | 6–9          | 2           |
 * | 10–13        | 3           |
 * | 14+          | 4           |
 *
 * Punctuation at the edges of the token is ignored so "word." and
 * "(word)" compute ORP on "word".
 */
fun focalIndex(token: String): Int {
    val clean = token.count { it.isLetterOrDigit() }
    return when {
        clean <= 1 -> 0
        clean <= 5 -> 1
        clean <= 9 -> 2
        clean <= 13 -> 3
        else -> 4
    }
}

/**
 * Maps a focal index (computed on clean letter/digit characters) back to
 * the position in the original token string, skipping leading non-word
 * characters. This way the highlight lands on the right character even
 * when the token starts with punctuation like `"(hello"`.
 */
fun focalPosition(token: String): Int {
    val targetCleanIdx = focalIndex(token)
    var cleanCount = 0
    for ((i, ch) in token.withIndex()) {
        if (ch.isLetterOrDigit()) {
            if (cleanCount == targetCleanIdx) return i
            cleanCount++
        }
    }
    // Fallback: return the target or 0.
    return targetCleanIdx.coerceAtMost((token.length - 1).coerceAtLeast(0))
}

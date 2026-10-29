package com.wikifyp.ui.reader

import kotlin.math.roundToInt

/**
 * Length of the bolded prefix for bionic-style reading: roughly the first
 * 40% of a word's characters, at least one, so the eye has a fixation
 * anchor in every ghost word. Tokens with no letters (bare numbers,
 * punctuation) get no prefix — bolding "(" or "42" guides nothing.
 */
fun bionicPrefixLength(token: String): Int {
    if (token.none { it.isLetter() }) return 0
    return (token.length * 0.4f).roundToInt().coerceIn(1, token.length)
}

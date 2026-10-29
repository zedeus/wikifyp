package com.wikifyp.ui.reader

import androidx.compose.ui.graphics.Color

/**
 * The reader's drawing colors, resolved from the reader theme setting.
 * The reader paints its own surface (a reading room, not a Material
 * screen), so these are deliberate raw tokens rather than the Material
 * color scheme — the one place in the app allowed to do that. They are
 * defined here, not in `theme/Color.kt`, because the reader's themes
 * (paper, sepia, ink, black) are independent of the app theme.
 */
data class ReaderPalette(
    /** The reading surface. */
    val background: Color,
    /** The focal word. */
    val word: Color,
    /** Ghost words around the focal word, and browse body text. */
    val ghost: Color,
    /** The ORP letter, guides and progress. */
    val accent: Color,
    /** Chrome text: title, WPM, percentages. */
    val chrome: Color,
    /** Raised surfaces: the WPM popup, browse scrim. */
    val surface: Color,
    /** Whether this palette is a dark one (drives icon/scrim choices). */
    val isDark: Boolean,
)

/** Resolves the palette for a [ReaderTheme] under the given system theme. */
fun readerPalette(
    theme: ReaderTheme,
    systemDark: Boolean,
): ReaderPalette =
    when (theme) {
        ReaderTheme.SYSTEM -> if (systemDark) darkPalette else lightPalette
        ReaderTheme.LIGHT -> lightPalette
        ReaderTheme.SEPIA -> sepiaPalette
        ReaderTheme.DARK -> darkPalette
        ReaderTheme.BLACK -> blackPalette
    }

// The accent is the reader's one vivid color: the ORP letter has to pop
// out of the word at 400 wpm, so it is a saturated vermillion tuned per
// surface — darker on paper for contrast, brighter on ink and brighter
// still on true black, where a muted tone reads as gray.

private val lightPalette =
    ReaderPalette(
        background = Color(0xFFFFFFFF),
        word = Color(0xFF202122),
        ghost = Color(0xFF54595D).copy(alpha = 0.75f),
        accent = Color(0xFFD2491D),
        chrome = Color(0xFF72777D),
        surface = Color(0xFFF1F3F5),
        isDark = false,
    )

private val sepiaPalette =
    ReaderPalette(
        background = Color(0xFFF8F1E3),
        word = Color(0xFF27221A),
        ghost = Color(0xFF6E6150).copy(alpha = 0.78f),
        accent = Color(0xFFBF4617),
        chrome = Color(0xFF6E6150),
        surface = Color(0xFFF0E6D6),
        isDark = false,
    )

private val darkPalette =
    ReaderPalette(
        background = Color(0xFF202122),
        word = Color(0xFFEAECF0),
        ghost = Color(0xFFA2A9B1).copy(alpha = 0.8f),
        accent = Color(0xFFFF7B52),
        chrome = Color(0xFFA2A9B1),
        surface = Color(0xFF2E3136),
        isDark = true,
    )

private val blackPalette =
    ReaderPalette(
        background = Color.Black,
        word = Color(0xFFECEDEF),
        ghost = Color(0xFF9FA6AD).copy(alpha = 0.75f),
        accent = Color(0xFFFF8257),
        chrome = Color(0xFF9FA6AD),
        surface = Color(0xFF17181A),
        isDark = true,
    )

package com.wikifyp.ui.reader

import androidx.compose.ui.graphics.Color
import com.wikifyp.theme.ClayDeep
import com.wikifyp.theme.ClaySoft
import com.wikifyp.theme.Ink
import com.wikifyp.theme.InkMuted
import com.wikifyp.theme.Night
import com.wikifyp.theme.NightRaised
import com.wikifyp.theme.Paper
import com.wikifyp.theme.PaperSunken
import com.wikifyp.theme.PaperText
import com.wikifyp.theme.PaperTextMuted

/**
 * The reader's drawing colors, resolved from the reader theme setting.
 * The reader paints its own surface (a reading room, not a Material
 * screen), so these are deliberate raw tokens rather than the Material
 * color scheme — the one place in the app allowed to do that.
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
        ReaderTheme.DARK -> darkPalette
        ReaderTheme.BLACK -> blackPalette
    }

private val lightPalette =
    ReaderPalette(
        background = Paper,
        word = Ink,
        ghost = InkMuted.copy(alpha = 0.75f),
        accent = ClayDeep,
        chrome = InkMuted,
        surface = PaperSunken,
        isDark = false,
    )

private val darkPalette =
    ReaderPalette(
        background = Night,
        word = PaperText,
        ghost = PaperTextMuted.copy(alpha = 0.8f),
        accent = ClaySoft,
        chrome = PaperTextMuted,
        surface = NightRaised,
        isDark = true,
    )

private val blackPalette =
    ReaderPalette(
        background = Color.Black,
        word = PaperText,
        ghost = PaperTextMuted.copy(alpha = 0.75f),
        accent = ClaySoft,
        chrome = PaperTextMuted,
        surface = Color(0xFF1A1713),
        isDark = true,
    )

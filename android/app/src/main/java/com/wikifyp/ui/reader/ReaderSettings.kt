package com.wikifyp.ui.reader

import android.content.SharedPreferences
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.unit.em
import androidx.core.content.edit

/** Which surface the reader draws on, independent of the app theme. */
enum class ReaderTheme {
    /** Follow the system: paper by day, ink at night. */
    SYSTEM,
    LIGHT,

    /** Warm paper, easy on the eyes in daylight. */
    SEPIA,
    DARK,

    /** True black, for OLED screens and dark rooms. */
    BLACK,
}

/** When the current section's heading shows above the focal line. */
enum class SectionLabelMode {
    /** Resting while paused, a brief flash on section changes while playing. */
    AUTO,

    /** Always visible, playing or not. */
    ALWAYS,
    HIDDEN,
}

/**
 * The reader's appearance and behaviour settings. Persisted via
 * [ReaderSettingsStore] so they carry across articles and launches.
 */
data class ReaderSettings(
    /** Reading speed in words per minute. */
    val wpm: Int = 250,
    /** Multiplier on the word type size. */
    val fontScale: Float = 1f,
    /** Extra tracking on reader text (focal line and browse), in em. */
    val letterSpacingEm: Float = 0f,
    /** Ghost words shown after the focal word; half as many before. */
    val ghostWords: Int = 8,
    /** Horizontal position of the focal column, as a fraction of width. */
    val orpFraction: Float = ORP_DEFAULT,
    val theme: ReaderTheme = ReaderTheme.SYSTEM,
    /** Bold the first part of each ghost word to guide fixation. */
    val bionic: Boolean = false,
    /** Hide the chrome (title, progress, WPM) while playing. */
    val focusMode: Boolean = true,
    /** Ease into reading speed over the first words instead of starting
     *  at full WPM. */
    val speedRamp: Boolean = true,
    /** When the current section's heading shows above the focal line. */
    val sectionLabel: SectionLabelMode = SectionLabelMode.AUTO,
    /** Drop the bookmark automatically when an article is finished. */
    val autoUnsaveOnFinish: Boolean = false,
) {
    companion object {
        const val WPM_MIN = 100
        const val WPM_MAX = 600
        const val FONT_SCALE_MIN = 0.8f
        const val FONT_SCALE_MAX = 1.4f
        const val LETTER_SPACING_MAX = 0.12f
        const val GHOST_WORDS_MAX = 12
        const val ORP_MIN = 0.25f
        const val ORP_MAX = 0.5f
        const val ORP_DEFAULT = 1f / 3f
    }
}

/** A reader text style scaled and spaced by these settings. */
fun ReaderSettings.scaledStyle(base: TextStyle): TextStyle =
    base.copy(
        fontSize = base.fontSize * fontScale,
        lineHeight = base.lineHeight * fontScale,
        letterSpacing = letterSpacingEm.em,
    )

/**
 * SharedPreferences-backed persistence for [ReaderSettings]. Values are
 * clamped on load so a bad or stale preference can never produce an
 * unreadable reader.
 */
class ReaderSettingsStore(
    private val prefs: SharedPreferences,
) {
    fun load(): ReaderSettings {
        val d = ReaderSettings()
        return ReaderSettings(
            wpm =
                prefs
                    .getInt(KEY_WPM, d.wpm)
                    .coerceIn(ReaderSettings.WPM_MIN, ReaderSettings.WPM_MAX),
            fontScale =
                prefs
                    .getFloat(KEY_FONT_SCALE, d.fontScale)
                    .coerceIn(ReaderSettings.FONT_SCALE_MIN, ReaderSettings.FONT_SCALE_MAX),
            letterSpacingEm =
                prefs
                    .getFloat(KEY_LETTER_SPACING, d.letterSpacingEm)
                    .coerceIn(0f, ReaderSettings.LETTER_SPACING_MAX),
            ghostWords =
                prefs
                    .getInt(KEY_GHOST_WORDS, d.ghostWords)
                    .coerceIn(0, ReaderSettings.GHOST_WORDS_MAX),
            orpFraction =
                prefs
                    .getFloat(KEY_ORP_FRACTION, d.orpFraction)
                    .coerceIn(ReaderSettings.ORP_MIN, ReaderSettings.ORP_MAX),
            theme =
                prefs.getString(KEY_THEME, null)?.let { stored ->
                    ReaderTheme.entries.find { it.name == stored }
                } ?: d.theme,
            bionic = prefs.getBoolean(KEY_BIONIC, d.bionic),
            focusMode = prefs.getBoolean(KEY_FOCUS_MODE, d.focusMode),
            speedRamp = prefs.getBoolean(KEY_SPEED_RAMP, d.speedRamp),
            sectionLabel =
                prefs.getString(KEY_SECTION_LABEL, null)?.let { stored ->
                    SectionLabelMode.entries.find { it.name == stored }
                } ?: d.sectionLabel,
            autoUnsaveOnFinish =
                prefs.getBoolean(KEY_AUTO_UNSAVE, d.autoUnsaveOnFinish),
        )
    }

    fun save(settings: ReaderSettings) {
        prefs.edit {
            putInt(KEY_WPM, settings.wpm)
            putFloat(KEY_FONT_SCALE, settings.fontScale)
            putFloat(KEY_LETTER_SPACING, settings.letterSpacingEm)
            putInt(KEY_GHOST_WORDS, settings.ghostWords)
            putFloat(KEY_ORP_FRACTION, settings.orpFraction)
            putString(KEY_THEME, settings.theme.name)
            putBoolean(KEY_BIONIC, settings.bionic)
            putBoolean(KEY_FOCUS_MODE, settings.focusMode)
            putBoolean(KEY_SPEED_RAMP, settings.speedRamp)
            putString(KEY_SECTION_LABEL, settings.sectionLabel.name)
            putBoolean(KEY_AUTO_UNSAVE, settings.autoUnsaveOnFinish)
        }
    }

    private companion object {
        // "wpm" is the key the reader used before settings were unified;
        // keeping it migrates existing installs for free.
        const val KEY_WPM = "wpm"
        const val KEY_FONT_SCALE = "readerFontScale"
        const val KEY_LETTER_SPACING = "readerLetterSpacing"
        const val KEY_GHOST_WORDS = "readerGhostWords"
        const val KEY_ORP_FRACTION = "readerOrpFraction"
        const val KEY_THEME = "readerTheme"
        const val KEY_BIONIC = "readerBionic"
        const val KEY_FOCUS_MODE = "readerFocusMode"
        const val KEY_SPEED_RAMP = "readerSpeedRamp"
        const val KEY_SECTION_LABEL = "readerSectionLabel"
        const val KEY_AUTO_UNSAVE = "readerAutoUnsave"
    }
}

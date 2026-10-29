package com.wikifyp.ui.reader

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class ReaderSettingsStoreTest {
    private lateinit var store: ReaderSettingsStore
    private lateinit var prefs: android.content.SharedPreferences

    @Before
    fun setUp() {
        val context = ApplicationProvider.getApplicationContext<Context>()
        prefs = context.getSharedPreferences("test", Context.MODE_PRIVATE)
        store = ReaderSettingsStore(prefs)
    }

    @Test
    fun loadWithoutSavedValuesGivesDefaults() {
        assertEquals(ReaderSettings(), store.load())
    }

    @Test
    fun savedSettingsRoundTrip() {
        val settings =
            ReaderSettings(
                fontScale = 1.2f,
                letterSpacingEm = 0.05f,
                ghostWords = 4,
                orpFraction = 0.4f,
                theme = ReaderTheme.BLACK,
                bionic = true,
                focusMode = false,
            )
        store.save(settings)
        assertEquals(settings, store.load())
    }

    @Test
    fun outOfRangeValuesAreClampedOnLoad() {
        prefs
            .edit()
            .putFloat("readerFontScale", 9f)
            .putFloat("readerLetterSpacing", -1f)
            .putInt("readerGhostWords", 99)
            .putFloat("readerOrpFraction", 0.9f)
            .apply()

        val loaded = store.load()
        assertEquals(ReaderSettings.FONT_SCALE_MAX, loaded.fontScale, 0f)
        assertEquals(0f, loaded.letterSpacingEm, 0f)
        assertEquals(ReaderSettings.GHOST_WORDS_MAX, loaded.ghostWords)
        assertEquals(ReaderSettings.ORP_MAX, loaded.orpFraction, 0f)
    }

    @Test
    fun unknownThemeNameFallsBackToDefault() {
        prefs.edit().putString("readerTheme", "SEPIA").apply()
        assertEquals(ReaderTheme.SYSTEM, store.load().theme)
    }
}

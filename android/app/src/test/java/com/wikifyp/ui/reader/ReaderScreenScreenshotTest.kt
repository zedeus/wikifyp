package com.wikifyp.ui.reader

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performTouchInput
import coil3.SingletonImageLoader
import coil3.annotation.DelicateCoilApi
import com.github.takahirom.roborazzi.RobolectricDeviceQualifiers
import com.github.takahirom.roborazzi.captureRoboImage
import com.wikifyp.data.ArticleImage
import com.wikifyp.data.FeedCard
import com.wikifyp.data.testCard
import com.wikifyp.reader.FakeClock
import com.wikifyp.reader.SentenceTokens
import com.wikifyp.theme.WikifypTheme
import org.junit.After
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * Golden-image screenshots for the reader screen: paused, playing,
 * mid-drag, browse overlay and the settings sheet, in the light and dark
 * reader themes (set explicitly through [ReaderSettings] so the goldens
 * don't depend on the test environment's system theme).
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(qualifiers = RobolectricDeviceQualifiers.MediumPhone)
class ReaderScreenScreenshotTest {
    @get:Rule val compose = createComposeRule()

    // The idle-cover tests swap the process-wide Coil singleton for a fake;
    // put it back so other screenshot classes in this JVM keep their own
    // (network-less) rendering regardless of execution order.
    @OptIn(DelicateCoilApi::class)
    @After
    fun resetImageLoader() {
        SingletonImageLoader.reset()
    }

    private val sentences =
        listOf(
            SentenceTokens(
                listOf(
                    "Philosophy",
                    "is",
                    "a",
                    "systematic",
                    "study",
                    "of",
                    "general",
                    "questions.",
                ),
            ),
            SentenceTokens(
                listOf(
                    "It",
                    "is",
                    "a",
                    "rational",
                    "and",
                    "critical",
                    "inquiry.",
                ),
            ),
            SentenceTokens(listOf("History"), isHeading = true),
            SentenceTokens(
                listOf(
                    "Historically,",
                    "many",
                    "sciences",
                    "formed",
                    "part",
                    "of",
                    "philosophy.",
                ),
            ),
            SentenceTokens(
                listOf(
                    "However,",
                    "they",
                    "are",
                    "now",
                    "separate",
                    "disciplines.",
                ),
            ),
            SentenceTokens(
                listOf(
                    "Western",
                    "philosophy",
                    "originated",
                    "in",
                    "Ancient",
                    "Greece.",
                ),
            ),
        )

    private fun settingsFor(dark: Boolean) = ReaderSettings(theme = if (dark) ReaderTheme.DARK else ReaderTheme.LIGHT)

    /** The idle reader: lead image and description above the hint. */
    @Test
    fun idle_cover_light() {
        renderIdleCover(dark = false)
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun idle_cover_dark() {
        renderIdleCover(dark = true)
        compose.onRoot().captureRoboImage()
    }

    /** Without a lead image the cover falls back to the initial tile. */
    @Test
    fun idle_cover_no_image_light() {
        renderIdleCover(dark = false, withImage = false)
        compose.onRoot().captureRoboImage()
    }

    private fun renderIdleCover(
        dark: Boolean,
        withImage: Boolean = true,
    ) {
        val settings = settingsFor(dark)
        val card =
            testCard(
                title = "Philosophy",
                description = "Study of general and fundamental questions",
                image = if (withImage) ArticleImage("https://example.com/philosophy.jpg", 960, 640) else null,
            )
        // The golden must not depend on the network: the lead image URL
        // resolves to a flat stand-in through a fake Coil engine.
        installFakeImageLoader()
        compose.setContent {
            WikifypTheme(darkTheme = dark) {
                ReaderScreen(
                    articleTitle = "Philosophy",
                    sentences = sentences,
                    articleId = 1L,
                    clock = FakeClock(),
                    settings = settings,
                    card = card,
                )
            }
        }
        compose.waitForIdle()
    }

    /** Paused state: word visible, ghost text and chrome showing. */
    @Test
    fun paused_light() {
        renderPaused(dark = false)
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun paused_dark() {
        renderPaused(dark = true)
        compose.onRoot().captureRoboImage()
    }

    /** Playing state: focus mode hides the chrome. */
    @Test
    fun playing_light() {
        renderPlaying(settingsFor(dark = false))
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun playing_dark() {
        renderPlaying(settingsFor(dark = true))
        compose.onRoot().captureRoboImage()
    }

    /** True-black theme for OLED reading. */
    @Test
    fun playing_black() {
        renderPlaying(ReaderSettings(theme = ReaderTheme.BLACK))
        compose.onRoot().captureRoboImage()
    }

    /** Sepia theme: warm paper for daylight reading. */
    @Test
    fun paused_sepia() {
        renderPaused(dark = false, settings = ReaderSettings(theme = ReaderTheme.SEPIA))
        compose.onRoot().captureRoboImage()
    }

    /** Table of contents sheet content on the reader surface. */
    @Test
    fun toc_light() {
        val palette = readerPalette(ReaderTheme.LIGHT, systemDark = false)
        compose.setContent {
            WikifypTheme(darkTheme = false) {
                TocSheetContent(
                    sections =
                        listOf(
                            Section(0, "Philosophy"),
                            Section(15, "History"),
                            Section(28, "Branches"),
                            Section(40, "Influence"),
                        ),
                    currentFlat = 20,
                    palette = palette,
                    onJump = {},
                    modifier =
                        Modifier
                            .fillMaxWidth()
                            .background(palette.surface),
                )
            }
        }
        compose.onRoot().captureRoboImage()
    }

    /** The up-next card shown when an article finishes. */
    @Test
    fun upnext_dark() {
        val palette = readerPalette(ReaderTheme.DARK, systemDark = true)
        compose.setContent {
            WikifypTheme(darkTheme = true) {
                UpNextCard(
                    card =
                        FeedCard(
                            id = 2,
                            title = "Roman Empire",
                            source = "wikipedia",
                            excerpt = "The Roman Empire ruled the Mediterranean.",
                            sentences = 240,
                            words = 6100,
                            attribution = "Text from the Wikipedia article \"Roman Empire\"",
                            description = "27 BC–476 AD state and civilization",
                        ),
                    palette = palette,
                    onNext = {},
                    onBackToFeed = {},
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }
        compose.onRoot().captureRoboImage()
    }

    /** Bionic ghost words: bolded prefixes in the periphery. */
    @Test
    fun paused_bionic_light() {
        renderPaused(dark = false, settings = ReaderSettings(theme = ReaderTheme.LIGHT, bionic = true))
        compose.onRoot().captureRoboImage()
    }

    /** Extreme settings: max type and spacing, no ghosts, centred ORP. */
    @Test
    fun paused_extremes_light() {
        renderPaused(
            dark = false,
            settings =
                ReaderSettings(
                    theme = ReaderTheme.LIGHT,
                    fontScale = ReaderSettings.FONT_SCALE_MAX,
                    letterSpacingEm = ReaderSettings.LETTER_SPACING_MAX,
                    ghostWords = 0,
                    orpFraction = ReaderSettings.ORP_MAX,
                ),
        )
        compose.onRoot().captureRoboImage()
    }

    /** Mid-drag state: scrubbing, ghost text visible. */
    @Test
    fun drag_light() {
        renderDrag(dark = false)
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun drag_dark() {
        renderDrag(dark = true)
        compose.onRoot().captureRoboImage()
    }

    /** Browse overlay: flowing text with the current word highlighted. */
    @Test
    fun browse_light() {
        renderBrowse(dark = false)
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun browse_dark() {
        renderBrowse(dark = true)
        compose.onRoot().captureRoboImage()
    }

    /** The settings sheet content on the reader surface. */
    @Test
    fun settings_light() {
        renderSettingsContent(dark = false)
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun settings_dark() {
        renderSettingsContent(dark = true)
        compose.onRoot().captureRoboImage()
    }

    private fun showReader(settings: ReaderSettings): FakeClock {
        val clock = FakeClock()
        compose.setContent {
            WikifypTheme(darkTheme = settings.theme == ReaderTheme.DARK) {
                ReaderScreen(
                    articleTitle = "Philosophy",
                    sentences = sentences,
                    articleId = 1L,
                    clock = clock,
                    settings = settings,
                )
            }
        }
        return clock
    }

    private fun renderPaused(
        dark: Boolean,
        settings: ReaderSettings = settingsFor(dark),
    ) {
        val clock = showReader(settings)
        // Hold to start.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        clock.advanceBy(151)
        compose.waitForIdle()
        // Advance a few words.
        clock.advanceBy(800)
        compose.waitForIdle()
        // Lift to pause.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            up()
        }
        compose.waitForIdle()
    }

    private fun renderPlaying(settings: ReaderSettings) {
        val clock = showReader(settings)
        // Hold to start playing.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        clock.advanceBy(151)
        compose.waitForIdle()
        // Advance a couple of words.
        clock.advanceBy(500)
        compose.waitForIdle()
    }

    private fun renderDrag(dark: Boolean) {
        val clock = showReader(settingsFor(dark))
        // Hold to start.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        clock.advanceBy(151)
        compose.waitForIdle()
        // Advance several words.
        clock.advanceBy(1500)
        compose.waitForIdle()
        // Lift to pause.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            up()
        }
        compose.waitForIdle()
        // Start a drag back.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
            moveBy(Offset(-150f, 0f))
        }
        compose.waitForIdle()
    }

    private fun renderBrowse(dark: Boolean) {
        val clock = showReader(settingsFor(dark))
        // Hold to start, advance into the article, pause.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        clock.advanceBy(151)
        compose.waitForIdle()
        clock.advanceBy(1500)
        compose.waitForIdle()
        compose.onNodeWithTag("readerSurface").performTouchInput { up() }
        compose.waitForIdle()
        // Vertical drag opens browse.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
            moveBy(Offset(0f, -200f))
            up()
        }
        compose.waitForIdle()
    }

    private fun renderSettingsContent(dark: Boolean) {
        val palette = readerPalette(if (dark) ReaderTheme.DARK else ReaderTheme.LIGHT, dark)
        compose.setContent {
            WikifypTheme(darkTheme = dark) {
                ReaderSettingsSheetContent(
                    settings = ReaderSettings(),
                    palette = palette,
                    onSettingsChange = {},
                    modifier =
                        Modifier
                            .fillMaxWidth()
                            .background(palette.surface),
                )
            }
        }
    }
}

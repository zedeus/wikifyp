package com.wikifyp.ui.reader

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performTouchInput
import com.github.takahirom.roborazzi.RobolectricDeviceQualifiers
import com.github.takahirom.roborazzi.captureRoboImage
import com.wikifyp.reader.FakeClock
import com.wikifyp.reader.SentenceTokens
import com.wikifyp.theme.WikifypTheme
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * Golden-image screenshots for the reader screen in three states
 * (paused, playing, mid-drag) × two themes (light, dark).
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(qualifiers = RobolectricDeviceQualifiers.MediumPhone)
class ReaderScreenScreenshotTest {
    @get:Rule val compose = createComposeRule()

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

    /** Paused state: word visible, ghost text showing. */
    @Test
    fun paused_light() {
        renderPaused(darkTheme = false)
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun paused_dark() {
        renderPaused(darkTheme = true)
        compose.onRoot().captureRoboImage()
    }

    /** Playing state: word visible, no ghost text. */
    @Test
    fun playing_light() {
        renderPlaying(darkTheme = false)
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun playing_dark() {
        renderPlaying(darkTheme = true)
        compose.onRoot().captureRoboImage()
    }

    /** Mid-drag state: scrubbing, ghost text visible. */
    @Test
    fun drag_light() {
        renderDrag(darkTheme = false)
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun drag_dark() {
        renderDrag(darkTheme = true)
        compose.onRoot().captureRoboImage()
    }

    private fun renderPaused(darkTheme: Boolean) {
        val clock = FakeClock()
        compose.setContent {
            WikifypTheme(darkTheme = darkTheme) {
                ReaderScreen(
                    articleTitle = "Philosophy",
                    sentences = sentences,
                    articleId = 1L,
                    clock = clock,
                    initialWpm = 250,
                )
            }
        }
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

    private fun renderPlaying(darkTheme: Boolean) {
        val clock = FakeClock()
        compose.setContent {
            WikifypTheme(darkTheme = darkTheme) {
                ReaderScreen(
                    articleTitle = "Philosophy",
                    sentences = sentences,
                    articleId = 1L,
                    clock = clock,
                    initialWpm = 250,
                )
            }
        }
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

    private fun renderDrag(darkTheme: Boolean) {
        val clock = FakeClock()
        compose.setContent {
            WikifypTheme(darkTheme = darkTheme) {
                ReaderScreen(
                    articleTitle = "Philosophy",
                    sentences = sentences,
                    articleId = 1L,
                    clock = clock,
                    initialWpm = 250,
                )
            }
        }
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
}

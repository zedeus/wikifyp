package com.wikifyp.ui.reader

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotDisplayed
import androidx.compose.ui.test.click
import androidx.compose.ui.test.hasAnyDescendant
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performTouchInput
import coil3.SingletonImageLoader
import coil3.annotation.DelicateCoilApi
import com.wikifyp.data.ArticleImage
import com.wikifyp.data.testCard
import com.wikifyp.reader.FakeClock
import com.wikifyp.reader.ReaderEngine
import com.wikifyp.reader.SentenceTokens
import com.wikifyp.theme.WikifypTheme
import org.junit.After
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/**
 * Focus mode (chrome hides while playing) and browse mode (vertical drag
 * opens the flowing-text overlay; tap closes it) on the reader screen.
 */
@RunWith(RobolectricTestRunner::class)
class ReaderScreenFocusBrowseTest {
    @get:Rule val compose = createComposeRule()

    // The masthead test swaps the Coil singleton for a fake; put it back
    // for other classes in this JVM.
    @OptIn(DelicateCoilApi::class)
    @After
    fun resetImageLoader() {
        SingletonImageLoader.reset()
    }

    private val testSentences =
        listOf(
            SentenceTokens(listOf("The", "quick", "brown", "fox")),
            SentenceTokens(listOf("jumps", "over", "the", "lazy", "dog.")),
        )

    private fun show(
        clock: FakeClock,
        settings: ReaderSettings = ReaderSettings(),
    ) {
        compose.setContent {
            WikifypTheme {
                ReaderScreen(
                    articleTitle = "Test",
                    sentences = testSentences,
                    articleId = 1L,
                    clock = clock,
                    settings = settings,
                )
            }
        }
    }

    private fun hold(clock: FakeClock) {
        compose.onNodeWithTag("readerSurface").performTouchInput { down(center) }
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS + 1)
        compose.waitForIdle()
    }

    // --- Focus mode ---

    @Test
    fun chromeShowsWhileIdle() {
        show(FakeClock())
        compose.onNodeWithTag("topChrome").assertIsDisplayed()
        compose.onNodeWithTag("bottomChrome").assertIsDisplayed()
    }

    @Test
    fun focusModeHidesChromeWhilePlaying() {
        val clock = FakeClock()
        show(clock)
        hold(clock)

        compose.onNodeWithTag("topChrome").assertDoesNotExistOrHidden()
        compose.onNodeWithTag("bottomChrome").assertDoesNotExistOrHidden()
    }

    @Test
    fun chromeReturnsOnPause() {
        val clock = FakeClock()
        show(clock)
        hold(clock)
        compose.onNodeWithTag("readerSurface").performTouchInput { up() }
        compose.waitForIdle()

        compose.onNodeWithTag("topChrome").assertIsDisplayed()
        compose.onNodeWithTag("bottomChrome").assertIsDisplayed()
    }

    @Test
    fun focusModeOffKeepsChromeWhilePlaying() {
        val clock = FakeClock()
        show(clock, settings = ReaderSettings(focusMode = false))
        hold(clock)

        compose.onNodeWithTag("topChrome").assertIsDisplayed()
        compose.onNodeWithTag("currentWord").assertIsDisplayed()
    }

    // --- Browse mode ---

    @Test
    fun verticalDragOpensBrowse() {
        show(FakeClock())
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
            moveBy(Offset(0f, -200f))
            up()
        }
        compose.waitForIdle()

        compose.onNodeWithTag("browseOverlay").assertIsDisplayed()
    }

    @Test
    fun verticalDragWhilePausedOpensBrowse() {
        val clock = FakeClock()
        show(clock)
        hold(clock)
        compose.onNodeWithTag("readerSurface").performTouchInput { up() }
        compose.waitForIdle()

        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
            moveBy(Offset(0f, 200f))
            up()
        }
        compose.waitForIdle()

        compose.onNodeWithTag("browseOverlay").assertIsDisplayed()
    }

    @Test
    fun horizontalDragDoesNotOpenBrowse() {
        val clock = FakeClock()
        show(clock)
        hold(clock)
        compose.onNodeWithTag("readerSurface").performTouchInput { up() }
        compose.waitForIdle()

        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
            moveBy(Offset(-200f, 0f))
            up()
        }
        compose.waitForIdle()

        compose.onNodeWithTag("browseOverlay").assertDoesNotExist()
    }

    @Test
    fun emptyArticleBrowseOpensAndCloses() {
        val clock = FakeClock()
        compose.setContent {
            WikifypTheme {
                ReaderScreen(
                    articleTitle = "Empty",
                    sentences = emptyList(),
                    articleId = 1L,
                    clock = clock,
                )
            }
        }
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
            moveBy(Offset(0f, -200f))
            up()
        }
        compose.waitForIdle()
        compose.onNodeWithTag("browseOverlay").assertIsDisplayed()

        compose.onNodeWithTag("browseOverlay").performTouchInput { click(center) }
        compose.waitForIdle()
        compose.onNodeWithTag("browseOverlay").assertDoesNotExist()
        compose.onNodeWithTag("idleHint").assertIsDisplayed()
    }

    /** Browse opens with the article's masthead: title, and the lead
     *  image when the card carries one. */
    @Test
    fun browseShowsTheMasthead() {
        // The masthead's lead image must resolve offline.
        installFakeImageLoader()
        val clock = FakeClock()
        compose.setContent {
            WikifypTheme {
                ReaderScreen(
                    articleTitle = "Test Article",
                    sentences = testSentences,
                    articleId = 7L,
                    clock = clock,
                    card =
                        testCard(
                            id = 7,
                            title = "Test Article",
                            image = ArticleImage("https://example.com/x.jpg", 400, 300),
                        ),
                )
            }
        }
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
            moveBy(Offset(0f, -200f))
            up()
        }
        compose.waitForIdle()
        // The title also lives in the pinned chrome and the cover behind
        // the overlay (which has its own card image), so assert within
        // the masthead subtree.
        val masthead = compose.onNodeWithTag("browseMasthead", useUnmergedTree = true)
        masthead.assertIsDisplayed()
        masthead.assert(hasAnyDescendant(hasText("Test Article")))
        masthead.assert(hasAnyDescendant(hasTestTag("cardImage")))
    }

    @Test
    fun singleWordArticleFinishesBackToIdle() {
        val clock = FakeClock()
        compose.setContent {
            WikifypTheme {
                ReaderScreen(
                    articleTitle = "One",
                    sentences = listOf(SentenceTokens(listOf("Word."))),
                    articleId = 1L,
                    clock = clock,
                )
            }
        }
        hold(clock)
        compose.onNodeWithTag("currentWord").assertIsDisplayed()
        // Let the word's (ramped) delay elapse; the article finishes and
        // the reader returns to idle.
        clock.advanceBy(10_000)
        compose.waitForIdle()
        compose.onNodeWithTag("idleHint").assertIsDisplayed()
    }

    @Test
    fun tapClosesBrowse() {
        show(FakeClock())
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
            moveBy(Offset(0f, -200f))
            up()
        }
        compose.waitForIdle()
        compose.onNodeWithTag("browseOverlay").assertIsDisplayed()

        compose.onNodeWithTag("browseOverlay").performTouchInput { click(center) }
        compose.waitForIdle()

        compose.onNodeWithTag("browseOverlay").assertDoesNotExist()
    }
}

/**
 * AnimatedVisibility keeps the node in the tree during the exit
 * animation; depending on clock timing it is either gone or not
 * displayed. Both mean "hidden" for these assertions.
 */
private fun SemanticsNodeInteraction.assertDoesNotExistOrHidden() {
    val exists =
        try {
            fetchSemanticsNode()
            true
        } catch (_: AssertionError) {
            false
        }
    if (exists) {
        assertIsNotDisplayed()
    }
}

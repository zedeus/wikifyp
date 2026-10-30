package com.wikifyp.ui.reader

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotDisplayed
import androidx.compose.ui.test.assertTextEquals
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTouchInput
import coil3.SingletonImageLoader
import coil3.annotation.DelicateCoilApi
import com.wikifyp.data.ArticleImage
import com.wikifyp.data.testCard
import com.wikifyp.reader.EventListener
import com.wikifyp.reader.EventType
import com.wikifyp.reader.FakeClock
import com.wikifyp.reader.ReaderEngine
import com.wikifyp.reader.ReadingEvent
import com.wikifyp.reader.SentenceTokens
import com.wikifyp.theme.WikifypTheme
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class ReaderScreenTest {
    @get:Rule val compose = createComposeRule()

    // The idle-cover test swaps the process-wide Coil singleton for a fake;
    // put it back so other classes in this JVM are unaffected.
    @OptIn(DelicateCoilApi::class)
    @After
    fun resetImageLoader() {
        SingletonImageLoader.reset()
    }

    private val testSentences =
        listOf(
            SentenceTokens(listOf("The", "quick", "brown", "fox")),
            SentenceTokens(listOf("jumps", "over", "the", "lazy", "dog.")),
            SentenceTokens(listOf("A", "third", "sentence", "here.")),
        )

    @Test
    fun showsIdleHintBeforeHold() {
        val clock = FakeClock()
        compose.setContent {
            WikifypTheme {
                ReaderScreen(
                    articleTitle = "Test Article",
                    sentences = testSentences,
                    articleId = 1L,
                    clock = clock,
                )
            }
        }

        compose.onNodeWithTag("idleHint").assertIsDisplayed()
        compose.onNodeWithText("Hold to read").assertIsDisplayed()
    }

    @Test
    fun showsArticleTitle() {
        val clock = FakeClock()
        compose.setContent {
            WikifypTheme {
                ReaderScreen(
                    articleTitle = "Philosophy",
                    sentences = testSentences,
                    articleId = 1L,
                    clock = clock,
                )
            }
        }

        compose.onNodeWithText("Philosophy").assertIsDisplayed()
    }

    @Test
    fun showsProgressText() {
        val clock = FakeClock()
        compose.setContent {
            WikifypTheme {
                ReaderScreen(
                    articleTitle = "Test",
                    sentences = testSentences,
                    articleId = 1L,
                    clock = clock,
                )
            }
        }

        // Progress hidden in IDLE, and focus mode hides the chrome while
        // playing: hold to start, lift to pause, then check percentage.
        compose.onNodeWithTag("readerSurface").performTouchInput { down(center) }
        clock.advanceBy(151)
        compose.waitForIdle()
        compose.onNodeWithTag("readerSurface").performTouchInput { up() }
        compose.waitForIdle()
        compose.onNodeWithText("0%").assertIsDisplayed()
    }

    @Test
    fun showsWpmControl() {
        val clock = FakeClock()
        compose.setContent {
            WikifypTheme {
                ReaderScreen(
                    articleTitle = "Test",
                    sentences = testSentences,
                    articleId = 1L,
                    clock = clock,
                )
            }
        }

        compose.onNodeWithTag("wpmControl").assertIsDisplayed()
        compose.onNodeWithText("250 wpm").assertIsDisplayed()
    }

    /**
     * The idle cover: the article's description shows before the first
     * play, leaves when playback starts, and must not come back on pause —
     * a pause keeps the word line on screen, not the cover.
     */
    @Test
    fun idleCoverShowsOnlyWhileIdle() {
        val clock = FakeClock()
        val card =
            testCard(
                id = 7,
                title = "Test Article",
                description = "A thing being tested",
                image = ArticleImage("https://example.com/x.jpg", 400, 300),
            )
        // Tests run offline: the cover's image URL must never reach the
        // network, so it resolves to a flat stand-in through a fake engine.
        installFakeImageLoader()
        compose.setContent {
            WikifypTheme {
                ReaderScreen(
                    articleTitle = "Test Article",
                    sentences = testSentences,
                    articleId = 7L,
                    clock = clock,
                    card = card,
                )
            }
        }

        compose.onNodeWithTag("idleCoverDescription").assertIsDisplayed()
        compose.onNodeWithTag("idleHint").assertIsDisplayed()
        compose.onNodeWithTag("cardImage", useUnmergedTree = true).assertExists()

        // Hold to play: the cover leaves so nothing competes with the
        // focal line.
        compose.onNodeWithTag("readerSurface").performTouchInput { down(center) }
        clock.advanceBy(151)
        compose.waitForIdle()
        compose.onNodeWithTag("idleCoverDescription").assertIsNotDisplayed()

        // Lift to pause: the word line stays, the image rests back in the
        // top third — without its idle-only description.
        compose.onNodeWithTag("readerSurface").performTouchInput { up() }
        compose.waitForIdle()
        compose.onNodeWithTag("idleCoverDescription").assertIsNotDisplayed()
        compose.onNodeWithTag("cardImage", useUnmergedTree = true).assertExists()
        compose.onNodeWithTag("currentWord").assertIsDisplayed()
    }

    /**
     * Leaving mid-read emits a skip that resets the engine to IDLE; the
     * screen must keep showing the word line while it animates out, not
     * flash back to the hold-to-read cover.
     */
    @Test
    fun leavingMidReadDoesNotFlashTheIdleScreen() {
        val clock = FakeClock()
        var backed = false
        compose.setContent {
            WikifypTheme {
                ReaderScreen(
                    articleTitle = "Test",
                    sentences = testSentences,
                    articleId = 1L,
                    clock = clock,
                    onBack = { backed = true },
                )
            }
        }

        // Play a few words, pause.
        compose.onNodeWithTag("readerSurface").performTouchInput { down(center) }
        clock.advanceBy(151)
        compose.waitForIdle()
        clock.advanceBy(600)
        compose.waitForIdle()
        compose.onNodeWithTag("readerSurface").performTouchInput { up() }
        compose.waitForIdle()

        // Leave: the screen keeps the word line, not the idle hints.
        compose.onNodeWithTag("backButton").performClick()
        compose.waitForIdle()
        assertTrue(backed)
        compose.onNodeWithTag("idleHint").assertDoesNotExist()
        compose.onNodeWithTag("currentWord").assertIsDisplayed()
    }

    /**
     * Plays for 480ms at 250 wpm (240ms a word) from a cold start and
     * reports the paused token index. With the speed ramp the first
     * interval alone stretches to 400ms (60% speed), so one word passes;
     * at full speed two do.
     */
    private fun pausedTokenAfterHalfASecond(settings: ReaderSettings): Int {
        val clock = FakeClock()
        val events = mutableListOf<ReadingEvent>()
        compose.setContent {
            WikifypTheme {
                ReaderScreen(
                    articleTitle = "Test",
                    sentences = testSentences,
                    articleId = 1L,
                    clock = clock,
                    onEvent = { events.add(it) },
                    settings = settings,
                )
            }
        }
        compose.onNodeWithTag("readerSurface").performTouchInput { down(center) }
        clock.advanceBy(151)
        compose.waitForIdle()
        clock.advanceBy(480)
        compose.waitForIdle()
        compose.onNodeWithTag("readerSurface").performTouchInput { up() }
        compose.waitForIdle()
        return events.last { it.type == EventType.PAUSE }.token.tokenIdx
    }

    @Test
    fun speedRampEasesTheFirstWords() {
        assertEquals(1, pausedTokenAfterHalfASecond(ReaderSettings()))
    }

    @Test
    fun speedRampOffStartsAtFullSpeed() {
        assertEquals(2, pausedTokenAfterHalfASecond(ReaderSettings(speedRamp = false)))
    }

    /** Without a card the idle reader still shows the hint, nothing broken. */
    @Test
    fun idleCoverDegradesToTheHintWithoutACard() {
        val clock = FakeClock()
        compose.setContent {
            WikifypTheme {
                ReaderScreen(
                    articleTitle = "Test",
                    sentences = testSentences,
                    articleId = 1L,
                    clock = clock,
                    card = null,
                )
            }
        }

        compose.onNodeWithTag("idleHint").assertIsDisplayed()
        compose.onNodeWithTag("idleCoverDescription").assertDoesNotExist()
    }

    @Test
    fun backButtonIsDisplayed() {
        val clock = FakeClock()
        compose.setContent {
            WikifypTheme {
                ReaderScreen(
                    articleTitle = "Test",
                    sentences = testSentences,
                    articleId = 1L,
                    clock = clock,
                )
            }
        }

        compose.onNodeWithTag("backButton").assertIsDisplayed()
    }
}

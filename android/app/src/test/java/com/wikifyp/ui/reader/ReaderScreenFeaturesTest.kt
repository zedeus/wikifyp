package com.wikifyp.ui.reader

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertTextEquals
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTouchInput
import com.wikifyp.data.FeedCard
import com.wikifyp.reader.EventType
import com.wikifyp.reader.FakeClock
import com.wikifyp.reader.ReaderEngine
import com.wikifyp.reader.ReadingEvent
import com.wikifyp.reader.SentenceTokens
import com.wikifyp.theme.WikifypTheme
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/**
 * The reader's round-2 features: the section label, the table of
 * contents, the save button, next-article flow, and the chrome staying
 * reachable while browsing.
 */
@RunWith(RobolectricTestRunner::class)
class ReaderScreenFeaturesTest {
    @get:Rule val compose = createComposeRule()

    private val sentences =
        listOf(
            SentenceTokens(listOf("The", "quick", "brown", "fox")),
            SentenceTokens(listOf("History"), isHeading = true),
            SentenceTokens(listOf("jumps", "over", "the", "lazy", "dog.")),
        )

    private val nextCard =
        FeedCard(
            id = 2,
            title = "Japan",
            source = "wikipedia",
            excerpt = "Japan is an island country.",
            sentences = 3,
            words = 40,
            attribution = "attribution",
            description = "Island country in East Asia",
        )

    private val events = mutableListOf<ReadingEvent>()
    private var nextCalls = 0
    private var savedCalls = 0

    private fun show(
        clock: FakeClock,
        upNext: FeedCard? = nextCard,
        saved: Boolean = false,
    ) {
        events.clear()
        compose.setContent {
            WikifypTheme {
                ReaderScreen(
                    articleTitle = "Test",
                    sentences = sentences,
                    articleId = 1L,
                    clock = clock,
                    onEvent = { events.add(it) },
                    upNext = upNext,
                    onNext = { nextCalls++ },
                    saved = saved,
                    onSaved = { savedCalls++ },
                )
            }
        }
    }

    private fun hold(clock: FakeClock) {
        compose.onNodeWithTag("readerSurface").performTouchInput { down(center) }
        clock.advanceBy(ReaderEngine.HOLD_THRESHOLD_MS + 1)
        compose.waitForIdle()
    }

    private fun pause() {
        compose.onNodeWithTag("readerSurface").performTouchInput { up() }
        compose.waitForIdle()
    }

    // --- Section label ---

    @Test
    fun sectionLabelShowsLeadTitleWhilePaused() {
        val clock = FakeClock()
        show(clock)
        hold(clock)
        pause()
        // Paused in the lead section: labelled with the article title.
        // (Crossing into a named section is covered by the TOC jump test.)
        compose.onNodeWithTag("sectionLabel").assertTextEquals("Test")
    }

    // --- Table of contents ---

    @Test
    fun tocJumpsToSectionAndReportsTheMove() {
        val clock = FakeClock()
        show(clock)
        hold(clock)
        pause()
        events.clear()

        compose.onNodeWithTag("tocButton").performClick()
        compose.waitForIdle()
        compose.onNodeWithTag("tocSheet").assertIsDisplayed()
        compose.onNodeWithText("History").performClick()
        compose.waitForIdle()

        // Jumped to the heading's first token; the reposition reports as
        // the same scroll_back a drag emits.
        compose.onNodeWithTag("sectionLabel").assertTextEquals("History")
        assertEquals(listOf(EventType.SCROLL_BACK), events.map { it.type })
        assertEquals(1, events[0].landedToken?.sentenceIdx)
        assertEquals(0, events[0].landedToken?.tokenIdx)
    }

    // --- Save ---

    @Test
    fun saveEmitsEventAndConfirms() {
        val clock = FakeClock()
        show(clock)

        compose.onNodeWithTag("saveButton").performClick()
        compose.waitForIdle()

        assertEquals(listOf(EventType.SAVE), events.map { it.type })
        assertEquals(1, savedCalls)
        compose.onNodeWithTag("savedPill").assertIsDisplayed()
    }

    @Test
    fun saveWhenAlreadySavedDoesNothing() {
        val clock = FakeClock()
        show(clock, saved = true)

        compose.onNodeWithTag("saveButton").performClick()
        compose.waitForIdle()

        assertTrue(events.isEmpty())
        assertEquals(0, savedCalls)
    }

    // --- Next article ---

    @Test
    fun finishingShowsUpNextAndTapGoesNext() {
        val clock = FakeClock()
        show(clock)
        hold(clock)
        clock.advanceBy(60_000) // play to the end
        compose.waitForIdle()
        pause()

        compose.onNodeWithTag("upNextCard").assertIsDisplayed()
        compose.onNodeWithText("Japan").assertIsDisplayed()
        compose.onNodeWithTag("upNextArticle").performClick()
        assertEquals(1, nextCalls)
    }

    @Test
    fun nextButtonMidArticleSkipsThenGoesNext() {
        val clock = FakeClock()
        show(clock)
        hold(clock)
        pause()
        events.clear()

        compose.onNodeWithTag("nextButton").performClick()
        compose.waitForIdle()

        // An unfinished read counts as a skip before moving on.
        assertEquals(listOf(EventType.SKIP), events.map { it.type })
        assertEquals(1, nextCalls)
    }

    @Test
    fun finishedArticleGoesNextWithoutSkip() {
        val clock = FakeClock()
        show(clock)
        hold(clock)
        clock.advanceBy(60_000)
        compose.waitForIdle()
        pause()
        events.clear()

        compose.onNodeWithTag("nextButton").performClick()
        compose.waitForIdle()

        assertTrue("unexpected events: ${events.map { it.type }}", events.isEmpty())
        assertEquals(1, nextCalls)
    }

    // --- Browse keeps the chrome ---

    @Test
    fun browseOverlayKeepsChromeReachable() {
        val clock = FakeClock()
        show(clock)
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
            moveBy(Offset(0f, -300f))
            up()
        }
        compose.waitForIdle()

        compose.onNodeWithTag("browseOverlay").assertIsDisplayed()
        compose.onNodeWithTag("topChrome").assertIsDisplayed()
        compose.onNodeWithTag("bottomChrome").assertIsDisplayed()
        // The back button still works over the open overlay.
        compose.onAllNodesWithTag("backButton").onFirst().assertIsDisplayed()
    }
}

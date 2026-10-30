package com.wikifyp

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.click
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTouchInput
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Emulator test: opens an article from the feed, then exercises hold
 * (play), lift (pause), drag (scroll back) and resume-from-drag gestures
 * against real time.
 */
@RunWith(AndroidJUnit4::class)
class ReaderTest {
    @get:Rule val compose = createAndroidComposeRule<MainActivity>()

    @OptIn(androidx.compose.ui.test.ExperimentalTestApi::class)
    @Test
    fun holdPauseDragAndResume() {
        // The feed loads asynchronously from bundled assets; wait for it.
        compose.waitUntilAtLeastOneExists(hasText("DNA"), timeoutMillis = 5_000)

        // 1. Open a fixture article.
        compose.onNodeWithText("DNA").performClick()
        compose.waitForIdle()

        // The reader should show the idle cover: hint, lead image and the
        // card's description line, carried over from the tapped feed card.
        // The image is CardImage's own node; the feed is gone, so the only
        // card image on screen is the cover's.
        compose.onNodeWithTag("idleHint").assertIsDisplayed()
        compose.onNodeWithTag("cardImage").assertIsDisplayed()
        compose.onNodeWithTag("idleCoverDescription").assertIsDisplayed()

        // 2. Hold to play: touch down and let real time pass for HandlerClock.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        // Let 400ms of real time pass so the 200ms hold threshold fires.
        Thread.sleep(400)
        compose.waitForIdle()

        // A word should now be displayed.
        compose.onNodeWithTag("currentWord").assertIsDisplayed()

        // 3. Lift to pause.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            up()
        }
        compose.waitForIdle()
        compose.onNodeWithTag("currentWord").assertIsDisplayed()

        // 4. Drag left to scroll back.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
            moveBy(Offset(-200f, 0f))
        }
        compose.waitForIdle()

        // 5. Lift after drag.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            up()
        }
        compose.waitForIdle()
        compose.onNodeWithTag("currentWord").assertIsDisplayed()

        // 6. Hold again to resume from the drag-end position.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
        }
        Thread.sleep(400)
        compose.waitForIdle()
        compose.onNodeWithTag("currentWord").assertIsDisplayed()

        // Clean up.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            up()
        }
    }

    @OptIn(androidx.compose.ui.test.ExperimentalTestApi::class)
    @Test
    fun browseOpensOnVerticalDragAndClosesOnTap() {
        compose.waitUntilAtLeastOneExists(hasText("DNA"), timeoutMillis = 5_000)
        compose.onNodeWithText("DNA").performClick()
        compose.waitForIdle()
        compose.onNodeWithTag("idleHint").assertIsDisplayed()

        // Vertical drag opens the browse overlay.
        compose.onNodeWithTag("readerSurface").performTouchInput {
            down(center)
            moveBy(Offset(0f, -300f))
            up()
        }
        compose.waitForIdle()
        compose.onNodeWithTag("browseOverlay").assertIsDisplayed()

        // A tap on the overlay (past the text) closes it back to the
        // reader, paused on the current word.
        compose.onNodeWithTag("browseOverlay").performTouchInput {
            click(centerRight)
        }
        compose.waitForIdle()
        compose.onNodeWithTag("browseOverlay").assertDoesNotExist()
        compose.onNodeWithTag("currentWord").assertIsDisplayed()
    }

    @OptIn(androidx.compose.ui.test.ExperimentalTestApi::class)
    @Test
    fun tocJumpsToASectionAndSaveConfirms() {
        compose.waitUntilAtLeastOneExists(hasText("DNA"), timeoutMillis = 5_000)
        compose.onNodeWithText("DNA").performClick()
        compose.waitForIdle()

        // Jump to a section from the table of contents. "Properties" is
        // the first heading in the bundled DNA article.
        compose.onNodeWithTag("tocButton").performClick()
        compose.waitUntilAtLeastOneExists(hasTestTag("tocEntry"), timeoutMillis = 5_000)
        compose
            .onNode(hasTestTag("tocEntry") and hasText("Properties"))
            .performClick()
        compose.waitForIdle()
        compose.onNodeWithTag("currentWord").assertIsDisplayed()
        compose.onNodeWithTag("sectionLabel").assertIsDisplayed()

        // Save the article; the bookmark confirms. The device persists
        // bookmarks across runs and the button toggles: if the first tap
        // removed an old bookmark (no pill), tap again to save.
        compose.onNodeWithTag("saveButton").performClick()
        compose.waitForIdle()
        if (compose.onAllNodes(hasTestTag("savedPill")).fetchSemanticsNodes().isEmpty()) {
            compose.onNodeWithTag("saveButton").performClick()
        }
        compose.waitUntilAtLeastOneExists(hasTestTag("savedPill"), timeoutMillis = 5_000)
    }

    @OptIn(androidx.compose.ui.test.ExperimentalTestApi::class)
    @Test
    fun nextButtonMovesToTheFollowingArticle() {
        compose.waitUntilAtLeastOneExists(hasText("DNA"), timeoutMillis = 5_000)
        compose.onNodeWithText("DNA").performClick()
        compose.waitForIdle()
        compose.onNodeWithTag("idleHint").assertIsDisplayed()

        // The feed orders the bundled articles alphabetically: after DNA
        // comes Marie Curie.
        compose.onNodeWithTag("nextButton").performClick()
        compose.waitUntilAtLeastOneExists(hasText("Marie Curie"), timeoutMillis = 5_000)
        compose.onNodeWithTag("idleHint").assertIsDisplayed()
    }

    @OptIn(androidx.compose.ui.test.ExperimentalTestApi::class)
    @Test
    fun settingsSheetTogglesAndCarriesAcrossArticles() {
        compose.waitUntilAtLeastOneExists(hasText("DNA"), timeoutMillis = 5_000)
        compose.onNodeWithText("DNA").performClick()
        compose.waitForIdle()

        // Open the settings sheet and flip bionic ghost words. The device
        // persists settings across runs, so assert against the flipped
        // value rather than assuming a fresh install's default.
        compose.onNodeWithTag("settingsButton").performClick()
        compose.waitUntilAtLeastOneExists(hasTestTag("bionicToggle"), timeoutMillis = 5_000)
        compose.onNodeWithTag("bionicToggle").performClick()
        compose.waitForIdle()
        val toggled = bionicState()

        // Leave the article through the activity's back dispatcher (the
        // reader unmounts, taking the sheet with it) and open another:
        // the setting carried over.
        compose.activityRule.scenario.onActivity {
            it.onBackPressedDispatcher.onBackPressed()
        }
        compose.waitForIdle()
        compose.onNodeWithText("Marie Curie").performClick()
        compose.waitForIdle()
        compose.onNodeWithTag("settingsButton").performClick()
        compose.waitUntilAtLeastOneExists(hasTestTag("bionicToggle"), timeoutMillis = 5_000)
        org.junit.Assert.assertEquals(toggled, bionicState())
    }

    @OptIn(androidx.compose.ui.test.ExperimentalTestApi::class)
    @Test
    fun savedScreenListsAndUnsaveRemoves() {
        compose.waitUntilAtLeastOneExists(hasText("DNA"), timeoutMillis = 5_000)
        compose.onNodeWithText("DNA").performClick()
        compose.waitForIdle()

        // The device persists bookmarks across runs, so drive to a known
        // state: the confirmation pill only appears on a save, never an
        // unsave — if the first tap removed an old bookmark, tap again.
        compose.onNodeWithTag("saveButton").performClick()
        compose.waitForIdle()
        if (compose.onAllNodes(hasTestTag("savedPill")).fetchSemanticsNodes().isEmpty()) {
            compose.onNodeWithTag("saveButton").performClick()
            compose.waitUntilAtLeastOneExists(hasTestTag("savedPill"), timeoutMillis = 5_000)
        }

        // The Saved screen lists the bookmark and opens the reader again.
        compose.activityRule.scenario.onActivity {
            it.onBackPressedDispatcher.onBackPressed()
        }
        compose.waitForIdle()
        compose.onNodeWithTag("savedListButton").performClick()
        compose.waitUntilAtLeastOneExists(hasTestTag("savedList"), timeoutMillis = 5_000)
        compose.onNodeWithText("DNA").performClick()
        compose.waitForIdle()
        compose.onNodeWithTag("idleHint").assertIsDisplayed()

        // Toggle the bookmark off: back lands on the Saved screen, which
        // no longer lists the article. Only this article's absence is
        // asserted — the device may carry other bookmarks from earlier
        // runs or manual use, and this test owns none of them (the empty
        // state itself is covered by the JVM SavedScreen tests).
        compose.onNodeWithTag("saveButton").performClick()
        compose.waitForIdle()
        compose.activityRule.scenario.onActivity {
            it.onBackPressedDispatcher.onBackPressed()
        }
        compose.waitForIdle()
        compose.onNodeWithText("DNA").assertDoesNotExist()

        // Back out to the feed so later tests start from it.
        compose.onNodeWithTag("savedBackButton").performClick()
        compose.waitForIdle()
    }

    private fun bionicState(): androidx.compose.ui.state.ToggleableState =
        compose
            .onNodeWithTag("bionicToggle")
            .fetchSemanticsNode()
            .config[androidx.compose.ui.semantics.SemanticsProperties.ToggleableState]
}

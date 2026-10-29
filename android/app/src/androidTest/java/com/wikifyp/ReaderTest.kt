package com.wikifyp

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsOn
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

        // The reader should show the idle hint.
        compose.onNodeWithTag("idleHint").assertIsDisplayed()

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
    fun settingsSheetTogglesAndCarriesAcrossArticles() {
        compose.waitUntilAtLeastOneExists(hasText("DNA"), timeoutMillis = 5_000)
        compose.onNodeWithText("DNA").performClick()
        compose.waitForIdle()

        // Open the settings sheet and turn bionic ghost words on.
        compose.onNodeWithTag("settingsButton").performClick()
        compose.waitUntilAtLeastOneExists(hasTestTag("bionicToggle"), timeoutMillis = 5_000)
        compose.onNodeWithTag("bionicToggle").performClick()
        compose.onNodeWithTag("bionicToggle").assertIsOn()

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
        compose.onNodeWithTag("bionicToggle").assertIsOn()
    }
}

package com.wikifyp

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.test.assertIsDisplayed
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

    @Test
    fun holdPauseDragAndResume() {
        compose.waitForIdle()

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
}

package com.wikifyp

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Emulator test: the history and stats buttons are present on the feed
 * screen and the feed loads normally with the new top bar icons.
 */
@RunWith(AndroidJUnit4::class)
class HistoryNavigationTest {
    @get:Rule val compose = createAndroidComposeRule<MainActivity>()

    @OptIn(androidx.compose.ui.test.ExperimentalTestApi::class)
    @Test
    fun feedShowsHistoryAndStatsButtons() {
        compose.onNodeWithText("wikifyp").assertIsDisplayed()
        // Wait for the feed to load.
        compose.waitUntilAtLeastOneExists(hasText("DNA"), timeoutMillis = 5_000)
        // History and stats buttons should be present in the top bar.
        compose.onNodeWithTag("historyButton").assertIsDisplayed()
        compose.onNodeWithTag("statsButton").assertIsDisplayed()
    }

    @OptIn(androidx.compose.ui.test.ExperimentalTestApi::class)
    @Test
    fun feedInfoButtonPresent() {
        compose.onNodeWithText("wikifyp").assertIsDisplayed()
        compose.waitUntilAtLeastOneExists(hasText("DNA"), timeoutMillis = 5_000)
        // Feed rows should be present (the non-hero cards).
        compose.waitUntilAtLeastOneExists(hasTestTag("feedRow"), timeoutMillis = 5_000)
    }
}

package com.wikifyp

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/** Launches the real activity on a device/emulator and checks it renders. */
@RunWith(AndroidJUnit4::class)
class MainActivityTest {
    @get:Rule val compose = createAndroidComposeRule<MainActivity>()

    @Test
    fun launch_showsFeed() {
        compose.onNodeWithText("wikifyp").assertIsDisplayed()
        // At least one bundled article card should appear in the feed.
        compose.onNodeWithText("DNA").assertIsDisplayed()
    }
}

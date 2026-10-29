package com.wikifyp

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/** Launches the real activity on a device/emulator and checks it renders. */
@RunWith(AndroidJUnit4::class)
class MainActivityTest {
    @get:Rule val compose = createAndroidComposeRule<MainActivity>()

    @OptIn(androidx.compose.ui.test.ExperimentalTestApi::class)
    @Test
    fun launch_showsFeed() {
        compose.onNodeWithText("wikifyp").assertIsDisplayed()
        // The feed loads asynchronously from bundled assets; wait for it.
        compose.waitUntilAtLeastOneExists(hasText("DNA"), timeoutMillis = 5_000)
        compose.onNodeWithText("DNA").assertIsDisplayed()

        // The lead article renders as the hero card with its bundled lead
        // image; descriptions come from the article metadata. The image
        // node lives inside the card's clickable, so query unmerged.
        compose.onNodeWithTag("heroCard").assertIsDisplayed()
        compose.waitUntil(timeoutMillis = 5_000) {
            compose
                .onAllNodes(hasTestTag("cardImage"), useUnmergedTree = true)
                .fetchSemanticsNodes()
                .isNotEmpty()
        }
        compose
            .onNodeWithText("Molecule that carries genetic information")
            .assertIsDisplayed()
    }
}

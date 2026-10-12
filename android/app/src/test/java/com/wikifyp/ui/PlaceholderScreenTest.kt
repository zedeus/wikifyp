package com.wikifyp.ui

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.wikifyp.theme.WikifypTheme
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class PlaceholderScreenTest {
    @get:Rule val compose = createComposeRule()

    @Test
    fun showsWordmarkTaglineAndEmptyState() {
        compose.setContent { WikifypTheme { PlaceholderScreen() } }

        compose.onNodeWithText("wikifyp").assertIsDisplayed()
        compose.onNodeWithText("An endless feed of Wikipedia, read one word at a time.").assertIsDisplayed()
        compose.onNodeWithText("Nothing to read yet").assertIsDisplayed()
    }

    @Test
    fun actionsInvokeTheirCallbacks() {
        var started = false
        var about = false
        compose.setContent {
            WikifypTheme {
                PlaceholderScreen(onStartReading = { started = true }, onAbout = { about = true })
            }
        }

        compose.onNodeWithText("Start reading").performClick()
        compose.onNodeWithText("About").performClick()

        assertTrue(started)
        assertTrue(about)
    }
}

package com.wikifyp.ui.reader

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertTextEquals
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import com.wikifyp.reader.EventListener
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

@RunWith(RobolectricTestRunner::class)
class ReaderScreenTest {
    @get:Rule val compose = createComposeRule()

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

        // Total tokens: 4 + 5 + 4 = 13. Position 0 → "1 / 13".
        compose.onNodeWithTag("progressText").assertIsDisplayed()
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

package com.wikifyp.ui

import androidx.activity.OnBackPressedDispatcherOwner
import androidx.activity.compose.LocalOnBackPressedDispatcherOwner
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performSemanticsAction
import com.wikifyp.data.FakeFeedRepository
import com.wikifyp.data.FeedResult
import com.wikifyp.data.testArticle
import com.wikifyp.data.testCard
import com.wikifyp.reader.Cancellable
import com.wikifyp.reader.ReaderClock
import com.wikifyp.theme.WikifypTheme
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import java.io.IOException

/**
 * The feed-to-reader flow over a fake repository: tapping a card opens the
 * reader, a failed article load keeps the feed with a notice, back returns.
 */
@RunWith(RobolectricTestRunner::class)
class WikifypAppTest {
    @get:Rule val compose = createComposeRule()

    // The reader's production clock is irrelevant here: nothing plays.
    private val clock =
        object : ReaderClock {
            override fun nowMs(): Long = 0

            override fun schedule(
                delayMs: Long,
                action: () -> Unit,
            ) = Cancellable {}
        }

    private var backOwner: OnBackPressedDispatcherOwner? = null

    private fun show(repository: FakeFeedRepository) {
        compose.setContent {
            backOwner = LocalOnBackPressedDispatcherOwner.current
            WikifypTheme { WikifypApp(repository = repository, clock = clock) }
        }
    }

    private fun pressBack() {
        compose.runOnIdle { backOwner!!.onBackPressedDispatcher.onBackPressed() }
        compose.waitForIdle()
    }

    @Test
    fun systemBackReturnsFromTheReaderToTheFeed() {
        show(
            FakeFeedRepository(
                onFeed = { FeedResult(listOf(testCard())) },
                onArticle = { testArticle(it) },
            ),
        )
        compose.onNodeWithText("Chemistry").performClick()
        compose.onNodeWithTag("idleHint").assertIsDisplayed()

        pressBack()

        // Back closes the article, not the app: the feed is showing again.
        compose.onNodeWithTag("feedList").assertIsDisplayed()
    }

    @Test
    fun wpmCarriesOverBetweenArticles() {
        show(
            FakeFeedRepository(
                onFeed = {
                    FeedResult(
                        listOf(
                            testCard(id = 1, title = "Chemistry"),
                            testCard(id = 2, title = "Physics"),
                        ),
                    )
                },
                onArticle = { testArticle(it, title = if (it == 1L) "Chemistry" else "Physics") },
            ),
        )
        compose.onNodeWithText("Chemistry").performClick()
        // Open the wpm popup and set the slider to its maximum.
        compose.onNodeWithTag("wpmControl").performClick()
        compose.onNodeWithTag("wpmSlider").performSemanticsAction(SemanticsActions.SetProgress) {
            it(600f)
        }
        compose.waitForIdle()

        pressBack()
        compose.onNodeWithText("Physics").performClick()

        // The speed chosen in the last article is still the speed here.
        compose.onNodeWithText("600 wpm").assertIsDisplayed()
    }

    @Test
    fun tappingACardOpensTheReader() {
        show(
            FakeFeedRepository(
                onFeed = { FeedResult(listOf(testCard())) },
                onArticle = { testArticle(it) },
            ),
        )
        compose.onNodeWithText("Chemistry").performClick()
        compose.onNodeWithTag("idleHint").assertIsDisplayed()
    }

    @Test
    fun failedArticleLoadKeepsTheFeedAndExplains() {
        show(
            FakeFeedRepository(
                onFeed = { FeedResult(listOf(testCard())) },
                onArticle = { throw IOException("offline, nothing cached") },
            ),
        )
        compose.onNodeWithText("Chemistry").performClick()
        compose.onNodeWithTag("feedNotice").assertIsDisplayed()
        compose.onNodeWithTag("feedList").assertIsDisplayed()
    }

    @Test
    fun backFromTheReaderReturnsToTheFeed() {
        show(
            FakeFeedRepository(
                onFeed = { FeedResult(listOf(testCard())) },
                onArticle = { testArticle(it) },
            ),
        )
        compose.onNodeWithText("Chemistry").performClick()
        compose.onNodeWithTag("idleHint").assertIsDisplayed()
        compose.onNodeWithTag("backButton").performClick()
        compose.onNodeWithTag("feedList").assertIsDisplayed()
    }
}

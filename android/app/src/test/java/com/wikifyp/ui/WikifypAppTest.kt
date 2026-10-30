package com.wikifyp.ui

import androidx.activity.OnBackPressedDispatcherOwner
import androidx.activity.compose.LocalOnBackPressedDispatcherOwner
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsOn
import androidx.compose.ui.test.assertTextEquals
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
                onFeed = { _ -> FeedResult(listOf(testCard())) },
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
                onFeed = { _ ->
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
    fun readerSettingsCarryOverBetweenArticles() {
        var lastSaved: com.wikifyp.ui.reader.ReaderSettings? = null
        val repository =
            FakeFeedRepository(
                onFeed = { _ ->
                    FeedResult(
                        listOf(
                            testCard(id = 1, title = "Chemistry"),
                            testCard(id = 2, title = "Physics"),
                        ),
                    )
                },
                onArticle = { testArticle(it, title = if (it == 1L) "Chemistry" else "Physics") },
            )
        compose.setContent {
            backOwner = LocalOnBackPressedDispatcherOwner.current
            WikifypTheme {
                WikifypApp(
                    repository = repository,
                    clock = clock,
                    onReaderSettingsChange = { lastSaved = it },
                )
            }
        }

        // Turn bionic ghost words on in the first article's settings.
        compose.onNodeWithText("Chemistry").performClick()
        compose.onNodeWithTag("settingsButton").performClick()
        compose.waitForIdle()
        // Robolectric can't inject touches into the sheet's dialog
        // window; trigger the toggle through its semantics action.
        compose.onNodeWithTag("bionicToggle").performSemanticsAction(SemanticsActions.OnClick)
        compose.waitForIdle()
        org.junit.Assert.assertEquals(true, lastSaved?.bionic)

        // The second article's settings sheet still shows it on: the
        // change carried across articles and reached the caller's store.
        pressBack()
        compose.onNodeWithText("Physics").performClick()
        compose.onNodeWithTag("settingsButton").performClick()
        compose.waitForIdle()
        compose.onNodeWithTag("bionicToggle").assertIsOn()
    }

    @Test
    fun tappingACardOpensTheReader() {
        show(
            FakeFeedRepository(
                onFeed = { _ -> FeedResult(listOf(testCard())) },
                onArticle = { testArticle(it) },
            ),
        )
        compose.onNodeWithText("Chemistry").performClick()
        compose.onNodeWithTag("idleHint").assertIsDisplayed()
    }

    /** The tapped card rides into the reader as its idle cover. */
    @Test
    fun theTappedCardBecomesTheReadersIdleCover() {
        show(
            FakeFeedRepository(
                onFeed = { _ -> FeedResult(listOf(testCard())) },
                onArticle = { testArticle(it) },
            ),
        )
        compose.onNodeWithText("Chemistry").performClick()
        // The card has no description, so the cover's line is the excerpt.
        compose
            .onNodeWithTag("idleCoverDescription")
            .assertTextEquals("Chemistry is the science of matter and how it changes.")
    }

    @Test
    fun failedArticleLoadKeepsTheFeedAndExplains() {
        show(
            FakeFeedRepository(
                onFeed = { _ -> FeedResult(listOf(testCard())) },
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
                onFeed = { _ -> FeedResult(listOf(testCard())) },
                onArticle = { testArticle(it) },
            ),
        )
        compose.onNodeWithText("Chemistry").performClick()
        compose.onNodeWithTag("idleHint").assertIsDisplayed()
        compose.onNodeWithTag("backButton").performClick()
        compose.onNodeWithTag("feedList").assertIsDisplayed()
    }

    @Test
    fun nextButtonOpensTheFollowingFeedArticle() {
        show(
            FakeFeedRepository(
                onFeed = { _ ->
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
        compose.onNodeWithTag("idleHint").assertIsDisplayed()

        // The skip-forward control moves straight to the next article in
        // feed order, without a trip through the feed.
        compose.onNodeWithTag("nextButton").performClick()
        compose.waitForIdle()
        compose.onNodeWithText("Physics").assertIsDisplayed()

        // The last article has nothing after it: no next button.
        compose.onNodeWithTag("nextButton").assertDoesNotExist()
    }

    @Test
    fun savedArticlesShowOnTheSavedScreenAndOpenFromIt() {
        var stored: List<com.wikifyp.data.FeedCard>? = null
        val repository =
            FakeFeedRepository(
                onFeed = { _ ->
                    FeedResult(
                        listOf(
                            testCard(id = 1, title = "Chemistry"),
                            testCard(id = 2, title = "Physics"),
                        ),
                    )
                },
                onArticle = { testArticle(it, title = if (it == 1L) "Chemistry" else "Physics") },
            )
        compose.setContent {
            backOwner = LocalOnBackPressedDispatcherOwner.current
            WikifypTheme {
                WikifypApp(
                    repository = repository,
                    clock = clock,
                    onSavedChange = { stored = it },
                )
            }
        }

        // Bookmark Chemistry in the reader; the snapshot reaches the store.
        compose.onNodeWithText("Chemistry").performClick()
        compose.onNodeWithTag("saveButton").performClick()
        compose.waitForIdle()
        org.junit.Assert.assertEquals(listOf("Chemistry"), stored?.map { it.title })

        // The Saved screen lists it, and tapping it opens the reader.
        compose.onNodeWithTag("backButton").performClick()
        compose.onNodeWithTag("savedListButton").performClick()
        compose.waitForIdle()
        compose.onNodeWithTag("savedList").assertIsDisplayed()
        compose.onNodeWithText("Chemistry").performClick()
        compose.onNodeWithTag("idleHint").assertIsDisplayed()

        // Unsaving removes it: the Saved screen is empty again on return.
        compose.onNodeWithTag("saveButton").performClick()
        compose.waitForIdle()
        org.junit.Assert.assertEquals(emptyList<String>(), stored?.map { it.title })
        compose.onNodeWithTag("backButton").performClick()
        compose.onNodeWithText("Nothing saved yet").assertIsDisplayed()
    }

    @Test
    fun savedScreenShowsAnEmptyStateAndGoesBack() {
        show(
            FakeFeedRepository(
                onFeed = { _ -> FeedResult(listOf(testCard())) },
                onArticle = { testArticle(it) },
            ),
        )
        compose.onNodeWithTag("savedListButton").performClick()
        compose.waitForIdle()
        compose.onNodeWithText("Nothing saved yet").assertIsDisplayed()
        compose.onNodeWithTag("savedBackButton").performClick()
        compose.onNodeWithTag("feedList").assertIsDisplayed()
    }

    @Test
    fun bookmarksPassedInAtStartAreListed() {
        // A restart hands the persisted snapshots back in: the Saved screen
        // lists them even when the feed no longer carries the article.
        compose.setContent {
            WikifypTheme {
                WikifypApp(
                    repository =
                        FakeFeedRepository(
                            onFeed = { _ -> FeedResult(emptyList()) },
                            onArticle = { testArticle(it) },
                        ),
                    clock = clock,
                    initialSaved = listOf(testCard(id = 9, title = "Kept Bookmark")),
                )
            }
        }
        compose.onNodeWithTag("savedListButton").performClick()
        compose.waitForIdle()
        compose.onNodeWithText("Kept Bookmark").assertIsDisplayed()
    }

    @Test
    fun feedPageOpensReaderSettings() {
        show(
            FakeFeedRepository(
                onFeed = { _ -> FeedResult(listOf(testCard())) },
                onArticle = { testArticle(it) },
            ),
        )
        compose.onNodeWithTag("feedSettingsButton").performClick()
        compose.waitForIdle()
        compose.onNodeWithTag("settingsSheet").assertIsDisplayed()
    }
}

package com.wikifyp.ui.feed

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.wikifyp.data.FeedCard
import com.wikifyp.data.testCard
import com.wikifyp.theme.WikifypTheme
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class FeedScreenTest {
    @get:Rule val compose = createComposeRule()

    private fun show(
        state: FeedState,
        notice: String? = null,
        onCardClick: (FeedCard) -> Unit = {},
        onRetry: () -> Unit = {},
        onLoadMore: () -> Unit = {},
        onRefresh: () -> Unit = {},
        isRefreshing: Boolean = false,
    ) {
        compose.setContent {
            WikifypTheme {
                FeedScreen(
                    state = state,
                    onCardClick = onCardClick,
                    onRetry = onRetry,
                    onLoadMore = onLoadMore,
                    onRefresh = onRefresh,
                    isRefreshing = isRefreshing,
                    notice = notice,
                )
            }
        }
    }

    @Test
    fun loadingShowsASpinner() {
        show(FeedState.Loading)
        compose.onNodeWithTag("feedLoading").assertIsDisplayed()
    }

    @Test
    fun emptyExplainsItself() {
        show(FeedState.Empty)
        compose.onNodeWithText("Nothing to read yet").assertIsDisplayed()
    }

    @Test
    fun errorOffersRetry() {
        var retried = false
        show(FeedState.Error, onRetry = { retried = true })
        compose.onNodeWithText("Couldn't load the feed").assertIsDisplayed()
        compose.onNodeWithText("Try again").performClick()
        assertTrue(retried)
    }

    @Test
    fun contentShowsCardsAndClicksThrough() {
        var clicked: FeedCard? = null
        show(
            FeedState.Content(listOf(testCard(), testCard(id = 2, title = "Japan", words = 200)), offline = false),
            onCardClick = { clicked = it },
        )
        compose.onNodeWithText("Chemistry").assertIsDisplayed()
        compose.onNodeWithText("Chemistry is the science of matter and how it changes.").assertIsDisplayed()
        compose.onNodeWithText("5 min", substring = true).assertIsDisplayed()
        compose.onNodeWithTag("offlineBanner").assertDoesNotExist()

        compose.onNodeWithText("Japan").performClick()
        assertEquals(2L, clicked?.id)
    }

    @Test
    fun offlineContentShowsTheBanner() {
        show(FeedState.Content(listOf(testCard()), offline = true))
        compose.onNodeWithTag("offlineBanner").assertIsDisplayed()
        compose.onNodeWithText("Offline — showing your last feed").assertIsDisplayed()
        compose.onNodeWithText("Chemistry").assertIsDisplayed()
    }

    @Test
    fun noticeIsShown() {
        show(FeedState.Content(listOf(testCard()), offline = false), notice = "Couldn't open \"Japan\"")
        compose.onNodeWithTag("feedNotice").assertIsDisplayed()
    }

    @Test
    fun loadingMoreShowsSpinner() {
        show(FeedState.Content(listOf(testCard()), offline = false, loadingMore = true))
        compose.onNodeWithTag("loadingMore").assertIsDisplayed()
    }

    @Test
    fun loadMoreTriggersNearEnd() {
        // With only 1 card, the end is immediately visible, so onLoadMore fires.
        var loadMoreCalled = false
        show(
            FeedState.Content(listOf(testCard()), offline = false, canLoadMore = true),
            onLoadMore = { loadMoreCalled = true },
        )
        compose.waitForIdle()
        assertTrue("expected onLoadMore to be called", loadMoreCalled)
    }

    @Test
    fun loadMoreDoesNotTriggerWhenCanLoadMoreIsFalse() {
        var loadMoreCalled = false
        show(
            FeedState.Content(listOf(testCard()), offline = false, canLoadMore = false),
            onLoadMore = { loadMoreCalled = true },
        )
        compose.waitForIdle()
        assertTrue("onLoadMore should not fire when canLoadMore=false", !loadMoreCalled)
    }
}

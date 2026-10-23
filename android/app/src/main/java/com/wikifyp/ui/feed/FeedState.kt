package com.wikifyp.ui.feed

import com.wikifyp.data.FeedCard
import com.wikifyp.data.FeedRepository
import kotlinx.coroutines.CancellationException

/** What the feed screen shows. */
sealed interface FeedState {
    data object Loading : FeedState

    data class Content(
        val cards: List<FeedCard>,
        val offline: Boolean,
    ) : FeedState

    data object Empty : FeedState

    data object Error : FeedState
}

/**
 * Loads the feed into a screen state. The repository already falls back to
 * its cache, so an exception here means there is nothing to show at all.
 */
suspend fun loadFeed(repository: FeedRepository): FeedState =
    try {
        val result = repository.feed()
        if (result.cards.isEmpty()) {
            FeedState.Empty
        } else {
            FeedState.Content(result.cards, result.offline)
        }
    } catch (e: CancellationException) {
        throw e
    } catch (e: Exception) {
        FeedState.Error
    }

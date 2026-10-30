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
        /** True while the next page is being fetched. */
        val loadingMore: Boolean = false,
        /** False once a page comes back empty (nothing left to load). */
        val canLoadMore: Boolean = true,
    ) : FeedState

    data object Empty : FeedState

    data object Error : FeedState
}

/**
 * Loads the feed into a screen state. The repository already falls back to
 * its cache, so an exception here means there is nothing to show at all.
 */
suspend fun loadFeed(
    repository: FeedRepository,
    exclude: Set<Long> = emptySet(),
): FeedState =
    try {
        val result = repository.feed(exclude)
        if (result.cards.isEmpty()) {
            // First page empty = nothing to read; subsequent page empty =
            // end of pagination, not an empty feed.
            if (exclude.isEmpty()) {
                FeedState.Empty
            } else {
                FeedState.Content(emptyList(), offline = false, canLoadMore = false)
            }
        } else {
            FeedState.Content(result.cards, result.offline)
        }
    } catch (e: CancellationException) {
        throw e
    } catch (e: Exception) {
        FeedState.Error
    }

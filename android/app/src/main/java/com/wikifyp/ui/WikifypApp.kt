package com.wikifyp.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import com.wikifyp.data.Article
import com.wikifyp.data.FeedCard
import com.wikifyp.data.FeedRepository
import com.wikifyp.data.toFeedCard
import com.wikifyp.data.toSentenceTokens
import com.wikifyp.reader.EventListener
import com.wikifyp.reader.ReaderClock
import com.wikifyp.ui.feed.FeedScreen
import com.wikifyp.ui.feed.FeedState
import com.wikifyp.ui.feed.SavedScreen
import com.wikifyp.ui.feed.loadFeed
import com.wikifyp.ui.reader.ReaderScreen
import com.wikifyp.ui.reader.ReaderSettings
import com.wikifyp.ui.reader.ReaderSettingsSheet
import com.wikifyp.ui.reader.readerPalette
import kotlinx.coroutines.CancellationException

/**
 * The app: the feed, and the reader over whichever article was tapped.
 * Loading and navigation state live here, hoisted out of the screens.
 */
@Composable
fun WikifypApp(
    repository: FeedRepository,
    clock: ReaderClock,
    eventListener: EventListener = EventListener {},
    initialReaderSettings: ReaderSettings = ReaderSettings(),
    onReaderSettingsChange: (ReaderSettings) -> Unit = {},
    initialSaved: List<FeedCard> = emptyList(),
    onSavedChange: (List<FeedCard>) -> Unit = {},
) {
    var feedState by remember { mutableStateOf<FeedState>(FeedState.Loading) }
    var retry by remember { mutableIntStateOf(0) }

    // Which article is open survives activity recreation (rotation, theme
    // change); the article itself is re-resolved from the repository, which
    // answers from the offline cache when the network is gone. Only the id
    // is saved — the article text is too big for instance state.
    var openArticleId by rememberSaveable { mutableStateOf<Long?>(null) }
    var article by remember { mutableStateOf<Article?>(null) }
    var notice by remember { mutableStateOf<String?>(null) }

    // Reader settings (speed, type, theme…) belong to the reader, not to
    // one article: they carry between articles. Recreation re-reads them
    // from the caller's store (saves are synchronous), so plain remember
    // is enough.
    var readerSettings by remember { mutableStateOf(initialReaderSettings) }
    val updateReaderSettings: (ReaderSettings) -> Unit = {
        readerSettings = it
        onReaderSettingsChange(it)
    }
    var showFeedSettings by remember { mutableStateOf(false) }

    // The reader's bookmarks, newest first, persisted through the caller's
    // store so the Saved screen survives restarts. The durable cross-device
    // record is the save/unsave event stream.
    var savedCards by remember { mutableStateOf(initialSaved) }
    val updateSaved: (List<FeedCard>) -> Unit = {
        savedCards = it
        onSavedChange(it)
    }
    var showSaved by rememberSaveable { mutableStateOf(false) }

    LaunchedEffect(retry) {
        feedState = FeedState.Loading
        val loaded = loadFeed(repository)
        feedState = loaded
        // With the feed on screen, quietly fill the offline cache so the
        // listed articles stay readable without network.
        if (loaded is FeedState.Content) {
            repository.prefetch(loaded.cards)
        }
    }

    LaunchedEffect(openArticleId) {
        val id = openArticleId
        if (id == null) {
            article = null
            return@LaunchedEffect
        }
        try {
            article = repository.article(id)
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            openArticleId = null
            notice = "Couldn't open that article — check your connection."
        }
    }

    // The system back gesture is handled inside ReaderScreen (which
    // emits a skip event before navigating back). No BackHandler here.

    val current = article
    if (current != null) {
        val sentences = remember(current) { current.toSentenceTokens() }
        // The article after this one in the feed order, for the reader's
        // next-article flow (the up-next card and the skip-forward button).
        val upNext =
            remember(feedState, current.id) {
                val cards = (feedState as? FeedState.Content)?.cards.orEmpty()
                val index = cards.indexOfFirst { it.id == current.id }
                if (index >= 0) cards.getOrNull(index + 1) else null
            }
        // key(): a new article is a new reader — every per-article state
        // (engine, word, overlays) resets structurally instead of each
        // variable remembering to key itself on the id.
        key(current.id) {
            ReaderScreen(
                articleTitle = current.title,
                sentences = sentences,
                articleId = current.id,
                clock = clock,
                onEvent = eventListener,
                onBack = { openArticleId = null },
                settings = readerSettings,
                onSettingsChange = updateReaderSettings,
                upNext = upNext,
                onNext = {
                    notice = null
                    openArticleId = upNext?.id
                },
                saved = savedCards.any { it.id == current.id },
                onSavedChange = { nowSaved ->
                    // A save normally means the article is not in the list
                    // yet, but filter its id anyway: the Saved list is
                    // keyed by id, and a doubled entry would crash it.
                    val others = savedCards.filter { it.id != current.id }
                    updateSaved(
                        if (nowSaved) {
                            listOf(cardFor(current, feedState)) + others
                        } else {
                            others
                        },
                    )
                },
            )
        }
    } else if (showSaved) {
        SavedScreen(
            cards = savedCards,
            onCardClick = { card ->
                notice = null
                openArticleId = card.id
            },
            onBack = { showSaved = false },
            // A failed open bounces back here, so the error banner must
            // show here too — not only on the feed behind it.
            notice = notice,
        )
    } else {
        FeedScreen(
            state = feedState,
            onCardClick = { card ->
                notice = null
                openArticleId = card.id
            },
            onRetry = {
                notice = null
                retry++
            },
            onOpenSettings = { showFeedSettings = true },
            onOpenSaved = { showSaved = true },
            notice = notice,
        )
        if (showFeedSettings) {
            // The same reader settings, reachable before opening an
            // article; drawn on the reader palette the theme setting picks.
            ReaderSettingsSheet(
                settings = readerSettings,
                palette = readerPalette(readerSettings.theme, isSystemInDarkTheme()),
                onSettingsChange = updateReaderSettings,
                onDismiss = { showFeedSettings = false },
            )
        }
    }
}

/**
 * The card snapshot stored for a bookmark: the feed's card when the
 * article is in it, otherwise one derived from the article itself, so
 * saving never depends on the feed still listing the page.
 */
private fun cardFor(
    article: Article,
    feedState: FeedState,
): FeedCard =
    (feedState as? FeedState.Content)?.cards?.find { it.id == article.id }
        ?: article.toFeedCard(source = "")

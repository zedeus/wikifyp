package com.wikifyp.ui

import android.app.Activity
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.ExperimentalSharedTransitionApi
import androidx.compose.animation.SharedTransitionLayout
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat
import com.wikifyp.data.Article
import com.wikifyp.data.FeedCard
import com.wikifyp.data.FeedRepository
import com.wikifyp.data.toFeedCard
import com.wikifyp.data.toSentenceTokens
import com.wikifyp.reader.EventListener
import com.wikifyp.reader.ReaderClock
import com.wikifyp.theme.WikifypTheme
import com.wikifyp.ui.feed.FeedScreen
import com.wikifyp.ui.feed.FeedState
import com.wikifyp.ui.feed.SavedScreen
import com.wikifyp.ui.feed.loadFeed
import com.wikifyp.ui.reader.ReaderScreen
import com.wikifyp.ui.reader.ReaderSettings
import com.wikifyp.ui.reader.ReaderSettingsSheet
import com.wikifyp.ui.reader.readerPalette
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch

/**
 * The app: the feed, and the reader over whichever article was tapped.
 * Loading and navigation state live here, hoisted out of the screens.
 */
@OptIn(ExperimentalSharedTransitionApi::class)
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

    // Pagination: the accumulated cards across pages, and the IDs already
    // served (sent as the exclude list to the server so it never repeats).
    var allCards by remember { mutableStateOf<List<FeedCard>>(emptyList()) }
    var canLoadMore by remember { mutableStateOf(true) }
    var isRefreshing by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    // Shared: reset the feed state and load the first page from scratch.
    suspend fun resetAndLoad() {
        allCards = emptyList()
        canLoadMore = true
        val loaded = loadFeed(repository)
        if (loaded is FeedState.Content) {
            allCards = loaded.cards
            canLoadMore = loaded.canLoadMore
            feedState = FeedState.Content(loaded.cards, loaded.offline, canLoadMore = loaded.canLoadMore)
            repository.prefetch(loaded.cards)
        } else {
            feedState = loaded
        }
    }

    LaunchedEffect(retry) {
        feedState = FeedState.Loading
        resetAndLoad()
    }

    // Load more: appends the next page to the accumulated cards.
    val onLoadMore: () -> Unit = {
        val current = feedState
        if (current is FeedState.Content && !current.loadingMore && canLoadMore) {
            feedState = current.copy(loadingMore = true)
            scope.launch {
                try {
                    val existingIds = allCards.map { it.id }.toSet()
                    val result = repository.feed(existingIds)
                    // Deduplicate: in case the server returns an already-shown
                    // article despite the exclude list (or the repository
                    // ignores it, like fixtures).
                    val newCards = result.cards.filter { it.id !in existingIds }
                    if (newCards.isEmpty()) {
                        canLoadMore = false
                        feedState = current.copy(loadingMore = false, canLoadMore = false)
                    } else {
                        val merged = allCards + newCards
                        allCards = merged
                        feedState =
                            FeedState.Content(
                                cards = merged,
                                offline = result.offline,
                                canLoadMore = true,
                            )
                        repository.prefetch(newCards)
                    }
                } catch (e: CancellationException) {
                    throw e
                } catch (_: Exception) {
                    feedState = current.copy(loadingMore = false)
                }
            }
        }
    }

    // Pull-to-refresh: reloads from scratch, keeping the user on the feed.
    val onRefresh: () -> Unit = {
        if (!isRefreshing) {
            isRefreshing = true
            scope.launch {
                resetAndLoad()
                isRefreshing = false
            }
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

    // The feed's scroll position lives here, above the AnimatedContent, so
    // coming back from the reader lands on the same spot — and the shared
    // image has its card on screen to fly back into.
    val feedListState = rememberLazyListState()

    // Which screen is showing. The reader carries its Article so the
    // outgoing screen can keep rendering it while it animates away, after
    // the `article` state has already moved on.
    val screen: Screen =
        when {
            article != null -> Screen.Reader(article!!)
            showSaved -> Screen.Saved
            else -> Screen.Feed
        }

    // The reader theme is the app theme: paper and sepia read as light,
    // ink and black as dark, system follows the device. One picker themes
    // the whole app, so the feed never glares around a dark reader.
    val appDark = readerPalette(readerSettings.theme, isSystemInDarkTheme()).isDark

    WikifypTheme(darkTheme = appDark) {
        // Status-bar icons must stay legible on the themed surface, which
        // can differ from the system theme. The app theme is the one owner
        // of the status bar; the reader surface follows the same setting.
        val view = LocalView.current
        DisposableEffect(appDark) {
            val window = (view.context as? Activity)?.window
            val controller = window?.let { WindowCompat.getInsetsController(it, view) }
            val wasLight = controller?.isAppearanceLightStatusBars
            controller?.isAppearanceLightStatusBars = !appDark
            onDispose {
                if (controller != null && wasLight != null) {
                    controller.isAppearanceLightStatusBars = wasLight
                }
            }
        }
        SharedTransitionLayout {
            AnimatedContent(
                targetState = screen,
                transitionSpec = { fadeThrough },
                // Key the reader by article id (matching Screen.Reader's own
                // id-based equality).
                contentKey = { if (it is Screen.Reader) it.article.id else it },
                label = "screens",
            ) { target ->
                // Remembered: a fresh NavScopes per recomposition would change
                // the local's value and invalidate every shared-element reader
                // below on each app-level state change. Both scopes are stable
                // for this content's lifetime.
                val navScopes = remember { NavScopes(this@SharedTransitionLayout, this) }
                CompositionLocalProvider(LocalNavScopes provides navScopes) {
                    when (target) {
                        is Screen.Reader -> {
                            val current = target.article
                            val sentences = remember(current) { current.toSentenceTokens() }
                            // The article after this one in the feed order, for the reader's
                            // next-article flow (the up-next card and the skip-forward button).
                            val upNext =
                                remember(feedState, current.id) {
                                    val cards = (feedState as? FeedState.Content)?.cards.orEmpty()
                                    val index = cards.indexOfFirst { it.id == current.id }
                                    if (index >= 0) cards.getOrNull(index + 1) else null
                                }
                            // Remembered: the fallback derives a card from the
                            // whole article, too heavy to redo per recomposition.
                            val card =
                                remember(feedState, current.id) {
                                    cardFor(current, feedState)
                                }
                            ReaderScreen(
                                articleTitle = current.title,
                                sentences = sentences,
                                articleId = current.id,
                                clock = clock,
                                onEvent = eventListener,
                                onBack = { openArticleId = null },
                                settings = readerSettings,
                                onSettingsChange = updateReaderSettings,
                                card = card,
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
                                            listOf(card) + others
                                        } else {
                                            others
                                        },
                                    )
                                },
                            )
                        }

                        Screen.Saved -> {
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
                        }

                        Screen.Feed -> {
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
                                onLoadMore = onLoadMore,
                                onRefresh = onRefresh,
                                isRefreshing = isRefreshing,
                                onOpenSettings = { showFeedSettings = true },
                                onOpenSaved = { showSaved = true },
                                notice = notice,
                                listState = feedListState,
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
                }
            }
        }
    }
}

/** The three top-level screens the app animates between. */
private sealed interface Screen {
    data object Feed : Screen

    data object Saved : Screen

    class Reader(
        val article: Article,
    ) : Screen {
        // A reader is its article id. `screen` is rebuilt every app-level
        // recomposition and AnimatedContent equality-checks it each time;
        // data-class equality would deep-compare the whole article text.
        override fun equals(other: Any?) = other is Reader && other.article.id == article.id

        override fun hashCode() = article.id.hashCode()
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

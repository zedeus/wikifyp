package com.wikifyp.ui

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import com.wikifyp.data.Article
import com.wikifyp.data.FeedRepository
import com.wikifyp.data.toSentenceTokens
import com.wikifyp.reader.EventListener
import com.wikifyp.reader.ReaderClock
import com.wikifyp.ui.feed.FeedScreen
import com.wikifyp.ui.feed.FeedState
import com.wikifyp.ui.feed.loadFeed
import com.wikifyp.ui.reader.ReaderScreen
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
    initialWpm: Int = 250,
    onWpmChange: (Int) -> Unit = {},
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

    // The reading speed belongs to the reader, not to one article: it
    // carries between articles, survives recreation, and the caller may
    // persist it across launches.
    var wpm by rememberSaveable { mutableIntStateOf(initialWpm) }

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
        ReaderScreen(
            articleTitle = current.title,
            sentences = sentences,
            articleId = current.id,
            clock = clock,
            onEvent = eventListener,
            onBack = { openArticleId = null },
            initialWpm = wpm,
            onWpmChange = {
                wpm = it
                onWpmChange(it)
            },
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
            notice = notice,
        )
    }
}

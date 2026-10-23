package com.wikifyp.ui.feed

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import com.wikifyp.data.FeedCard
import com.wikifyp.theme.Spacing
import com.wikifyp.ui.components.ContentCard
import com.wikifyp.ui.components.PrimaryButton

/**
 * The feed: a list of article cards, with loading, empty, error and offline
 * states. Tapping a card opens the reader; [notice] is a transient line
 * under the title (e.g. an article that failed to open).
 */
@Composable
fun FeedScreen(
    state: FeedState,
    onCardClick: (FeedCard) -> Unit,
    onRetry: () -> Unit,
    notice: String? = null,
    modifier: Modifier = Modifier,
) {
    Surface(modifier = modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            modifier =
                Modifier
                    .safeDrawingPadding()
                    .fillMaxSize(),
        ) {
            Text(
                text = "wikifyp",
                style = MaterialTheme.typography.displayLarge,
                color = MaterialTheme.colorScheme.onBackground,
                modifier = Modifier.padding(horizontal = Spacing.lg, vertical = Spacing.md),
            )
            if (state is FeedState.Content && state.offline) {
                Banner("Offline — showing your last feed", Modifier.testTag("offlineBanner"))
            }
            if (notice != null) {
                Banner(notice, Modifier.testTag("feedNotice"))
            }
            when (state) {
                FeedState.Loading -> {
                    Centered {
                        CircularProgressIndicator(modifier = Modifier.testTag("feedLoading"))
                    }
                }

                FeedState.Empty -> {
                    Centered {
                        Message("Nothing to read yet", "Check back once articles have been added.")
                    }
                }

                FeedState.Error -> {
                    Centered {
                        Message("Couldn't load the feed", "Check your connection and try again.")
                        Spacer(Modifier.height(Spacing.md))
                        PrimaryButton(text = "Try again", onClick = onRetry)
                    }
                }

                is FeedState.Content -> {
                    LazyColumn(
                        modifier =
                            Modifier
                                .fillMaxSize()
                                .padding(horizontal = Spacing.lg)
                                .testTag("feedList"),
                        verticalArrangement = Arrangement.spacedBy(Spacing.sm),
                        contentPadding = PaddingValues(bottom = Spacing.lg),
                    ) {
                        items(state.cards, key = { it.id }) { card ->
                            FeedCardItem(card, onClick = { onCardClick(card) })
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun FeedCardItem(
    card: FeedCard,
    onClick: () -> Unit,
) {
    ContentCard(
        modifier =
            Modifier
                .fillMaxWidth()
                .clickable(onClick = onClick),
    ) {
        Text(
            text = card.title,
            style = MaterialTheme.typography.titleLarge,
            color = MaterialTheme.colorScheme.onSurface,
        )
        if (card.excerpt.isNotEmpty()) {
            Spacer(Modifier.height(Spacing.xs))
            Text(
                text = card.excerpt,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
        }
        Spacer(Modifier.height(Spacing.xs))
        Text(
            text = readingTime(card.words),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

/** A rough length signal for the card, at a typical reading speed. */
internal fun readingTime(words: Int): String {
    val minutes = (words + 249) / 250
    return if (minutes <= 1) "1 min" else "$minutes min"
}

@Composable
private fun Centered(content: @Composable () -> Unit) {
    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            modifier = Modifier.padding(Spacing.xl),
        ) {
            content()
        }
    }
}

@Composable
private fun Message(
    title: String,
    detail: String,
) {
    Text(
        text = title,
        style = MaterialTheme.typography.titleLarge,
        color = MaterialTheme.colorScheme.onBackground,
        textAlign = TextAlign.Center,
    )
    Spacer(Modifier.height(Spacing.xs))
    Text(
        text = detail,
        style = MaterialTheme.typography.bodyMedium,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        textAlign = TextAlign.Center,
    )
}

@Composable
private fun Banner(
    text: String,
    modifier: Modifier = Modifier,
) {
    Surface(
        color = MaterialTheme.colorScheme.surfaceContainerHigh,
        shape = MaterialTheme.shapes.medium,
        modifier =
            modifier
                .fillMaxWidth()
                .padding(horizontal = Spacing.lg)
                .padding(bottom = Spacing.sm),
    ) {
        Text(
            text = text,
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(horizontal = Spacing.md, vertical = Spacing.sm),
        )
    }
}

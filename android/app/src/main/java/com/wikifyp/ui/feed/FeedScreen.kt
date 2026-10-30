package com.wikifyp.ui.feed

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.BookmarkBorder
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.wikifyp.data.FeedCard
import com.wikifyp.theme.BlueSoft
import com.wikifyp.theme.Reading
import com.wikifyp.theme.Spacing
import com.wikifyp.ui.components.PrimaryButton
import com.wikifyp.ui.sharedArticleImage

/**
 * The feed: a hero card for the lead article, then a grouped list of the
 * rest, under a compact top bar with the wordmark and reader settings.
 * Tapping a card opens the reader; [notice] is a transient banner (e.g. an
 * article that failed to open).
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun FeedScreen(
    state: FeedState,
    onCardClick: (FeedCard) -> Unit,
    onRetry: () -> Unit,
    onLoadMore: () -> Unit = {},
    onRefresh: () -> Unit = {},
    isRefreshing: Boolean = false,
    onOpenSettings: () -> Unit = {},
    onOpenSaved: () -> Unit = {},
    notice: String? = null,
    listState: LazyListState = rememberLazyListState(),
    modifier: Modifier = Modifier,
) {
    Surface(modifier = modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            modifier =
                Modifier
                    .safeDrawingPadding()
                    .fillMaxSize(),
        ) {
            TopBar(onOpenSettings, onOpenSaved)
            if (state is FeedState.Content && state.offline) {
                Banner("Offline — showing your last feed", Modifier.testTag("offlineBanner"))
            }
            if (notice != null) {
                Banner(notice, Modifier.testTag("feedNotice"))
            }
            when (state) {
                FeedState.Loading -> {
                    FeedSkeleton()
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
                    PullToRefreshBox(
                        isRefreshing = isRefreshing,
                        onRefresh = onRefresh,
                        modifier = Modifier.fillMaxSize().testTag("feedPullRefresh"),
                    ) {
                        FeedList(
                            cards = state.cards,
                            onCardClick = onCardClick,
                            onLoadMore = onLoadMore,
                            loadingMore = state.loadingMore,
                            canLoadMore = state.canLoadMore,
                            listState = listState,
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun TopBar(
    onOpenSettings: () -> Unit,
    onOpenSaved: () -> Unit,
) {
    Row(
        modifier =
            Modifier
                .fillMaxWidth()
                .padding(start = Spacing.md, end = Spacing.xs, top = Spacing.xs),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = "wikifyp",
            style = MaterialTheme.typography.headlineSmall,
            color = MaterialTheme.colorScheme.onBackground,
            modifier = Modifier.weight(1f),
        )
        IconButton(
            onClick = onOpenSaved,
            modifier = Modifier.testTag("savedListButton"),
        ) {
            Icon(
                imageVector = Icons.Outlined.BookmarkBorder,
                contentDescription = "Saved articles",
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        IconButton(
            onClick = onOpenSettings,
            modifier = Modifier.testTag("feedSettingsButton"),
        ) {
            Icon(
                imageVector = Icons.Outlined.Settings,
                contentDescription = "Reader settings",
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

/** How close to the end of the list (in items) the user must scroll before
 *  the next page loads. Three items of lead time hides latency on a typical
 *  connection without eager-loading pages the reader will never see. */
private const val LOAD_MORE_THRESHOLD = 3

@Composable
private fun FeedList(
    cards: List<FeedCard>,
    onCardClick: (FeedCard) -> Unit,
    onLoadMore: () -> Unit,
    loadingMore: Boolean,
    canLoadMore: Boolean,
    listState: LazyListState,
) {
    // Trigger load-more when the user scrolls near the end. Keyed on the
    // captured params: they are plain values, not snapshot state, so an
    // unkeyed derivedStateOf would read the first composition's values
    // forever (e.g. a stale canLoadMore=true after the last page).
    val shouldLoadMore by remember(listState, canLoadMore, loadingMore) {
        derivedStateOf {
            val info = listState.layoutInfo
            val lastVisible = info.visibleItemsInfo.lastOrNull()?.index ?: 0
            canLoadMore && !loadingMore && lastVisible >= info.totalItemsCount - LOAD_MORE_THRESHOLD
        }
    }
    LaunchedEffect(shouldLoadMore) {
        if (shouldLoadMore) onLoadMore()
    }

    LazyColumn(
        state = listState,
        modifier =
            Modifier
                .fillMaxSize()
                .testTag("feedList"),
        contentPadding =
            PaddingValues(start = Spacing.md, end = Spacing.md, bottom = Spacing.lg),
    ) {
        itemsIndexed(cards, key = { _, card -> card.id }) { index, card ->
            if (index == 0) {
                HeroCard(card, onClick = { onCardClick(card) })
                Spacer(Modifier.height(Spacing.md))
            } else {
                FeedRow(
                    card = card,
                    shape = rowShape(index - 1, cards.size - 1),
                    onClick = { onCardClick(card) },
                )
                if (index < cards.lastIndex) Spacer(Modifier.height(2.dp))
            }
        }
        if (loadingMore) {
            item(key = "__loading_more__") {
                Box(
                    modifier =
                        Modifier
                            .fillMaxWidth()
                            .padding(vertical = Spacing.md)
                            .testTag("loadingMore"),
                    contentAlignment = Alignment.Center,
                ) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(24.dp),
                        strokeWidth = 2.dp,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }
    }
}

/**
 * Grouped-list corners: the first row rounds its top, the last its bottom,
 * middles stay nearly square — with 2dp gaps the group reads as one card.
 */
internal fun rowShape(
    index: Int,
    count: Int,
): Shape {
    val big = 16.dp
    val small = 4.dp
    val top = if (index == 0) big else small
    val bottom = if (index == count - 1) big else small
    return RoundedCornerShape(topStart = top, topEnd = top, bottomStart = bottom, bottomEnd = bottom)
}

/** The hero image's aspect ratio; the loading skeleton mirrors it. */
internal const val HERO_IMAGE_ASPECT = 16f / 10f

// The scrim under the hero title: an eased ramp (not a two-stop gradient)
// so the image darkens gently before the text needs full contrast.
private val HeroScrim =
    Brush.verticalGradient(
        0f to Color.Transparent,
        0.18f to Color.Black.copy(alpha = 0.05f),
        0.38f to Color.Black.copy(alpha = 0.15f),
        0.58f to Color.Black.copy(alpha = 0.35f),
        0.76f to Color.Black.copy(alpha = 0.58f),
        0.90f to Color.Black.copy(alpha = 0.76f),
        1f to Color.Black.copy(alpha = 0.85f),
    )

/** The lead article: full-width image, title on an eased scrim, excerpt. */
@Composable
private fun HeroCard(
    card: FeedCard,
    onClick: () -> Unit,
) {
    Column(
        modifier =
            Modifier
                .fillMaxWidth()
                .clip(MaterialTheme.shapes.large)
                .background(MaterialTheme.colorScheme.surfaceContainer)
                .clickable(onClick = onClick)
                .testTag("heroCard"),
    ) {
        Box {
            CardImage(
                image = card.image,
                title = card.title,
                cardId = card.id,
                modifier =
                    Modifier
                        .fillMaxWidth()
                        .aspectRatio(HERO_IMAGE_ASPECT)
                        .sharedArticleImage(card.id),
            )
            Box(
                Modifier
                    .matchParentSize()
                    .background(HeroScrim),
            )
            Column(
                modifier =
                    Modifier
                        .align(Alignment.BottomStart)
                        .padding(Spacing.md),
            ) {
                // The scrim is dark in every theme, so the eyebrow keeps
                // the soft (dark-surface) accent rather than the theme's.
                Text(
                    text = "FEATURED",
                    style =
                        MaterialTheme.typography.labelSmall.copy(
                            fontWeight = FontWeight.SemiBold,
                            letterSpacing = 1.2.sp,
                        ),
                    color = BlueSoft,
                )
                Spacer(Modifier.height(2.dp))
                Text(
                    text = card.title,
                    style = MaterialTheme.typography.headlineMedium,
                    color = Color.White,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                if (card.description.isNotEmpty()) {
                    Text(
                        text = card.description,
                        style = MaterialTheme.typography.labelLarge,
                        color = Color.White.copy(alpha = 0.85f),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
        }
        Column(modifier = Modifier.padding(Spacing.md)) {
            if (card.excerpt.isNotEmpty()) {
                Text(
                    text = card.excerpt,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 3,
                    overflow = TextOverflow.Ellipsis,
                )
                Spacer(Modifier.height(Spacing.sm))
            }
            MetaLine(card)
        }
    }
}

/** One article in the grouped list: text left, thumbnail right. */
@Composable
internal fun FeedRow(
    card: FeedCard,
    shape: Shape,
    onClick: () -> Unit,
) {
    Row(
        modifier =
            Modifier
                .fillMaxWidth()
                .clip(shape)
                .background(MaterialTheme.colorScheme.surfaceContainer)
                .clickable(onClick = onClick)
                .padding(Spacing.md)
                .testTag("feedRow"),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = card.title,
                style =
                    MaterialTheme.typography.bodyLarge.copy(
                        fontFamily = Reading,
                        fontWeight = FontWeight.Medium,
                    ),
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            val sub = card.subtitle
            if (sub.isNotEmpty()) {
                Spacer(Modifier.height(2.dp))
                Text(
                    text = sub,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            Spacer(Modifier.height(Spacing.xs))
            MetaLine(card)
        }
        Spacer(Modifier.size(Spacing.md, 0.dp))
        CardImage(
            image = card.image,
            title = card.title,
            cardId = card.id,
            modifier =
                Modifier
                    .size(64.dp)
                    .sharedArticleImage(card.id)
                    .clip(MaterialTheme.shapes.small),
        )
    }
}

@Composable
private fun MetaLine(card: FeedCard) {
    Text(
        text = "${readingTime(card.words)} · ${card.words} words",
        style = MaterialTheme.typography.labelSmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
    )
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
internal fun Banner(
    text: String,
    modifier: Modifier = Modifier,
) {
    Surface(
        color = MaterialTheme.colorScheme.surfaceContainerHigh,
        shape = MaterialTheme.shapes.medium,
        modifier =
            modifier
                .fillMaxWidth()
                .padding(horizontal = Spacing.md)
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

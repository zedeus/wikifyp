package com.wikifyp.ui.reader

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.wikifyp.data.FeedCard
import com.wikifyp.theme.Spacing
import com.wikifyp.ui.feed.CardImage
import com.wikifyp.ui.sharedArticleImage

/**
 * The card shown when an article finishes: the next feed article one tap
 * away, so reading flows on without a trip back to the feed. With nothing
 * left in the feed it offers the way back instead.
 */
@Composable
internal fun UpNextCard(
    card: FeedCard?,
    palette: ReaderPalette,
    onNext: () -> Unit,
    onBackToFeed: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Surface(
        color = palette.surface,
        contentColor = palette.word,
        shape = MaterialTheme.shapes.large,
        modifier = modifier.testTag("upNextCard"),
    ) {
        if (card == null) {
            Text(
                text = "That's everything — back to the feed",
                style = MaterialTheme.typography.bodyLarge,
                color = palette.word,
                modifier =
                    Modifier
                        .clickable(onClick = onBackToFeed)
                        .padding(Spacing.md)
                        .testTag("backToFeed"),
            )
        } else {
            Row(
                modifier =
                    Modifier
                        .clickable(onClick = onNext)
                        .padding(Spacing.md)
                        .fillMaxWidth()
                        .testTag("upNextArticle"),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                CardImage(
                    image = card.image,
                    title = card.title,
                    cardId = card.id,
                    modifier =
                        Modifier
                            .size(56.dp)
                            .sharedArticleImage(card.id)
                            .clip(MaterialTheme.shapes.small),
                )
                Spacer(Modifier.width(Spacing.md))
                Column(Modifier.weight(1f)) {
                    Text(
                        text = "UP NEXT",
                        style =
                            MaterialTheme.typography.labelSmall.copy(letterSpacing = 1.2.sp),
                        color = palette.accent,
                    )
                    Text(
                        text = card.title,
                        style = MaterialTheme.typography.titleMedium,
                        color = palette.word,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    val sub = card.subtitle
                    if (sub.isNotEmpty()) {
                        Text(
                            text = sub,
                            style = MaterialTheme.typography.labelMedium,
                            color = palette.chrome,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
            }
        }
    }
}

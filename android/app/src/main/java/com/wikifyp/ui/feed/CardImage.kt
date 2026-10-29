package com.wikifyp.ui.feed

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.testTag
import coil3.compose.AsyncImage
import com.wikifyp.data.ArticleImage
import com.wikifyp.theme.TileGrounds
import com.wikifyp.theme.TileLetters

/**
 * An article's lead image, cropped to fill, or — for articles without a
 * free lead image — a designed stand-in: a deep muted ground with the
 * title's initial in a matching pastel, rotated by article id so
 * neighbouring cards differ. The caller owns size, shape and aspect ratio.
 */
@Composable
fun CardImage(
    image: ArticleImage?,
    title: String,
    cardId: Long,
    modifier: Modifier = Modifier,
) {
    if (image == null) {
        InitialTile(title, cardId, modifier)
    } else {
        AsyncImage(
            model = image.url,
            contentDescription = null,
            contentScale = ContentScale.Crop,
            modifier =
                modifier
                    .background(MaterialTheme.colorScheme.surfaceVariant)
                    .testTag("cardImage"),
        )
    }
}

@Composable
private fun InitialTile(
    title: String,
    cardId: Long,
    modifier: Modifier = Modifier,
) {
    val slot = Math.floorMod(cardId, TileGrounds.size.toLong()).toInt()
    Box(
        modifier = modifier.background(TileGrounds[slot]).testTag("initialTile"),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = title.take(1).uppercase(),
            style = MaterialTheme.typography.displayMedium,
            color = TileLetters[slot],
        )
    }
}

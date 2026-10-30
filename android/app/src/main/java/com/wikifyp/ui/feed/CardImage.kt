package com.wikifyp.ui.feed

import android.graphics.ColorMatrix
import android.graphics.ColorMatrixColorFilter
import android.graphics.RenderEffect
import android.os.Build
import android.util.LruCache
import androidx.compose.foundation.background
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Box
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.asComposeRenderEffect
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.testTag
import coil3.BitmapImage
import coil3.compose.AsyncImage
import coil3.compose.AsyncImagePainter
import com.wikifyp.data.ArticleImage
import com.wikifyp.theme.TileGrounds
import com.wikifyp.theme.TileLetters
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch

// Dark-mode dimming decisions by image URL. Deciding samples a software
// copy of the whole (hardware) bitmap — too heavy to redo every time a
// card scrolls back into view.
private val dimCache = LruCache<String, Boolean>(128)

/**
 * An article's lead image, cropped to fill, or — for articles without a
 * free lead image — a designed stand-in: a deep muted ground with the
 * title's initial in a matching pastel, rotated by article id so
 * neighbouring cards differ. The caller owns size, shape and aspect ratio —
 * and the [com.wikifyp.ui.sharedArticleImage] modifier when the image
 * should morph across screens (apply it before the clip).
 *
 * In dark mode, images with bright corners (white-background diagrams)
 * are dimmed to reduce glare.
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
        val isDark = isSystemInDarkTheme()
        var shouldDim by remember(image.url) { mutableStateOf(false) }
        val scope = rememberCoroutineScope()

        AsyncImage(
            model = image.url,
            contentDescription = null,
            contentScale = ContentScale.Crop,
            onState = { state ->
                if (state is AsyncImagePainter.State.Success && isDark) {
                    val cached = dimCache.get(image.url)
                    if (cached != null) {
                        shouldDim = cached
                    } else {
                        val bitmap = (state.result.image as? BitmapImage)?.bitmap
                        if (bitmap != null) {
                            // Deciding copies the whole hardware bitmap to
                            // software — a GPU readback, too slow for the
                            // main thread while the feed scrolls. Cache only
                            // real decisions: an unreadable image now may be
                            // readable on the next load.
                            scope.launch(Dispatchers.Default) {
                                val dim = needsDarkModeDimming(bitmap)
                                dimCache.put(image.url, dim)
                                shouldDim = dim
                            }
                        }
                    }
                }
            },
            modifier =
                modifier
                    .background(MaterialTheme.colorScheme.surfaceVariant)
                    .then(
                        // isDark gates the effect too: a decision made in dark
                        // mode must not keep dimming after a flip to light.
                        if (isDark && shouldDim && Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                            // Reduce brightness to 70% — dims white backgrounds
                            // without destroying the image. The effect is
                            // hoisted so every dimmed card shares one instance.
                            val dimEffect =
                                remember {
                                    val cm = ColorMatrix().apply { setScale(0.7f, 0.7f, 0.7f, 1f) }
                                    RenderEffect
                                        .createColorFilterEffect(ColorMatrixColorFilter(cm))
                                        .asComposeRenderEffect()
                                }
                            Modifier.graphicsLayer { renderEffect = dimEffect }
                        } else {
                            Modifier
                        },
                    ).testTag("cardImage"),
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

package com.wikifyp.ui.feed

import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.State
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import com.wikifyp.theme.Spacing

/**
 * A skeleton shaped like the loaded feed — hero card, then list rows — with
 * a soft diagonal shimmer sweeping across, so the wait reads as the feed
 * arriving rather than a spinner.
 */
@Composable
internal fun FeedSkeleton(modifier: Modifier = Modifier) {
    val transition = rememberInfiniteTransition(label = "shimmer")
    // A fraction, not pixels: each block maps it onto its own diagonal in
    // the draw phase, so the band sweeps corner to corner on any size.
    val sweep =
        transition.animateFloat(
            initialValue = 0f,
            targetValue = 1f,
            animationSpec =
                infiniteRepeatable(tween(durationMillis = 1100, easing = LinearEasing)),
            label = "shimmerSweep",
        )
    val base = MaterialTheme.colorScheme.surfaceVariant
    val highlight = MaterialTheme.colorScheme.surfaceContainerHigh

    Column(
        modifier =
            modifier
                .fillMaxWidth()
                .padding(horizontal = Spacing.md)
                .testTag("feedLoading"),
    ) {
        Box(
            Modifier
                .fillMaxWidth()
                .aspectRatio(HERO_IMAGE_ASPECT)
                .clip(MaterialTheme.shapes.large)
                .shimmerFill(sweep, base, highlight),
        )
        Spacer(Modifier.height(Spacing.md))
        repeat(4) { i ->
            Box(
                Modifier
                    .fillMaxWidth()
                    .height(84.dp)
                    .clip(rowShape(i, 4))
                    .shimmerFill(sweep, base, highlight),
            )
            Spacer(Modifier.height(2.dp))
        }
    }
}

/**
 * Fills with the sweeping gradient, reading [sweep] only in the draw
 * phase: the animation repaints the blocks without recomposing the
 * skeleton every frame.
 */
private fun Modifier.shimmerFill(
    sweep: State<Float>,
    base: Color,
    highlight: Color,
): Modifier =
    drawBehind {
        // The band's leading edge travels the block's full diagonal (plus
        // the band width on each side, so the loop restarts off-screen).
        val band = (size.width + size.height) / 3f
        val s = sweep.value * (size.width + size.height + 2 * band) - band
        drawRect(
            Brush.linearGradient(
                colors = listOf(base, highlight, base),
                start = Offset(s - band, s - band),
                end = Offset(s, s),
            ),
        )
    }

package com.wikifyp.ui

import androidx.compose.animation.AnimatedVisibilityScope
import androidx.compose.animation.BoundsTransform
import androidx.compose.animation.ContentTransform
import androidx.compose.animation.ExperimentalSharedTransitionApi
import androidx.compose.animation.SharedTransitionScope
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.togetherWith
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.ui.Modifier
import com.wikifyp.theme.Motion

/**
 * The scopes a screen needs to take part in shared-element morphs across
 * the app-level [androidx.compose.animation.AnimatedContent]. Null outside
 * it (previews, screens composed alone in tests), in which case the shared
 * modifiers below are no-ops.
 */
@OptIn(ExperimentalSharedTransitionApi::class)
class NavScopes(
    val shared: SharedTransitionScope,
    val animated: AnimatedVisibilityScope,
)

val LocalNavScopes = compositionLocalOf<NavScopes?> { null }

/**
 * Fade-through between screens: the old screen gets out of the way
 * quickly, the new one fades in and settles up from 96%. Shared elements
 * (the article image) fly across on top of it.
 */
val fadeThrough: ContentTransform =
    (
        fadeIn(tween(Motion.STANDARD_MS, delayMillis = Motion.QUICK_MS, easing = Motion.Settle)) +
            scaleIn(
                tween(Motion.STANDARD_MS, delayMillis = Motion.QUICK_MS, easing = Motion.Settle),
                initialScale = 0.96f,
            )
    ) togetherWith fadeOut(tween(Motion.QUICK_MS, easing = Motion.Leave))

// Spatial morphs are the largest motion on screen; they get the slow
// token so the eye can follow the card into the reader.
@OptIn(ExperimentalSharedTransitionApi::class)
private val articleBoundsTransform =
    BoundsTransform { _, _ -> tween(Motion.SLOW_MS, easing = Motion.Settle) }

/**
 * Marks this element as the article's lead image for cross-screen morphs:
 * a feed or saved card's thumbnail flies into the reader's idle cover and
 * back. Content is remeasured in flight so the crop follows the bounds.
 */
@OptIn(ExperimentalSharedTransitionApi::class)
@Composable
fun Modifier.sharedArticleImage(articleId: Long): Modifier {
    val scopes = LocalNavScopes.current ?: return this
    return with(scopes.shared) {
        this@sharedArticleImage.sharedBounds(
            sharedContentState = rememberSharedContentState("article-image-$articleId"),
            animatedVisibilityScope = scopes.animated,
            boundsTransform = articleBoundsTransform,
            resizeMode = SharedTransitionScope.ResizeMode.RemeasureToBounds,
            // Mid-flight corners between the row thumbnail's small shape
            // and the cover's large one.
            clipInOverlayDuringTransition = OverlayClip(MaterialTheme.shapes.medium),
        )
    }
}

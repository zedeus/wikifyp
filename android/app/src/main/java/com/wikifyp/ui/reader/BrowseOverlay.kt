package com.wikifyp.ui.reader

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectTapGestures
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextLayoutResult
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import com.wikifyp.data.FeedCard
import com.wikifyp.theme.Motion
import com.wikifyp.theme.Spacing
import com.wikifyp.ui.feed.CardImage
import com.wikifyp.ui.feed.HERO_IMAGE_ASPECT
import kotlinx.coroutines.flow.first

/**
 * Browse mode: the article as flowing text over the reader, with the
 * current word highlighted. Tap a word to jump there; tap anywhere else
 * to close; hold to resume reading (the overlay fades so the focal word
 * shows under the finger, and closes on release).
 *
 * The overlay stays mounted while [faded] so the hold-to-resume press
 * keeps its pointer stream until release — unmounting mid-gesture would
 * drop the final lift and leave the engine playing.
 */
@Composable
internal fun BrowseOverlay(
    blocks: List<BrowseBlock>,
    currentFlat: Int,
    palette: ReaderPalette,
    settings: ReaderSettings,
    faded: Boolean,
    onTapWord: (Int) -> Unit,
    onTapBackground: () -> Unit,
    onHold: () -> Unit,
    onRelease: () -> Unit,
    modifier: Modifier = Modifier,
    // The masthead at the top of the flowing text: the article's title
    // and, when the card carries one, its lead image.
    articleTitle: String = "",
    card: FeedCard? = null,
    // The reader chrome stays pinned above the overlay; these keep the
    // flowing text clear of the top bar and the bottom controls.
    topPadding: Dp = Spacing.xl,
    bottomPadding: Dp = Spacing.xl,
) {
    val alpha by animateFloatAsState(
        targetValue = if (faded) 0f else 1f,
        animationSpec = tween(Motion.STANDARD_MS, easing = Motion.Leave),
        label = "browseFade",
    )

    // The hold-to-resume press outlives many recompositions (the overlay
    // recomposes as the position advances while faded). Route every
    // callback through rememberUpdatedState so the gesture detectors'
    // keys stay stable: a pointerInput that restarted mid-press would
    // drop the final release and leave the engine playing with no finger
    // down (same pattern as WpmControl in ReaderScreen).
    val currentOnTapWord by rememberUpdatedState(onTapWord)
    val currentOnTapBackground by rememberUpdatedState(onTapBackground)
    val currentOnHold by rememberUpdatedState(onHold)
    val currentOnRelease by rememberUpdatedState(onRelease)
    val stableOnTapWord = remember { { flat: Int -> currentOnTapWord(flat) } }
    val stableOnTapBackground = remember { { currentOnTapBackground() } }
    val stableOnHold = remember { { currentOnHold() } }
    val stableOnRelease = remember { { currentOnRelease() } }

    // Freeze the highlighted word while faded: the overlay is invisible
    // for the whole hold-to-resume press (minutes of reading, with the
    // position advancing several words a second), and following it would
    // rebuild and re-lay-out a block's text per word with nothing shown.
    val highlightFlat = remember { mutableIntStateOf(currentFlat) }
    if (!faded) highlightFlat.intValue = currentFlat

    // Open on the block holding the current word: the list starts laid
    // out there (+1 for the masthead item), and the exact centring scroll
    // below runs before the content is shown — no visible repositioning.
    val startBlock = remember { blockIndexFor(blocks, currentFlat) }
    val listState =
        rememberLazyListState(
            initialFirstVisibleItemIndex = if (currentFlat > 0) startBlock + 1 else 0,
        )
    var positioned by remember { mutableStateOf(false) }
    val contentAlpha by animateFloatAsState(
        targetValue = if (positioned) 1f else 0f,
        animationSpec = tween(Motion.QUICK_MS, easing = Motion.Settle),
        label = "browseContent",
    )

    // Collapsible sections, official-app style: tapping a heading folds
    // its body blocks away. Keyed by the heading's block index; the lead
    // blocks before the first heading have no heading to fold under.
    var collapsed by remember { mutableStateOf(setOf<Int>()) }
    val headingOf =
        remember(blocks) {
            IntArray(blocks.size).also { arr ->
                var heading = -1
                for ((i, block) in blocks.withIndex()) {
                    if (block.isHeading) heading = i
                    arr[i] = heading
                }
            }
        }

    // A jump into a folded section (table of contents, scrub) unfolds it,
    // or the highlighted word would be invisible. With nothing folded —
    // the common case — skip the block scan the effect would otherwise
    // run on every word the highlight advances.
    LaunchedEffect(highlightFlat.intValue) {
        if (collapsed.isEmpty()) return@LaunchedEffect
        val block = blockIndexFor(blocks, highlightFlat.intValue)
        val heading = headingOf.getOrNull(block) ?: -1
        if (heading in collapsed) collapsed = collapsed - heading
    }

    // Centre the block holding the current word when the overlay opens;
    // the viewport size is only known after the first layout. From the
    // very start (nothing read yet) the masthead stays at the top.
    LaunchedEffect(Unit) {
        if (currentFlat > 0) {
            val viewport =
                snapshotFlow { listState.layoutInfo.viewportSize.height }
                    .first { it > 0 }
            listState.scrollToItem(startBlock + 1, -viewport / 3)
        }
        positioned = true
    }

    Box(
        modifier =
            modifier
                .fillMaxSize()
                .graphicsLayer { this.alpha = alpha }
                .background(palette.background)
                .safeDrawingPadding()
                .browseGestures(
                    onHold = stableOnHold,
                    onRelease = stableOnRelease,
                    onTap = { stableOnTapBackground() },
                ).testTag("browseOverlay"),
    ) {
        LazyColumn(
            state = listState,
            modifier =
                Modifier
                    .fillMaxSize()
                    .graphicsLayer { this.alpha = contentAlpha },
            contentPadding =
                PaddingValues(
                    start = Spacing.lg,
                    end = Spacing.lg,
                    top = topPadding,
                    // Extra room under the last line so the hint pill
                    // never sits on top of it at full scroll.
                    bottom = bottomPadding + Spacing.xxl,
                ),
        ) {
            item(key = "__masthead__") {
                BrowseMasthead(
                    title = articleTitle,
                    card = card,
                    palette = palette,
                    settings = settings,
                )
            }
            itemsIndexed(blocks) { index, block ->
                if (block.isHeading) {
                    BrowseHeading(
                        block = block,
                        collapsed = index in collapsed,
                        palette = palette,
                        settings = settings,
                        onToggle = {
                            collapsed =
                                if (index in collapsed) collapsed - index else collapsed + index
                        },
                        onHold = stableOnHold,
                        onRelease = stableOnRelease,
                    )
                } else if (headingOf[index] !in collapsed) {
                    BrowseBlockText(
                        block = block,
                        currentFlat = highlightFlat.intValue,
                        palette = palette,
                        settings = settings,
                        onTapWord = stableOnTapWord,
                        onTapBackground = stableOnTapBackground,
                        onHold = stableOnHold,
                        onRelease = stableOnRelease,
                    )
                }
            }
        }

        ReaderPill(
            text = "Tap a word to jump · hold to read",
            palette = palette,
            modifier =
                Modifier
                    .align(Alignment.BottomCenter)
                    .padding(bottom = bottomPadding + Spacing.xs),
        )
    }
}

/**
 * The article's masthead at the top of the flowing text: the title in
 * display size, then the lead image when the card carries one (the
 * initial-letter stand-in is a card treatment, not article content, so a
 * missing image just means no image here). Taps fall through to the
 * overlay background, closing browse like any other dead space.
 */
@Composable
private fun BrowseMasthead(
    title: String,
    card: FeedCard?,
    palette: ReaderPalette,
    settings: ReaderSettings,
) {
    val base = MaterialTheme.typography.headlineLarge
    val style = remember(base, settings) { settings.scaledStyle(base) }
    Column(modifier = Modifier.testTag("browseMasthead")) {
        Text(
            text = title,
            style = style,
            color = palette.word,
            modifier = Modifier.padding(top = Spacing.sm),
        )
        val image = card?.image
        if (image != null) {
            Spacer(Modifier.height(Spacing.md))
            CardImage(
                image = image,
                title = title,
                cardId = card.id,
                modifier =
                    Modifier
                        .fillMaxWidth()
                        .aspectRatio(HERO_IMAGE_ASPECT)
                        .clip(MaterialTheme.shapes.large),
            )
        }
    }
}

/**
 * The browse gesture contract, shared by the overlay background and each
 * text block: tap reports a position, long-press starts hold-to-resume,
 * and every end of a press — lift or cancellation — is reported so the
 * hold can end. Reporting cancellations too is what keeps hold-to-resume
 * correct: once [onHold] has started playback, a stolen or cancelled
 * press must still stop it, or the article keeps playing with no finger
 * down. A plain scroll never held, so its onRelease is a no-op upstream.
 */
private fun Modifier.browseGestures(
    onHold: () -> Unit,
    onRelease: () -> Unit,
    onTap: (Offset) -> Unit,
): Modifier =
    this.pointerInput(onHold, onRelease, onTap) {
        detectTapGestures(
            onTap = onTap,
            onLongPress = { onHold() },
            onPress = {
                tryAwaitRelease()
                onRelease()
            },
        )
    }

/**
 * A section heading in browse: a fold handle, not a jump target — tap
 * folds or unfolds the section's body; hold still resumes reading. Jumps
 * into sections go through the body words or the table of contents.
 */
@Composable
private fun BrowseHeading(
    block: BrowseBlock,
    collapsed: Boolean,
    palette: ReaderPalette,
    settings: ReaderSettings,
    onToggle: () -> Unit,
    onHold: () -> Unit,
    onRelease: () -> Unit,
) {
    val base = MaterialTheme.typography.titleLarge
    val style = remember(base, settings) { settings.scaledStyle(base) }
    val chevron by animateFloatAsState(
        targetValue = if (collapsed) -90f else 0f,
        animationSpec = tween(Motion.QUICK_MS, easing = Motion.Settle),
        label = "chevron",
    )
    Row(
        modifier =
            Modifier
                .fillMaxWidth()
                .padding(top = Spacing.lg, bottom = Spacing.xs)
                .browseGestures(
                    onHold = onHold,
                    onRelease = onRelease,
                    onTap = { onToggle() },
                ).testTag("browseHeading"),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = block.text,
            style = style,
            color = palette.word,
            modifier = Modifier.weight(1f),
        )
        Icon(
            imageVector = Icons.Filled.KeyboardArrowDown,
            contentDescription = if (collapsed) "Expand section" else "Collapse section",
            tint = palette.chrome,
            modifier = Modifier.graphicsLayer { rotationZ = chevron },
        )
    }
}

@Composable
private fun BrowseBlockText(
    block: BrowseBlock,
    currentFlat: Int,
    palette: ReaderPalette,
    settings: ReaderSettings,
    onTapWord: (Int) -> Unit,
    onTapBackground: () -> Unit,
    onHold: () -> Unit,
    onRelease: () -> Unit,
) {
    var layout by remember { mutableStateOf<TextLayoutResult?>(null) }

    val base =
        if (block.isHeading) {
            MaterialTheme.typography.titleLarge
        } else {
            MaterialTheme.typography.bodyLarge
        }
    val style = remember(base, settings) { settings.scaledStyle(base) }

    // Resolve the highlighted token first so only the block containing
    // the current word rebuilds its string as playback advances (the
    // overlay keeps rendering, faded, during hold-to-resume).
    val current = block.tokens.find { it.flatIndex == currentFlat }
    val text =
        remember(block, current, palette) {
            buildAnnotatedString {
                append(block.text)
                // Full-strength text: browse is a reading mode in its own
                // right, not a dimmed periphery — the highlight still pops
                // through its accent and background.
                addStyle(
                    SpanStyle(color = palette.word),
                    0,
                    block.text.length,
                )
                if (current != null) {
                    addStyle(
                        SpanStyle(
                            color = palette.accent,
                            fontWeight = FontWeight.Bold,
                            background = palette.accent.copy(alpha = 0.14f),
                        ),
                        current.start,
                        current.end,
                    )
                }
            }
        }

    Text(
        text = text,
        style = style,
        onTextLayout = { layout = it },
        modifier =
            Modifier
                .fillMaxWidth()
                .padding(
                    top = if (block.isHeading) Spacing.lg else Spacing.sm,
                    bottom = Spacing.xs,
                ).browseGestures(
                    onHold = onHold,
                    onRelease = onRelease,
                    onTap = { offset ->
                        val ch = layout?.getOffsetForPosition(offset)
                        // A tap on the joining space counts as the word
                        // before it — the miss zone between two words
                        // should jump, not close the overlay. Only a tap
                        // past the end of the text is background.
                        val token =
                            ch?.takeIf { it < block.text.length }?.let { c ->
                                block.tokens.firstOrNull { c >= it.start && c <= it.end }
                            }
                        if (token != null) onTapWord(token.flatIndex) else onTapBackground()
                    },
                ),
    )
}

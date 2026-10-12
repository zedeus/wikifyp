package com.wikifyp.ui.reader

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import com.wikifyp.reader.EventListener
import com.wikifyp.reader.ReaderClock
import com.wikifyp.reader.ReaderEngine
import com.wikifyp.reader.ReaderState
import com.wikifyp.reader.SentenceTokens
import com.wikifyp.theme.Motion
import com.wikifyp.theme.Spacing

/**
 * The RSVP reader screen. The current word stays centred at a fixed size.
 * On pause, ghost words fade in on either side on the same line, plus a
 * continuous context line above and below so the reader can see where they
 * are in the text without the focal word moving or changing size.
 */
@Composable
fun ReaderScreen(
    articleTitle: String,
    sentences: List<SentenceTokens>,
    articleId: Long,
    clock: ReaderClock,
    onEvent: EventListener = EventListener {},
    onBack: () -> Unit = {},
    initialWpm: Int = 250,
    modifier: Modifier = Modifier,
) {
    var currentWord by remember { mutableStateOf("") }
    var readerState by remember { mutableStateOf(ReaderState.IDLE) }
    var position by remember { mutableIntStateOf(0) }
    var isHeading by remember { mutableStateOf(false) }
    var wpm by remember { mutableIntStateOf(initialWpm) }

    val engine =
        remember(articleId) {
            val sync =
                object {
                    var fn: () -> Unit = {}
                }
            ReaderEngine(
                clock,
                EventListener { event -> onEvent.onEvent(event) },
                onStateChanged = { sync.fn() },
            ).also { eng ->
                sync.fn = {
                    currentWord = eng.currentToken() ?: ""
                    readerState = eng.state
                    position = eng.currentPosition()
                    isHeading = eng.currentIsHeading()
                }
                eng.wpm = initialWpm
                eng.load(articleId, sentences)
            }
        }

    DisposableEffect(wpm) {
        engine.wpm = wpm
        onDispose {}
    }

    val totalTokens = remember(sentences) { sentences.sumOf { it.tokens.size } }
    val isPausedOrScrubbing =
        readerState == ReaderState.PAUSED || readerState == ReaderState.SCRUBBING
    val ghostAlpha by animateFloatAsState(
        targetValue = if (isPausedOrScrubbing) 0.22f else 0f,
        animationSpec =
            tween(if (isPausedOrScrubbing) Motion.STANDARD_MS else Motion.QUICK_MS),
        label = "ghostAlpha",
    )

    // Flatten tokens once for ghost word lookups.
    val allTokens = remember(sentences) { sentences.flatMap { it.tokens } }

    // Ghost window bounds: how many tokens left/right of current position.
    val ghostCount = 5
    val pos = position.coerceIn(0, (allTokens.size - 1).coerceAtLeast(0))
    val ghostLeft = (pos - ghostCount).coerceAtLeast(0)
    val ghostRight = (pos + ghostCount).coerceAtMost(allTokens.size - 1)

    Box(
        modifier =
            modifier
                .fillMaxSize()
                .background(MaterialTheme.colorScheme.background)
                .safeDrawingPadding()
                .readerGestures(engine = engine)
                .testTag("readerSurface"),
    ) {
        Column(
            modifier = Modifier.fillMaxSize(),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            // Top bar.
            Row(
                modifier =
                    Modifier
                        .fillMaxWidth()
                        .padding(horizontal = Spacing.lg, vertical = Spacing.md),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = "←",
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.primary,
                    modifier =
                        Modifier
                            .pointerInput(onBack) { detectTapGestures { onBack() } }
                            .padding(end = Spacing.sm)
                            .testTag("backButton"),
                )
                Text(
                    text = articleTitle,
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onBackground,
                    modifier = Modifier.weight(1f),
                    maxLines = 1,
                )
            }

            Spacer(Modifier.weight(1f))

            // Context line above: continuous with the ghost window's left edge.
            if (ghostAlpha > 0.01f && ghostLeft > 0) {
                val aboveStart = (ghostLeft - 12).coerceAtLeast(0)
                val aboveText = allTokens.subList(aboveStart, ghostLeft).joinToString(" ")
                Text(
                    text = aboveText,
                    style = MaterialTheme.typography.bodyLarge,
                    color =
                        MaterialTheme.colorScheme.onBackground.copy(
                            alpha = ghostAlpha * 0.6f,
                        ),
                    textAlign = TextAlign.Center,
                    maxLines = 1,
                    overflow = TextOverflow.Clip,
                    modifier =
                        Modifier
                            .fillMaxWidth()
                            .padding(horizontal = Spacing.lg),
                )
                Spacer(Modifier.height(Spacing.sm))
            }

            // Main reading line: left ghosts | focal word | right ghosts.
            Box(
                modifier =
                    Modifier
                        .fillMaxWidth()
                        .padding(horizontal = Spacing.sm)
                        .testTag("currentWordBox"),
                contentAlignment = Alignment.Center,
            ) {
                if (readerState == ReaderState.IDLE) {
                    Text(
                        text = "Hold to read",
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.testTag("idleHint"),
                    )
                } else if (currentWord.isNotEmpty()) {
                    FocalLine(
                        position = pos,
                        allTokens = allTokens,
                        isHeading = isHeading,
                        ghostAlpha = ghostAlpha,
                        ghostCount = ghostCount,
                        modifier = Modifier.testTag("currentWord"),
                    )
                }
            }

            // Context line below: continuous with the ghost window's right edge.
            if (ghostAlpha > 0.01f && ghostRight < allTokens.size - 1) {
                Spacer(Modifier.height(Spacing.sm))
                val belowEnd = (ghostRight + 13).coerceAtMost(allTokens.size)
                val belowText =
                    allTokens.subList(ghostRight + 1, belowEnd).joinToString(" ")
                Text(
                    text = belowText,
                    style = MaterialTheme.typography.bodyLarge,
                    color =
                        MaterialTheme.colorScheme.onBackground.copy(
                            alpha = ghostAlpha * 0.6f,
                        ),
                    textAlign = TextAlign.Center,
                    maxLines = 1,
                    overflow = TextOverflow.Clip,
                    modifier =
                        Modifier
                            .fillMaxWidth()
                            .padding(horizontal = Spacing.lg),
                )
            }

            Spacer(Modifier.weight(1f))

            // Bottom: progress + WPM slider.
            Column(
                modifier =
                    Modifier
                        .fillMaxWidth()
                        .padding(horizontal = Spacing.lg)
                        .padding(bottom = Spacing.md),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                if (totalTokens > 0) {
                    Text(
                        text = "${position + 1} / $totalTokens",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.testTag("progressText"),
                    )
                }
                Spacer(Modifier.height(Spacing.sm))
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    modifier = Modifier.fillMaxWidth().testTag("wpmControl"),
                ) {
                    Text(
                        text = "$wpm wpm",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.width(64.dp),
                    )
                    Slider(
                        value = wpm.toFloat(),
                        onValueChange = { wpm = it.toInt() },
                        valueRange = 100f..600f,
                        steps = 0,
                        modifier =
                            Modifier.weight(1f).testTag("wpmSlider"),
                        colors =
                            SliderDefaults.colors(
                                thumbColor = MaterialTheme.colorScheme.primary,
                                activeTrackColor = MaterialTheme.colorScheme.primary,
                                inactiveTrackColor =
                                    MaterialTheme.colorScheme.outlineVariant,
                            ),
                    )
                }
            }
        }
    }
}

/**
 * Focal word dead-centre with ghost words on either side. Uses a
 * three-segment Row so the word never shifts. Ghost words fade via
 * [ghostAlpha].
 */
@Composable
private fun FocalLine(
    position: Int,
    allTokens: List<String>,
    isHeading: Boolean,
    ghostAlpha: Float,
    ghostCount: Int,
    modifier: Modifier = Modifier,
) {
    val colors = MaterialTheme.colorScheme
    val accent: Color = if (isHeading) colors.secondary else colors.primary
    val muted = colors.onBackground.copy(alpha = 0.6f)
    val ghostColor = colors.onBackground.copy(alpha = ghostAlpha)

    val pos = position.coerceIn(0, (allTokens.size - 1).coerceAtLeast(0))
    val word = allTokens.getOrElse(pos) { "" }
    val focal = focalPosition(word)

    val leftStart = (pos - ghostCount).coerceAtLeast(0)
    val rightEnd = (pos + ghostCount).coerceAtMost(allTokens.size - 1)

    Row(
        modifier = modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // Left ghost words: right-aligned, filling available space.
        Text(
            text =
                if (leftStart < pos) {
                    allTokens.subList(leftStart, pos).joinToString(" ") + " "
                } else {
                    ""
                },
            style = MaterialTheme.typography.bodyLarge,
            color = ghostColor,
            textAlign = TextAlign.End,
            maxLines = 1,
            overflow = TextOverflow.Clip,
            modifier = Modifier.weight(1f),
        )

        // Focal word: ORP-highlighted, fixed in the centre.
        val focalAnnotated =
            buildAnnotatedString {
                if (focal > 0) {
                    withStyle(SpanStyle(color = muted)) {
                        append(word.substring(0, focal))
                    }
                }
                withStyle(
                    SpanStyle(color = accent, fontWeight = FontWeight.Bold),
                ) {
                    append(word[focal].toString())
                }
                if (focal + 1 < word.length) {
                    withStyle(SpanStyle(color = muted)) {
                        append(word.substring(focal + 1))
                    }
                }
            }
        Text(
            text = focalAnnotated,
            style = MaterialTheme.typography.headlineMedium,
        )

        // Right ghost words: left-aligned, filling available space.
        Text(
            text =
                if (pos < rightEnd) {
                    " " + allTokens.subList(pos + 1, rightEnd + 1).joinToString(" ")
                } else {
                    ""
                },
            style = MaterialTheme.typography.bodyLarge,
            color = ghostColor,
            textAlign = TextAlign.Start,
            maxLines = 1,
            overflow = TextOverflow.Clip,
            modifier = Modifier.weight(1f),
        )
    }
}

/**
 * Displays a single word with the focal letter highlighted at the ORP.
 * Used only by tests that need the standalone focal word without context.
 */
@Composable
fun FocalWord(
    word: String,
    isHeading: Boolean = false,
    modifier: Modifier = Modifier,
) {
    val focal = focalPosition(word)
    val colors = MaterialTheme.colorScheme
    val muted = colors.onBackground.copy(alpha = 0.6f)
    val accent: Color = if (isHeading) colors.secondary else colors.primary

    val annotated =
        buildAnnotatedString {
            if (focal > 0) {
                withStyle(SpanStyle(color = muted)) {
                    append(word.substring(0, focal))
                }
            }
            withStyle(
                SpanStyle(color = accent, fontWeight = FontWeight.Bold),
            ) {
                append(word[focal].toString())
            }
            if (focal + 1 < word.length) {
                withStyle(SpanStyle(color = muted)) {
                    append(word.substring(focal + 1))
                }
            }
        }

    Text(
        text = annotated,
        style = MaterialTheme.typography.headlineMedium,
        modifier = modifier,
    )
}

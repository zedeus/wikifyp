package com.wikifyp.ui.reader

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
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
 * The RSVP reader screen. Displays the current word anchored on its focal
 * letter, with faint ghost words either side. On pause, surrounding lines
 * of ghost text fade in. Hold to play, lift to pause, drag horizontally to
 * scroll back, double-tap to ask.
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
    // Compose-observable state, synced automatically by the engine's
    // onStateChanged callback — no manual sync() calls needed.
    var currentWord by remember { mutableStateOf("") }
    var readerState by remember { mutableStateOf(ReaderState.IDLE) }
    var position by remember { mutableIntStateOf(0) }
    var isHeading by remember { mutableStateOf(false) }
    var wpm by remember { mutableIntStateOf(initialWpm) }

    val engine =
        remember(articleId) {
            // Late-bind the sync callback so it can read from the engine.
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
    val showGhostText = readerState != ReaderState.PLAYING

    // Only compute context lines when ghost text is visible.
    val contextLines =
        remember(position, sentences, showGhostText) {
            if (showGhostText) buildContextLines(position, sentences) else EMPTY_CONTEXT
        }

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
            // Top bar: back arrow and title.
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

            // Ghost text above (visible when paused/idle/scrubbing).
            GhostLines(
                lines = contextLines.before,
                visible = showGhostText,
                arrangement = Arrangement.Bottom,
                modifier = Modifier.weight(1f),
            )

            if (!showGhostText || contextLines.before.isEmpty()) {
                Spacer(Modifier.weight(1f))
            }

            // Current word — the focal centre.
            Box(
                modifier =
                    Modifier
                        .fillMaxWidth()
                        .padding(vertical = Spacing.lg)
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
                    FocalWord(
                        word = currentWord,
                        isHeading = isHeading,
                        modifier = Modifier.testTag("currentWord"),
                    )
                }
            }

            // Ghost text below.
            GhostLines(
                lines = contextLines.after,
                visible = showGhostText,
                arrangement = Arrangement.Top,
                modifier = Modifier.weight(1f),
            )

            if (!showGhostText || contextLines.after.isEmpty()) {
                Spacer(Modifier.weight(1f))
            }

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

/** Faint sentence lines shown above or below the focal word when paused. */
@Composable
private fun GhostLines(
    lines: List<String>,
    visible: Boolean,
    arrangement: Arrangement.Vertical,
    modifier: Modifier = Modifier,
) {
    AnimatedVisibility(
        visible = visible && lines.isNotEmpty(),
        enter = fadeIn(tween(Motion.STANDARD_MS)),
        exit = fadeOut(tween(Motion.QUICK_MS)),
        modifier = modifier,
    ) {
        Column(
            modifier =
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = Spacing.xl),
            verticalArrangement = arrangement,
        ) {
            for (line in lines) {
                Text(
                    text = line,
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onBackground.copy(alpha = 0.25f),
                    textAlign = TextAlign.Center,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }
    }
}

/**
 * Displays a word with the focal letter highlighted at the ORP (Optimal
 * Recognition Point). Letters before and after the focal point are muted;
 * the focal letter is the accent colour. Headings use a distinct colour
 * so the reader knows they're in a section title.
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
    // Headings get a warmer accent (the secondary colour) to distinguish
    // them from body text without shouting.
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
        style = MaterialTheme.typography.displayLarge,
        modifier = modifier,
    )
}

/**
 * Context lines above and below the current position for ghost-text display.
 */
data class ContextLines(
    val before: List<String>,
    val after: List<String>,
)

private val EMPTY_CONTEXT = ContextLines(emptyList(), emptyList())

/**
 * Builds surrounding sentence text from the current flat token position.
 */
internal fun buildContextLines(
    flatPosition: Int,
    sentences: List<SentenceTokens>,
    linesBefore: Int = 3,
    linesAfter: Int = 3,
): ContextLines {
    if (sentences.isEmpty()) return EMPTY_CONTEXT

    var remaining = flatPosition
    var currentSentIdx = 0
    for ((idx, sentence) in sentences.withIndex()) {
        if (remaining < sentence.tokens.size) {
            currentSentIdx = idx
            break
        }
        remaining -= sentence.tokens.size
        if (idx == sentences.lastIndex) currentSentIdx = idx
    }

    val before =
        (currentSentIdx - linesBefore until currentSentIdx)
            .filter { it >= 0 }
            .map { sentences[it].tokens.joinToString(" ") }

    val after =
        (currentSentIdx + 1..currentSentIdx + linesAfter)
            .filter { it < sentences.size }
            .map { sentences[it].tokens.joinToString(" ") }

    return ContextLines(before, after)
}

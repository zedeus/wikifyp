package com.wikifyp.ui.reader

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectDragGestures
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
import androidx.compose.foundation.shape.RoundedCornerShape
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
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.layout
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Constraints
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Popup
import com.wikifyp.reader.EventListener
import com.wikifyp.reader.ReaderClock
import com.wikifyp.reader.ReaderEngine
import com.wikifyp.reader.ReaderState
import com.wikifyp.reader.SentenceTokens
import com.wikifyp.theme.Spacing

/**
 * The RSVP reader screen. Pure black background. ORP letter offset left
 * with Spritz guide dashes. Ghost words always visible. Tap WPM to
 * adjust via popup slider.
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
    var showWpmPopup by remember { mutableStateOf(false) }

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
    val allTokens = remember(sentences) { sentences.flatMap { it.tokens } }
    val ghostLeftCount = 4
    val ghostRightCount = 8
    val pos = position.coerceIn(0, (allTokens.size - 1).coerceAtLeast(0))
    val ghostLeft = (pos - ghostLeftCount).coerceAtLeast(0)
    val ghostRight = (pos + ghostRightCount).coerceAtMost(allTokens.size - 1)

    // Pure black background for the reader.
    Box(
        modifier =
            modifier
                .fillMaxSize()
                .background(Color.Black)
                .safeDrawingPadding()
                .testTag("readerSurface"),
    ) {
        Column(
            modifier = Modifier.fillMaxSize(),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            // Top bar: bigger title.
            Row(
                modifier =
                    Modifier
                        .fillMaxWidth()
                        .padding(horizontal = Spacing.lg, vertical = Spacing.md),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = "←",
                    style = MaterialTheme.typography.titleLarge,
                    color = Color.White.copy(alpha = 0.6f),
                    modifier =
                        Modifier
                            .pointerInput(onBack) { detectTapGestures { onBack() } }
                            .padding(end = Spacing.sm)
                            .testTag("backButton"),
                )
                Text(
                    text = articleTitle,
                    style = MaterialTheme.typography.titleLarge,
                    color = Color.White.copy(alpha = 0.7f),
                    modifier = Modifier.weight(1f),
                    maxLines = 1,
                )
                if (totalTokens > 0 && readerState != ReaderState.IDLE) {
                    Text(
                        text = "${(position * 100) / totalTokens}%",
                        style = MaterialTheme.typography.titleMedium,
                        color = ORP_COLOR,
                    )
                }
            }

            // Reading area.
            Box(
                modifier =
                    Modifier
                        .fillMaxWidth()
                        .weight(1f)
                        .readerGestures(engine = engine),
                contentAlignment = Alignment.Center,
            ) {
                if (readerState == ReaderState.IDLE || allTokens.isEmpty()) {
                    Text(
                        text = "Hold to read",
                        style = MaterialTheme.typography.bodyLarge,
                        color = Color.White.copy(alpha = 0.4f),
                        modifier = Modifier.testTag("idleHint"),
                    )
                } else if (currentWord.isNotEmpty()) {
                    val ghostColor = Color.White.copy(alpha = 0.40f)
                    val wordColor = Color.White
                    val textStyle = MaterialTheme.typography.headlineMedium

                    val word = allTokens.getOrElse(pos) { "" }
                    val focal = focalPosition(word)
                    val beforeFocal =
                        if (focal > 0) word.substring(0, focal) else ""
                    val focalChar = word[focal].toString()
                    val afterFocal =
                        if (focal + 1 < word.length) {
                            word.substring(focal + 1)
                        } else {
                            ""
                        }

                    Column(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalAlignment = Alignment.CenterHorizontally,
                    ) {
                        GuideLines(color = ORP_COLOR.copy(alpha = 0.6f))
                        Spacer(Modifier.height(Spacing.xs))

                        // Focal line.
                        Row(
                            modifier =
                                Modifier
                                    .fillMaxWidth()
                                    .testTag("currentWord"),
                        ) {
                            // Left: ghost words + word prefix.
                            Box(
                                modifier =
                                    Modifier
                                        .weight(1f)
                                        .alignByBaseline()
                                        .clipToBounds(),
                            ) {
                                Text(
                                    text =
                                        buildAnnotatedString {
                                            if (ghostLeft < pos) {
                                                withStyle(
                                                    SpanStyle(color = ghostColor),
                                                ) {
                                                    append(
                                                        allTokens
                                                            .subList(ghostLeft, pos)
                                                            .joinToString(" "),
                                                    )
                                                    append(" ")
                                                }
                                            }
                                            if (beforeFocal.isNotEmpty()) {
                                                withStyle(
                                                    SpanStyle(
                                                        color = wordColor,
                                                        fontWeight = FontWeight.Bold,
                                                    ),
                                                ) {
                                                    append(beforeFocal)
                                                }
                                            }
                                        },
                                    style = textStyle,
                                    maxLines = 1,
                                    softWrap = false,
                                    modifier =
                                        Modifier.layout { measurable, constraints ->
                                            val p =
                                                measurable.measure(
                                                    constraints.copy(
                                                        maxWidth =
                                                            Constraints.Infinity,
                                                    ),
                                                )
                                            val w = constraints.maxWidth
                                            val bl =
                                                if (p[
                                                        androidx.compose.ui
                                                            .layout
                                                            .FirstBaseline,
                                                    ] !=
                                                    androidx.compose.ui
                                                        .layout
                                                        .AlignmentLine
                                                        .Unspecified
                                                ) {
                                                    p[
                                                        androidx.compose.ui
                                                            .layout
                                                            .FirstBaseline,
                                                    ]
                                                } else {
                                                    p.height
                                                }
                                            layout(
                                                w,
                                                p.height,
                                                mapOf(
                                                    androidx.compose.ui
                                                        .layout
                                                        .FirstBaseline to bl,
                                                    androidx.compose.ui
                                                        .layout
                                                        .LastBaseline to bl,
                                                ),
                                            ) {
                                                p.placeRelative(
                                                    w - p.width,
                                                    0,
                                                )
                                            }
                                        },
                                )
                            }

                            // Centre: ORP letter.
                            Text(
                                text = focalChar,
                                style =
                                    textStyle.copy(
                                        fontWeight = FontWeight.Bold,
                                        color = ORP_COLOR,
                                    ),
                                modifier = Modifier.alignByBaseline(),
                            )

                            // Right: word suffix + ghost words.
                            // Use Clip overflow so text extends to
                            // the edge and gets cut off naturally.
                            Text(
                                text =
                                    buildAnnotatedString {
                                        if (afterFocal.isNotEmpty()) {
                                            withStyle(
                                                SpanStyle(
                                                    color = wordColor,
                                                    fontWeight = FontWeight.Bold,
                                                ),
                                            ) {
                                                append(afterFocal)
                                            }
                                        }
                                        if (pos < ghostRight) {
                                            withStyle(
                                                SpanStyle(color = ghostColor),
                                            ) {
                                                append(" ")
                                                append(
                                                    allTokens
                                                        .subList(
                                                            pos + 1,
                                                            ghostRight + 1,
                                                        ).joinToString(" "),
                                                )
                                            }
                                        }
                                    },
                                style = textStyle,
                                maxLines = 1,
                                softWrap = false,
                                overflow = TextOverflow.Clip,
                                modifier =
                                    Modifier
                                        .weight(2f)
                                        .alignByBaseline(),
                            )
                        }

                        Spacer(Modifier.height(Spacing.xs))
                        GuideLines(color = ORP_COLOR.copy(alpha = 0.6f))
                    }
                }
            }

            // Bottom: WPM as tappable red number.
            Box(
                modifier =
                    Modifier
                        .fillMaxWidth()
                        .padding(bottom = Spacing.lg),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    text = "$wpm wpm",
                    style = MaterialTheme.typography.titleMedium,
                    color = ORP_COLOR.copy(alpha = 0.7f),
                    modifier =
                        Modifier
                            .pointerInput(Unit) {
                                detectTapGestures {
                                    showWpmPopup = !showWpmPopup
                                }
                            }.pointerInput(Unit) {
                                detectDragGestures { _, dragAmount ->
                                    // Right or up = faster, left or
                                    // down = slower. Use the dominant axis.
                                    val dx = dragAmount.x
                                    val dy = -dragAmount.y
                                    val d =
                                        if (kotlin.math.abs(dx) > kotlin.math.abs(dy)) dx else dy
                                    val delta = (d / 2.dp.toPx()).toInt()
                                    wpm = (wpm + delta).coerceIn(100, 600)
                                }
                            }.padding(Spacing.md)
                            .testTag("wpmControl"),
                )
                if (showWpmPopup) {
                    Popup(
                        alignment = Alignment.BottomCenter,
                        onDismissRequest = { showWpmPopup = false },
                    ) {
                        Box(
                            modifier =
                                Modifier
                                    .fillMaxWidth(0.8f)
                                    .background(
                                        Color(0xFF1A1A1A),
                                        RoundedCornerShape(12.dp),
                                    ).padding(
                                        horizontal = Spacing.lg,
                                        vertical = Spacing.md,
                                    ),
                        ) {
                            Column(
                                horizontalAlignment =
                                    Alignment.CenterHorizontally,
                            ) {
                                Text(
                                    text = "$wpm wpm",
                                    style =
                                        MaterialTheme.typography.headlineSmall,
                                    color = ORP_COLOR,
                                )
                                Spacer(Modifier.height(Spacing.sm))
                                Slider(
                                    value = wpm.toFloat(),
                                    onValueChange = { wpm = it.toInt() },
                                    valueRange = 100f..600f,
                                    steps = 0,
                                    modifier =
                                        Modifier
                                            .fillMaxWidth()
                                            .testTag("wpmSlider"),
                                    colors =
                                        SliderDefaults.colors(
                                            thumbColor = ORP_COLOR,
                                            activeTrackColor = ORP_COLOR,
                                            inactiveTrackColor =
                                                Color.White.copy(
                                                    alpha = 0.15f,
                                                ),
                                        ),
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

/** Spritz guide dashes at the ORP position (1/3 from left). */
@Composable
private fun GuideLines(
    color: Color,
    modifier: Modifier = Modifier,
) {
    Canvas(
        modifier =
            modifier
                .fillMaxWidth()
                .height(4.dp),
    ) {
        val cx = size.width / 3f
        val dashLen = 14.dp.toPx()
        val strokeWidth = 1.5.dp.toPx()
        val cy = size.height / 2f
        drawLine(
            color = color,
            start = Offset(cx - dashLen / 2, cy),
            end = Offset(cx + dashLen / 2, cy),
            strokeWidth = strokeWidth,
        )
    }
}

/** Single focal word for standalone use (tests). */
@Composable
fun FocalWord(
    word: String,
    isHeading: Boolean = false,
    modifier: Modifier = Modifier,
) {
    val focal = focalPosition(word)

    Text(
        text =
            buildAnnotatedString {
                if (focal > 0) {
                    withStyle(SpanStyle(color = Color.White)) {
                        append(word.substring(0, focal))
                    }
                }
                withStyle(
                    SpanStyle(color = ORP_COLOR, fontWeight = FontWeight.Bold),
                ) {
                    append(word[focal].toString())
                }
                if (focal + 1 < word.length) {
                    withStyle(SpanStyle(color = Color.White)) {
                        append(word.substring(focal + 1))
                    }
                }
            },
        style = MaterialTheme.typography.headlineMedium,
        modifier = modifier,
    )
}

/** Compute above-context text for a given position. Testable. */
internal fun aboveContextText(
    pos: Int,
    allTokens: List<String>,
): String {
    val aboveEnd = (pos - 2).coerceIn(0, allTokens.size)
    if (aboveEnd <= 0) return ""
    val aboveStart = (aboveEnd - 12).coerceAtLeast(0)
    return allTokens.subList(aboveStart, aboveEnd).joinToString(" ")
}

/** Compute below-context text for a given position. Testable. */
internal fun belowContextText(
    pos: Int,
    allTokens: List<String>,
): String {
    val belowStart = (pos + 3).coerceIn(0, allTokens.size)
    if (belowStart >= allTokens.size) return ""
    val belowEnd = (belowStart + 12).coerceAtMost(allTokens.size)
    return allTokens.subList(belowStart, belowEnd).joinToString(" ")
}

// ORP colour: red, the standard across Spritz-style RSVP readers.
private val ORP_COLOR = Color(0xFFCC4444)

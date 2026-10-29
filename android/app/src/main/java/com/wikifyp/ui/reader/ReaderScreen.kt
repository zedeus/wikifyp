package com.wikifyp.ui.reader

import android.app.Activity
import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.Toc
import androidx.compose.material.icons.filled.Bookmark
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.SkipNext
import androidx.compose.material.icons.outlined.BookmarkBorder
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Slider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.AlignmentLine
import androidx.compose.ui.layout.FirstBaseline
import androidx.compose.ui.layout.LastBaseline
import androidx.compose.ui.layout.layout
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Constraints
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Popup
import androidx.core.view.WindowCompat
import com.wikifyp.data.FeedCard
import com.wikifyp.reader.EventListener
import com.wikifyp.reader.ReaderClock
import com.wikifyp.reader.ReaderEngine
import com.wikifyp.reader.ReaderState
import com.wikifyp.reader.SentenceTokens
import com.wikifyp.theme.Motion
import com.wikifyp.theme.Spacing
import kotlinx.coroutines.delay

// Chrome enter/exit: settle in from the nearest edge, leave quickly.
private val chromeEnterFromTop =
    fadeIn(tween(Motion.STANDARD_MS, easing = Motion.Settle)) +
        slideInVertically(tween(Motion.STANDARD_MS, easing = Motion.Settle)) { -it / 2 }
private val chromeExitToTop =
    fadeOut(tween(Motion.QUICK_MS, easing = Motion.Leave)) +
        slideOutVertically(tween(Motion.QUICK_MS, easing = Motion.Leave)) { -it / 2 }
private val chromeEnterFromBottom =
    fadeIn(tween(Motion.STANDARD_MS, easing = Motion.Settle)) +
        slideInVertically(tween(Motion.STANDARD_MS, easing = Motion.Settle)) { it / 2 }
private val chromeExitToBottom =
    fadeOut(tween(Motion.QUICK_MS, easing = Motion.Leave)) +
        slideOutVertically(tween(Motion.QUICK_MS, easing = Motion.Leave)) { it / 2 }

/**
 * The RSVP reader screen. The surface follows the reader-theme setting
 * (paper, sepia, ink or true black); the ORP letter is offset to the
 * configured focal column with guide dashes. Ghost words surround the
 * focal word, and the current section's heading sits above the line —
 * flashing briefly when playback crosses into a new section, resting
 * visible while paused.
 *
 * Focus mode hides the chrome (title, progress, WPM) while playing and
 * brings it back on pause. A vertical drag opens the browse overlay: the
 * article as flowing text under the pinned chrome, the current word
 * highlighted; tap a word to jump there, hold to resume reading. The
 * table of contents jumps by section, the bookmark saves the article for
 * later, and when an article finishes the next one is one tap away.
 */
@Composable
fun ReaderScreen(
    articleTitle: String,
    sentences: List<SentenceTokens>,
    articleId: Long,
    clock: ReaderClock,
    onEvent: EventListener = EventListener {},
    onBack: () -> Unit = {},
    settings: ReaderSettings = ReaderSettings(),
    onSettingsChange: (ReaderSettings) -> Unit = {},
    upNext: FeedCard? = null,
    onNext: () -> Unit = {},
    saved: Boolean = false,
    onSavedChange: (Boolean) -> Unit = {},
    modifier: Modifier = Modifier,
) {
    // The caller wraps the reader in key(articleId), so every remember
    // here is born fresh per article; nothing needs its own id key.
    var currentWord by remember { mutableStateOf("") }
    var readerState by remember { mutableStateOf(ReaderState.IDLE) }
    var position by remember { mutableIntStateOf(0) }
    var showWpmPopup by remember { mutableStateOf(false) }
    var showSettings by remember { mutableStateOf(false) }
    var showToc by remember { mutableStateOf(false) }
    var savedPill by remember { mutableStateOf(false) }
    var browseOpen by remember { mutableStateOf(false) }
    var resumePressed by remember { mutableStateOf(false) }
    var finished by remember { mutableStateOf(false) }

    val engine =
        remember {
            val sync =
                object {
                    var fn: () -> Unit = {}
                }
            ReaderEngine(
                clock,
                onEvent,
                onStateChanged = { sync.fn() },
                speedScaleFor = ::rampScale,
            ).also { eng ->
                sync.fn = {
                    currentWord = eng.currentToken() ?: ""
                    readerState = eng.state
                    position = eng.currentPosition()
                    // Mirrored into snapshot state: composition (the
                    // up-next card) must observe it, and a plain engine
                    // property read wouldn't invalidate anything.
                    finished = eng.hasFinished
                }
                eng.wpm = settings.wpm
                eng.load(articleId, sentences)
            }
        }

    DisposableEffect(settings.wpm) {
        engine.wpm = settings.wpm
        onDispose {}
    }

    // "Unsave on finish" acts on the mirrored finish state so it reads the
    // composition's current `saved` and settings at the moment of finishing.
    LaunchedEffect(finished) {
        if (finished && settings.autoUnsaveOnFinish && saved) {
            engine.unsave()
            onSavedChange(false)
        }
    }

    /** Every exit from browse: end the scrub session, drop the overlay. */
    val closeBrowse = {
        engine.onTouchUp()
        browseOpen = false
        resumePressed = false
    }

    /**
     * Emits a skip event when the reader actually abandoned the article:
     * it played, but never finished. Browsing without playing is a
     * preview, and leaving after FINISH is a completed read — neither is
     * an abandonment the server should hear about.
     */
    val skipIfAbandoned = {
        if (readerState != ReaderState.IDLE && engine.hasPlayed && !engine.hasFinished) {
            engine.skip()
        }
    }

    val exitArticle = {
        skipIfAbandoned()
        onBack()
    }

    /** Moves on to the next article, counting an unfinished read as a skip. */
    val goNext = {
        skipIfAbandoned()
        onNext()
    }

    // System back closes the browse overlay if it is open; otherwise it
    // leaves the article.
    BackHandler {
        if (browseOpen) {
            closeBrowse()
        } else {
            exitArticle()
        }
    }

    val palette = readerPalette(settings.theme, isSystemInDarkTheme())

    // Status-bar icons must stay legible on whatever surface the reader
    // theme picked, which can differ from the system theme.
    val view = LocalView.current
    DisposableEffect(palette.isDark) {
        val window = (view.context as? Activity)?.window
        val controller = window?.let { WindowCompat.getInsetsController(it, view) }
        val wasLight = controller?.isAppearanceLightStatusBars
        controller?.isAppearanceLightStatusBars = !palette.isDark
        onDispose {
            if (controller != null && wasLight != null) {
                controller.isAppearanceLightStatusBars = wasLight
            }
        }
    }

    val totalTokens = remember(sentences) { sentences.sumOf { it.tokens.size } }
    val allTokens = remember(sentences) { sentences.flatMap { it.tokens } }
    val sections = remember(sentences, articleTitle) { sectionIndex(sentences, articleTitle) }

    // Browse blocks are only needed once the overlay opens; building them
    // walks the whole article, so don't pay that on every article open.
    val blocks = remember(sentences) { lazy { browseBlocks(sentences) } }
    val pos = position.coerceIn(0, (allTokens.size - 1).coerceAtLeast(0))

    // Focus mode: chrome leaves while playing, settles back on pause.
    val chromeVisible = !settings.focusMode || readerState != ReaderState.PLAYING

    // The browse overlay slides under the pinned chrome, so its text needs
    // to start below the top bar and end above the bottom controls.
    val density = LocalDensity.current
    var topChromeHeight by remember { mutableStateOf(0.dp) }
    var bottomChromeHeight by remember { mutableStateOf(0.dp) }

    Box(
        modifier =
            modifier
                .fillMaxSize()
                .background(palette.background)
                .safeDrawingPadding()
                .testTag("readerSurface"),
    ) {
        // Reading layer: fills the screen beneath the chrome; the focal
        // word sits at the vertical centre either way.
        Box(
            modifier =
                Modifier
                    .fillMaxSize()
                    .readerGestures(
                        engine = engine,
                        onBrowseOpen = { browseOpen = true },
                    ),
            contentAlignment = Alignment.Center,
        ) {
            if (readerState == ReaderState.IDLE || allTokens.isEmpty()) {
                IdleHint(palette)
            } else if (currentWord.isNotEmpty()) {
                // While playing the ghosts recede so the focal word
                // carries the eye; on pause the surroundings fade in.
                val ghostAlpha by animateFloatAsState(
                    targetValue = if (readerState == ReaderState.PLAYING) 0.45f else 1f,
                    animationSpec = tween(Motion.STANDARD_MS, easing = Motion.Settle),
                    label = "ghostFade",
                )
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    if (settings.sectionLabel != SectionLabelMode.HIDDEN) {
                        SectionLabel(
                            title = currentSection(sections, pos)?.title ?: articleTitle,
                            playing = readerState == ReaderState.PLAYING,
                            always = settings.sectionLabel == SectionLabelMode.ALWAYS,
                            palette = palette,
                        )
                        Spacer(Modifier.height(Spacing.md))
                    }
                    FocalLine(
                        allTokens = allTokens,
                        pos = pos,
                        palette = palette,
                        settings = settings,
                        ghostAlpha = ghostAlpha,
                    )
                }
            }
        }

        if (browseOpen) {
            // Hold-to-resume: the overlay fades out the moment playback
            // starts (so the reader sees the focal word under their
            // finger) and fully closes when that press is released. The
            // chrome stays pinned above it so back, save and contents
            // remain reachable while browsing.
            BrowseOverlay(
                blocks = blocks.value,
                currentFlat = pos,
                palette = palette,
                settings = settings,
                faded = readerState == ReaderState.PLAYING,
                topPadding = topChromeHeight + Spacing.sm,
                bottomPadding = bottomChromeHeight + Spacing.sm,
                onTapWord = { flat ->
                    engine.seekTo(flat)
                    closeBrowse()
                },
                onTapBackground = closeBrowse,
                onHold = {
                    resumePressed = true
                    engine.onTouchDown()
                },
                onRelease = {
                    if (resumePressed) {
                        closeBrowse()
                    }
                },
            )
        }

        // Chrome layer: pinned above the reading surface and the browse
        // overlay. The middle spacer has no input handling, so gestures
        // pass through to whatever is beneath it.
        Column(modifier = Modifier.fillMaxSize()) {
            AnimatedVisibility(
                visible = chromeVisible,
                enter = chromeEnterFromTop,
                exit = chromeExitToTop,
                modifier =
                    Modifier
                        .onSizeChanged {
                            topChromeHeight = with(density) { it.height.toDp() }
                        }.testTag("topChrome"),
            ) {
                TopChrome(
                    articleTitle = articleTitle,
                    palette = palette,
                    position = pos,
                    totalTokens = totalTokens,
                    started = readerState != ReaderState.IDLE,
                    saved = saved,
                    onBack = exitArticle,
                    onToc = { showToc = true },
                    onSave = {
                        if (saved) {
                            engine.unsave()
                            onSavedChange(false)
                        } else {
                            engine.save()
                            onSavedChange(true)
                            savedPill = true
                        }
                    },
                )
            }

            Spacer(Modifier.weight(1f))

            AnimatedVisibility(
                visible = chromeVisible,
                enter = chromeEnterFromBottom,
                exit = chromeExitToBottom,
                modifier =
                    Modifier
                        .onSizeChanged {
                            bottomChromeHeight = with(density) { it.height.toDp() }
                        }.testTag("bottomChrome"),
            ) {
                Box(
                    modifier =
                        Modifier
                            .fillMaxWidth()
                            .background(palette.background)
                            .padding(bottom = Spacing.sm),
                ) {
                    if (upNext != null) {
                        IconButton(
                            onClick = goNext,
                            modifier =
                                Modifier
                                    .align(Alignment.CenterStart)
                                    .padding(start = Spacing.sm)
                                    .testTag("nextButton"),
                        ) {
                            Icon(
                                imageVector = Icons.Filled.SkipNext,
                                contentDescription = "Next article",
                                tint = palette.chrome,
                                modifier = Modifier.size(22.dp),
                            )
                        }
                    }
                    WpmControl(
                        wpm = settings.wpm,
                        palette = palette,
                        onTap = { showWpmPopup = !showWpmPopup },
                        onSetWpm = { target ->
                            val next =
                                target.coerceIn(ReaderSettings.WPM_MIN, ReaderSettings.WPM_MAX)
                            onSettingsChange(settings.copy(wpm = next))
                        },
                        modifier = Modifier.align(Alignment.Center),
                    )
                    IconButton(
                        onClick = { showSettings = true },
                        modifier =
                            Modifier
                                .align(Alignment.CenterEnd)
                                .padding(end = Spacing.sm)
                                .testTag("settingsButton"),
                    ) {
                        Icon(
                            imageVector = Icons.Filled.Settings,
                            contentDescription = "Reader settings",
                            tint = palette.chrome,
                            modifier = Modifier.size(20.dp),
                        )
                    }
                    if (showWpmPopup) {
                        WpmPopup(
                            wpm = settings.wpm,
                            palette = palette,
                            onChange = { onSettingsChange(settings.copy(wpm = it)) },
                            onDismiss = { showWpmPopup = false },
                        )
                    }
                }
            }
        }

        // A finished article offers the next one: the feed's whole point
        // is that reading flows on.
        AnimatedVisibility(
            visible = readerState == ReaderState.IDLE && finished && !browseOpen,
            enter = chromeEnterFromBottom,
            exit = chromeExitToBottom,
            modifier =
                Modifier
                    .align(Alignment.BottomCenter)
                    .padding(horizontal = Spacing.md)
                    .padding(bottom = bottomChromeHeight + Spacing.md),
        ) {
            UpNextCard(
                card = upNext,
                palette = palette,
                onNext = onNext,
                onBackToFeed = exitArticle,
            )
        }

        // Transient confirmation for the bookmark.
        if (savedPill) {
            LaunchedEffect(Unit) {
                delay(1600)
                savedPill = false
            }
        }
        AnimatedVisibility(
            visible = savedPill,
            enter = fadeIn(tween(Motion.QUICK_MS)),
            exit = fadeOut(tween(Motion.STANDARD_MS)),
            modifier =
                Modifier
                    .align(Alignment.BottomCenter)
                    .padding(bottom = bottomChromeHeight + Spacing.lg),
        ) {
            ReaderPill(
                text = "Saved for later",
                palette = palette,
                modifier = Modifier.testTag("savedPill"),
            )
        }

        if (showToc) {
            TocSheet(
                sections = sections,
                currentFlat = pos,
                palette = palette,
                onJump = { section ->
                    engine.jumpTo(section.startFlat)
                    showToc = false
                },
                onDismiss = { showToc = false },
            )
        }

        if (showSettings) {
            ReaderSettingsSheet(
                settings = settings,
                palette = palette,
                onSettingsChange = onSettingsChange,
                onDismiss = { showSettings = false },
            )
        }
    }
}

@Composable
private fun IdleHint(palette: ReaderPalette) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(
            text = "Hold to read",
            style = MaterialTheme.typography.bodyLarge,
            color = palette.word.copy(alpha = 0.6f),
            modifier = Modifier.testTag("idleHint"),
        )
        Spacer(Modifier.height(Spacing.xs))
        Text(
            text = "Swipe up to browse",
            style = MaterialTheme.typography.bodySmall,
            color = palette.chrome.copy(alpha = 0.6f),
        )
    }
}

/**
 * The current section's heading above the focal line. While paused it
 * rests visible; while playing it stays out of the way, flashing in
 * briefly — fast in, slow out — when playback crosses into a new section
 * (per the product sketch: `== History ==` above the word line).
 */
@Composable
private fun SectionLabel(
    title: String,
    playing: Boolean,
    always: Boolean,
    palette: ReaderPalette,
) {
    val alpha = remember { Animatable(1f) }
    var restingTitle by remember { mutableStateOf(title) }

    LaunchedEffect(title, playing, always) {
        if (always || !playing) {
            restingTitle = title
            alpha.animateTo(1f, tween(Motion.STANDARD_MS, easing = Motion.Settle))
            return@LaunchedEffect
        }
        if (restingTitle == title) {
            // Play began inside the same section: the label just recedes.
            alpha.animateTo(0f, tween(Motion.SLOW_MS, easing = Motion.Leave))
            return@LaunchedEffect
        }
        // Crossed into a new section mid-play: flash it.
        restingTitle = title
        alpha.snapTo(0f)
        alpha.animateTo(1f, tween(140, easing = Motion.Settle))
        delay(800)
        alpha.animateTo(0f, tween(1100, easing = Motion.Leave))
    }

    Text(
        text = title,
        style = MaterialTheme.typography.labelLarge,
        color = palette.chrome,
        maxLines = 1,
        overflow = TextOverflow.Ellipsis,
        modifier =
            Modifier
                .padding(horizontal = Spacing.lg)
                .graphicsLayer { this.alpha = alpha.value }
                .testTag("sectionLabel"),
    )
}

/** Title row with back, contents and save actions, plus a hairline progress track. */
@Composable
private fun TopChrome(
    articleTitle: String,
    palette: ReaderPalette,
    position: Int,
    totalTokens: Int,
    started: Boolean,
    saved: Boolean,
    onBack: () -> Unit,
    onToc: () -> Unit,
    onSave: () -> Unit,
) {
    Column(
        modifier =
            Modifier
                .fillMaxWidth()
                .background(palette.background),
    ) {
        Row(
            modifier =
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = Spacing.sm, vertical = Spacing.xs),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconButton(
                onClick = onBack,
                modifier = Modifier.testTag("backButton"),
            ) {
                Icon(
                    imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                    contentDescription = "Back to feed",
                    tint = palette.chrome,
                )
            }
            Text(
                text = articleTitle,
                style = MaterialTheme.typography.titleLarge,
                color = palette.word.copy(alpha = 0.85f),
                modifier = Modifier.weight(1f),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (totalTokens > 0 && started) {
                Text(
                    text = "${(position * 100) / totalTokens}%",
                    style = MaterialTheme.typography.labelLarge,
                    color = palette.chrome,
                    modifier = Modifier.padding(horizontal = Spacing.xs),
                )
            }
            IconButton(
                onClick = onToc,
                modifier = Modifier.testTag("tocButton"),
            ) {
                Icon(
                    imageVector = Icons.AutoMirrored.Filled.Toc,
                    contentDescription = "Contents",
                    tint = palette.chrome,
                    modifier = Modifier.size(22.dp),
                )
            }
            IconButton(
                onClick = onSave,
                modifier = Modifier.testTag("saveButton"),
            ) {
                Icon(
                    imageVector =
                        if (saved) Icons.Filled.Bookmark else Icons.Outlined.BookmarkBorder,
                    contentDescription = if (saved) "Saved" else "Save for later",
                    tint = if (saved) palette.accent else palette.chrome,
                    modifier = Modifier.size(22.dp),
                )
            }
        }
        // Hairline progress track under the title. Drawn directly: each
        // per-word step is sub-pixel, so animating it buys nothing.
        val progress =
            if (totalTokens > 0 && started) position.toFloat() / totalTokens else 0f
        Canvas(
            modifier =
                Modifier
                    .fillMaxWidth()
                    .height(2.dp)
                    .padding(horizontal = Spacing.md),
        ) {
            drawLine(
                color = palette.ghost.copy(alpha = 0.25f),
                start = Offset(0f, size.height / 2),
                end = Offset(size.width, size.height / 2),
                strokeWidth = size.height,
            )
            if (progress > 0f) {
                drawLine(
                    color = palette.accent,
                    start = Offset(0f, size.height / 2),
                    end = Offset(size.width * progress, size.height / 2),
                    strokeWidth = size.height,
                )
            }
        }
    }
}

/**
 * The focal word with its ORP letter pinned to the configured column,
 * ghost words flowing off both sides, and guide dashes above and below.
 */
@Composable
private fun FocalLine(
    allTokens: List<String>,
    pos: Int,
    palette: ReaderPalette,
    settings: ReaderSettings,
    ghostAlpha: Float,
) {
    val ghostLeft = (pos - settings.ghostWords / 2).coerceAtLeast(0)
    val ghostRight = (pos + settings.ghostWords).coerceAtMost(allTokens.size - 1)
    val ghostColor = palette.ghost.copy(alpha = palette.ghost.alpha * ghostAlpha)

    val base = MaterialTheme.typography.headlineMedium
    val textStyle = remember(base, settings) { settings.scaledStyle(base) }
    val focalStyle =
        remember(textStyle, palette) {
            textStyle.copy(fontWeight = FontWeight.Bold, color = palette.accent)
        }

    val word = allTokens.getOrElse(pos) { "" }
    val (beforeFocal, focalChar, afterFocal) = focalSlices(word)

    Column(
        modifier = Modifier.fillMaxWidth(),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        GuideLines(color = palette.accent.copy(alpha = 0.6f), fraction = settings.orpFraction)
        Spacer(Modifier.height(Spacing.xs))

        Row(
            modifier =
                Modifier
                    .fillMaxWidth()
                    .testTag("currentWord"),
        ) {
            // Left: ghost words + word prefix, right-aligned to the focal
            // letter and clipped at the screen edge.
            Box(
                modifier =
                    Modifier
                        .weight(settings.orpFraction)
                        .alignByBaseline()
                        .clipToBounds(),
            ) {
                Text(
                    text =
                        buildAnnotatedString {
                            if (ghostLeft < pos) {
                                appendGhostWords(
                                    words = allTokens.subList(ghostLeft, pos),
                                    color = ghostColor,
                                    bionic = settings.bionic,
                                )
                                append(" ")
                            }
                            if (beforeFocal.isNotEmpty()) {
                                withStyle(
                                    SpanStyle(
                                        color = palette.word,
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
                    modifier = Modifier.rightAlignedUnbounded(),
                )
            }

            // Centre: ORP letter.
            Text(
                text = focalChar,
                style = focalStyle,
                modifier = Modifier.alignByBaseline(),
            )

            // Right: word suffix + ghost words, clipped at the edge.
            Text(
                text =
                    buildAnnotatedString {
                        if (afterFocal.isNotEmpty()) {
                            withStyle(
                                SpanStyle(
                                    color = palette.word,
                                    fontWeight = FontWeight.Bold,
                                ),
                            ) {
                                append(afterFocal)
                            }
                        }
                        if (pos < ghostRight) {
                            append(" ")
                            appendGhostWords(
                                words = allTokens.subList(pos + 1, ghostRight + 1),
                                color = ghostColor,
                                bionic = settings.bionic,
                            )
                        }
                    },
                style = textStyle,
                maxLines = 1,
                softWrap = false,
                overflow = TextOverflow.Clip,
                modifier =
                    Modifier
                        .weight(1f - settings.orpFraction)
                        .alignByBaseline(),
            )
        }

        Spacer(Modifier.height(Spacing.xs))
        GuideLines(color = palette.accent.copy(alpha = 0.6f), fraction = settings.orpFraction)
    }
}

/**
 * Appends ghost words, optionally with a bionic bold prefix on each so
 * the eye has fixation anchors in the periphery.
 */
private fun androidx.compose.ui.text.AnnotatedString.Builder.appendGhostWords(
    words: List<String>,
    color: Color,
    bionic: Boolean,
) {
    if (!bionic) {
        withStyle(SpanStyle(color = color)) {
            append(words.joinToString(" "))
        }
        return
    }
    for ((i, w) in words.withIndex()) {
        if (i > 0) append(" ")
        val prefix = bionicPrefixLength(w)
        withStyle(
            SpanStyle(color = color, fontWeight = FontWeight.Bold),
        ) {
            append(w.take(prefix))
        }
        withStyle(SpanStyle(color = color)) {
            append(w.drop(prefix))
        }
    }
}

/**
 * Measures the text at unbounded width and right-aligns it inside the
 * incoming constraints, publishing its baseline so [Row.alignByBaseline]
 * works across the focal columns.
 */
private fun Modifier.rightAlignedUnbounded(): Modifier =
    this.layout { measurable, constraints ->
        val placeable =
            measurable.measure(constraints.copy(maxWidth = Constraints.Infinity))
        val width = constraints.maxWidth
        val baseline =
            if (placeable[FirstBaseline] != AlignmentLine.Unspecified) {
                placeable[FirstBaseline]
            } else {
                placeable.height
            }
        layout(
            width,
            placeable.height,
            mapOf(FirstBaseline to baseline, LastBaseline to baseline),
        ) {
            placeable.placeRelative(width - placeable.width, 0)
        }
    }

/** Spritz guide dashes at the configured ORP column. */
@Composable
private fun GuideLines(
    color: Color,
    fraction: Float,
    modifier: Modifier = Modifier,
) {
    Canvas(
        modifier =
            modifier
                .fillMaxWidth()
                .height(4.dp),
    ) {
        val cx = size.width * fraction
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

/** The tappable, draggable WPM readout. */
@Composable
private fun WpmControl(
    wpm: Int,
    palette: ReaderPalette,
    onTap: () -> Unit,
    onSetWpm: (Int) -> Unit,
    modifier: Modifier = Modifier,
) {
    // The pointerInput blocks below are long-lived, but the callbacks
    // close over the current settings and are recreated on every WPM
    // change — read them through rememberUpdatedState or a drag keeps
    // calling a stale closure.
    val currentOnTap by rememberUpdatedState(onTap)
    val currentOnSetWpm by rememberUpdatedState(onSetWpm)
    val currentWpm by rememberUpdatedState(wpm)
    Text(
        text = "$wpm wpm",
        style = MaterialTheme.typography.labelLarge,
        color = palette.accent.copy(alpha = 0.8f),
        modifier =
            modifier
                .pointerInput(Unit) {
                    detectTapGestures { currentOnTap() }
                }.pointerInput(Unit) {
                    // The target is absolute from the drag's starting WPM
                    // (fractional pixels accumulated so slow, precise
                    // drags still register): relative deltas applied to
                    // the latest-composed value drop increments when two
                    // drag events land before a recomposition.
                    var dragBase = 0
                    var lastSent = 0
                    var accumulator = 0f
                    detectDragGestures(
                        onDragStart = {
                            dragBase = currentWpm
                            lastSent = currentWpm
                            accumulator = 0f
                        },
                    ) { _, dragAmount ->
                        // Right or up = faster, left or down = slower;
                        // the dominant axis wins.
                        val dx = dragAmount.x
                        val dy = -dragAmount.y
                        val d = if (kotlin.math.abs(dx) > kotlin.math.abs(dy)) dx else dy
                        accumulator += d / 2.dp.toPx()
                        val target = dragBase + accumulator.toInt()
                        if (target != lastSent) {
                            lastSent = target
                            currentOnSetWpm(target)
                        }
                    }
                }.padding(Spacing.md)
                .testTag("wpmControl"),
    )
}

@Composable
private fun WpmPopup(
    wpm: Int,
    palette: ReaderPalette,
    onChange: (Int) -> Unit,
    onDismiss: () -> Unit,
) {
    Popup(
        alignment = Alignment.BottomCenter,
        onDismissRequest = onDismiss,
    ) {
        Box(
            modifier =
                Modifier
                    .fillMaxWidth(0.8f)
                    .background(palette.surface, MaterialTheme.shapes.medium)
                    .padding(horizontal = Spacing.lg, vertical = Spacing.md),
        ) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Text(
                    text = "$wpm wpm",
                    style = MaterialTheme.typography.headlineSmall,
                    color = palette.accent,
                )
                Spacer(Modifier.height(Spacing.sm))
                Slider(
                    value = wpm.toFloat(),
                    onValueChange = { onChange(it.toInt()) },
                    valueRange =
                        ReaderSettings.WPM_MIN.toFloat()..ReaderSettings.WPM_MAX.toFloat(),
                    steps = 0,
                    modifier =
                        Modifier
                            .fillMaxWidth()
                            .testTag("wpmSlider"),
                    colors = readerSliderColors(palette),
                )
            }
        }
    }
}

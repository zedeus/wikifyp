package com.wikifyp.ui.reader

import androidx.compose.ui.Modifier
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.unit.dp
import com.wikifyp.reader.ReaderEngine

/**
 * Pointer-input modifier that translates raw touch events into
 * [ReaderEngine] gesture calls: hold, lift, drag and double-tap — and
 * routes vertical drags to [onBrowseOpen] (the browse overlay) instead
 * of the engine's horizontal scrub.
 *
 * A single `awaitPointerEventScope` loop handles everything so gesture
 * detectors don't compete. The first move past the threshold decides the
 * gesture by its dominant axis: horizontal scrubs tokens, vertical opens
 * browse (the engine still enters scrubbing, so the browse session ends
 * in the same scroll_back event a drag does). Double-tap is detected by
 * tracking the time between consecutive short taps.
 *
 * No manual sync is needed — the engine's [onStateChanged] callback
 * fires after every gesture call.
 */
fun Modifier.readerGestures(
    engine: ReaderEngine,
    onBrowseOpen: () -> Unit = {},
): Modifier =
    this.pointerInput(engine) {
        val dragThresholdPx = 12.dp.toPx()
        val tokensPerPx = 24.dp.toPx()
        val doubleTapWindowMs = 300L
        var lastTapTimeMs = 0L

        awaitPointerEventScope {
            while (true) {
                // Wait for a down event.
                val downEvent = awaitPointerEvent()
                val downChange = downEvent.changes.firstOrNull() ?: continue
                if (!downChange.pressed) continue

                engine.onTouchDown()

                val downX = downChange.position.x
                val downY = downChange.position.y
                val downTimeMs = downChange.uptimeMillis
                var isDragging = false
                var browseOpened = false
                var dragAccumulator = 0f

                // Track movement until lift.
                while (true) {
                    val event = awaitPointerEvent()
                    val change = event.changes.firstOrNull() ?: break

                    if (!change.pressed) {
                        if (browseOpened) {
                            // The browse overlay owns the rest of this
                            // interaction; the lift that opened it is not
                            // a pause or a tap.
                            break
                        }
                        // Finger lifted.
                        engine.onTouchUp()

                        // Check for double-tap: two short taps in quick succession.
                        val touchDurationMs = change.uptimeMillis - downTimeMs
                        val holdThreshold = ReaderEngine.HOLD_THRESHOLD_MS
                        if (!isDragging && touchDurationMs < holdThreshold) {
                            if (change.uptimeMillis - lastTapTimeMs < doubleTapWindowMs) {
                                engine.onDoubleTap()
                                lastTapTimeMs = 0L
                            } else {
                                lastTapTimeMs = change.uptimeMillis
                            }
                        } else {
                            lastTapTimeMs = 0L
                        }
                        break
                    }

                    if (browseOpened) {
                        change.consume()
                        continue
                    }

                    val dx = change.position.x - change.previousPosition.x
                    val totalDx = change.position.x - downX
                    val totalDy = change.position.y - downY

                    // The first move past the threshold decides the
                    // gesture by its dominant axis.
                    if (!isDragging) {
                        if (kotlin.math.abs(totalDx) > dragThresholdPx &&
                            kotlin.math.abs(totalDx) >= kotlin.math.abs(totalDy)
                        ) {
                            isDragging = true
                            engine.onDragStart()
                            dragAccumulator = 0f
                        } else if (kotlin.math.abs(totalDy) > dragThresholdPx) {
                            browseOpened = true
                            engine.onDragStart()
                            onBrowseOpen()
                            change.consume()
                            continue
                        }
                    }

                    if (isDragging) {
                        change.consume()
                        dragAccumulator += dx
                        val tokensMoved = (dragAccumulator / tokensPerPx).toInt()
                        if (tokensMoved != 0) {
                            // Drag left (negative dx) = scroll back = negative offset.
                            engine.onDragMove(tokensMoved)
                            dragAccumulator -= tokensMoved * tokensPerPx
                        }
                    }
                }
            }
        }
    }

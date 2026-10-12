package com.wikifyp.ui.reader

import androidx.compose.ui.Modifier
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.unit.dp
import com.wikifyp.reader.ReaderEngine

/**
 * Pointer-input modifier that translates raw touch events into
 * [ReaderEngine] gesture calls: hold, lift, drag and double-tap.
 *
 * A single `awaitPointerEventScope` loop handles everything so gesture
 * detectors don't compete. Double-tap is detected by tracking the time
 * between consecutive short taps (both under the hold threshold and
 * within 300 ms of each other).
 *
 * No manual sync is needed — the engine's [onStateChanged] callback
 * fires after every gesture call.
 */
fun Modifier.readerGestures(engine: ReaderEngine): Modifier =
    this.pointerInput(engine) {
        val dragThresholdPx = 20.dp.toPx()
        val tokensPerPx = 30.dp.toPx()
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
                val downTimeMs = downChange.uptimeMillis
                var isDragging = false
                var dragAccumulator = 0f

                // Track movement until lift.
                while (true) {
                    val event = awaitPointerEvent()
                    val change = event.changes.firstOrNull() ?: break

                    if (!change.pressed) {
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

                    val dx = change.position.x - change.previousPosition.x

                    // Start dragging once movement exceeds the threshold.
                    if (!isDragging &&
                        kotlin.math.abs(change.position.x - downX) > dragThresholdPx
                    ) {
                        isDragging = true
                        engine.onDragStart()
                        dragAccumulator = 0f
                    }

                    if (isDragging) {
                        change.consume()
                        dragAccumulator += dx
                        val tokensMoved = (dragAccumulator / tokensPerPx).toInt()
                        if (tokensMoved != 0) {
                            engine.onDragMove(-tokensMoved)
                            dragAccumulator -= tokensMoved * tokensPerPx
                        }
                    }
                }
            }
        }
    }

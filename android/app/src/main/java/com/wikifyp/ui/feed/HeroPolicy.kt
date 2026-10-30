package com.wikifyp.ui.feed

import android.graphics.Bitmap
import android.os.Build
import com.wikifyp.data.ArticleImage

// Hero image policy: whether an image is hero-worthy (suitable for a
// full-bleed card where text sits on a dark scrim). Diagram-like images
// (charts, maps, scientific figures with white backgrounds) look terrible
// full-bleed because the scrim washes them out and the white edges glare
// in dark mode.
//
// Two-stage check:
// 1. isHeroCandidateByMetadata — fast check on aspect ratio and dimensions.
// 2. isHeroWorthyBitmap — corner luminance sampling on the loaded bitmap.
// The first gates the second, so a portrait image never loads to be sampled.

/** Minimum aspect ratio for a hero candidate: wider than roughly 4:3. */
private const val MIN_HERO_ASPECT = 1.2f

/** Minimum width for a hero candidate: below this it would stretch. */
private const val MIN_HERO_WIDTH = 300

/**
 * Fast metadata-only check: is this image shaped like a hero?
 * Null images and images without dimensions are not hero candidates.
 */
fun isHeroCandidateByMetadata(image: ArticleImage?): Boolean {
    if (image == null) return false
    if (image.width < MIN_HERO_WIDTH) return false
    if (image.height <= 0) return false
    return image.width.toFloat() / image.height >= MIN_HERO_ASPECT
}

/**
 * Corner luminance threshold: if any sampled corner is brighter than this,
 * the image is likely a diagram on a white or near-white background.
 * 0.92 ≈ rgb(235,235,235): catches white-background charts, maps and
 * scientific figures while letting warm-toned photos through.
 */
private const val BRIGHT_CORNER_THRESHOLD = 0.92f

/** How many of the four corners must be bright to reject the image. */
private const val BRIGHT_CORNER_MIN_COUNT = 3

/** Pixel inset from each corner: sample a small patch rather than the
 *  very edge, which may be antialiased or have a 1px border. */
private const val CORNER_INSET = 4

/** Patch size at each corner: average over a small area for stability. */
private const val PATCH_SIZE = 6

/**
 * Bitmap-level hero check: samples corner patches and rejects images where
 * most corners are bright (white-background diagrams, charts, maps).
 *
 * Returns true when the image is suitable for full-bleed hero display.
 * Returns true for null (caller handles missing bitmap separately).
 */
fun isHeroWorthyBitmap(bitmap: Bitmap?): Boolean {
    if (bitmap == null) return true // no bitmap yet; let the metadata decide
    return countBrightCorners(bitmap).let { it < 0 || it < BRIGHT_CORNER_MIN_COUNT }
}

/**
 * Whether a loaded bitmap needs a dimming treatment in dark mode: images
 * with bright corners (white-background diagrams) glare against a dark
 * feed — even two bright corners on a dark feed look harsh.
 */
fun needsDarkModeDimming(bitmap: Bitmap?): Boolean {
    if (bitmap == null) return false
    val count = countBrightCorners(bitmap)
    return count >= 0 && count >= 2
}

/**
 * Counts how many of the four corner patches exceed the brightness
 * threshold. Returns -1 if the bitmap is too small to sample.
 */
internal fun countBrightCorners(bitmap: Bitmap): Int {
    val w = bitmap.width
    val h = bitmap.height
    if (w < CORNER_INSET + PATCH_SIZE || h < CORNER_INSET + PATCH_SIZE) return -1

    // Coil delivers Config.HARDWARE bitmaps on device (API 26+), and those
    // forbid getPixels(); sample a software copy instead. -1 (don't dim)
    // if the copy fails — a wrongly bright card beats a crash.
    val readable =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O &&
            bitmap.config == Bitmap.Config.HARDWARE
        ) {
            bitmap.copy(Bitmap.Config.ARGB_8888, false) ?: return -1
        } else {
            bitmap
        }

    val corners =
        listOf(
            CORNER_INSET to CORNER_INSET,
            w - CORNER_INSET - PATCH_SIZE to CORNER_INSET,
            CORNER_INSET to h - CORNER_INSET - PATCH_SIZE,
            w - CORNER_INSET - PATCH_SIZE to h - CORNER_INSET - PATCH_SIZE,
        )

    var bright = 0
    for ((cx, cy) in corners) {
        if (patchLuminance(readable, cx, cy, PATCH_SIZE) > BRIGHT_CORNER_THRESHOLD) {
            bright++
        }
    }
    return bright
}

/**
 * Average perceived luminance of a square patch starting at (x, y).
 * Uses the sRGB luminance formula (ITU-R BT.709).
 */
internal fun patchLuminance(
    bitmap: Bitmap,
    x: Int,
    y: Int,
    size: Int,
): Float {
    var totalLum = 0f
    var count = 0
    val pixels = IntArray(size * size)
    bitmap.getPixels(pixels, 0, size, x, y, size, size)
    for (pixel in pixels) {
        val r = (pixel shr 16 and 0xFF) / 255f
        val g = (pixel shr 8 and 0xFF) / 255f
        val b = (pixel and 0xFF) / 255f
        totalLum += 0.2126f * r + 0.7152f * g + 0.0722f * b
        count++
    }
    return if (count > 0) totalLum / count else 0f
}

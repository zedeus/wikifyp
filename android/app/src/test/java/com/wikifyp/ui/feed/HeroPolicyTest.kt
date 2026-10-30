package com.wikifyp.ui.feed

import android.graphics.Bitmap
import com.wikifyp.data.ArticleImage
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class HeroPolicyTest {
    // -- isHeroCandidateByMetadata --

    @Test
    fun nullImageIsNotAHeroCandidate() {
        assertFalse(isHeroCandidateByMetadata(null))
    }

    @Test
    fun landscapeImageIsAHeroCandidate() {
        assertTrue(isHeroCandidateByMetadata(ArticleImage("https://example.com/photo.jpg", 960, 640)))
    }

    @Test
    fun portraitImageIsNotAHeroCandidate() {
        assertFalse(isHeroCandidateByMetadata(ArticleImage("https://example.com/portrait.jpg", 400, 600)))
    }

    @Test
    fun squareImageIsNotAHeroCandidate() {
        // 1:1 is below the 1.2 minimum aspect ratio.
        assertFalse(isHeroCandidateByMetadata(ArticleImage("https://example.com/square.jpg", 500, 500)))
    }

    @Test
    fun tooSmallImageIsNotAHeroCandidate() {
        assertFalse(isHeroCandidateByMetadata(ArticleImage("https://example.com/tiny.jpg", 200, 100)))
    }

    @Test
    fun imageWithZeroHeightIsNotAHeroCandidate() {
        assertFalse(isHeroCandidateByMetadata(ArticleImage("https://example.com/bad.jpg", 960, 0)))
    }

    @Test
    fun imageWithoutDimensionsIsNotAHeroCandidate() {
        // Default width/height are 0.
        assertFalse(isHeroCandidateByMetadata(ArticleImage("https://example.com/nodims.jpg")))
    }

    @Test
    fun barelyLandscapeIsACandidate() {
        // 1.2:1 exactly should pass.
        assertTrue(isHeroCandidateByMetadata(ArticleImage("https://example.com/wide.jpg", 600, 500)))
    }

    // -- isHeroWorthyBitmap --

    private fun solidBitmap(
        width: Int,
        height: Int,
        color: Int,
    ): Bitmap {
        val bmp = Bitmap.createBitmap(width, height, Bitmap.Config.ARGB_8888)
        bmp.eraseColor(color)
        return bmp
    }

    @Test
    fun photographyBitmapIsHeroWorthy() {
        // A dark, colorful bitmap should be hero-worthy.
        val bmp = solidBitmap(200, 150, 0xFF3366AA.toInt())
        assertTrue(isHeroWorthyBitmap(bmp))
    }

    @Test
    fun whiteBackgroundBitmapIsNotHeroWorthy() {
        // A white bitmap mimics a diagram on a white background.
        val bmp = solidBitmap(200, 150, 0xFFFFFFFF.toInt())
        assertFalse(isHeroWorthyBitmap(bmp))
    }

    @Test
    fun nearWhiteBitmapIsNotHeroWorthy() {
        // Very light gray (245, 245, 245) is above the threshold.
        val bmp = solidBitmap(200, 150, 0xFFF5F5F5.toInt())
        assertFalse(isHeroWorthyBitmap(bmp))
    }

    @Test
    fun nullBitmapDefaultsToHeroWorthy() {
        // Before the bitmap loads, we don't reject.
        assertTrue(isHeroWorthyBitmap(null))
    }

    @Test
    fun tinyBitmapDefaultsToHeroWorthy() {
        // Too small to sample corners meaningfully.
        val bmp = solidBitmap(5, 5, 0xFFFFFFFF.toInt())
        assertTrue(isHeroWorthyBitmap(bmp))
    }

    @Test
    fun bitmapWithMixedCornersIsHeroWorthy() {
        // Two bright corners, two dark — should be hero-worthy (threshold is 3).
        val bmp = solidBitmap(200, 150, 0xFF333333.toInt())
        // Make top-left and top-right corners white.
        for (x in 0..20) {
            for (y in 0..20) {
                bmp.setPixel(x, y, 0xFFFFFFFF.toInt())
                bmp.setPixel(bmp.width - 1 - x, y, 0xFFFFFFFF.toInt())
            }
        }
        assertTrue(isHeroWorthyBitmap(bmp))
    }

    // -- needsDarkModeDimming --

    /**
     * Coil delivers Config.HARDWARE bitmaps on device, and those forbid
     * getPixels(): sampling must work on a hardware bitmap instead of
     * crashing (the dark-mode feed died on every loaded image).
     */
    @Test
    fun hardwareBitmapIsSampledWithoutCrashing() {
        val soft = solidBitmap(200, 150, 0xFFFFFFFF.toInt())
        val hard = soft.copy(Bitmap.Config.HARDWARE, false)
        assertTrue("hardware copy unsupported in this environment", hard != null)
        assertTrue(needsDarkModeDimming(hard))
    }

    @Test
    fun whiteBackgroundNeedsDimming() {
        val bmp = solidBitmap(200, 150, 0xFFFFFFFF.toInt())
        assertTrue(needsDarkModeDimming(bmp))
    }

    @Test
    fun darkPhotographyDoesNotNeedDimming() {
        val bmp = solidBitmap(200, 150, 0xFF222222.toInt())
        assertFalse(needsDarkModeDimming(bmp))
    }

    @Test
    fun nullBitmapDoesNotNeedDimming() {
        assertFalse(needsDarkModeDimming(null))
    }

    // -- patchLuminance --

    @Test
    fun whitePatchHasHighLuminance() {
        val bmp = solidBitmap(20, 20, 0xFFFFFFFF.toInt())
        val lum = patchLuminance(bmp, 0, 0, 6)
        assertTrue("expected luminance near 1.0, got $lum", lum > 0.99f)
    }

    @Test
    fun blackPatchHasLowLuminance() {
        val bmp = solidBitmap(20, 20, 0xFF000000.toInt())
        val lum = patchLuminance(bmp, 0, 0, 6)
        assertTrue("expected luminance near 0.0, got $lum", lum < 0.01f)
    }
}

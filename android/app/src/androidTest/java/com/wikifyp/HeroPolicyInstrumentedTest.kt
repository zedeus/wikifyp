package com.wikifyp

import android.graphics.Bitmap
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.wikifyp.ui.feed.needsDarkModeDimming
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Pixel sampling against real ART bitmaps. Coil delivers Config.HARDWARE
 * bitmaps on device and those forbid getPixels() — Robolectric's shadows
 * allow it, so only this test reproduces the dark-mode crash the feed had
 * on every loaded image.
 */
@RunWith(AndroidJUnit4::class)
class HeroPolicyInstrumentedTest {
    @Test
    fun hardwareBitmapIsSampledWithoutCrashing() {
        val soft = Bitmap.createBitmap(200, 150, Bitmap.Config.ARGB_8888)
        soft.eraseColor(0xFFFFFFFF.toInt())
        val hard = soft.copy(Bitmap.Config.HARDWARE, false)
        assertNotNull("hardware copy unsupported on this device", hard)
        assertTrue(needsDarkModeDimming(hard))
    }
}

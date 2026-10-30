package com.wikifyp.ui.reader

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import coil3.ColorImage
import coil3.ImageLoader
import coil3.SingletonImageLoader
import coil3.annotation.DelicateCoilApi
import coil3.annotation.ExperimentalCoilApi
import coil3.test.FakeImageLoaderEngine

/**
 * Swaps the process-wide Coil singleton for a fake that resolves every
 * image URL to a flat stand-in, so tests never touch the network. Pair
 * with [SingletonImageLoader.reset] in an @After so other classes in the
 * same JVM keep their own rendering regardless of execution order.
 * setUnsafe because other tests in this JVM already touched the singleton.
 */
@OptIn(ExperimentalCoilApi::class, DelicateCoilApi::class)
fun installFakeImageLoader() {
    val engine =
        FakeImageLoaderEngine
            .Builder()
            .default(ColorImage(0xFF607D8B.toInt()))
            .build()
    val context = ApplicationProvider.getApplicationContext<Context>()
    SingletonImageLoader.setUnsafe(
        ImageLoader.Builder(context).components { add(engine) }.build(),
    )
}

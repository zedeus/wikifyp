package com.wikifyp.ui

import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onRoot
import com.github.takahirom.roborazzi.RobolectricDeviceQualifiers
import com.github.takahirom.roborazzi.captureRoboImage
import com.wikifyp.theme.WikifypTheme
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

// Golden images live in src/test/screenshots (the roborazzi outputDir).
// `./gradlew test` verifies against them (roborazzi.test.verify=true in
// gradle.properties); re-record with `./gradlew recordRoborazziDebug`.
// MediumPhone = the generic 411x914dp @ 420dpi profile (identical qualifiers
// to Pixel 7/8/8a — phone screens have converged); matches the medium_phone
// emulator AVD used for instrumented tests.
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(qualifiers = RobolectricDeviceQualifiers.MediumPhone)
class PlaceholderScreenshotTest {
    @get:Rule val compose = createComposeRule()

    @Test
    fun light() {
        compose.setContent { WikifypTheme(darkTheme = false) { PlaceholderScreen() } }
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun dark() {
        compose.setContent { WikifypTheme(darkTheme = true) { PlaceholderScreen() } }
        compose.onRoot().captureRoboImage()
    }
}

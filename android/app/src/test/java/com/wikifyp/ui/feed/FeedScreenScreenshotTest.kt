package com.wikifyp.ui.feed

import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onRoot
import com.github.takahirom.roborazzi.RobolectricDeviceQualifiers
import com.github.takahirom.roborazzi.captureRoboImage
import com.wikifyp.data.FeedCard
import com.wikifyp.theme.WikifypTheme
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

// Golden images live in src/test/screenshots; `./gradlew test` verifies,
// `./gradlew recordRoborazziDebug` re-records. Same profile as the other
// screenshot tests (see PlaceholderScreenshotTest).
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(qualifiers = RobolectricDeviceQualifiers.MediumPhone)
class FeedScreenScreenshotTest {
    @get:Rule val compose = createComposeRule()

    // Fixed cards so the goldens never move with the fixture set.
    private val cards =
        listOf(
            FeedCard(
                id = 1,
                title = "Photosynthesis",
                source = "wikipedia",
                excerpt =
                    "Photosynthesis is a system of biological processes by which " +
                        "photosynthetic organisms convert light energy into chemical energy.",
                sentences = 180,
                words = 4200,
                attribution = "Text from the Wikipedia article \"Photosynthesis\"",
                description = "Biological process to convert light into chemical energy",
            ),
            FeedCard(
                id = 2,
                title = "Roman Empire",
                source = "wikipedia",
                excerpt = "The Roman Empire ruled the Mediterranean and much of Europe.",
                sentences = 240,
                words = 6100,
                attribution = "Text from the Wikipedia article \"Roman Empire\"",
                description = "27 BC–476 AD state and civilization",
            ),
            FeedCard(
                id = 3,
                title = "Fossil word",
                source = "simplewiki",
                excerpt = "A fossil word is a word that isn't used anymore except in one idiom or phrase.",
                sentences = 6,
                words = 32,
                attribution = "Text from the Simple English Wikipedia article \"Fossil word\"",
            ),
        )

    private fun capture(
        darkTheme: Boolean,
        state: FeedState,
    ) {
        compose.setContent {
            WikifypTheme(darkTheme = darkTheme) {
                FeedScreen(state = state, onCardClick = {}, onRetry = {})
            }
        }
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun light() = capture(darkTheme = false, state = FeedState.Content(cards, offline = false))

    @Test
    fun dark() = capture(darkTheme = true, state = FeedState.Content(cards, offline = false))

    @Test
    fun offlineLight() = capture(darkTheme = false, state = FeedState.Content(cards, offline = true))

    @Test
    fun offlineDark() = capture(darkTheme = true, state = FeedState.Content(cards, offline = true))

    @Test
    fun errorLight() = capture(darkTheme = false, state = FeedState.Error)

    @Test
    fun errorDark() = capture(darkTheme = true, state = FeedState.Error)
}

package com.wikifyp.ui

import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onRoot
import com.github.takahirom.roborazzi.RobolectricDeviceQualifiers
import com.github.takahirom.roborazzi.captureRoboImage
import com.wikifyp.data.FakeFeedRepository
import com.wikifyp.data.FeedResult
import com.wikifyp.data.testArticle
import com.wikifyp.data.testCard
import com.wikifyp.reader.Cancellable
import com.wikifyp.reader.ReaderClock
import com.wikifyp.ui.reader.ReaderSettings
import com.wikifyp.ui.reader.ReaderTheme
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * The theme picker is app-wide: a dark reader theme (ink, black) darkens
 * the feed too, a light one (paper, sepia) keeps it light. These goldens
 * pin the whole app under each, composed without any outer theme —
 * exactly how MainActivity hosts it.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(qualifiers = RobolectricDeviceQualifiers.MediumPhone)
class WikifypAppScreenshotTest {
    @get:Rule val compose = createComposeRule()

    private val clock =
        object : ReaderClock {
            override fun nowMs(): Long = 0

            override fun schedule(
                delayMs: Long,
                action: () -> Unit,
            ) = Cancellable {}
        }

    private fun show(theme: ReaderTheme) {
        compose.setContent {
            WikifypApp(
                repository =
                    FakeFeedRepository(
                        onFeed = { _ ->
                            FeedResult(
                                listOf(
                                    testCard(),
                                    testCard(id = 2, title = "Physics"),
                                    testCard(id = 3, title = "Biology"),
                                ),
                            )
                        },
                        onArticle = { testArticle(it) },
                    ),
                clock = clock,
                initialReaderSettings = ReaderSettings(theme = theme),
            )
        }
        compose.waitForIdle()
    }

    @Test
    fun feed_dark_via_black_reader_theme() {
        show(ReaderTheme.BLACK)
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun feed_light_via_sepia_reader_theme() {
        show(ReaderTheme.SEPIA)
        compose.onRoot().captureRoboImage()
    }
}

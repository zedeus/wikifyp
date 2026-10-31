package com.wikifyp.ui.history

import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import com.github.takahirom.roborazzi.RobolectricDeviceQualifiers
import com.github.takahirom.roborazzi.captureRoboImage
import com.wikifyp.data.FakeHistoryRepository
import com.wikifyp.data.HistoryEntry
import com.wikifyp.data.HistoryPage
import com.wikifyp.data.ResumeToken
import com.wikifyp.theme.WikifypTheme
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(qualifiers = RobolectricDeviceQualifiers.MediumPhone)
class HistoryScreenScreenshotTest {
    @get:Rule val compose = createComposeRule()

    private val entries =
        listOf(
            HistoryEntry(
                id = 1,
                pageId = 101,
                source = "wikipedia",
                title = "Photosynthesis",
                description = "Biological process to convert light into chemical energy",
                state = "finished",
                completion = 1f,
                words = 4200,
                updatedAt = "2026-10-31T10:00:00Z",
            ),
            HistoryEntry(
                id = 2,
                pageId = 102,
                source = "wikipedia",
                title = "Roman Empire",
                description = "27 BC–476 AD state and civilization",
                state = "",
                saved = true,
                completion = 0.65f,
                words = 6100,
                updatedAt = "2026-10-31T08:00:00Z",
                resumeToken = ResumeToken(2, 42, 3),
            ),
            HistoryEntry(
                id = 3,
                pageId = 103,
                source = "wikipedia",
                title = "DNA",
                description = "Chemical compound",
                state = "skipped",
                completion = 0.1f,
                words = 3500,
                updatedAt = "2026-10-30T14:00:00Z",
            ),
            HistoryEntry(
                id = 4,
                pageId = 104,
                source = "simplewiki",
                title = "Fossil word",
                state = "",
                completion = 0.42f,
                words = 32,
                updatedAt = "2026-10-29T20:00:00Z",
            ),
        )

    private fun repo() =
        FakeHistoryRepository().apply {
            page = HistoryPage(entries = entries)
        }

    private fun show(
        darkTheme: Boolean,
        repository: FakeHistoryRepository = repo(),
    ) {
        compose.setContent {
            WikifypTheme(darkTheme = darkTheme) {
                HistoryScreen(
                    repository = repository,
                    onEntryClick = {},
                    onBack = {},
                )
            }
        }
    }

    @Test
    fun cardLight() {
        show(darkTheme = false)
        compose.waitForIdle()
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun cardDark() {
        show(darkTheme = true)
        compose.waitForIdle()
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun listLight() {
        show(darkTheme = false)
        compose.waitForIdle()
        // Toggle to list view.
        compose.onNodeWithContentDescription("List view").performClick()
        compose.waitForIdle()
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun listDark() {
        show(darkTheme = true)
        compose.waitForIdle()
        compose.onNodeWithContentDescription("List view").performClick()
        compose.waitForIdle()
        compose.onRoot().captureRoboImage()
    }

    @Test
    fun emptyLight() {
        show(darkTheme = false, repository = FakeHistoryRepository())
        compose.waitForIdle()
        compose.onRoot().captureRoboImage()
    }
}

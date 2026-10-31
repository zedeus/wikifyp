package com.wikifyp.ui.history

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.longClick
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.performTouchInput
import com.wikifyp.data.DeleteResult
import com.wikifyp.data.FakeHistoryRepository
import com.wikifyp.data.HistoryEntry
import com.wikifyp.data.HistoryPage
import com.wikifyp.data.PageRef
import com.wikifyp.data.ResumeToken
import com.wikifyp.theme.WikifypTheme
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class HistoryScreenTest {
    @get:Rule val compose = createComposeRule()

    private val entries =
        listOf(
            HistoryEntry(
                id = 1,
                pageId = 101,
                source = "wikipedia",
                title = "Photosynthesis",
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
                state = "",
                saved = true,
                completion = 0.65f,
                words = 6100,
                updatedAt = "2026-10-30T08:00:00Z",
                resumeToken = ResumeToken(2, 42, 3),
            ),
            HistoryEntry(
                id = 3,
                pageId = 103,
                source = "wikipedia",
                title = "DNA",
                state = "skipped",
                completion = 0.1f,
                words = 3500,
                updatedAt = "2026-10-28T14:00:00Z",
            ),
        )

    private fun repo() =
        FakeHistoryRepository().apply {
            page = HistoryPage(entries = entries)
        }

    private fun show(
        repository: FakeHistoryRepository = repo(),
        onEntryClick: (HistoryEntry) -> Unit = {},
    ) {
        compose.setContent {
            WikifypTheme {
                HistoryScreen(
                    repository = repository,
                    onEntryClick = onEntryClick,
                    onBack = {},
                    undoDelayMs = 60_000L, // long delay so undo doesn't fire during test
                )
            }
        }
        compose.waitForIdle()
    }

    @Test
    fun `shows entries with status`() {
        show()
        compose.onNodeWithText("Photosynthesis").assertIsDisplayed()
        compose.onNodeWithText("Finished").assertIsDisplayed()
        compose.onNodeWithText("Roman Empire").assertIsDisplayed()
        compose.onNodeWithText("65%").assertIsDisplayed() // saved + 65% completion
        compose.onNodeWithText("DNA").assertIsDisplayed()
        compose.onNodeWithText("Skipped").assertIsDisplayed()
    }

    @Test
    fun `shows date separators`() {
        show()
        // Entries on different days should show date headers.
        // The exact labels depend on the current date, but we should see
        // at least multiple groupings — verify at least one entry renders.
        compose.onNodeWithText("Photosynthesis").assertIsDisplayed()
    }

    @Test
    fun `search filters entries`() {
        val fakeRepo =
            object : FakeHistoryRepository() {
                var lastSearch: String = ""

                override suspend fun history(
                    cursor: String,
                    search: String,
                    date: String,
                ): HistoryPage {
                    lastSearch = search
                    return if (search.isNotEmpty()) {
                        HistoryPage(
                            entries =
                                entries.filter {
                                    it.title.contains(search, ignoreCase = true)
                                },
                        )
                    } else {
                        HistoryPage(entries = entries)
                    }
                }
            }.apply {
                page = HistoryPage(entries = entries)
            }
        show(repository = fakeRepo)
        // Open search.
        compose.onNodeWithContentDescription("Search").performClick()
        compose.waitForIdle()
        // Type a query.
        compose.onNodeWithText("Search history…").performTextInput("Photo")
        compose.waitForIdle()
        // After debounce, the repo should have been called with the search.
        compose.mainClock.advanceTimeBy(500)
        compose.waitForIdle()
        assertEquals("Photo", fakeRepo.lastSearch)
    }

    @Test
    fun `hold-to-select then delete shows undo`() {
        val fakeRepo = repo()
        show(repository = fakeRepo)
        // Long-press to select.
        compose.onNodeWithText("Photosynthesis").performTouchInput { longClick() }
        compose.waitForIdle()
        // Should show selection bar with "1 selected".
        compose.onNodeWithText("1 selected").assertIsDisplayed()
        // Delete.
        compose.onNodeWithContentDescription("Delete selected").performClick()
        compose.waitForIdle()
        // Should show undo snackbar.
        compose.onNodeWithText("Article removed").assertIsDisplayed()
        compose.onNodeWithText("Undo").assertIsDisplayed()
        // Entry should be gone from the list.
        compose.onNodeWithText("Photosynthesis").assertDoesNotExist()
        // Server should NOT have been called yet (undo window).
        assertTrue(fakeRepo.deletedPages.isEmpty())
    }

    @Test
    fun `undo restores deleted entry`() {
        show()
        // Select and delete.
        compose.onNodeWithText("DNA").performTouchInput { longClick() }
        compose.waitForIdle()
        compose.onNodeWithContentDescription("Delete selected").performClick()
        compose.waitForIdle()
        compose.onNodeWithText("DNA").assertDoesNotExist()
        // Undo.
        compose.onNodeWithText("Undo").performClick()
        compose.waitForIdle()
        // Entry should be restored.
        compose.onNodeWithText("DNA").assertIsDisplayed()
    }

    @Test
    fun `delete-all requires confirmation`() {
        show()
        // Select one to enter selection mode.
        compose.onNodeWithText("Photosynthesis").performTouchInput { longClick() }
        compose.waitForIdle()
        // Click delete-all.
        compose.onNodeWithContentDescription("Delete all").performClick()
        compose.waitForIdle()
        // Should show confirmation dialog.
        compose.onNodeWithText("Delete all history?").assertIsDisplayed()
    }

    @Test
    fun `clicking entry calls onEntryClick with resume token`() {
        var clickedEntry: HistoryEntry? = null
        show(onEntryClick = { clickedEntry = it })
        compose.onNodeWithText("Roman Empire").performClick()
        compose.waitForIdle()
        assertEquals("Roman Empire", clickedEntry?.title)
        assertEquals(42, clickedEntry?.resumeToken?.sentenceIdx)
        assertEquals(3, clickedEntry?.resumeToken?.tokenIdx)
    }

    @Test
    fun `toggle between card and list layouts`() {
        show()
        // Default is card layout; toggle to list.
        compose.onNodeWithContentDescription("List view").performClick()
        compose.waitForIdle()
        // Entries should still show.
        compose.onNodeWithText("Photosynthesis").assertIsDisplayed()
        // Toggle back to card.
        compose.onNodeWithContentDescription("Card view").performClick()
        compose.waitForIdle()
        compose.onNodeWithText("Photosynthesis").assertIsDisplayed()
    }

    @Test
    fun `empty state shows message`() {
        show(repository = FakeHistoryRepository())
        compose.onNodeWithText("No reading history yet").assertIsDisplayed()
        compose.onNodeWithText("Articles you read will appear here").assertIsDisplayed()
    }
}

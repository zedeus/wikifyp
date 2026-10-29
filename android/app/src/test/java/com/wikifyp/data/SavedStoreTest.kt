package com.wikifyp.data

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class SavedStoreTest {
    private lateinit var store: SavedStore
    private lateinit var prefs: android.content.SharedPreferences

    @Before
    fun setUp() {
        val context = ApplicationProvider.getApplicationContext<Context>()
        prefs = context.getSharedPreferences("test", Context.MODE_PRIVATE)
        store = SavedStore(prefs)
    }

    @Test
    fun emptyStoreLoadsNoBookmarks() {
        assertEquals(emptyList<FeedCard>(), store.load())
    }

    @Test
    fun bookmarksRoundTripInOrder() {
        val cards =
            listOf(
                testCard(id = 2, title = "Physics"),
                testCard(id = 1, title = "Chemistry"),
            )
        store.save(cards)
        assertEquals(cards, store.load())
    }

    @Test
    fun corruptStoredJsonLoadsAsNoBookmarks() {
        prefs.edit().putString("savedArticles", "{not json").apply()
        assertEquals(emptyList<FeedCard>(), store.load())
    }
}

package com.wikifyp.data

import android.content.SharedPreferences
import androidx.core.content.edit
import kotlinx.serialization.json.Json

/**
 * Local persistence for the reader's bookmarks: the saved articles' cards,
 * newest first, as JSON in SharedPreferences. The durable cross-device
 * record is the save/unsave event stream; this snapshot is what makes the
 * Saved screen instant and offline. A card is small (title, description,
 * image URL), so even hundreds of bookmarks stay well under a megabyte.
 */
class SavedStore(
    private val prefs: SharedPreferences,
) {
    fun load(): List<FeedCard> =
        prefs.getString(KEY, null)?.let { stored ->
            runCatching { json.decodeFromString<List<FeedCard>>(stored) }.getOrNull()
        } ?: emptyList()

    fun save(cards: List<FeedCard>) {
        prefs.edit { putString(KEY, json.encodeToString(cards)) }
    }

    private companion object {
        const val KEY = "savedArticles"
        val json = Json { ignoreUnknownKeys = true }
    }
}

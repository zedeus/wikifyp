package com.wikifyp.data

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/** One article in the reader's history. */
@Serializable
data class HistoryEntry(
    val id: Long,
    @SerialName("page_id") val pageId: Long,
    val source: String,
    val title: String,
    val description: String = "",
    @SerialName("image_url") val imageUrl: String = "",
    @SerialName("image_width") val imageWidth: Int = 0,
    @SerialName("image_height") val imageHeight: Int = 0,
    val state: String = "",
    val saved: Boolean = false,
    val completion: Float = 0f,
    val words: Int = 0,
    @SerialName("updated_at") val updatedAt: String = "",
    @SerialName("resume_token") val resumeToken: ResumeToken? = null,
    val categories: List<String> = emptyList(),
) {
    val image: ArticleImage?
        get() = if (imageUrl.isNotEmpty()) ArticleImage(imageUrl, imageWidth, imageHeight) else null

    val statusLabel: String
        get() =
            when {
                state == "finished" -> "Finished"
                state == "skipped" -> "Skipped"
                completion > 0f -> "${(completion * 100).toInt()}%"
                saved -> "Saved"
                else -> ""
            }
}

/** Where to resume reading an unfinished article. */
@Serializable
data class ResumeToken(
    @SerialName("article_id") val articleId: Long,
    @SerialName("sentence_idx") val sentenceIdx: Int,
    @SerialName("token_idx") val tokenIdx: Int,
)

/** A page of history results with an optional cursor for the next page. */
@Serializable
data class HistoryPage(
    val entries: List<HistoryEntry> = emptyList(),
    @SerialName("next_cursor") val nextCursor: String = "",
)

/** Aggregate reading statistics. */
@Serializable
data class ReadingStats(
    @SerialName("total_words_read") val totalWordsRead: Long = 0,
    @SerialName("total_articles") val totalArticles: Int = 0,
    @SerialName("articles_finished") val articlesFinished: Int = 0,
    @SerialName("articles_skipped") val articlesSkipped: Int = 0,
    @SerialName("total_reading_time_s") val totalReadingTimeS: Long = 0,
    @SerialName("current_streak") val currentStreak: Int = 0,
    @SerialName("longest_streak") val longestStreak: Int = 0,
    @SerialName("wpm_over_time") val wpmOverTime: List<WpmBucket> = emptyList(),
    @SerialName("top_categories") val topCategories: List<CategoryStat> = emptyList(),
)

@Serializable
data class WpmBucket(
    val week: String = "",
    @SerialName("avg_wpm") val avgWpm: Float = 0f,
    val events: Int = 0,
)

@Serializable
data class CategoryStat(
    val category: String = "",
    val articles: Int = 0,
    val words: Long = 0,
    @SerialName("avg_completion") val avgCompletion: Float = 0f,
)

/** A reader's inferred interest profile. */
@Serializable
data class ReaderInterests(
    val clusters: List<InterestCluster> = emptyList(),
    @SerialName("top_categories") val topCategories: List<WeightedCategory> = emptyList(),
    @SerialName("total_articles") val totalArticles: Int = 0,
    @SerialName("explore_share") val exploreShare: Float = 0f,
)

@Serializable
data class InterestCluster(
    val categories: List<String> = emptyList(),
    val weight: Float = 0f,
    val articles: List<ClusterArticle> = emptyList(),
    @SerialName("article_count") val articleCount: Int = 0,
)

@Serializable
data class ClusterArticle(
    val title: String = "",
    val completion: Float = 0f,
    val state: String = "",
)

@Serializable
data class WeightedCategory(
    val category: String = "",
    val weight: Float = 0f,
)

/** Why a feed card was recommended. */
@Serializable
data class RecommendReason(
    val kind: String = "",
    @SerialName("seed_title") val seedTitle: String = "",
    @SerialName("seed_categories") val seedCategories: List<String> = emptyList(),
) {
    /** Human-friendly reason text for display. */
    val displayText: String
        get() =
            when (kind) {
                "momentum" -> if (seedTitle.isNotEmpty()) "Continuing from $seedTitle" else "Based on your latest read"
                "relevance" -> if (seedTitle.isNotEmpty()) "Because you read $seedTitle" else "Matches your interests"
                "exploration" -> "Exploring something new"
                else -> "Recommended for you"
            }
}

@Serializable
data class DeleteResult(
    @SerialName("pages_deleted") val pagesDeleted: Int = 0,
    @SerialName("events_deleted") val eventsDeleted: Int = 0,
)

/** Provides reading history, stats and interests from the server. */
interface HistoryRepository {
    suspend fun history(
        cursor: String = "",
        search: String = "",
        date: String = "",
    ): HistoryPage

    suspend fun stats(): ReadingStats

    suspend fun interests(): ReaderInterests

    suspend fun deletePages(pages: List<PageRef>): DeleteResult

    suspend fun deleteAll(): DeleteResult
}

@Serializable
data class PageRef(
    val source: String,
    @SerialName("page_id") val pageId: Long,
)

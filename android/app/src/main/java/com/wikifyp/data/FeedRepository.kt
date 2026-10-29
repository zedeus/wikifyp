package com.wikifyp.data

import kotlinx.serialization.Serializable

/**
 * One feed entry: an article the server (or the bundled fixture set) offers
 * next, with what the card shows.
 */
@Serializable
data class FeedCard(
    val id: Long,
    val title: String,
    val source: String,
    val excerpt: String,
    val sentences: Int,
    val words: Int,
    val attribution: String,
    /** Wikidata short description, e.g. "Chemical compound"; may be empty. */
    val description: String = "",
    /** Lead image thumbnail; null when the article has no free image. */
    val image: ArticleImage? = null,
) {
    /** The card's one-line subtitle: the description, or failing that the
     *  start of the article itself. One rule for every card surface. */
    val subtitle: String get() = description.ifEmpty { excerpt }
}

/** A feed plus where it came from: [offline] means it was served from cache. */
data class FeedResult(
    val cards: List<FeedCard>,
    val offline: Boolean = false,
)

/**
 * Where the feed and article text come from. Implementations: the HTTP
 * client against the server, the disk-cache wrapper around it, and the
 * bundled fixture articles (the default when no server is configured, and
 * what tests fake).
 */
interface FeedRepository {
    /** The reader's next articles, best first. */
    suspend fun feed(): FeedResult

    /** One article's text, ready for the reader. */
    suspend fun article(id: Long): Article

    /**
     * Makes [cards]'s articles readable offline, where the implementation
     * has an offline story; a no-op elsewhere. Call it once the feed is
     * already on screen — it may download every listed article.
     */
    suspend fun prefetch(cards: List<FeedCard>) {}
}

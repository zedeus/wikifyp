package com.wikifyp.data

import com.wikifyp.reader.SentenceTokens
import com.wikifyp.reader.tokenize
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * One article the reader can play: the server's `/v1/articles/{id}` response,
 * which is also the format of the bundled fixture assets, so a downloaded
 * article and a bundled one are the same thing to the app.
 */
@Serializable
data class Article(
    val id: Long,
    val title: String,
    val attribution: String,
    val sentences: List<Sentence>,
    /** Wikidata short description, e.g. "Chemical compound"; may be empty. */
    val description: String = "",
    /** Lead image thumbnail; null when the article has no free image. */
    val image: ArticleImage? = null,
)

/**
 * An article's lead image thumbnail. Width and height travel with the URL
 * so a surface that wants the image's own aspect ratio can reserve it
 * before the bitmap arrives; the current cards crop to fixed shapes and
 * don't read them yet.
 */
@Serializable
data class ArticleImage(
    val url: String,
    val width: Int = 0,
    val height: Int = 0,
)

@Serializable
data class Sentence(
    val text: String,
    @SerialName("heading_level") val headingLevel: Int = 0,
)

/**
 * Converts an article's sentences to the engine's token format.
 */
fun Article.toSentenceTokens(): List<SentenceTokens> =
    sentences.map { SentenceTokens(tokenize(it.text), isHeading = it.headingLevel > 0) }

/**
 * Derives the article's feed card the same way the server does: first
 * prose sentence as the excerpt, words = tokens of every sentence. The one
 * derivation rule serves the fixture feed and locally built cards alike.
 */
fun Article.toFeedCard(source: String): FeedCard =
    FeedCard(
        id = id,
        title = title,
        source = source,
        excerpt =
            sentences
                .firstOrNull { it.headingLevel == 0 }
                ?.text
                .orEmpty(),
        sentences = sentences.size,
        words = sentences.sumOf { tokenize(it.text).size },
        attribution = attribution,
        description = description,
        image = image,
    )

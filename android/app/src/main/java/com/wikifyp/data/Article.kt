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

package com.wikifyp.data

import android.content.Context
import com.wikifyp.reader.SentenceTokens
import com.wikifyp.reader.tokenize
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

/**
 * An article bundled as an asset, produced by the server's sentence splitter
 * so token addresses agree between platforms.
 */
@Serializable
data class FixtureArticle(
    val id: Int,
    val title: String,
    val attribution: String,
    val sentences: List<FixtureSentence>,
)

@Serializable
data class FixtureSentence(
    val text: String,
    @SerialName("heading_level") val headingLevel: Int = 0,
)

private val json = Json { ignoreUnknownKeys = true }

/**
 * Lists the bundled fixture articles (metadata only, no sentence loading).
 */
fun listFixtureArticles(context: Context): List<FixtureArticle> {
    val files = context.assets.list("articles") ?: return emptyList()
    return files
        .filter { it.endsWith(".json") }
        .sorted()
        .map { filename ->
            val text =
                context.assets
                    .open("articles/$filename")
                    .bufferedReader()
                    .readText()
            json.decodeFromString<FixtureArticle>(text)
        }
}

/**
 * Converts a fixture article's sentences to the engine's token format.
 */
fun FixtureArticle.toSentenceTokens(): List<SentenceTokens> =
    sentences.map { SentenceTokens(tokenize(it.text), isHeading = it.headingLevel > 0) }

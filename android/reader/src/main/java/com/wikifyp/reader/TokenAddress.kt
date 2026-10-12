package com.wikifyp.reader

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Stable address of a single token within an article:
 * `(articleId, sentenceIdx, tokenIdx)`.
 *
 * A token is one whitespace-delimited chunk of a sentence's stored text.
 * Addresses are never renumbered once stored.
 */
@Serializable
data class TokenAddress(
    @SerialName("article_id") val articleId: Long,
    @SerialName("sentence_idx") val sentenceIdx: Int,
    @SerialName("token_idx") val tokenIdx: Int,
)

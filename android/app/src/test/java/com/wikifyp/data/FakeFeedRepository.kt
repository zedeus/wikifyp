package com.wikifyp.data

import java.io.IOException

/** Test double: each call runs the currently assigned block. */
class FakeFeedRepository(
    var onFeed: suspend (Set<Long>) -> FeedResult = { FeedResult(emptyList()) },
    var onArticle: suspend (Long) -> Article = { throw IOException("no network") },
) : FeedRepository {
    override suspend fun feed(exclude: Set<Long>): FeedResult = onFeed(exclude)

    override suspend fun article(id: Long): Article = onArticle(id)
}

fun testCard(
    id: Long = 1,
    title: String = "Chemistry",
    words: Int = 1200,
    image: ArticleImage? = null,
    description: String = "",
) = FeedCard(
    id = id,
    title = title,
    source = "simplewiki",
    excerpt = "$title is the science of matter and how it changes.",
    sentences = 40,
    words = words,
    attribution = "Text from the Simple English Wikipedia article \"$title\"",
    description = description,
    image = image,
)

fun testArticle(
    id: Long = 1,
    title: String = "Chemistry",
) = Article(
    id = id,
    title = title,
    attribution = "Text from the Simple English Wikipedia article \"$title\"",
    sentences =
        listOf(
            Sentence("$title is a science."),
            Sentence("History", headingLevel = 2),
            Sentence("It is old."),
        ),
)

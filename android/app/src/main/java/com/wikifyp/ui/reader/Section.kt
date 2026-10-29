package com.wikifyp.ui.reader

import com.wikifyp.reader.SentenceTokens

/**
 * A section of the article: where it starts in flat token coordinates and
 * its heading text. The region before the first heading is the implicit
 * lead section, titled by the article.
 */
data class Section(
    val startFlat: Int,
    val title: String,
)

/**
 * The article's sections in reading order: one entry per heading sentence,
 * plus a leading entry at 0 titled [articleTitle] when the article has any
 * prose before its first heading (or no headings at all).
 */
fun sectionIndex(
    sentences: List<SentenceTokens>,
    articleTitle: String,
): List<Section> {
    val sections = mutableListOf<Section>()
    var flat = 0
    for (sentence in sentences) {
        if (sentence.isHeading) {
            if (sections.isEmpty() && flat > 0) {
                sections.add(Section(0, articleTitle))
            }
            sections.add(Section(flat, sentence.tokens.joinToString(" ")))
        }
        flat += sentence.tokens.size
    }
    if (sections.isEmpty() && flat > 0) {
        sections.add(Section(0, articleTitle))
    }
    return sections
}

/** The section containing [flat]: the last one starting at or before it. */
fun currentSection(
    sections: List<Section>,
    flat: Int,
): Section? = sections.lastOrNull { it.startFlat <= flat }

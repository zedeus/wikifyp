package com.wikifyp.ui.reader

import com.wikifyp.reader.SentenceTokens

/**
 * One tappable word inside a [BrowseBlock]: its flat token index (the
 * engine's position space) and its character range in the block's text.
 */
data class BrowseToken(
    val flatIndex: Int,
    val start: Int,
    val end: Int,
)

/**
 * One paragraph-sized piece of the browse overlay: a heading on its own,
 * or a run of sentences joined into flowing text. Blocks keep their
 * tokens' character ranges so a tap maps back to an engine position.
 */
data class BrowseBlock(
    val text: String,
    val isHeading: Boolean,
    val tokens: List<BrowseToken>,
)

/**
 * Splits an article's sentences into browse blocks. Headings become their
 * own blocks; runs of body sentences are chunked into blocks of at most
 * [sentencesPerBlock] so the text flows in paragraph-sized pieces (the
 * fixtures carry no paragraph boundaries — headings are the only real
 * structure) and no single composable has to lay out a whole section.
 */
fun browseBlocks(
    sentences: List<SentenceTokens>,
    sentencesPerBlock: Int = 6,
): List<BrowseBlock> {
    val blocks = mutableListOf<BrowseBlock>()
    var run = mutableListOf<Pair<Int, SentenceTokens>>() // (flat base, sentence)
    var flat = 0

    fun flushRun() {
        run.chunked(sentencesPerBlock).forEach { chunk ->
            blocks.add(block(chunk, isHeading = false))
        }
        run = mutableListOf()
    }

    for (sentence in sentences) {
        if (sentence.tokens.isEmpty()) continue
        if (sentence.isHeading) {
            flushRun()
            blocks.add(block(listOf(flat to sentence), isHeading = true))
        } else {
            run.add(flat to sentence)
        }
        flat += sentence.tokens.size
    }
    flushRun()
    return blocks
}

private fun block(
    chunk: List<Pair<Int, SentenceTokens>>,
    isHeading: Boolean,
): BrowseBlock {
    val text = StringBuilder()
    val tokens = mutableListOf<BrowseToken>()
    for ((base, sentence) in chunk) {
        for ((i, token) in sentence.tokens.withIndex()) {
            if (text.isNotEmpty()) text.append(' ')
            val start = text.length
            text.append(token)
            tokens.add(BrowseToken(base + i, start, text.length))
        }
    }
    return BrowseBlock(text.toString(), isHeading, tokens)
}

/** The index of the block containing [flatIndex], or 0 when none does. */
fun blockIndexFor(
    blocks: List<BrowseBlock>,
    flatIndex: Int,
): Int {
    val idx =
        blocks.indexOfFirst { block ->
            block.tokens.isNotEmpty() &&
                flatIndex >= block.tokens.first().flatIndex &&
                flatIndex <= block.tokens.last().flatIndex
        }
    return if (idx >= 0) idx else 0
}

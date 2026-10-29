package com.wikifyp.ui.reader

import com.wikifyp.reader.SentenceTokens
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class BrowseBlocksTest {
    @Test
    fun emptyArticleGivesNoBlocks() {
        assertEquals(emptyList<BrowseBlock>(), browseBlocks(emptyList()))
    }

    @Test
    fun emptySentencesAreSkipped() {
        val blocks =
            browseBlocks(
                listOf(
                    SentenceTokens(emptyList()),
                    SentenceTokens(listOf("Hello.")),
                ),
            )
        assertEquals(1, blocks.size)
        assertEquals("Hello.", blocks[0].text)
    }

    @Test
    fun headingGetsItsOwnBlock() {
        val blocks =
            browseBlocks(
                listOf(
                    SentenceTokens(listOf("Body", "one.")),
                    SentenceTokens(listOf("History"), isHeading = true),
                    SentenceTokens(listOf("Body", "two.")),
                ),
            )
        assertEquals(3, blocks.size)
        assertEquals("Body one.", blocks[0].text)
        assertEquals("History", blocks[1].text)
        assertTrue(blocks[1].isHeading)
        assertEquals("Body two.", blocks[2].text)
    }

    @Test
    fun bodySentencesJoinIntoOneBlock() {
        val blocks =
            browseBlocks(
                listOf(
                    SentenceTokens(listOf("One.")),
                    SentenceTokens(listOf("Two.")),
                ),
            )
        assertEquals(1, blocks.size)
        assertEquals("One. Two.", blocks[0].text)
    }

    @Test
    fun longRunsChunkAtSentencesPerBlock() {
        val sentences = List(7) { SentenceTokens(listOf("s$it.")) }
        val blocks = browseBlocks(sentences, sentencesPerBlock = 3)
        assertEquals(listOf("s0. s1. s2.", "s3. s4. s5.", "s6."), blocks.map { it.text })
    }

    @Test
    fun flatIndexesAreContinuousAcrossBlocks() {
        val blocks =
            browseBlocks(
                listOf(
                    SentenceTokens(listOf("A", "B")),
                    SentenceTokens(listOf("H"), isHeading = true),
                    SentenceTokens(listOf("C", "D")),
                ),
            )
        val flat = blocks.flatMap { b -> b.tokens.map { it.flatIndex } }
        assertEquals(listOf(0, 1, 2, 3, 4), flat)
    }

    @Test
    fun tokenRangesSliceBackToTheToken() {
        val blocks =
            browseBlocks(listOf(SentenceTokens(listOf("The", "quick", "fox."))))
        val block = blocks.single()
        for (t in block.tokens) {
            val slice = block.text.substring(t.start, t.end)
            assertTrue("range of ${t.flatIndex} gives a token", slice.isNotEmpty())
        }
        assertEquals("quick", block.text.substring(block.tokens[1].start, block.tokens[1].end))
    }

    @Test
    fun blockIndexForFindsTheBlock() {
        val blocks =
            browseBlocks(
                listOf(
                    SentenceTokens(listOf("A", "B")),
                    SentenceTokens(listOf("H"), isHeading = true),
                    SentenceTokens(listOf("C")),
                ),
            )
        assertEquals(0, blockIndexFor(blocks, 0))
        assertEquals(0, blockIndexFor(blocks, 1))
        assertEquals(1, blockIndexFor(blocks, 2))
        assertEquals(2, blockIndexFor(blocks, 3))
        // Out of range falls back to the first block.
        assertEquals(0, blockIndexFor(blocks, 99))
    }
}

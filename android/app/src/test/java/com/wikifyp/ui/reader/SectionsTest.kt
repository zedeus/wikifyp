package com.wikifyp.ui.reader

import com.wikifyp.reader.SentenceTokens
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class SectionsTest {
    private fun heading(vararg tokens: String) = SentenceTokens(tokens.toList(), isHeading = true)

    private fun prose(vararg tokens: String) = SentenceTokens(tokens.toList())

    @Test
    fun leadProseBecomesImplicitTitleSection() {
        val sections =
            sectionIndex(
                listOf(
                    prose("DNA", "is", "a", "molecule."),
                    heading("Structure"),
                    prose("It", "has", "two", "strands."),
                ),
                articleTitle = "DNA",
            )
        assertEquals(
            listOf(Section(0, "DNA"), Section(4, "Structure")),
            sections,
        )
    }

    @Test
    fun headingFirstArticleHasNoImplicitLead() {
        val sections =
            sectionIndex(
                listOf(heading("History"), prose("Long", "ago.")),
                articleTitle = "Rome",
            )
        assertEquals(listOf(Section(0, "History")), sections)
    }

    @Test
    fun noHeadingsYieldsSingleTitleSection() {
        val sections =
            sectionIndex(
                listOf(prose("Just", "prose.")),
                articleTitle = "Stub",
            )
        assertEquals(listOf(Section(0, "Stub")), sections)
    }

    @Test
    fun emptyArticleYieldsNoSections() {
        assertEquals(emptyList<Section>(), sectionIndex(emptyList(), "Empty"))
        assertNull(currentSection(emptyList(), 0))
    }

    @Test
    fun multiWordHeadingJoinsTokens() {
        val sections =
            sectionIndex(
                listOf(heading("Early", "life", "and", "career"), prose("Born.")),
                articleTitle = "Someone",
            )
        assertEquals("Early life and career", sections[0].title)
    }

    @Test
    fun currentSectionIsLastAtOrBeforePosition() {
        val sections =
            listOf(Section(0, "Lead"), Section(4, "Structure"), Section(9, "History"))
        assertEquals("Lead", currentSection(sections, 0)!!.title)
        assertEquals("Lead", currentSection(sections, 3)!!.title)
        assertEquals("Structure", currentSection(sections, 4)!!.title)
        assertEquals("Structure", currentSection(sections, 8)!!.title)
        assertEquals("History", currentSection(sections, 99)!!.title)
    }
}

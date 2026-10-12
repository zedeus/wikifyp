package com.wikifyp.reader

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Validates that the Kotlin tokenization (normalize then split on " ")
 * agrees with the shared fixture at fixtures/tokens.json. The same
 * fixture is tested on the Go side in server/internal/text, so the two
 * platforms can never disagree on token addressing.
 */
class TokensTest {
    @Serializable
    data class TokenCase(
        val comment: String,
        val text: String,
        val tokens: List<String>,
    )

    private val cases: List<TokenCase> by lazy {
        val stream =
            javaClass.classLoader!!.getResourceAsStream("tokens.json")
                ?: error("fixtures/tokens.json not on test classpath")
        Json.decodeFromString<List<TokenCase>>(stream.bufferedReader().readText())
    }

    @Test
    fun sharedFixtureIsNotEmpty() {
        assert(cases.isNotEmpty()) { "fixture must have at least one case" }
    }

    @Test
    fun normalizeAndSplitMatchesFixture() {
        for (tc in cases) {
            val normalized = normalize(tc.text)
            val got = tokenize(normalized)
            assertEquals(
                "token count for '${tc.comment}'",
                tc.tokens.size,
                got.size,
            )
            for (i in got.indices) {
                assertEquals(
                    "token[$i] for '${tc.comment}'",
                    tc.tokens[i],
                    got[i],
                )
            }
        }
    }

    @Test
    fun emptyAndBlankInput() {
        assertEquals(emptyList<String>(), tokenize(""))
        assertEquals(emptyList<String>(), tokenize("   "))
    }

    @Test
    fun normalizeCollapsesTabs() {
        assertEquals("a b c", normalize("a\tb\tc"))
    }

    @Test
    fun normalizeTrimsEdges() {
        assertEquals("hello world", normalize("  hello   world  "))
    }
}

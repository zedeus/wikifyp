package com.wikifyp.data

import com.sun.net.httpserver.HttpServer
import kotlinx.coroutines.test.runTest
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.fail
import org.junit.Before
import org.junit.Test
import java.io.IOException
import java.net.InetSocketAddress

/** Exercises the HTTP client against a local in-process server, offline. */
class HttpFeedRepositoryTest {
    private lateinit var server: HttpServer
    private lateinit var repo: HttpFeedRepository
    private var lastFeedQuery: String? = null

    @Before
    fun start() {
        server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0)
        server.createContext("/v1/feed") { exchange ->
            lastFeedQuery = exchange.requestURI.query
            val body =
                """
                {"articles":[{"id":12,"title":"Chemistry","source":"simplewiki",
                  "excerpt":"Chemistry is a science.","sentences":40,"words":1200,
                  "attribution":"Text from the Simple English Wikipedia article \"Chemistry\"",
                  "description":"Scientific study of matter",
                  "image":{"url":"https://example.test/chem.jpg","width":960,"height":640},
                  "unknown_field":true}]}
                """.trimIndent().toByteArray()
            exchange.sendResponseHeaders(200, body.size.toLong())
            exchange.responseBody.use { it.write(body) }
        }
        server.createContext("/v1/articles/12") { exchange ->
            val body =
                """
                {"id":12,"title":"Chemistry",
                 "attribution":"Text from the Simple English Wikipedia article \"Chemistry\"",
                 "sentences":[{"text":"Chemistry is a science."},
                              {"text":"History","heading_level":2}]}
                """.trimIndent().toByteArray()
            exchange.sendResponseHeaders(200, body.size.toLong())
            exchange.responseBody.use { it.write(body) }
        }
        server.createContext("/v1/articles/404") { exchange ->
            exchange.sendResponseHeaders(404, -1)
            exchange.close()
        }
        server.start()
        repo = HttpFeedRepository("http://127.0.0.1:${server.address.port}", readerId = "reader 1")
    }

    @After
    fun stop() {
        server.stop(0)
    }

    @Test
    fun feedParsesAndSendsTheReaderID() =
        runTest {
            val result = repo.feed()
            assertEquals("reader=reader+1", lastFeedQuery)
            assertEquals(1, result.cards.size)
            val card = result.cards[0]
            assertEquals(12, card.id)
            assertEquals("Chemistry", card.title)
            assertEquals(1200, card.words)
            assertEquals("Scientific study of matter", card.description)
            assertEquals(ArticleImage("https://example.test/chem.jpg", 960, 640), card.image)
        }

    @Test
    fun articleWithoutImageOrDescriptionDecodesToDefaults() =
        runTest {
            val article = repo.article(12)
            assertEquals("", article.description)
            assertEquals(null, article.image)
        }

    @Test
    fun articleParsesSentencesWithHeadingLevels() =
        runTest {
            val article = repo.article(12)
            assertEquals("Chemistry", article.title)
            assertEquals(2, article.sentences.size)
            assertEquals(0, article.sentences[0].headingLevel)
            assertEquals(2, article.sentences[1].headingLevel)
        }

    @Test
    fun httpErrorsBecomeExceptions() =
        runTest {
            try {
                repo.article(404)
                fail("a 404 did not fail the fetch")
            } catch (expected: IOException) {
            }
        }
}

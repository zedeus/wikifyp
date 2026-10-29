package com.wikifyp.ui.reader

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.test.click
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.performTouchInput
import com.wikifyp.reader.SentenceTokens
import com.wikifyp.theme.WikifypTheme
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.GraphicsMode

/** Word hit-testing in the browse overlay: a tap on a word reports its
 *  flat index; a tap past the text reports background. Needs NATIVE
 *  graphics so text is measured with real glyph widths. */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
class BrowseOverlayTest {
    @get:Rule val compose = createComposeRule()

    private val blocks =
        browseBlocks(
            listOf(
                SentenceTokens(listOf("Alpha", "beta", "gamma.")),
            ),
        )

    @Test
    fun tapOnFirstWordReportsItsFlatIndex() {
        val tapped = mutableListOf<Int>()
        var background = 0
        compose.setContent {
            WikifypTheme {
                BrowseOverlay(
                    blocks = blocks,
                    currentFlat = 0,
                    palette = readerPalette(ReaderTheme.LIGHT, systemDark = false),
                    settings = ReaderSettings(),
                    faded = false,
                    onTapWord = { tapped.add(it) },
                    onTapBackground = { background++ },
                    onHold = {},
                    onRelease = {},
                )
            }
        }

        // The block text starts at the content padding; tap just inside
        // the first word.
        val textNode = compose.onAllNodesWithText("Alpha beta gamma.").onFirst()
        textNode.performTouchInput { click(Offset(10f, height / 2f)) }
        compose.waitForIdle()

        assertEquals("tapped=$tapped background=$background", listOf(0), tapped)
        assertEquals(0, background)
    }

    @Test
    fun tapPastTheTextReportsBackground() {
        var background = 0
        val tapped = mutableListOf<Int>()
        compose.setContent {
            WikifypTheme {
                BrowseOverlay(
                    blocks = blocks,
                    currentFlat = 0,
                    palette = readerPalette(ReaderTheme.LIGHT, systemDark = false),
                    settings = ReaderSettings(),
                    faded = false,
                    onTapWord = { tapped.add(it) },
                    onTapBackground = { background++ },
                    onHold = {},
                    onRelease = {},
                )
            }
        }

        // Tap at the far right of the single-line text: past the last
        // glyph, so no token owns the position.
        val textNode = compose.onAllNodesWithText("Alpha beta gamma.").onFirst()
        textNode.performTouchInput { click(Offset(width - 2f, height / 2f)) }
        compose.waitForIdle()

        assertTrue(tapped.isEmpty())
        assertEquals(1, background)
    }
}

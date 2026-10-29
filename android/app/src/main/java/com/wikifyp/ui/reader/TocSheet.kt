package com.wikifyp.ui.reader

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import com.wikifyp.theme.Spacing

/**
 * Table of contents: the article's sections, the current one highlighted.
 * Tapping a section jumps the reader there.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun TocSheet(
    sections: List<Section>,
    currentFlat: Int,
    palette: ReaderPalette,
    onJump: (Section) -> Unit,
    onDismiss: () -> Unit,
) {
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        containerColor = palette.surface,
        contentColor = palette.word,
        modifier = Modifier.testTag("tocSheet"),
    ) {
        TocSheetContent(
            sections = sections,
            currentFlat = currentFlat,
            palette = palette,
            onJump = onJump,
        )
    }
}

/**
 * The sheet's content, separate from the sheet container so screenshot
 * tests can render it directly (modal windows escape composition-root
 * captures).
 */
@Composable
internal fun TocSheetContent(
    sections: List<Section>,
    currentFlat: Int,
    palette: ReaderPalette,
    onJump: (Section) -> Unit,
    modifier: Modifier = Modifier,
) {
    val current = currentSection(sections, currentFlat)
    LazyColumn(
        modifier =
            modifier
                .fillMaxWidth()
                .padding(horizontal = Spacing.lg)
                .padding(bottom = Spacing.xl),
    ) {
        item {
            Text(
                text = "Contents",
                style = MaterialTheme.typography.titleLarge,
                color = palette.word,
            )
            Spacer(Modifier.height(Spacing.md))
        }
        items(sections) { section ->
            val selected = section == current
            Text(
                text = section.title,
                style = MaterialTheme.typography.bodyLarge,
                fontWeight = if (selected) FontWeight.Medium else FontWeight.Normal,
                color = if (selected) palette.accent else palette.word.copy(alpha = 0.9f),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier =
                    Modifier
                        .fillMaxWidth()
                        .clip(MaterialTheme.shapes.small)
                        .background(
                            if (selected) palette.accent.copy(alpha = 0.12f) else Color.Transparent,
                        ).clickable { onJump(section) }
                        .padding(horizontal = Spacing.sm, vertical = Spacing.sm)
                        .testTag("tocEntry"),
            )
        }
    }
}

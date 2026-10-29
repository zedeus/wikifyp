package com.wikifyp.ui.reader

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import com.wikifyp.theme.Spacing

/**
 * A small floating caption on the reader surface: the browse hint and the
 * save confirmation. One composable keeps the two visually locked.
 */
@Composable
internal fun ReaderPill(
    text: String,
    palette: ReaderPalette,
    modifier: Modifier = Modifier,
) {
    Text(
        text = text,
        style = MaterialTheme.typography.labelMedium,
        color = palette.word,
        modifier =
            modifier
                .background(
                    palette.surface.copy(alpha = 0.94f),
                    RoundedCornerShape(50),
                ).padding(horizontal = Spacing.md, vertical = Spacing.sm),
    )
}

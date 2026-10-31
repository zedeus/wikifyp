package com.wikifyp.ui.history

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import com.wikifyp.data.RecommendReason
import com.wikifyp.theme.Spacing

/**
 * A bottom sheet explaining why an article was recommended.
 * Shows the recommendation kind, seed article, and shared categories.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ReasonSheet(
    title: String,
    reason: RecommendReason,
    onDismiss: () -> Unit,
) {
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true),
        containerColor = MaterialTheme.colorScheme.surface,
    ) {
        Column(
            modifier =
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = Spacing.lg, vertical = Spacing.md)
                    .padding(bottom = Spacing.xl),
        ) {
            Text(
                "Why this article?",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
            )
            Spacer(Modifier.height(Spacing.md))
            Text(
                reason.displayText,
                style = MaterialTheme.typography.bodyLarge,
            )
            Spacer(Modifier.height(Spacing.sm))
            // Detailed explanation.
            val explanation =
                when (reason.kind) {
                    "momentum" -> {
                        "This article continues from your most recent reading. " +
                            "The feed prioritizes content related to what you just finished."
                    }

                    "relevance" -> {
                        "This article matches your reading interests. " +
                            "It was picked based on categories and links shared with articles you've read."
                    }

                    "exploration" -> {
                        "This is a discovery pick — something outside your usual interests. " +
                            "The feed mixes in new topics to help you find things you didn't know you'd enjoy."
                    }

                    else -> {
                        "This article was recommended based on your reading history."
                    }
                }
            Text(
                explanation,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            // Shared categories.
            if (reason.seedCategories.isNotEmpty()) {
                Spacer(Modifier.height(Spacing.md))
                Text(
                    "Topics in common",
                    style = MaterialTheme.typography.labelMedium,
                    fontWeight = FontWeight.SemiBold,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Spacer(Modifier.height(Spacing.xs))
                Text(
                    reason.seedCategories.joinToString(" · "),
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.primary,
                )
            }
        }
    }
}

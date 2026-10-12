package com.wikifyp.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import com.wikifyp.data.FixtureArticle
import com.wikifyp.theme.Spacing
import com.wikifyp.ui.components.ContentCard

/**
 * Placeholder article list showing bundled fixture articles. Each card
 * shows the title and sentence count; tapping opens the reader.
 * Replaced by the real feed when #6 lands.
 */
@Composable
fun ArticleListScreen(
    articles: List<FixtureArticle>,
    onArticleClick: (FixtureArticle) -> Unit,
    modifier: Modifier = Modifier,
) {
    Surface(modifier = modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            modifier =
                Modifier
                    .safeDrawingPadding()
                    .fillMaxSize(),
        ) {
            Text(
                text = "wikifyp",
                style = MaterialTheme.typography.displayLarge,
                color = MaterialTheme.colorScheme.onBackground,
                modifier = Modifier.padding(horizontal = Spacing.lg, vertical = Spacing.md),
            )
            LazyColumn(
                modifier = Modifier.fillMaxSize().padding(horizontal = Spacing.lg),
                verticalArrangement = Arrangement.spacedBy(Spacing.sm),
            ) {
                items(articles, key = { it.id }) { article ->
                    ContentCard(
                        modifier =
                            Modifier
                                .fillMaxWidth()
                                .clickable { onArticleClick(article) },
                    ) {
                        Text(
                            text = article.title,
                            style = MaterialTheme.typography.titleLarge,
                            color = MaterialTheme.colorScheme.onSurface,
                        )
                        Spacer(Modifier.height(Spacing.xs))
                        Text(
                            text = "${article.sentences.size} sentences",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
            }
        }
    }
}

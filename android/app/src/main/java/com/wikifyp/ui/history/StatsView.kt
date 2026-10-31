package com.wikifyp.ui.history

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.PrimaryTabRow
import androidx.compose.material3.Surface
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.wikifyp.data.HistoryRepository
import com.wikifyp.data.ReaderInterests
import com.wikifyp.data.ReadingStats
import com.wikifyp.theme.Spacing
import kotlinx.coroutines.CancellationException

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun StatsScreen(
    repository: HistoryRepository,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
) {
    var selectedTab by remember { mutableIntStateOf(0) }
    var stats by remember { mutableStateOf<ReadingStats?>(null) }
    var interests by remember { mutableStateOf<ReaderInterests?>(null) }
    var isLoading by remember { mutableStateOf(true) }

    LaunchedEffect(Unit) {
        try {
            isLoading = true
            stats = repository.stats()
            interests = repository.interests()
        } catch (e: CancellationException) {
            throw e
        } catch (_: Exception) {
        }
        isLoading = false
    }

    Surface(modifier = modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(modifier = Modifier.safeDrawingPadding()) {
            TopAppBar(
                title = { Text("Reading Insights") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
                colors =
                    TopAppBarDefaults.topAppBarColors(
                        containerColor = MaterialTheme.colorScheme.background,
                    ),
            )
            PrimaryTabRow(
                selectedTabIndex = selectedTab,
                containerColor = MaterialTheme.colorScheme.background,
            ) {
                Tab(
                    selected = selectedTab == 0,
                    onClick = { selectedTab = 0 },
                    text = { Text("Stats") },
                )
                Tab(
                    selected = selectedTab == 1,
                    onClick = { selectedTab = 1 },
                    text = { Text("Interests") },
                )
            }

            if (isLoading) {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator()
                }
            } else {
                when (selectedTab) {
                    0 -> stats?.let { StatsContent(it) }
                    1 -> interests?.let { InterestsContent(it) }
                }
            }
        }
    }
}

@Composable
private fun StatsContent(stats: ReadingStats) {
    Column(
        modifier =
            Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(Spacing.md),
        verticalArrangement = Arrangement.spacedBy(Spacing.md),
    ) {
        // Summary cards.
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(Spacing.sm),
        ) {
            StatCard("Articles", stats.totalArticles.toString(), Modifier.weight(1f))
            StatCard("Finished", stats.articlesFinished.toString(), Modifier.weight(1f))
        }
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(Spacing.sm),
        ) {
            StatCard("Words read", formatWords(stats.totalWordsRead), Modifier.weight(1f))
            StatCard("Time reading", formatTime(stats.totalReadingTimeS), Modifier.weight(1f))
        }

        // Streaks.
        if (stats.currentStreak > 0 || stats.longestStreak > 0) {
            SectionTitle("Streaks")
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(Spacing.sm),
            ) {
                StatCard(
                    "Current",
                    "${stats.currentStreak} day${if (stats.currentStreak != 1) "s" else ""}",
                    Modifier.weight(1f),
                )
                StatCard(
                    "Longest",
                    "${stats.longestStreak} day${if (stats.longestStreak != 1) "s" else ""}",
                    Modifier.weight(1f),
                )
            }
        }

        // WPM over time.
        if (stats.wpmOverTime.isNotEmpty()) {
            SectionTitle("Reading speed")
            WpmChart(stats.wpmOverTime)
        }

        // Top categories.
        if (stats.topCategories.isNotEmpty()) {
            SectionTitle("Top topics")
            stats.topCategories.take(10).forEach { cat ->
                TopicRow(
                    category = cat.category,
                    articles = cat.articles,
                    avgCompletion = cat.avgCompletion,
                )
            }
        }
    }
}

@Composable
private fun InterestsContent(interests: ReaderInterests) {
    Column(
        modifier =
            Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(Spacing.md),
        verticalArrangement = Arrangement.spacedBy(Spacing.md),
    ) {
        // Explore share indicator.
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(Spacing.sm),
        ) {
            StatCard(
                "Articles read",
                interests.totalArticles.toString(),
                Modifier.weight(1f),
            )
            StatCard(
                "Exploration",
                "${(interests.exploreShare * 100).toInt()}%",
                Modifier.weight(1f),
            )
        }

        if (interests.clusters.isNotEmpty()) {
            Text(
                "Your feed is shaped by these interests:",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }

        // Interest clusters.
        interests.clusters.forEachIndexed { index, cluster ->
            ClusterCard(cluster, index + 1)
        }

        // Top weighted categories.
        if (interests.topCategories.isNotEmpty()) {
            SectionTitle("Category weights")
            Text(
                "How much each topic influences your recommendations",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(Modifier.height(Spacing.xs))
            val maxWeight = interests.topCategories.maxOfOrNull { it.weight } ?: 1f
            interests.topCategories.take(10).forEach { cat ->
                CategoryWeightBar(cat.category, cat.weight, maxWeight)
            }
        }
    }
}

@Composable
private fun StatCard(
    label: String,
    value: String,
    modifier: Modifier = Modifier,
) {
    Surface(
        modifier = modifier,
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surfaceContainerLow,
    ) {
        Column(modifier = Modifier.padding(Spacing.md)) {
            Text(
                value,
                style = MaterialTheme.typography.headlineSmall,
                fontWeight = FontWeight.Bold,
            )
            Text(
                label,
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
private fun SectionTitle(title: String) {
    Text(
        title,
        style = MaterialTheme.typography.titleMedium,
        fontWeight = FontWeight.SemiBold,
        modifier = Modifier.padding(top = Spacing.sm),
    )
}

@Composable
private fun WpmChart(buckets: List<com.wikifyp.data.WpmBucket>) {
    // Simple bar chart of WPM over time.
    val maxWpm = buckets.maxOfOrNull { it.avgWpm } ?: 1f
    Surface(
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surfaceContainerLow,
    ) {
        Column(modifier = Modifier.padding(Spacing.md)) {
            Row(
                modifier = Modifier.fillMaxWidth().height(100.dp),
                horizontalArrangement = Arrangement.spacedBy(2.dp),
                verticalAlignment = Alignment.Bottom,
            ) {
                buckets.takeLast(12).forEach { bucket ->
                    val height = (bucket.avgWpm / maxWpm * 80).dp
                    Box(
                        modifier =
                            Modifier
                                .weight(1f)
                                .height(height)
                                .clip(RoundedCornerShape(topStart = 4.dp, topEnd = 4.dp))
                                .background(MaterialTheme.colorScheme.primary.copy(alpha = 0.7f)),
                    )
                }
            }
            Spacer(Modifier.height(Spacing.xs))
            Text(
                "Average: ${buckets.map { it.avgWpm }.average().toInt()} WPM",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
private fun TopicRow(
    category: String,
    articles: Int,
    avgCompletion: Float,
) {
    Row(
        modifier =
            Modifier
                .fillMaxWidth()
                .padding(vertical = Spacing.xs),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            category,
            modifier = Modifier.weight(1f),
            style = MaterialTheme.typography.bodyMedium,
            maxLines = 1,
        )
        Text(
            "$articles articles",
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun ClusterCard(
    cluster: com.wikifyp.data.InterestCluster,
    index: Int,
) {
    Surface(
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surfaceContainerLow,
    ) {
        Column(modifier = Modifier.padding(Spacing.md)) {
            // Cluster label from categories.
            val label = cluster.categories.take(3).joinToString(" · ")
            Text(
                label.ifEmpty { "Interest #$index" },
                style = MaterialTheme.typography.titleSmall,
                fontWeight = FontWeight.SemiBold,
            )
            Text(
                "${cluster.articleCount} article${if (cluster.articleCount != 1) "s" else ""}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(Modifier.height(Spacing.sm))
            // Top articles in this cluster.
            cluster.articles.take(3).forEach { article ->
                Row(
                    modifier =
                        Modifier
                            .fillMaxWidth()
                            .padding(vertical = 2.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        article.title,
                        modifier = Modifier.weight(1f),
                        style = MaterialTheme.typography.bodySmall,
                        maxLines = 1,
                    )
                    if (article.completion > 0f) {
                        LinearProgressIndicator(
                            progress = { article.completion },
                            modifier = Modifier.size(width = 40.dp, height = 3.dp),
                            color = MaterialTheme.colorScheme.primary,
                            trackColor = MaterialTheme.colorScheme.surfaceVariant,
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun CategoryWeightBar(
    category: String,
    weight: Float,
    maxWeight: Float,
) {
    Column(modifier = Modifier.padding(vertical = 2.dp)) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Text(
                category,
                style = MaterialTheme.typography.bodySmall,
                maxLines = 1,
                modifier = Modifier.weight(1f),
            )
            Text(
                "%.1f".format(weight),
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        LinearProgressIndicator(
            progress = { if (maxWeight > 0) weight / maxWeight else 0f },
            modifier = Modifier.fillMaxWidth().height(4.dp).clip(RoundedCornerShape(2.dp)),
            color = MaterialTheme.colorScheme.primary,
            trackColor = MaterialTheme.colorScheme.surfaceVariant,
        )
    }
}

private fun formatWords(n: Long): String =
    when {
        n >= 1_000_000 -> "%.1fM".format(n / 1_000_000.0)
        n >= 1_000 -> "%.1fK".format(n / 1_000.0)
        else -> n.toString()
    }

private fun formatTime(seconds: Long): String {
    val hours = seconds / 3600
    val minutes = (seconds % 3600) / 60
    return when {
        hours > 0 -> "${hours}h ${minutes}m"
        minutes > 0 -> "${minutes}m"
        else -> "${seconds}s"
    }
}

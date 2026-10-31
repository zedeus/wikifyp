package com.wikifyp.ui.history

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.outlined.ViewList
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.outlined.DeleteSweep
import androidx.compose.material.icons.outlined.GridView
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Snackbar
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import com.wikifyp.data.HistoryEntry
import com.wikifyp.data.HistoryPage
import com.wikifyp.data.HistoryRepository
import com.wikifyp.data.PageRef
import com.wikifyp.theme.Spacing
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import java.text.SimpleDateFormat
import java.util.Calendar
import java.util.Locale
import java.util.TimeZone
import java.util.concurrent.TimeUnit

/** Parses an RFC3339 timestamp to a Calendar in the local timezone, or null. */
private fun parseDate(updatedAt: String): Calendar? =
    try {
        val fmt = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'", Locale.US)
        fmt.timeZone = TimeZone.getTimeZone("UTC")
        val date = fmt.parse(updatedAt) ?: return null
        Calendar.getInstance().apply { time = date }
    } catch (_: Exception) {
        null
    }

/** Returns how many calendar days ago a date is relative to today. */
private fun daysAgo(cal: Calendar): Long {
    val today = Calendar.getInstance()
    val todayStart =
        Calendar.getInstance().apply {
            set(Calendar.HOUR_OF_DAY, 0)
            set(Calendar.MINUTE, 0)
            set(Calendar.SECOND, 0)
            set(Calendar.MILLISECOND, 0)
        }
    val dateStart =
        Calendar.getInstance().apply {
            timeInMillis = cal.timeInMillis
            set(Calendar.HOUR_OF_DAY, 0)
            set(Calendar.MINUTE, 0)
            set(Calendar.SECOND, 0)
            set(Calendar.MILLISECOND, 0)
        }
    return TimeUnit.MILLISECONDS.toDays(todayStart.timeInMillis - dateStart.timeInMillis)
}

/** Groups entries by date for section headers. */
private fun dateLabel(updatedAt: String): String {
    val cal = parseDate(updatedAt) ?: return ""
    val days = daysAgo(cal)
    return when {
        days == 0L -> {
            "Today"
        }

        days == 1L -> {
            "Yesterday"
        }

        days < 7 -> {
            SimpleDateFormat("EEEE", Locale.getDefault()).format(cal.time)
        }

        cal.get(Calendar.YEAR) == Calendar.getInstance().get(Calendar.YEAR) -> {
            SimpleDateFormat("MMMM d", Locale.getDefault()).format(cal.time)
        }

        else -> {
            SimpleDateFormat("MMMM d, yyyy", Locale.getDefault()).format(cal.time)
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class, ExperimentalFoundationApi::class)
@Composable
fun HistoryScreen(
    repository: HistoryRepository,
    onEntryClick: (HistoryEntry) -> Unit,
    onBack: () -> Unit,
    undoDelayMs: Long = 5000L,
    modifier: Modifier = Modifier,
) {
    var entries by remember { mutableStateOf<List<HistoryEntry>>(emptyList()) }
    var nextCursor by remember { mutableStateOf("") }
    var isLoading by remember { mutableStateOf(true) }
    var error by remember { mutableStateOf<String?>(null) }
    var searchQuery by remember { mutableStateOf("") }
    var showSearch by remember { mutableStateOf(false) }
    var isCardLayout by remember { mutableStateOf(true) }

    // Selection state.
    val selectedIds = remember { mutableStateListOf<Long>() }
    val isSelecting by remember { derivedStateOf { selectedIds.isNotEmpty() } }

    // Undo state.
    var pendingDelete by remember { mutableStateOf<List<HistoryEntry>?>(null) }
    var showDeleteAllConfirm by remember { mutableStateOf(false) }

    val scope = rememberCoroutineScope()
    val listState = rememberLazyListState()

    // Load history.
    suspend fun loadHistory(
        cursor: String = "",
        search: String = "",
    ) {
        try {
            isLoading = true
            error = null
            val page = repository.history(cursor = cursor, search = search)
            if (cursor.isEmpty()) {
                entries = page.entries
            } else {
                entries = entries + page.entries
            }
            nextCursor = page.nextCursor
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            error = e.message ?: "Failed to load history"
        } finally {
            isLoading = false
        }
    }

    LaunchedEffect(searchQuery) {
        if (searchQuery.isNotEmpty()) {
            delay(300) // debounce
        }
        loadHistory(search = searchQuery)
    }

    // Infinite scroll.
    val shouldLoadMore by remember {
        derivedStateOf {
            val lastVisible =
                listState.layoutInfo.visibleItemsInfo
                    .lastOrNull()
                    ?.index ?: 0
            lastVisible >= entries.size - 5 && nextCursor.isNotEmpty() && !isLoading
        }
    }
    LaunchedEffect(shouldLoadMore) {
        if (shouldLoadMore) {
            loadHistory(cursor = nextCursor, search = searchQuery)
        }
    }

    fun toggleSelect(entry: HistoryEntry) {
        if (selectedIds.contains(entry.pageId)) {
            selectedIds.remove(entry.pageId)
        } else {
            selectedIds.add(entry.pageId)
        }
    }

    fun executeDelete(toDelete: List<HistoryEntry>) {
        scope.launch {
            try {
                val refs = toDelete.map { PageRef(it.source, it.pageId) }
                repository.deletePages(refs)
            } catch (_: Exception) {
                // Deletion failed; entries are already removed from the UI
                // as the undo window passed. A future sync will reconcile.
            }
        }
    }

    fun deleteSelected() {
        val toDelete = entries.filter { selectedIds.contains(it.pageId) }
        if (toDelete.isEmpty()) return
        pendingDelete = toDelete
        entries = entries.filter { !selectedIds.contains(it.pageId) }
        selectedIds.clear()
        // Schedule the actual server delete after undo window.
        scope.launch {
            delay(undoDelayMs)
            if (pendingDelete == toDelete) {
                pendingDelete = null
                executeDelete(toDelete)
            }
        }
    }

    fun undoDelete() {
        val restored = pendingDelete ?: return
        pendingDelete = null
        // Merge back and re-sort by updatedAt.
        entries = (entries + restored).sortedByDescending { it.updatedAt }
    }

    fun deleteAll() {
        val all = entries.toList()
        pendingDelete = all
        entries = emptyList()
        scope.launch {
            delay(undoDelayMs)
            if (pendingDelete == all) {
                pendingDelete = null
                try {
                    repository.deleteAll()
                } catch (_: Exception) {
                }
            }
        }
    }

    // Group entries by date.
    val grouped by remember(entries) {
        derivedStateOf {
            entries
                .groupBy { dateLabel(it.updatedAt) }
                .toList()
        }
    }

    Surface(
        modifier = modifier.fillMaxSize(),
        color = MaterialTheme.colorScheme.background,
    ) {
        Column(modifier = Modifier.safeDrawingPadding()) {
            // Top bar.
            if (isSelecting) {
                SelectionBar(
                    count = selectedIds.size,
                    onClearSelection = { selectedIds.clear() },
                    onDelete = { deleteSelected() },
                    onDeleteAll = { showDeleteAllConfirm = true },
                )
            } else {
                TopAppBar(
                    title = {
                        if (showSearch) {
                            TextField(
                                value = searchQuery,
                                onValueChange = { searchQuery = it },
                                placeholder = { Text("Search history…") },
                                singleLine = true,
                                colors =
                                    TextFieldDefaults.colors(
                                        focusedContainerColor = Color.Transparent,
                                        unfocusedContainerColor = Color.Transparent,
                                        focusedIndicatorColor = Color.Transparent,
                                        unfocusedIndicatorColor = Color.Transparent,
                                    ),
                                modifier = Modifier.fillMaxWidth(),
                            )
                        } else {
                            Text("History")
                        }
                    },
                    navigationIcon = {
                        IconButton(onClick = onBack) {
                            Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                        }
                    },
                    actions = {
                        IconButton(onClick = {
                            showSearch = !showSearch
                            if (!showSearch) searchQuery = ""
                        }) {
                            Icon(
                                if (showSearch) Icons.Filled.Close else Icons.Filled.Search,
                                contentDescription = if (showSearch) "Close search" else "Search",
                            )
                        }
                        IconButton(onClick = { isCardLayout = !isCardLayout }) {
                            Icon(
                                if (isCardLayout) Icons.AutoMirrored.Outlined.ViewList else Icons.Outlined.GridView,
                                contentDescription = if (isCardLayout) "List view" else "Card view",
                            )
                        }
                    },
                    colors =
                        TopAppBarDefaults.topAppBarColors(
                            containerColor = MaterialTheme.colorScheme.background,
                        ),
                )
            }

            // Content.
            Box(modifier = Modifier.weight(1f)) {
                when {
                    isLoading && entries.isEmpty() -> {
                        CircularProgressIndicator(
                            modifier = Modifier.align(Alignment.Center),
                            color = MaterialTheme.colorScheme.primary,
                        )
                    }

                    error != null && entries.isEmpty() -> {
                        Column(
                            modifier = Modifier.align(Alignment.Center).padding(Spacing.lg),
                            horizontalAlignment = Alignment.CenterHorizontally,
                        ) {
                            Text(
                                error ?: "Something went wrong",
                                style = MaterialTheme.typography.bodyLarge,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                    }

                    entries.isEmpty() && !isLoading -> {
                        Column(
                            modifier = Modifier.align(Alignment.Center).padding(Spacing.lg),
                            horizontalAlignment = Alignment.CenterHorizontally,
                        ) {
                            Text(
                                if (searchQuery.isNotEmpty()) {
                                    "No articles match \"$searchQuery\""
                                } else {
                                    "No reading history yet"
                                },
                                style = MaterialTheme.typography.bodyLarge,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                            if (searchQuery.isEmpty()) {
                                Spacer(Modifier.height(Spacing.sm))
                                Text(
                                    "Articles you read will appear here",
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f),
                                )
                            }
                        }
                    }

                    else -> {
                        LazyColumn(
                            state = listState,
                            contentPadding =
                                PaddingValues(
                                    start = Spacing.md,
                                    end = Spacing.md,
                                    bottom = Spacing.lg,
                                ),
                            modifier = Modifier.fillMaxSize().testTag("historyList"),
                        ) {
                            grouped.forEach { (dateHeader, group) ->
                                // Date separator.
                                stickyHeader(key = "header_$dateHeader") {
                                    DateSeparator(dateHeader)
                                }
                                items(
                                    items = group,
                                    key = { it.pageId },
                                ) { entry ->
                                    val selected = selectedIds.contains(entry.pageId)
                                    if (isCardLayout) {
                                        HistoryCard(
                                            entry = entry,
                                            selected = selected,
                                            isSelecting = isSelecting,
                                            onClick = {
                                                if (isSelecting) {
                                                    toggleSelect(entry)
                                                } else {
                                                    onEntryClick(entry)
                                                }
                                            },
                                            onLongClick = { toggleSelect(entry) },
                                        )
                                    } else {
                                        HistoryListItem(
                                            entry = entry,
                                            selected = selected,
                                            isSelecting = isSelecting,
                                            onClick = {
                                                if (isSelecting) {
                                                    toggleSelect(entry)
                                                } else {
                                                    onEntryClick(entry)
                                                }
                                            },
                                            onLongClick = { toggleSelect(entry) },
                                        )
                                    }
                                }
                            }
                            // Loading indicator at the bottom.
                            if (isLoading && entries.isNotEmpty()) {
                                item {
                                    Box(
                                        modifier = Modifier.fillMaxWidth().padding(Spacing.md),
                                        contentAlignment = Alignment.Center,
                                    ) {
                                        CircularProgressIndicator(
                                            modifier = Modifier.size(24.dp),
                                            strokeWidth = 2.dp,
                                        )
                                    }
                                }
                            }
                        }
                    }
                }

                // Undo snackbar.
                if (pendingDelete != null) {
                    val count = pendingDelete?.size ?: 0
                    Snackbar(
                        modifier = Modifier.align(Alignment.BottomCenter).padding(Spacing.md),
                        action = {
                            TextButton(onClick = { undoDelete() }) {
                                Text("Undo", color = MaterialTheme.colorScheme.inversePrimary)
                            }
                        },
                    ) {
                        Text(
                            if (count == 1) {
                                "Article removed"
                            } else {
                                "$count articles removed"
                            },
                        )
                    }
                }
            }
        }
    }

    // Delete-all confirmation dialog.
    if (showDeleteAllConfirm) {
        AlertDialog(
            onDismissRequest = { showDeleteAllConfirm = false },
            title = { Text("Delete all history?") },
            text = {
                Text(
                    "This will permanently remove all reading history, events and engagement data. This cannot be undone after the undo window.",
                )
            },
            confirmButton = {
                TextButton(onClick = {
                    showDeleteAllConfirm = false
                    selectedIds.clear()
                    deleteAll()
                }) {
                    Text("Delete all", color = MaterialTheme.colorScheme.error)
                }
            },
            dismissButton = {
                TextButton(onClick = { showDeleteAllConfirm = false }) {
                    Text("Cancel")
                }
            },
        )
    }
}

@Composable
private fun DateSeparator(label: String) {
    Box(
        modifier =
            Modifier
                .fillMaxWidth()
                .background(MaterialTheme.colorScheme.background)
                .padding(vertical = Spacing.sm),
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            fontWeight = FontWeight.SemiBold,
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun SelectionBar(
    count: Int,
    onClearSelection: () -> Unit,
    onDelete: () -> Unit,
    onDeleteAll: () -> Unit,
) {
    TopAppBar(
        title = { Text("$count selected") },
        navigationIcon = {
            IconButton(onClick = onClearSelection) {
                Icon(Icons.Filled.Close, contentDescription = "Clear selection")
            }
        },
        actions = {
            IconButton(onClick = onDelete) {
                Icon(Icons.Filled.Delete, contentDescription = "Delete selected")
            }
            IconButton(onClick = onDeleteAll) {
                Icon(Icons.Outlined.DeleteSweep, contentDescription = "Delete all")
            }
        },
        colors =
            TopAppBarDefaults.topAppBarColors(
                containerColor = MaterialTheme.colorScheme.primaryContainer,
                titleContentColor = MaterialTheme.colorScheme.onPrimaryContainer,
                navigationIconContentColor = MaterialTheme.colorScheme.onPrimaryContainer,
                actionIconContentColor = MaterialTheme.colorScheme.onPrimaryContainer,
            ),
    )
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun HistoryCard(
    entry: HistoryEntry,
    selected: Boolean,
    isSelecting: Boolean,
    onClick: () -> Unit,
    onLongClick: () -> Unit,
) {
    val shape = RoundedCornerShape(12.dp)
    val containerColor =
        if (selected) {
            MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.3f)
        } else {
            MaterialTheme.colorScheme.surfaceContainerLow
        }
    Surface(
        modifier =
            Modifier
                .fillMaxWidth()
                .padding(vertical = Spacing.xs)
                .clip(shape)
                .combinedClickable(
                    onClick = onClick,
                    onLongClick = onLongClick,
                ),
        shape = shape,
        color = containerColor,
    ) {
        Row(modifier = Modifier.padding(Spacing.sm)) {
            // Thumbnail.
            val img = entry.image
            if (img != null) {
                AsyncImage(
                    model = img.url,
                    contentDescription = null,
                    contentScale = ContentScale.Crop,
                    modifier =
                        Modifier
                            .size(80.dp)
                            .clip(RoundedCornerShape(8.dp)),
                )
                Spacer(Modifier.width(Spacing.sm))
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    entry.title,
                    style = MaterialTheme.typography.titleSmall,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                if (entry.description.isNotEmpty()) {
                    Text(
                        entry.description,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                Spacer(Modifier.height(Spacing.xs))
                Row(
                    horizontalArrangement = Arrangement.spacedBy(Spacing.sm),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    StatusChip(entry)
                    if (entry.completion > 0f && entry.completion < 1f) {
                        LinearProgressIndicator(
                            progress = { entry.completion },
                            modifier = Modifier.weight(1f).height(3.dp),
                            color = MaterialTheme.colorScheme.primary,
                            trackColor = MaterialTheme.colorScheme.surfaceVariant,
                        )
                    }
                }
            }
        }
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun HistoryListItem(
    entry: HistoryEntry,
    selected: Boolean,
    isSelecting: Boolean,
    onClick: () -> Unit,
    onLongClick: () -> Unit,
) {
    val bgColor =
        if (selected) {
            MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.3f)
        } else {
            Color.Transparent
        }
    Row(
        modifier =
            Modifier
                .fillMaxWidth()
                .background(bgColor, RoundedCornerShape(8.dp))
                .combinedClickable(
                    onClick = onClick,
                    onLongClick = onLongClick,
                ).padding(horizontal = Spacing.xs, vertical = Spacing.sm),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // Title.
        Text(
            entry.title,
            modifier = Modifier.weight(1f),
            style = MaterialTheme.typography.bodyMedium,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        Spacer(Modifier.width(Spacing.sm))
        // Status.
        StatusChip(entry)
        Spacer(Modifier.width(Spacing.sm))
        // Date.
        Text(
            shortDate(entry.updatedAt),
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun StatusChip(entry: HistoryEntry) {
    val label = entry.statusLabel
    if (label.isEmpty()) return
    val (bg, fg) =
        when (entry.state) {
            "finished" -> {
                MaterialTheme.colorScheme.primary.copy(alpha = 0.12f) to
                    MaterialTheme.colorScheme.primary
            }

            "skipped" -> {
                MaterialTheme.colorScheme.errorContainer to
                    MaterialTheme.colorScheme.onErrorContainer
            }

            else -> {
                MaterialTheme.colorScheme.secondaryContainer to
                    MaterialTheme.colorScheme.onSecondaryContainer
            }
        }
    Text(
        label,
        modifier =
            Modifier
                .background(bg, CircleShape)
                .padding(horizontal = 8.dp, vertical = 2.dp),
        style = MaterialTheme.typography.labelSmall,
        color = fg,
    )
}

private fun shortDate(updatedAt: String): String =
    run {
        val cal = parseDate(updatedAt) ?: return@run ""
        val days = daysAgo(cal)
        when {
            days == 0L -> "Today"
            days == 1L -> "Yesterday"
            days < 7 -> "${days}d ago"
            else -> SimpleDateFormat("MMM d", Locale.getDefault()).format(cal.time)
        }
    }

package com.wikifyp

import android.content.Context
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.core.content.edit
import androidx.lifecycle.lifecycleScope
import com.wikifyp.data.CachedFeedRepository
import com.wikifyp.data.EventQueue
import com.wikifyp.data.EventUploader
import com.wikifyp.data.FeedCache
import com.wikifyp.data.FeedRepository
import com.wikifyp.data.FixtureFeedRepository
import com.wikifyp.data.HttpFeedRepository
import com.wikifyp.reader.EventListener
import com.wikifyp.theme.WikifypTheme
import com.wikifyp.ui.WikifypApp
import com.wikifyp.ui.reader.HandlerClock
import kotlinx.coroutines.launch
import java.io.File
import java.util.UUID

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val repository = createRepository(this)
        val clock = HandlerClock()
        val queue = EventQueue(this)
        val eventListener = EventListener { event -> queue.enqueue(event) }

        // Upload events in the background when a server is configured.
        if (BuildConfig.SERVER_URL.isNotEmpty() && !underInstrumentation()) {
            val uploader = EventUploader(queue, BuildConfig.SERVER_URL, readerId(this))
            lifecycleScope.launch { uploader.run() }
        }

        enableEdgeToEdge()
        setContent {
            WikifypTheme {
                val prefs = getSharedPreferences("wikifyp", Context.MODE_PRIVATE)
                WikifypApp(
                    repository = repository,
                    clock = clock,
                    eventListener = eventListener,
                    initialWpm = prefs.getInt("wpm", 250),
                    onWpmChange = { prefs.edit { putInt("wpm", it) } },
                )
            }
        }
    }
}

/**
 * The real repository: the server behind an offline cache when one is
 * configured (debug builds default to the dev server on the emulator's
 * host; `-PserverUrl=…` overrides), otherwise the bundled fixture
 * articles. Instrumented tests always get the fixtures: they assert on
 * fixture content and run on emulators with no server.
 */
private fun createRepository(context: Context): FeedRepository {
    if (BuildConfig.SERVER_URL.isEmpty() || underInstrumentation()) {
        return FixtureFeedRepository(context)
    }
    return CachedFeedRepository(
        remote = HttpFeedRepository(BuildConfig.SERVER_URL, readerId(context)),
        cache = FeedCache(File(context.filesDir, "feed-cache")),
    )
}

/** Whether an androidTest instrumentation is driving the app. */
private fun underInstrumentation(): Boolean =
    try {
        Class.forName("androidx.test.platform.app.InstrumentationRegistry")
        true
    } catch (_: ClassNotFoundException) {
        false
    }

/** This install's pseudonymous reader ID: random once, then stable. */
private fun readerId(context: Context): String {
    val prefs = context.getSharedPreferences("wikifyp", Context.MODE_PRIVATE)
    prefs.getString("reader_id", null)?.let { return it }
    val id = UUID.randomUUID().toString()
    prefs.edit { putString("reader_id", id) }
    return id
}

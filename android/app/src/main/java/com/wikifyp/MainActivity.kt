package com.wikifyp

import android.content.Context
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.runtime.remember
import androidx.core.content.edit
import androidx.lifecycle.lifecycleScope
import coil3.ImageLoader
import coil3.SingletonImageLoader
import coil3.network.okhttp.OkHttpNetworkFetcherFactory
import com.wikifyp.data.CachedFeedRepository
import com.wikifyp.data.EventQueue
import com.wikifyp.data.EventUploader
import com.wikifyp.data.FeedCache
import com.wikifyp.data.FeedRepository
import com.wikifyp.data.FixtureFeedRepository
import com.wikifyp.data.HistoryRepository
import com.wikifyp.data.HttpFeedRepository
import com.wikifyp.data.HttpHistoryRepository
import com.wikifyp.data.SavedStore
import com.wikifyp.reader.EventListener
import com.wikifyp.ui.WikifypApp
import com.wikifyp.ui.reader.HandlerClock
import com.wikifyp.ui.reader.ReaderSettingsStore
import kotlinx.coroutines.launch
import okhttp3.OkHttpClient
import java.io.File
import java.util.UUID

/** How the app introduces itself to Wikimedia's servers (their User-Agent
 *  policy wants a name and a contact; anonymous agents get 403). */
private const val USER_AGENT = "wikifyp/0.1 (https://github.com/zedeus/wikifyp)"

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // Lead images come from Wikimedia's CDN, which rejects anonymous
        // library user agents (HTTP 403, per their User-Agent policy):
        // every image request identifies the app.
        SingletonImageLoader.setSafe { context ->
            ImageLoader
                .Builder(context)
                .components {
                    add(
                        OkHttpNetworkFetcherFactory(
                            callFactory = {
                                OkHttpClient
                                    .Builder()
                                    .addInterceptor { chain ->
                                        chain.proceed(
                                            chain
                                                .request()
                                                .newBuilder()
                                                .header("User-Agent", USER_AGENT)
                                                .build(),
                                        )
                                    }.build()
                            },
                        ),
                    )
                }.build()
        }

        val repository = createRepository(this)
        val historyRepo = createHistoryRepository(this)
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
            // No theme wrapper here: WikifypApp themes itself from the
            // reader-theme setting (one picker for the whole app).
            val prefs = getSharedPreferences("wikifyp", Context.MODE_PRIVATE)
            val settingsStore = remember { ReaderSettingsStore(prefs) }
            val initialSettings = remember { settingsStore.load() }
            val savedStore = remember { SavedStore(prefs) }
            val initialSaved = remember { savedStore.load() }
            WikifypApp(
                repository = repository,
                clock = clock,
                eventListener = eventListener,
                initialReaderSettings = initialSettings,
                onReaderSettingsChange = settingsStore::save,
                initialSaved = initialSaved,
                onSavedChange = savedStore::save,
                historyRepository = historyRepo,
            )
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

/** The history repository: server-backed when configured, null for fixtures. */
private fun createHistoryRepository(context: Context): HistoryRepository? {
    if (BuildConfig.SERVER_URL.isEmpty() || underInstrumentation()) {
        return null
    }
    return HttpHistoryRepository(BuildConfig.SERVER_URL, readerId(context))
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

package com.wikifyp.data

import android.content.ContentValues
import android.content.Context
import android.database.sqlite.SQLiteDatabase
import android.database.sqlite.SQLiteOpenHelper
import com.wikifyp.reader.ReadingEvent
import com.wikifyp.reader.eventJson

/**
 * Durable queue for reading events. Events are written immediately and
 * survive app restarts. The upload worker drains them in FIFO order,
 * removing only after a successful server response.
 *
 * Deduplication: the event's UUID is the primary key, so re-enqueuing
 * the same event (e.g. after a crash between emit and enqueue) is a
 * no-op.
 */
class EventQueue(
    context: Context,
    dbName: String = "event_queue.db",
) {
    private val helper = Helper(context.applicationContext, dbName)

    /** Enqueue one event. Idempotent by event ID. */
    fun enqueue(event: ReadingEvent) {
        val json = eventJson.encodeToString(ReadingEvent.serializer(), event)
        val values =
            ContentValues(3).apply {
                put("id", event.id)
                put("json", json)
                put("created_at", System.currentTimeMillis())
            }
        helper.writableDatabase.insertWithOnConflict(
            "event_queue",
            null,
            values,
            SQLiteDatabase.CONFLICT_IGNORE,
        )
    }

    /**
     * Returns the oldest [limit] events as (id, json) pairs, in FIFO
     * order. Does not remove them.
     */
    fun peek(limit: Int = 50): List<QueuedEvent> {
        val cursor =
            helper.readableDatabase.query(
                "event_queue",
                arrayOf("id", "json"),
                null,
                null,
                null,
                null,
                "created_at ASC",
                limit.toString(),
            )
        return cursor.use {
            val out = mutableListOf<QueuedEvent>()
            while (it.moveToNext()) {
                out +=
                    QueuedEvent(
                        id = it.getString(0),
                        json = it.getString(1),
                    )
            }
            out
        }
    }

    /** Remove events by ID after a successful upload. */
    fun remove(ids: List<String>) {
        if (ids.isEmpty()) return
        val db = helper.writableDatabase
        db.beginTransaction()
        try {
            for (id in ids) {
                db.delete("event_queue", "id = ?", arrayOf(id))
            }
            db.setTransactionSuccessful()
        } finally {
            db.endTransaction()
        }
    }

    /** The number of events waiting to be uploaded. */
    fun count(): Int {
        val cursor = helper.readableDatabase.rawQuery("SELECT count(*) FROM event_queue", null)
        return cursor.use {
            it.moveToFirst()
            it.getInt(0)
        }
    }

    /** Close the underlying database. */
    fun close() {
        helper.close()
    }
}

data class QueuedEvent(
    val id: String,
    val json: String,
)

private class Helper(
    context: Context,
    dbName: String,
) : SQLiteOpenHelper(context, dbName, null, 1) {
    override fun onCreate(db: SQLiteDatabase) {
        db.execSQL(
            """
            CREATE TABLE event_queue (
                id TEXT PRIMARY KEY,
                json TEXT NOT NULL,
                created_at INTEGER NOT NULL
            )
            """,
        )
    }

    override fun onUpgrade(
        db: SQLiteDatabase,
        oldVersion: Int,
        newVersion: Int,
    ) {
        // v1 only; future versions add migration steps here.
    }
}

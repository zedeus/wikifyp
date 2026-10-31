package com.wikifyp.data

/**
 * Test double for [HistoryRepository]: returns whatever the test assigns.
 * Keeps track of delete calls so tests can assert on them.
 */
open class FakeHistoryRepository : HistoryRepository {
    var page: HistoryPage = HistoryPage()
    var readingStats: ReadingStats = ReadingStats()
    var readerInterests: ReaderInterests = ReaderInterests()
    var deletedPages: List<PageRef> = emptyList()
    var deletedAll: Boolean = false

    override suspend fun history(
        cursor: String,
        search: String,
        date: String,
    ): HistoryPage = page

    override suspend fun stats(): ReadingStats = readingStats

    override suspend fun interests(): ReaderInterests = readerInterests

    override suspend fun deletePages(pages: List<PageRef>): DeleteResult {
        deletedPages = pages
        return DeleteResult(pagesDeleted = pages.size)
    }

    override suspend fun deleteAll(): DeleteResult {
        deletedAll = true
        return DeleteResult(pagesDeleted = 10)
    }
}

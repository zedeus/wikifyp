package com.wikifyp

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.wikifyp.data.FixtureArticle
import com.wikifyp.data.listFixtureArticles
import com.wikifyp.data.toSentenceTokens
import com.wikifyp.theme.WikifypTheme
import com.wikifyp.ui.ArticleListScreen
import com.wikifyp.ui.reader.HandlerClock
import com.wikifyp.ui.reader.ReaderScreen

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val articles = listFixtureArticles(this)
        val clock = HandlerClock()

        enableEdgeToEdge()
        setContent {
            WikifypTheme {
                var currentArticle by remember { mutableStateOf<FixtureArticle?>(null) }

                val article = currentArticle
                if (article != null) {
                    val sentences = remember(article) { article.toSentenceTokens() }
                    ReaderScreen(
                        articleTitle = article.title,
                        sentences = sentences,
                        articleId = article.id.toLong(),
                        clock = clock,
                        onBack = { currentArticle = null },
                    )
                } else {
                    ArticleListScreen(
                        articles = articles,
                        onArticleClick = { currentArticle = it },
                    )
                }
            }
        }
    }
}

package com.wikifyp.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import com.wikifyp.theme.Spacing
import com.wikifyp.theme.WikifypTheme
import com.wikifyp.ui.components.ContentCard
import com.wikifyp.ui.components.PrimaryButton
import com.wikifyp.ui.components.QuietButton

/**
 * Stand-in for the feed until it exists: shows the design system doing its
 * job on one screen. Replaced by the real feed when the feed issue lands.
 */
@Composable
fun PlaceholderScreen(
    modifier: Modifier = Modifier,
    onStartReading: () -> Unit = {},
    onAbout: () -> Unit = {},
) {
    Surface(modifier = modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            modifier =
                Modifier
                    .safeDrawingPadding()
                    .padding(horizontal = Spacing.lg)
                    .fillMaxSize(),
            verticalArrangement = Arrangement.Center,
        ) {
            Text(
                text = "wikifyp",
                style = MaterialTheme.typography.displayLarge,
                color = MaterialTheme.colorScheme.onBackground,
            )
            Spacer(Modifier.height(Spacing.sm))
            Text(
                text = "An endless feed of Wikipedia, read one word at a time.",
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(Modifier.height(Spacing.xl))
            ContentCard(modifier = Modifier.fillMaxWidth()) {
                Text(
                    text = "Nothing to read yet",
                    style = MaterialTheme.typography.titleLarge,
                    color = MaterialTheme.colorScheme.onSurface,
                )
                Spacer(Modifier.height(Spacing.sm))
                Text(
                    text = "Your feed will appear here. Hold to play, lift to pause, drag back to reread.",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            Spacer(Modifier.height(Spacing.xl))
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(Spacing.sm),
            ) {
                PrimaryButton(text = "Start reading", onClick = onStartReading)
                QuietButton(text = "About", onClick = onAbout)
            }
        }
    }
}

@Preview(showBackground = true)
@Composable
private fun PlaceholderScreenPreview() {
    WikifypTheme { PlaceholderScreen() }
}

@Preview(showBackground = true)
@Composable
private fun PlaceholderScreenDarkPreview() {
    WikifypTheme(darkTheme = true) { PlaceholderScreen() }
}

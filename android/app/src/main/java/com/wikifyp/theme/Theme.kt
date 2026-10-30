package com.wikifyp.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.runtime.Composable
import androidx.compose.ui.unit.dp

// Soft but not bubbly corners; `large` is the shape of content cards.
val Shapes =
    Shapes(
        extraSmall = RoundedCornerShape(4.dp),
        small = RoundedCornerShape(8.dp),
        medium = RoundedCornerShape(12.dp),
        large = RoundedCornerShape(16.dp),
        extraLarge = RoundedCornerShape(24.dp),
    )

/**
 * App theme: paper in the light, ink at night, one serif voice. No dynamic
 * (Material You) color on purpose — the reading surface should look the same
 * on every device. [black] is the OLED variant for the black reader theme:
 * the canvas goes pure black so the app chrome matches the reading surface.
 */
@Composable
fun WikifypTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    black: Boolean = false,
    content: @Composable () -> Unit,
) {
    MaterialTheme(
        colorScheme =
            when {
                black -> BlackColors
                darkTheme -> DarkColors
                else -> LightColors
            },
        typography = Typography,
        shapes = Shapes,
        content = content,
    )
}

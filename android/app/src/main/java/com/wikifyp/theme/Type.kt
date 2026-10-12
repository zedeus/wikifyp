package com.wikifyp.theme

import androidx.compose.material3.Typography
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.sp

// Type tokens. Serif for anything a reader dwells on (article text, titles),
// the system sans for controls and labels. Body sizes get generous line
// height; this app is read, not scanned. Every Material3 style is set so
// nothing silently falls back to the default Roboto voice.

val Reading = FontFamily.Serif
val Label = FontFamily.SansSerif

private fun style(
    family: FontFamily,
    size: TextUnit,
    line: TextUnit,
    spacing: TextUnit = 0.sp,
    weight: FontWeight = FontWeight.Normal,
) = TextStyle(
    fontFamily = family,
    fontWeight = weight,
    fontSize = size,
    lineHeight = line,
    letterSpacing = spacing,
)

private val Medium = FontWeight.Medium

val Typography =
    Typography(
        displayLarge = style(Reading, 44.sp, 52.sp, (-0.5).sp),
        displayMedium = style(Reading, 36.sp, 44.sp, (-0.25).sp),
        displaySmall = style(Reading, 30.sp, 38.sp),
        headlineLarge = style(Reading, 30.sp, 38.sp, weight = Medium),
        headlineMedium = style(Reading, 26.sp, 34.sp, weight = Medium),
        headlineSmall = style(Reading, 23.sp, 31.sp, weight = Medium),
        titleLarge = style(Reading, 21.sp, 28.sp, weight = Medium),
        titleMedium = style(Label, 16.sp, 24.sp, 0.1.sp, Medium),
        titleSmall = style(Label, 14.sp, 20.sp, 0.1.sp, Medium),
        bodyLarge = style(Reading, 17.sp, 27.sp, 0.2.sp),
        bodyMedium = style(Reading, 15.sp, 23.sp, 0.2.sp),
        bodySmall = style(Reading, 13.sp, 20.sp, 0.2.sp),
        labelLarge = style(Label, 14.sp, 20.sp, 0.2.sp, Medium),
        labelMedium = style(Label, 12.sp, 16.sp, 0.4.sp, Medium),
        labelSmall = style(Label, 11.sp, 16.sp, 0.4.sp, Medium),
    )

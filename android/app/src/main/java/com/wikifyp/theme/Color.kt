package com.wikifyp.theme

import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.ui.graphics.Color

// Color tokens. Neutral grays with a single confident blue accent, in the
// tradition of Wikipedia's own palette: the app is for long stretches of
// reading, so the chrome stays quiet and text carries the screen. Cards are
// white on a gray canvas in the light, raised grays at night.

// Light ("paper")
val Paper = Color(0xFFF8F9FA)
val PaperRaised = Color(0xFFFFFFFF)
val PaperSunken = Color(0xFFEAECF0)
val Ink = Color(0xFF202122)
val InkMuted = Color(0xFF54595D)
val InkFaint = Color(0xFFC8CCD1)
val BlueDeep = Color(0xFF3366CC)
val BlueWash = Color(0xFFEAF3FF)
val OnBlueDeep = Color(0xFFFFFFFF)
val OnBlueWash = Color(0xFF1D3A73)
val RedDeep = Color(0xFFB32424)
val OnRedDeep = Color(0xFFFFFFFF)
val RedWash = Color(0xFFFDE7E7)
val OnRedWash = Color(0xFF731919)

// Dark ("ink")
val Night = Color(0xFF202122)
val NightRaised = Color(0xFF2E3136)
val NightSunken = Color(0xFF27292D)
val PaperText = Color(0xFFEAECF0)
val PaperTextMuted = Color(0xFFA2A9B1)
val PaperTextFaint = Color(0xFF404448)
val BlueSoft = Color(0xFF6699FF)
val BlueNight = Color(0xFF233A66)
val OnBlueSoft = Color(0xFF102A52)
val OnBlueNight = Color(0xFFC9DAFF)
val RedSoft = Color(0xFFFF6B6B)
val RedNight = Color(0xFF4A1F1F)
val OnRedNight = Color(0xFFFFC9C9)

// Placeholder-tile art for articles without a lead image: deep muted
// grounds with matching pastel letters, rotated by article id. Fixed in
// both themes, like a photograph would be — these are stand-in images,
// not chrome.
val TileGrounds = listOf(Color(0xFF854848), Color(0xFF534FA3), Color(0xFF82456A))
val TileLetters = listOf(Color(0xFFDCB5B3), Color(0xFFC5B9DD), Color(0xFFD9B4CD))

internal val LightColors =
    lightColorScheme(
        primary = BlueDeep,
        onPrimary = OnBlueDeep,
        primaryContainer = BlueWash,
        onPrimaryContainer = OnBlueWash,
        secondary = InkMuted,
        onSecondary = PaperRaised,
        secondaryContainer = PaperSunken,
        onSecondaryContainer = Ink,
        background = Paper,
        onBackground = Ink,
        surface = Paper,
        onSurface = Ink,
        surfaceContainer = PaperRaised,
        surfaceContainerHigh = PaperRaised,
        surfaceVariant = PaperSunken,
        onSurfaceVariant = InkMuted,
        outline = InkFaint,
        outlineVariant = PaperSunken,
        error = RedDeep,
        onError = OnRedDeep,
        errorContainer = RedWash,
        onErrorContainer = OnRedWash,
    )

internal val DarkColors =
    darkColorScheme(
        primary = BlueSoft,
        onPrimary = OnBlueSoft,
        primaryContainer = BlueNight,
        onPrimaryContainer = OnBlueNight,
        secondary = PaperTextMuted,
        onSecondary = Night,
        secondaryContainer = NightSunken,
        onSecondaryContainer = PaperText,
        background = Night,
        onBackground = PaperText,
        surface = Night,
        onSurface = PaperText,
        surfaceContainer = NightRaised,
        surfaceContainerHigh = NightRaised,
        surfaceVariant = NightSunken,
        onSurfaceVariant = PaperTextMuted,
        outline = PaperTextFaint,
        outlineVariant = PaperTextFaint,
        error = RedSoft,
        onError = RedNight,
        errorContainer = RedNight,
        onErrorContainer = OnRedNight,
    )

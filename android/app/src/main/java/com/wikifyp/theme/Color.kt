package com.wikifyp.theme

import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.ui.graphics.Color

// Color tokens. The palette is warm paper and ink with one restrained
// moss-green accent: the app is for long stretches of reading, so the
// chrome stays quiet and text carries the screen.

// Light ("paper")
val Paper = Color(0xFFFAF7F1)
val PaperRaised = Color(0xFFFFFDF8)
val PaperSunken = Color(0xFFF1ECE2)
val Ink = Color(0xFF211E19)
val InkMuted = Color(0xFF6E675C)
val InkFaint = Color(0xFFD9D2C5)
val MossDeep = Color(0xFF47614F)
val MossWash = Color(0xFFDCE5DB)
val OnMossDeep = Color(0xFFF4F1E9)
val OnMossWash = Color(0xFF24352A)
val ClayDeep = Color(0xFF8E4A3C)
val OnClayDeep = Color(0xFFF4F1E9)
val ClayWash = Color(0xFFF2DFD9)
val OnClayWash = Color(0xFF52241B)

// Dark ("ink")
val Night = Color(0xFF16130E)
val NightRaised = Color(0xFF1E1A14)
val NightSunken = Color(0xFF282319)
val PaperText = Color(0xFFEAE4D8)
val PaperTextMuted = Color(0xFF9D968A)
val PaperTextFaint = Color(0xFF433D32)
val MossSoft = Color(0xFFA3BFA9)
val MossNight = Color(0xFF2C3C31)
val OnMossSoft = Color(0xFF1C2920)
val OnMossNight = Color(0xFFC9D8CC)
val ClaySoft = Color(0xFFD9A496)
val ClayNight = Color(0xFF4A2920)
val OnClayNight = Color(0xFFEFD4CB)

internal val LightColors =
    lightColorScheme(
        primary = MossDeep,
        onPrimary = OnMossDeep,
        primaryContainer = MossWash,
        onPrimaryContainer = OnMossWash,
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
        outlineVariant = InkFaint,
        error = ClayDeep,
        onError = OnClayDeep,
        errorContainer = ClayWash,
        onErrorContainer = OnClayWash,
    )

internal val DarkColors =
    darkColorScheme(
        primary = MossSoft,
        onPrimary = OnMossSoft,
        primaryContainer = MossNight,
        onPrimaryContainer = OnMossNight,
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
        error = ClaySoft,
        onError = ClayNight,
        errorContainer = ClayNight,
        onErrorContainer = OnClayNight,
    )

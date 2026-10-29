package com.wikifyp.ui.reader

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import com.wikifyp.theme.Spacing
import kotlin.math.roundToInt

/** Slider colors on the reader palette, shared with the WPM popup. */
@Composable
internal fun readerSliderColors(palette: ReaderPalette) =
    SliderDefaults.colors(
        thumbColor = palette.accent,
        activeTrackColor = palette.accent,
        inactiveTrackColor = palette.ghost.copy(alpha = 0.3f),
    )

/** Bottom sheet hosting the reader settings. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun ReaderSettingsSheet(
    settings: ReaderSettings,
    palette: ReaderPalette,
    onSettingsChange: (ReaderSettings) -> Unit,
    onDismiss: () -> Unit,
) {
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        containerColor = palette.surface,
        contentColor = palette.word,
        modifier = Modifier.testTag("settingsSheet"),
    ) {
        ReaderSettingsSheetContent(
            settings = settings,
            palette = palette,
            onSettingsChange = onSettingsChange,
        )
    }
}

/**
 * The sheet's content, separate from the sheet container so screenshot
 * tests can render it directly (modal windows escape composition-root
 * captures).
 */
@Composable
internal fun ReaderSettingsSheetContent(
    settings: ReaderSettings,
    palette: ReaderPalette,
    onSettingsChange: (ReaderSettings) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier =
            modifier
                .fillMaxWidth()
                .padding(horizontal = Spacing.lg)
                .padding(bottom = Spacing.xl),
    ) {
        Text(
            text = "Reader",
            style = MaterialTheme.typography.titleLarge,
            color = palette.word,
        )
        Spacer(Modifier.height(Spacing.md))

        SettingSlider(
            label = "Text size",
            value = settings.fontScale,
            valueText = "${(settings.fontScale * 100).roundToInt()}%",
            range = ReaderSettings.FONT_SCALE_MIN..ReaderSettings.FONT_SCALE_MAX,
            palette = palette,
            testTag = "fontScaleSlider",
            onChange = { onSettingsChange(settings.copy(fontScale = it)) },
        )
        SettingSlider(
            label = "Letter spacing",
            value = settings.letterSpacingEm,
            valueText = "%.2f".format(java.util.Locale.ROOT, settings.letterSpacingEm),
            range = 0f..ReaderSettings.LETTER_SPACING_MAX,
            palette = palette,
            testTag = "letterSpacingSlider",
            onChange = { onSettingsChange(settings.copy(letterSpacingEm = it)) },
        )
        SettingSlider(
            label = "Ghost words",
            value = settings.ghostWords.toFloat(),
            valueText = if (settings.ghostWords == 0) "off" else "${settings.ghostWords}",
            range = 0f..ReaderSettings.GHOST_WORDS_MAX.toFloat(),
            steps = ReaderSettings.GHOST_WORDS_MAX - 1,
            palette = palette,
            testTag = "ghostWordsSlider",
            onChange = { onSettingsChange(settings.copy(ghostWords = it.roundToInt())) },
        )
        SettingSlider(
            label = "Focal column",
            value = settings.orpFraction,
            valueText = "${(settings.orpFraction * 100).roundToInt()}%",
            range = ReaderSettings.ORP_MIN..ReaderSettings.ORP_MAX,
            palette = palette,
            testTag = "orpSlider",
            onChange = { onSettingsChange(settings.copy(orpFraction = it)) },
        )

        Spacer(Modifier.height(Spacing.sm))

        SettingToggle(
            label = "Bionic ghost words",
            description = "Bold the start of each ghost word",
            checked = settings.bionic,
            palette = palette,
            testTag = "bionicToggle",
            onChange = { onSettingsChange(settings.copy(bionic = it)) },
        )
        SettingToggle(
            label = "Focus mode",
            description = "Hide title and controls while reading",
            checked = settings.focusMode,
            palette = palette,
            testTag = "focusModeToggle",
            onChange = { onSettingsChange(settings.copy(focusMode = it)) },
        )

        Spacer(Modifier.height(Spacing.md))

        Text(
            text = "Theme",
            style = MaterialTheme.typography.labelLarge,
            color = palette.chrome,
        )
        Spacer(Modifier.height(Spacing.sm))
        ThemeRow(
            selected = settings.theme,
            palette = palette,
            onChange = { onSettingsChange(settings.copy(theme = it)) },
        )
    }
}

@Composable
private fun SettingSlider(
    label: String,
    value: Float,
    valueText: String,
    range: ClosedFloatingPointRange<Float>,
    palette: ReaderPalette,
    testTag: String,
    onChange: (Float) -> Unit,
    steps: Int = 0,
) {
    Column(modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                text = label,
                style = MaterialTheme.typography.labelLarge,
                color = palette.word.copy(alpha = 0.9f),
                modifier = Modifier.weight(1f),
            )
            Text(
                text = valueText,
                style = MaterialTheme.typography.labelMedium,
                color = palette.chrome,
            )
        }
        Slider(
            value = value,
            onValueChange = onChange,
            valueRange = range,
            steps = steps,
            modifier =
                Modifier
                    .fillMaxWidth()
                    .testTag(testTag),
            colors = readerSliderColors(palette),
        )
    }
}

@Composable
private fun SettingToggle(
    label: String,
    description: String,
    checked: Boolean,
    palette: ReaderPalette,
    testTag: String,
    onChange: (Boolean) -> Unit,
) {
    Row(
        modifier =
            Modifier
                .fillMaxWidth()
                .padding(vertical = Spacing.sm),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = label,
                style = MaterialTheme.typography.labelLarge,
                color = palette.word.copy(alpha = 0.9f),
            )
            Text(
                text = description,
                style = MaterialTheme.typography.labelMedium,
                color = palette.chrome,
            )
        }
        Switch(
            checked = checked,
            onCheckedChange = onChange,
            modifier = Modifier.testTag(testTag),
            colors =
                SwitchDefaults.colors(
                    checkedThumbColor = palette.background,
                    checkedTrackColor = palette.accent,
                    uncheckedThumbColor = palette.chrome,
                    uncheckedTrackColor = palette.ghost.copy(alpha = 0.3f),
                ),
        )
    }
}

@Composable
private fun ThemeRow(
    selected: ReaderTheme,
    palette: ReaderPalette,
    onChange: (ReaderTheme) -> Unit,
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(Spacing.sm),
    ) {
        ReaderTheme.entries.forEach { theme ->
            val isSelected = theme == selected
            Box(
                modifier =
                    Modifier
                        .weight(1f)
                        .background(
                            color =
                                if (isSelected) {
                                    palette.accent.copy(alpha = 0.18f)
                                } else {
                                    palette.ghost.copy(alpha = 0.12f)
                                },
                            shape = MaterialTheme.shapes.small,
                        ).clickable { onChange(theme) }
                        .padding(vertical = Spacing.sm)
                        .testTag("theme${theme.name}"),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    text =
                        theme.name.lowercase().replaceFirstChar { it.uppercase() },
                    style = MaterialTheme.typography.labelMedium,
                    fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Normal,
                    color = if (isSelected) palette.accent else palette.chrome,
                    textAlign = TextAlign.Center,
                )
            }
        }
    }
}

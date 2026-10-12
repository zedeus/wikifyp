# Design system

The app is a place to read for long stretches, so the design system is built
to disappear: warm paper and ink instead of bright surfaces, one quiet accent,
a serif voice for anything the reader dwells on, and motion that settles
quickly instead of drawing the eye. Nothing on screen should compete with the
sentence being read.

Tokens live in `android/app/src/main/java/com/wikifyp/theme/` and reach
composables through `WikifypTheme` (MaterialTheme under the hood, dynamic
color deliberately off so the reading surface looks the same everywhere).

## Tokens

- **Color** (`Color.kt`): a paper palette for light (`Paper`, `Ink`,
  `InkMuted`…) and an ink palette for dark (`Night`, `PaperText`…), with a
  muted moss green as the single accent and clay red for errors. Mapped onto
  the Material3 color scheme, so components use `MaterialTheme.colorScheme`
  roles, never raw colors.
- **Type** (`Type.kt`): `Reading` (serif) for display, titles and body;
  `Label` (sans) for controls. Body styles get generous line height.
- **Spacing** (`Spacing.kt`): one 4dp-based scale, `xs` through `xxl`. Use
  the scale, not raw dp.
- **Motion** (`Motion.kt`): three durations (`QUICK_MS`, `STANDARD_MS`,
  `SLOW_MS`) and two easings (`Settle` for arriving, `Leave` for exiting).
- **Shape** (`Theme.kt`): soft rounded corners; `large` is the content-card
  shape.

Values live in the token files, not here — this doc names the vocabulary.

## Base components

`android/app/src/main/java/com/wikifyp/ui/components/`:

- `PrimaryButton` — the one emphasized action on a screen.
- `QuietButton` — low-emphasis text action.
- `ContentCard` — hairline-bordered surface for a piece of content; no
  shadows.

## Screenshots

Golden images live in `android/app/src/test/screenshots/` and are verified on
every `./gradlew test` run (`roborazzi.test.verify=true`). After an intended
visual change, re-record with `./gradlew recordRoborazziDebug` and commit the
new images.

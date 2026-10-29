# Design system

The app is a place to read for long stretches, so the design system is built
to disappear: quiet neutral surfaces, one confident accent, a serif voice for
anything the reader dwells on, and motion that settles quickly instead of
drawing the eye. Nothing on screen should compete with the sentence being
read.

Tokens live in `android/app/src/main/java/com/wikifyp/theme/` and reach
composables through `WikifypTheme` (MaterialTheme under the hood, dynamic
color deliberately off so the reading surface looks the same everywhere).

## Tokens

- **Color** (`Color.kt`): neutral grays in the tradition of Wikipedia's own
  palette — a gray canvas with white cards for light (`Paper`, `PaperRaised`,
  `Ink`…), raised grays for dark (`Night`, `NightRaised`, `PaperText`…), one
  confident blue accent (`BlueDeep`/`BlueSoft`) and red for errors. Mapped
  onto the Material3 color scheme, so components use
  `MaterialTheme.colorScheme` roles, never raw colors. The one exception is
  the reader surface (`ui/reader/ReaderPalette.kt`), which owns its raw
  tokens and resolves them from the reader-theme *setting* (system, light,
  sepia, dark or true black) because that choice is independent of the app
  theme; its vermillion accent is tuned per surface so the focal letter
  stays vivid on paper, ink and true black alike. Stand-in art is also
  fixed across themes: the no-image tile colors (`TileGrounds`/
  `TileLetters` in `Color.kt`) and the scrim over hero photographs act as
  images, not chrome.
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

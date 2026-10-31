# Green Slime app materials: gel tiles

`generate.mjs` builds the three vector tiles behind the Green Slime look inside app windows:

| Token | Tile | What it is |
| --- | --- | --- |
| `--vd-mat-bar` | 420 x 12 px, `repeat-x`, at `calc(37% + 9em) 0` | a translucent lime gel lip (about 3.5 px, soft sag, crisp highlight line) along the **top** edge of a bar with four fat drops hanging into the bar's top padding: wide shoulders out of the lip (about 1.6 x the bulb width), a short neck, a round bulb 9.6 to 12 px wide, 8.5 to 10.9 px below the bar's top edge. Each bulb has a specular blob, a pale crescent on its far side, a darker core and a soft cast shadow |
| `--vd-mat-side` | 12 x 240 px, `repeat-y` at `100% 0` | a 3 px glossy gel rail flush with the **right** edge of a sidebar, three elongated beads (tail up, bulb down, 4.6 / 3.6 / 5.2 px bulge) and one small droplet sliding down it |
| `--vd-mat-canvas` | 520 x 380 px | seven soft gel bubbles of varied size at irregular places: a faint radial fill that is brightest at the rim, a very thin faint rim, one small specular highlight, no hard stroke |

Backgrounds cannot paint outside their element, so the drops hang inside the bar and slip behind buttons and tabs
(which paint their own backgrounds). Gravity is vertical, so the side's drops slide down the rail instead of dripping
sideways. The geometry (drop positions, depths, widths, bead heights and bulges, bubble positions and radii) is a set
of constants at the top of the script; the SVG is generated, minified and URL-encoded (`%3C`, `%23`, single quotes),
so edit the constants and regenerate instead of touching the SVG in the stylesheet. The tiles stay inside the limits of
`docs/superpowers/specs/2026-10-31-desktop-app-materials-design.md`: each data URI is at most 4 KB (here about
1.6 KB canvas, 2.9 KB bar, 2.5 KB side), the last background layer is `var(--vd-mat-<x>-color)`, no animation, no
external assets.

## Bar tile position (no drop columns)

The bar tile is positioned at 37% of the free width plus 9 em instead of `0`. The percentage gives bars of different
widths (sidebar header, full-width toolbar, editor-width tab strip) different offsets; the `em` term gives bars with
different font sizes (menu 13 px, toolbar 12 px, status bar 11 px) different offsets. Bars that have the same width
**and** the same font size (for example the tab strip and the terminal tab bar of one editor column) still show their
drops at the same x. A token cannot tell them apart; an app that needs more variety can set its own
`background-position-x` on one of the bars.

## Contrast on the canvas

Text sits directly on the canvas, so the bubbles stay faint: rim at most 0.15 alpha, specular highlight 0.14 alpha
(`CANVAS.rim`, `CANVAS.spec`). Measured at 4x over the canvas colour plus the strongest lime haze (alpha .08), the
brightest bubble pixel is `#4a553b`: contrast 5.04:1 against the muted text colour `#C6D4A6` and 7.19:1 against the
text colour `#F1F7DC` (the canvas colour itself gives 8.86:1 with the muted text). Re-measure after changing the two
constants; keep the muted-text ratio at 4.5:1 or better.

## Run

```sh
node assets/desktop-materials-green-slime/generate.mjs                 # prints the three CSS values and their byte sizes
node assets/desktop-materials-green-slime/generate.mjs <previewDir>    # also writes preview.html, bar.svg, side.svg, canvas.svg
```

No dependencies; the output is deterministic (no randomness).

`preview.html` loads the real `ui/css/desktop-materials-green-slime.css` for every other token and overrides
`--vd-mat-canvas`, `--vd-mat-bar` and `--vd-mat-side` with the fresh values. Open it in a browser, or pick a view with
the query string: `?view=stack` (a Code-Studio-like window with bars of different widths: menu bar and toolbar full
width, a 260 px sidebar header, an editor-width tab strip and terminal tab bar, status bar), `?view=status` (22 px and
26 px status bars and a menu bar with text), `?view=side` (sidebar next to a well), `?view=canvas` (text and cards on
the canvas), `?view=tiles` (the raw tiles), `?view=zoom&z=10&x=0&y=0[&side=1|&canvas=1]` (a zoomed window into a
tile). Use headless Chrome with `--force-device-scale-factor=2` for close-ups. `--css=<file>` previews another
stylesheet and `--keep` previews it without overriding the three tokens (handy for before/after comparisons).

## Where to paste

Copy the three printed lines (`--vd-mat-canvas: ...;`, `--vd-mat-bar: ...;` and `--vd-mat-side: ...;`) over the lines
with the same names in `ui/css/desktop-materials-green-slime.css`, inside the `.desktop-body[data-theme="green_slime"]`
rule. Then check `go test ./ui -run 'DesktopAppMaterials|DesktopThemeCSSContract' -count=1`, and run
`npm run build:ui` so that `ui/css/desktop-shell.bundle.css` (which contains the material files) picks the change up.

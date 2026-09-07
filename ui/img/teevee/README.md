# TeeVee material sources

The visual source is the user-supplied, unchanged
`documentation/assets/teevee-retro-reference.png` (1672 × 941).
Only static physical parts are images. Menus, station names, counts, controls
and playback remain live DOM/video.

Source extractions, with coordinates in the reference:

| File | Source region / processing |
| --- | --- |
| `bezel.png` | x400 y84 w750 h643, enlarged to 1355×1161; glass opening made transparent with an antialiased fitted mask. |
| `brand-tv.png` | x93 y119 w58 h62, unchanged pixels. |
| `wood.png` | x3 y75 w33 h770, unchanged pixels. |
| `metal.png` | x950 y879 w220 h36, unchanged pixels. |
| `screw.png` | x48 y90 w18 h18, unchanged pixels. |
| `display-frame.png` | x67 y99 w318 h114; original outer 12px band, transparent center for CSS nine-slice. |
| `channel-frame.png` | x1182 y217 w393 h78; original outer 9px band, transparent center. |
| `button-frame.png` | x980 y760 w63 h63; original outer 10px band, transparent center. |

`glass.png`, `tv.png`, `speaker.png`, `stripe.png`, `bars.png`, `power.png`,
`signature.png` and `color-mark.png` were produced
with ImageGen using the reference as visual direction, then prepared for their
runtime slots. `glass.png` is RGB on black for screen blending, deliberately
separate from the video signal filter. `tv.png` is 256×256 RGBA; `speaker.png`
is 339×360 RGB. Production inputs, discarded versions and visual comparisons
are kept in the ignored `reports/teevee-assets/` directory.

## Icons

SVGs are unmodified [Bootstrap Icons 1.13.1](https://github.com/twbs/icons/tree/v1.13.1/icons),
under MIT; see `ICONS-LICENSE.txt`. Runtime filename → upstream name:

`globe` → `globe`; `heart` → `heart`; `heart-filled` → `heart-fill`;
`news` → `newspaper`; `sports` → `dribbble`; `film` → `film`;
`music` → `music-note-beamed`; `people` → `people-fill`; `search` → `search`;
`close` → `x-lg`; `minimize` → `dash-lg`; `maximize` → `square`;
`stop` → `stop-btn`; `play` → `play-fill`; `pause` → `pause-fill`;
`volume` → `volume-up`.

## Fonts

Both font files are unmodified Google Fonts downloads, with their OFL notices
beside them in `ui/fonts/`; no external font request at runtime.

- [Roboto Condensed](https://github.com/google/fonts/tree/main/ofl/robotocondensed),
  `RobotoCondensed-Variable.ttf`, SHA-256
  `dace262afcee68a5276f200d8026c57221735c0118ab5fda8c2c0d3dc409a8d0`.
- [Share Tech Mono](https://github.com/google/fonts/tree/main/ofl/sharetechmono),
  `ShareTechMono-Regular.ttf`, SHA-256
  `9ceab1f87414829af259c0f537573ae03ef7dd3147c0b27a36a1a0beb6732677`.

Downloaded on 2026-09-07. The installed hashes, rather than the moving upstream
branch, identify the exact files used here.

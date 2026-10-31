# Desktop Themes

The Virtual Desktop has fourteen themes. A theme sets the shell layout (taskbar or dock), the colour scheme, the icon set, the look of windows, menus and controls and, for some themes, subtle visual effects. Apps keep their own content and take their colours from the shared theme tokens.

This page covers choosing a theme, accessibility and the developer contract. The implementation rules live in `ui/AGENTS.md` (Desktop theme system) and `ui/js/desktop/apps/AGENTS.md` (App theme bridge contract).

## The Themes

| Theme | Shell | Scheme | Icon set | Accent | Look |
| --- | --- | --- | --- | --- | --- |
| Standard | Taskbar | Dark | Papirus | Your accent | Dark and focused, with a clear taskbar and a structured start menu. |
| Fruity | Dock and top menu bar | Light or dark | WhiteSur | Your accent | Bright and soft, with a floating dock and rounded windows. |
| Amiga | Taskbar and top menu bar | Light | Papirus | Fixed blue | Workbench 3.1 classic: grey bevels, blue title bars, a VGA pixel type and no motion. |
| Cyberpunk | Taskbar | Dark | Papirus | Fixed yellow | Neon yellow and cyan on deep night blue, with sharp HUD frames. |
| Black Gloss | Centred taskbar | Dark | WhiteSur | Your accent | Piano black with glossy highlights. |
| Crazy | Taskbar | Dark | Papirus | Fixed pink | Rainbow frames, bouncing windows, confetti and sparkles. |
| Lollipop | Dock and top menu bar | Light | Papirus | Fixed plum pink | Candy pastels, jelly-bean buttons and a sweet dock. |
| Organic | Taskbar | Dark | Papirus | Fixed brass | Walnut grain, brushed brass hardware and soft rounded shapes. |
| Electronic Dreams | Taskbar | Dark | Papirus | Fixed amber | Graphite circuit boards, copper traces and quiet signal lights. |
| Windows XP | Taskbar | Light | Windows XP | Fixed blue | Luna blue gradients, a green Start button, warm controls and a two-column Start menu. |
| Green Slime | Taskbar | Dark | Papirus | Fixed lime | Glossy lime drips, mossy frames and soft, squishy controls. |
| Voxel World | Taskbar | Light | Papirus | Fixed forest green | A live voxel island behind the shell, with block-framed windows and an inventory-style taskbar. |
| Orbit | Taskbar | Dark | Papirus | Fixed ice cyan | A live 3D orbit behind the shell: a ringed gas giant, a space station, a shuttle and asteroids, with smoked-glass windows that fly in and out in 3D. |
| Minimal | Taskbar | Light or dark | Papirus | Fixed ink | Paper or graphite, hairlines and typography, nothing else. |

"Your accent" means the Accent color setting applies. Fruity's and Minimal's schemes follow their own Light/Dark setting.

## Choosing a Theme

- Open Settings, category **Appearance**, and pick a card under **Theme**. A click, Enter or Space selects it.
- A theme also sets the **Icon theme** (Papirus, WhiteSur or Windows XP). You can change the icon set afterwards; picking a theme pairs its icon set again.
- The wallpaper follows the theme while it is still the previous theme's suggestion, or the default *Group Shot*, in every space. Suggestions: Amiga *Slate*, Cyberpunk *City Rain*, Black Gloss *Midnight*, Crazy *Fractal Trip (animated)*, Lollipop *Paper Waves*, Organic *Forest*, Electronic Dreams *Electronic Dreams*, Windows XP *Windows XP Bliss*, Green Slime *Green Slime*, Voxel World *Voxel World*, Orbit *Orbit* and Minimal *Minimal*. Standard and Fruity use *Group Shot*. A wallpaper you picked yourself is never changed.
- **Fruity appearance** and **Minimal appearance** (Light or Dark) appear only while their theme is selected. Minimal starts in Light. The other themes have a fixed scheme.
- **Accent color** (teal, orange, blue, violet or green) applies to Standard, Fruity and Black Gloss. The other eleven themes use their fixed accent and hide the row.
- The settings are `appearance.theme`, `appearance.icon_theme`, `appearance.fruity_mode`, `appearance.minimal_mode`, `appearance.accent`, `appearance.wallpaper` and `appearance.wallpaper_by_space`. The UI saves them through `PUT /api/desktop/settings`.

## Accessibility

- **Reduced motion.** When the system requests reduced motion (`prefers-reduced-motion: reduce`), a global override ends every transition and animation at once: durations drop to 0 ms and animations run a single iteration. The shell also skips its window, menu and start-menu transitions, and Crazy's confetti and sparkles stop.
- **Window animations.** Settings, category **Windows**, *Window animations* turns off theme animations and animated effects; themes with a static effect layer retain that layer. This setting also applies when the system asks for reduced motion.
- **Contrast.** Body text on the app and panel backgrounds, and muted text on the app background, are tested to a contrast ratio of at least 4.5:1 for every skinned theme.
- **Focus.** Keyboard focus shows a ring in every theme. Amiga draws a dashed black outline and Crazy a cyan ring.
- **Amiga window gadgets.** The Amiga skin keeps the authentic Workbench gadget layout (close far left, depth and zoom gadgets on the right) purely visually through CSS grid placement, so keyboard Tab order follows the DOM (AI, minimize, maximize, close; AI and maximize are omitted where a window lacks them) rather than the on-screen order, and the dashed black focus ring makes the focused gadget visible.
- **Windows XP Start menu.** The Luna-style Start menu uses two columns: the app list fills the left pane, with system links and categories stacked on the right. The CSS adapts the column widths on narrow screens while keeping each action reachable.
- **Theme motion.** Electronic Dreams' signal light and the Green Slime, Voxel World and Orbit effects switch to a still rendering when *Window animations* is off or reduced motion is requested. The Windows XP, Green Slime and Voxel World skins keep their gradients, glossy surfaces and block edges when glass polish is off.
- Touch: on a coarse pointer the global menu bar grows to 44 px and every tray control fills it; below 1024 px the window menus move to a second 44 px row and the bar is 88 px (`css/desktop-chrome.css`). Amiga sizes its gadgets from the bar height in its skin and keeps a single row.

## Developer Guide

### Matching cursors and sounds

Amiga, Cyberpunk, Black Gloss, Crazy, Lollipop, Organic, Electronic Dreams,
Windows XP, Green Slime, Voxel World and Orbit include original cursor artwork and
individual sound sets. In **Settings → Appearance**, **Theme cursors**
switches between the theme artwork and system cursors. There are twelve cursor
roles per theme, including text, links, drag, resize and busy states. High-contrast
mode and touch-only devices retain system cursors. App canvases and remote desktop
surfaces keep their own tool cursors. Standard, Fruity and Minimal retain system cursors.

In **Settings → Sound**, enable **UI sounds** to hear the feedback. Sounds remain
off by default. **Follow desktop theme** automatically picks the matching sound
set (Crystal for Standard, Fruity and Minimal); a manually chosen set remains selected
when changing the desktop theme. Existing saved sound choices are preserved.
Volume and the window, notification, navigation and file category switches still
apply. The **Preview** buttons play a short sample without enabling UI sounds.

| Theme | Cursor artwork | Sound character |
| --- | --- | --- |
| Amiga | Pixel steps, white outlines, orange inlays | Short square waves and retro arpeggios |
| Cyberpunk | Angular neon-yellow pointers and cyan circuit details | FM pulses, metallic signals, static transients |
| Black Gloss | Graphite and silver bevels | Glass partials and restrained reverberation |
| Crazy | Purple ribbons and mint accents | Rising bubble tones and unexpected intervals |
| Lollipop | Rounded candy stripes and heart details | Celesta chimes and light pops |
| Organic | Leaf silhouette with walnut and brass inlays | Wooden harmonics and soft filtered air |
| Electronic Dreams | Teal signal traces and warm pointer lights | Relay ticks, soft analog sweeps and filtered signal noise |
| Windows XP | Classic pointer with blue and gold details | Bright friendly chimes with short, rounded tails |
| Green Slime | Glossy lime slime pointer | Low-pass wet plips, elastic sweeps and soft squelches, plus quiet blub pops for bursting bubbles and a wet thwip-plop for parting strands |
| Voxel World | Block-edged pointer in earth and green tones | Dry taps and wood, stone and earth impacts |
| Orbit | Sleek cyan-violet arrow with an amber orbit ring | Soft synth whooshes, sonar pings and airlock hiss; a window opens with a rising whoosh and closes with a falling whoosh and sparkle grains |

`cursorTheme` and `soundTheme` on each registry descriptor own the pairing.
`core/cursor-runtime.js` applies versioned, local SVG cursor URLs with native
hotspots and keyword fallbacks. `css/desktop-cursors.css` preserves semantic
cursor roles without a pointer-following overlay or animation loop.
The 132 cursor SVGs, eleven cursor preview sheets and eleven theme sound-set covers are original AuraGo artwork, generated
by `node scripts/build-desktop-cursors.mjs`; use `--check` to verify reproducibility.
No third-party cursor artwork or sampled operating-system sounds are included.

Electronic Dreams, Windows XP, Green Slime, Voxel World and Orbit each have a separate
sound bank in `sound/theme-electronic-dreams.js`, `sound/theme-windows-xp.js`,
`sound/theme-green-slime.js`, `sound/theme-voxel-world.js` and
`sound/theme-orbit.js`. Each bank supplies
the shell events through the shared lazy Web Audio synthesizer; the five themes
do not share one generic theme bank. In-flight synthesis is shared; mute, theme
changes and tab hiding cancel pending playback. Audio unlock remains bound to a
user gesture. Turning sound off releases the AudioContext immediately.
Verify `TestDesktopThemeSensoryBrowser`, `TestDesktopSoundBrowser` (all sixteen
sets, the voxel, slime and orbit effect sounds and their rate limits) and
`TestThemeSensorySettingsPersistIndependently`, then rebuild UI bundles.

**Effect sounds.** Voxel World, Green Slime and Orbit also have sounds that their desktop
effect asks for by name, not shell events: `window.fracture` and `window.impact`
(Voxel World; wood, stone or earth) and `slime.pop` and `slime.snap` (Green Slime;
small, medium or large) and `orbit.ping` (Orbit; small, medium or large). A bank supplies them as
`specialFx[event][variant]`. The effect only calls `DesktopSounds.play(event, { variant | material, strength, pan })`
(`strength` 0 to 1, `pan` -1 to 1) and `core/sound-runtime.js` decides whether
anything is heard. It plays them only when UI sounds are on and unlocked, the
event's category is on (**Windows** for the Voxel World and Green Slime sounds,
**Navigation** for `orbit.ping`), the effective sound theme is the owning theme, the page
is visible, no session restore runs and the call is not a preview. Calls within
75 ms (Voxel World) or 80 ms (Green Slime and Orbit) merge into the loudest one, and a call
that is older than 240 ms by the time its sound is rendered is dropped. Each sound
is rendered on first use and cached per theme, event and variant, so the full bank
is never rendered for it. `inspect().themeFx` counts requests, plays, rate-limited
and expired calls per family.

- Green Slime pop: a soft sine blop with a fast upward glide (about 240 to 520 Hz for
  a large bubble, 520 to 1100 Hz for a small one), a very short band-passed tick of
  the film tearing and a faint low-passed tail, under 180 ms in all.
- Green Slime snap: a band-passed noise sweep downward (25 to 40 ms), then about
  40 ms later a soft low plop (about 180 to 120 Hz for a large strand), under 260 ms
  in all. The plop carries a quiet octave above it so small speakers still carry it.
- Orbit ping: a sine at 1320, 1100 or 880 Hz for a small, medium or large ping,
  with a 6 ms attack, a 420 to 700 ms release, a faint noise click at the start
  and an echo 180 ms later at 9 dB less. The runtime plays it at 0.38 of a window
  sound.
- Levels: the synth peak-normalises every buffer, so the runtime sets the level. A pop
  plays at 0.40 and a snap at 0.42 of a window sound at full strength (about 25 to
  33 % in 20 ms loudness), and the strength curve makes the smallest ones softer
  still. The master volume and the category of the event apply as usual.
- Rate limits: at most one pop per 350 ms, one snap per 900 ms and three Green Slime
  sounds per 2 s; Voxel World keeps four per second and 190 ms apart. Orbit allows one ping per
  350 ms and three Orbit sounds per 2 s.
- No layer of these banks uses the synth's reverb pool: its convolver is shared by all
  layers of one render and would smear one layer into the envelope of the next.

### Architecture

1. `DESKTOP_THEMES` in `ui/js/desktop/core/theme-runtime.js` describes every theme. `desktop.DesktopThemeIDs()` in `internal/desktop/themes.go` lists the same ids and is the allowed value list of `appearance.theme`.
2. `applyDesktopSettings()` in `ui/js/desktop/core/desktop-foundation.js` calls `applyDesktopThemeAttributes()`, which sets `data-theme`, `data-shell`, `data-scheme` and `data-polish` on `<body>`.
3. CSS keys off these attributes: `[data-theme]` in the skins and the shell-overrides bridge, `[data-shell]` in the layout rules (`desktop-base.css`, `desktop-windows.css`, `desktop-chrome.css`), and `[data-polish]` in `desktop-polish.css`.
4. JavaScript asks the capability helpers (`shellLayout()`, `themeUsesGlobalMenu()`, `themeScheme()`, `themeMotionMs(kind)`, `themeIconFamily()`) and never compares a theme id. Apps read `document.body.dataset.scheme`.
5. `syncDesktopThemeFx()` starts or stops the effects of an `fx` theme.

Layering in `ui/css/desktop-shell.bundle.css`: the shared shell files (`desktop-base.css` carries the Standard and Fruity core tokens), then `desktop-shell-overrides.css` (the `--vd-theme-*` bridge), `desktop-chrome.css`, `desktop-wallpaper-live.css`, `desktop-wallpaper-themes.css` (the locally packaged theme wallpapers: Electronic Dreams, Windows XP, Green Slime, Voxel World, Orbit, Forest and Minimal), the skins `desktop-theme-amiga.css`, `-cyberpunk`, `-black-gloss`, `-crazy`, `-lollipop`, `-organic`, `-electronic-dreams`, `-windows-xp`, `-green-slime`, `-voxel-world`, `-orbit` and `-minimal`, then `desktop-cursors.css`, and last `desktop-polish.css`. `scripts/build-ui-bundles.js` assembles this bundle and the `desktop-theme-fx` bundle.

### Body attributes

| Attribute | Values | Used by |
| --- | --- | --- |
| `data-theme` | theme id | skin files and the `.desktop-body[data-theme]` bridge |
| `data-shell` | `taskbar`, `dock` | layout rules and `shellLayout()` |
| `data-start-menu` | `classic`, `xp` | theme-specific Start menu structure; XP supplies a two-column layout |
| `data-scheme` | `light`, `dark` | apps (Code Studio editor, CYD Creator code, Layerling, Leafy) |
| `data-polish` | `on`, `off` | `desktop-polish.css` gate, from `glassPolish` |
| `data-fruity-mode` | `light`, `dark` | Fruity rules |
| `data-accent` | `teal`, `orange`, `blue`, `violet`, `green` | accent palette in `desktop-base.css` |
| `data-animations` | `true`, `false` | every theme animation and the effects (*Window animations*) |
| `data-orbit-quality` | `0` to `4`, only while Orbit's scene runs | Orbit skin: from quality 2 the window frames drop their backdrop blur and get a more opaque fill |

### Descriptor Reference

Each `DESKTOP_THEMES` entry is one `Object.freeze({ ... })` line with these keys in this order. The registry test parses the file as text, so keep single quotes and the key order.

| Key | Values | Meaning |
| --- | --- | --- |
| `id` | `^[a-z][a-z0-9_]*$`, not `light` or `dark` | `appearance.theme` value, `data-theme`, skin file name (`_` becomes `-`) and keyframe prefix |
| `labelKey`, `descKey` | desktop locale keys | card title and description |
| `scheme` | `light`, `dark` | colour scheme; must equal the skin's `color-scheme` |
| `schemeSetting` | empty, `appearance.fruity_mode` or `appearance.minimal_mode` | the setting that picks the scheme; shows the light/dark row, labelled for the theme (Fruity appearance, Minimal appearance). A skin of such a theme also needs a dark root rule (see Token Contract) |
| `shell` | `taskbar`, `dock` | layout; `dock` requires `globalMenu: true` |
| `startMenu` | `classic`, `xp` | Start menu presentation; XP adds a branded header and right-hand links/categories |
| `globalMenu` | boolean | top menu bar; read by `themeUsesGlobalMenu()` |
| `iconTheme` | `papirus`, `whitesur`, `windows_xp` | icon set paired on selection (`appearance.icon_theme`) |
| `iconFamily` | `standard`, `fruity` | mini action-icon sheet (`refreshMiniIconTheme()`) |
| `accent` | `user`, `fixed` | `user` shows the accent row and follows the user accent; `fixed` needs the fixed-accent tokens on the root |
| `glassPolish` | boolean | `data-polish` on or off |
| `fx` | boolean | loads the lazy effects bundle for the theme |
| `staticFx` | boolean | keeps a theme effect module mounted when animations are disabled; the module receives `options.motion: false` to render its still form |
| `wallpaper` | an `appearance.wallpaper` value | suggested wallpaper (`followThemeWallpaper()`) |
| `thumb` | `/img/themes/<id>.svg` | picker thumbnail |
| `cursorTheme` | cursor artwork directory id, or empty | themed cursor set; empty keeps system cursors |
| `soundTheme` | registered sound set id | sound set used by **Follow desktop theme** |
| `motion` | eight keys in ms, 20 to 800: `open`, `close`, `minimize`, `restore`, `menuOpen`, `menuClose`, `startOpen`, `startClose` | fallback timers of `animateThen` via `themeMotionMs(kind)`; an unknown kind gets 160 ms |

### Token Contract

Every skin root (`.desktop-body[data-theme="<id>"]`) defines:

- `color-scheme`, equal to the registry `scheme`.
- Core `--vd-*` tokens: surfaces, text, borders, shadows, window radius and UI font.
- The 17 `--vd-theme-*` bridge tokens: app and panel backgrounds, chrome, overlay, controls, borders, muted text, accent soft and border, radii, panel and modal shadows, blur.
- `--vd-chrome-highlight`, `--vd-chrome-ring` and `--vd-chrome-hairline`.
- The `--ds-*` colour, blur, surface and elevation tokens.
- `--vd-accent-contrast`: the text colour on the accent, required on every theme root and at least 4.5:1 on a fixed accent.
- App interiors: an app that paints on the accent exposes a token on its root and a fixed-accent skin overrides it in an "App tokens" rule of its skin file: Video Studio `--vs-accent-strong` and `--vs-accent-hover` on `.vs-app` (cyberpunk sets both), Code Studio `--cs-accent-contrast` on `.code-studio` (amiga, cyberpunk, crazy and lollipop set it to their `--vd-accent-contrast`). Mission Control needs no skin rule; it mixes `--mc-accent-ink` and `--mc-danger-ink` from the accent, the danger red and `--vd-text`, with the danger share lowered under `[data-scheme="light"]`.
- For `accent: "fixed"`: `--vd-accent`, `--vd-accent-r`, `--vd-accent-g`, `--vd-accent-b` and `--vd-accent-glow`. User-accent themes must not set them.
- Contrast: `--vd-text` on `--vd-theme-app-bg` and `--vd-theme-panel-bg`, and `--vd-theme-muted` on `--vd-theme-app-bg`, at least 4.5:1, with opaque `#rrggbb` app and panel backgrounds.
- Dark root rule, for a theme with a `schemeSetting`: the skin also has a plain rule `.desktop-body[data-theme="<id>"][data-scheme="dark"]` that sets `color-scheme: dark`. That rule may set the root-only tokens (`color-scheme`, `--vd-theme-app-bg`, `--vd-theme-panel-bg`, `--vd-text`, `--vd-theme-muted`, `--vd-accent` and `--vd-accent-contrast`). The contract merges it over the root rule and measures its contrast and scheme the same way. Those tokens set in any other rule fail the contract. Minimal is the skin with this rule; Fruity has no skin file.

Standard and Fruity have no skin file. Their core tokens live in `desktop-base.css` and their `--vd-theme-*` bridge in `desktop-shell-overrides.css`. The contract tests check skins only.

### Rules and Budgets

- Every selector in a skin starts with `.desktop-body[data-theme="<id>"]`. The first rule is that bare root rule.
- Every `animation` sits behind `:not([data-animations="false"])`. Reduced motion is enforced globally by the override in `desktop-shell-overrides.css`, so skins need no reduced-motion query of their own.
- Keyframes are named `vd-theme-<id>-*` and animate only `transform`, `opacity`, `offset` and `animation-timing-function`.
- `@font-face` families start with `Desktop Theme `.
- A skin file has at most 900 lines. Crazy has at most 6 `infinite` animations (it has 5).
- A thumbnail has at most 8192 bytes and no `<script>`, `<foreignObject>`, `on*=` handler, `javascript:` or `http(s)://` URL, apart from the XML namespace declarations.
- For `!important`, `clip-path` and `will-change` conventions, see `ui/AGENTS.md`.

### Effects API

- The host `ui/js/desktop/theme-fx/fx-core.js` exposes `window.AuraDesktopThemeFx` with `register(id, impl)`, `start(id, options)`, `stop()`, `activeId()`, `inspect()` and `command(name, payload)`.
- `inspect()` returns the active effect's diagnostics with `activeId`, `motion`, `paused` and `aborted` added. With no active effect it returns `activeId: ''`.
- `command(name, payload)` forwards to the active effect's `command(name, payload)` and returns its result. It returns `null` when no effect is active, when the effect has no `command` method, when the call throws, or when the result is `undefined`. Tests and debugging use it to trigger effect actions, for example Green Slime's `bubble`, `poke`, `drop` and `quality` commands and Orbit's `orbit.time`, `orbit.quality`, `orbit.capture`, `orbit.ping` and `orbit.burst` commands.
- An effect module `ui/js/desktop/theme-fx/<id>.js` registers `{ start }` under its theme id. `start({ layer, signal, options })` returns `{ pause, resume, stop }` and may add `inspect` and `command`; missing methods are no-ops.
- The host creates `.vd-theme-fx-layer` (fixed, full viewport, `pointer-events: none`, one z-index step below `--vd-z-dock`; the rule lives in `desktop-base.css`), an AbortController and a `visibilitychange` listener. A hidden tab calls `pause()` and a visible tab calls `resume()`.
- `stop()` runs before the signal is aborted and the layer is removed. Bind listeners to `signal` and release everything in `stop()`.
- `options.motion` separates material rendering from movement. Electronic Dreams, Green Slime, Voxel World and Orbit declare `staticFx: true`: reduced motion and disabled animations keep a still frame with no continuous animation loop. Crazy remains entirely inert in this mode. Mobile desktop mode uses the CSS skins and local wallpaper without loading a material renderer.
- `window.AuraDesktopWindowEffects.snapshot()` exposes visible window geometry. The `aurago:desktop-window` event reports open, focus, drag/resize start/update/end, minimize, restore, maximize, snap and accepted close, with window ID, rectangle, z order, workspace, velocity and restoration-silence information. An `open` precedes focus; a rejected close never emits destruction. Effects must not mutate application state or intercept input.
- The voxel island renders in `voxel-scene.js`, the block frames and fragments in `voxel-graphics.js`; `voxel_world.js` owns physics and lifecycle. The separate `vendor/desktop-themes/voxel-runtime.js` ESM bundle loads Three.js and Rapier only when needed. Frame cubes use instancing and ranked depth occlusion; shell overlays and window contents are stencil masked. WebGL failure retains the Canvas/CSS fallback.
- Voxel physics advances at 60 fixed steps per second with at most 256 pooled fragments. Long frame intervals lower render resolution before reducing scenery and fragment budgets. Hidden tabs suspend work; switching themes releases observers, bodies, render targets and WebGL contexts. Hardware frame-rate acceptance is separate from the software-WebGL browser fixture.
- Budget pattern (`theme-fx/crazy.js`): pools of spans created once, one Web Animation per particle on `transform` and `opacity`, no per-frame loop and no timer. Crazy has 36 confetti and 14 sparkle spans, one confetti burst at a time, at most one sparkle every 33 ms, and no sparkles over app content, the taskbar, menus, dialogs or toasts.

### Green Slime

The Green Slime effect (`ui/js/desktop/theme-fx/green_slime*.js`) draws a translucent lime gel around each window frame, with drips, bubbles, strands and free drops. The simulation is DOM-free. The bridge in `green_slime.js` passes it window geometry, overlay masks and pointer input, and draws the finished scene with one renderer. The gel never paints over window content, higher windows or overlays. The layer has `pointer-events: none`, and the pointer listeners are passive, so the desktop keeps every event.

**Files and bundle order.** The `desktop-theme-fx` bundle loads these parts in this order, after `fx-core.js`, `crazy.js` and `electronic_dreams.js`:

1. `green_slime_geometry.js` (`AuraGreenSlimeGeometry`): constants, seeded RNG, rounded-rectangle perimeter and gel profile.
2. `green_slime_bubbles.js` (`AuraGreenSlimeBubbles`): bubble lifecycle.
3. `green_slime_touch.js` (`AuraGreenSlimeTouch`): hover, wake, poke and strand.
4. `green_slime_sim.js` (`AuraGreenSlimeSim`): surfaces, ripple wave, drops, splats, the scene, the bounded event queue (`takeEvents()`) and `soundCue()`.
5. `green_slime_canvas2d.js` (`AuraGreenSlimeCanvas2D`): Canvas 2D renderer.
6. `green_slime_gl_shaders.js` (`AuraGreenSlimeGLShaders`): GLSL sources.
7. `green_slime_gl.js` (`AuraGreenSlimeGL`): WebGL2 renderer.
8. `green_slime.js`: DOM bridge, renderer choice, `inspect()` and `command()`, registered as `green_slime`.

Each part is an IIFE that attaches one global. Keep the order: `green_slime_gl.js` reads the shader module when it loads, and the bubble and touch factories read the geometry global. The split keeps each file under 1,100 lines.

**Renderer.** `chooseRenderer()` tries WebGL2 first. Both renderers draw into a single `.vd-fx-slime-canvas` element inside the effect layer, and the fallback replaces that element instead of adding a second canvas. The Canvas 2D fallback takes over when:

- WebGL2 is unavailable, or the GPU is a software renderer (SwiftShader, llvmpipe, lavapipe, softpipe, or a "software" or "basic render" string). The test hook `window.__auraSlimeSoftwareGL === true` overrides the software check.
- The programs do not build or link, fewer than 120 fragment uniform vectors are available, or the build runs longer than 30 s.
- The WebGL context is lost. The GL canvas prevents the default, and the fallback stays for the rest of this effect instance.
- Frames stay slow. An average frame time above 22 ms over two consecutive 2 s windows lowers the render scale to 0.75. An average above 45 ms over two windows at that scale switches to the fallback.

A canvas that has handed out a WebGL context cannot give it up, so the fallback always gets a fresh canvas. A theme change starts the effect again, and WebGL2 is tried first again.

**Async build and fade-in.** The gel, drop and mask programs link as one job. With `KHR_parallel_shader_compile`, the driver builds them on its own threads and the effect polls their status every 8 ms; nothing is drawn until they finish. Once the build completes, the gel fades in over 400 ms with a cubic ease-out, so it oozes into view instead of popping. The fade multiplies the gel, its cast shadow, the drops and the splats, but not the stencil masks. It is recomputed for each rendered frame from the time since the build finished. Without the extension, the link blocks instead. `inspect()` reports `fade`, `compileMs`, `blockedMs` and `parallelCompile`.

**Backdrop.** `texture` mode refracts the real wallpaper. It needs the body to paint exactly one `url()` image with `background-size: cover`, `background-attachment: fixed`, `background-repeat: no-repeat` and a centred position, and no visible live wallpaper (`#vd-wallpaper-live`) or `.vd-wallpaper-fx` overlay. The image is decoded, downscaled so that its longer side is at most 2048 px and the GPU's texture limit, and uploaded with mipmaps. `inspect().backdrop` reads `loading` meanwhile. Every other case is `translucent`, with the reason in `inspect().backdropReason`. The gel then uses a fixed wallpaper-like tone in place of the refracted image, keeps at least 90% opacity within 8.5 px of the frame (falling to zero by 13 px), and turns see-through where it is thin. The Canvas 2D renderer is always translucent. In texture mode, pixels inside an under-rectangle (lower windows, desktop icons and widgets) are drawn translucent instead of refracted. The backdrop is checked again when the body, the live wallpaper or the overlay changes, and 350 ms, 1.2 s and 3.5 s after start.

**Material.** The gel is a height field over each frame: a dome that rises from the outline, a wetting dip at the frame, three slow flow waves and a fine relief, with normals from finite differences of the field (threads use their own round section). Each pixel refracts the backdrop, frosted with thickness, absorbs light per colour channel with depth, and adds a milky lime in-scatter glow and a dark refracted rim. Reflections come from a soft box and a sky, weighted by Fresnel. On top sit a sharp and a broad highlight, a thin white line on the lit flank, a lime caustic band on the side away from the light, a dark contact line with a meniscus glint at the frame, and a soft cast shadow with a lime fringe. Trapped air bubbles and sparse particles drift inside. Drops and splats use the same shading.

**Bubbles.** Each bubble passes through five stages. The stage lengths below are random within the ranges shown. Fast bubbles, from pokes and hard drag stops, use shorter stages with a 0.4–0.6 s grow.

| Stage | Length | What is drawn |
| --- | --- | --- |
| `nucleate` | 0.5–0.9 s | Gas pocket inside the gel: a lens with a clear centre and a dark rim |
| `grow` | 1.4–3.2 s | The dome rises and swells from the gel while the film thins |
| `hold` | 0.2–0.8 s | A quiver, with a faint thin-film tint at the apex |
| `burst` | 0.11–0.16 s | A hole opens above the apex, the rim retracts across the cap and 3–7 micro-droplets fly off |
| `crater` | 0.675–1.125 s | A depression with a damped ring and a small central rebound, then the gel heals |

Ambient bubbles follow a Poisson process with a mean interval of about 9 s per window. A window qualifies after 1.5 s, when it is fully open and not being dragged. There are at most 2 bubbles per window and 5 on the page. Sites favour thick parts of the gel (bottom, lower sides) and avoid corners, the area within 30 px of a drip root and hidden areas. A poke starts a small fast bubble 0.3 s later with a 60% chance, at the press point. A hard drag stop starts one or two.

**Interactions.** All pointer handling is passive and motion-only.

- **Hover lean:** a slow pointer within 60 px of the gel makes the nearest part lean up to 10 px toward it. The lean fades as the pointer speeds up and is gone by about 480 px/s.
- **Wake:** a pointer faster than 320 px/s crossing the gel sends a ripple through it. The strength grows with the speed.
- **Poke:** a left press on visible gel dents it and sends a ring ripple. Presses on windows, icons, widgets, the taskbar, menus or dialogs belong to the desktop and do not poke.
- **Strand pull:** after a poke, dragging the press past its threshold (at least 18 px) draws a thread from the gel. The thread thins as it lengthens and breaks into 2–4 beads when very thin. It snaps past 170 px, on release, or 2.5 s after it was pulled out. The tip becomes a drop, one or two satellites fall, and the base recoils with a short overshoot.
- **Drop merge:** a falling drop that enters another window's gel merges into it and sends a ripple. A drop cannot merge into the window it came from during its first 0.6 s.
- **Taskbar splat:** a drop that reaches the taskbar's top edge lands as a splat. It spreads with a short wobble and fades over its last 1.2 s of a 2.2–4 s life. Splats that touch merge.
- **Drag fling:** dragging a window swings its drips like pendulums. A hard stop (speed from at least 520 px/s down to below 40 %, at most once per 1.2 s per window) flings ripe drips, 2–4 droplets and 1–2 small bubbles off the leading edge.
- **Open ooze:** a newly opened window's gel oozes out from nothing to full size in about 0.5 s, with a slight overshoot. A closing window drops five droplets from its bottom corners.
- **Sounds:** a burst and a snap are the only events that make a sound (see *Effect sounds* above). The simulation queues `{ type: 'burst', x, y, base }` when a bubble bursts and `{ type: 'snap', x, y, length }` when a strand parts, at most 12 unread. The bridge drains them after each update, and `soundCue()` maps them to `slime.pop` (small below 6 px base radius, medium below 9 px, otherwise large) or `slime.snap` (small below 60 px length, medium below 110 px, otherwise large), with a strength of 0.35 to 1 that grows with the size. The bridge adds a pan from x (at most 0.8 either way) and calls `DesktopSounds.play`; it does nothing in static mode, while the page is hidden, or when the sound system is missing. Bursts behind a higher window or an overlay, spray, drops, splats, craters and the still bubble of static mode are silent.

**Budgets.**

- Free drops 72 and spray 24, in separate pools. The oldest drop is removed when a pool is full. Splats 12.
- Bubbles 5 on the page and 2 per window. One strand at a time.
- Occluders 24 per window: windows at or above its z-order, and solid overlay masks. Overflow merges into one bounding box, so content stays protected. Under-rectangles 16.
- WebGL2 draws the 16 topmost windows. Each window uses up to 5 strip quads and never a full-window quad. Drops and spray share one instanced draw, splats another. The stencil holds up to 1024 overlay rectangles.
- Gel radius 10–44 px (`MIN_GEL_RADIUS`, `MAX_GEL_RADIUS`). Spring node displacement up to 18 px.
- Device pixel ratio capped at 1.5 in motion and 1.25 in static mode. Backdrop textures at most 2048 px per side.

**Static mode.** When `options.motion` is false (Window animations is off or reduced motion is requested), the effect renders once per change, such as a resize, a scroll, a transition or a window event, instead of running a loop. The gel is drawn at full strength at once, with no fade-in. It uses the same material, with no pointer listeners, drops, strands or ambient bubbles. Each window keeps one deterministic, half-grown bubble. Motion-only commands return `null`.

**Commands and diagnostics.** The host's `command()` reaches these Green Slime commands:

- `command('bubble', { windowId?, at?: { x, y }, base?, fast? })` spawns a bubble now and returns its id, or `null`.
- `command('poke', { x, y })` applies a poke at that point and returns the window id, or `null`.
- `command('drop', { x, y, vx?, vy?, r? })` adds a falling drop and returns `true`, or `null` without numeric `x` and `y`.
- `command('quality', { scale })` pins the render scale between 0.25 and 1, turns off the adaptive watch and returns the scale. It also works in static mode.

`inspect()` returns `ready`, `renderer` (`webgl2` or `canvas2d`), `backdrop` (`texture`, `translucent` or `loading`), `backdropReason`, `qualityScale`, `contextLost` and `error`, which holds the reason for a fallback. WebGL2 timings and counters are `fade`, `gpu`, `compileMs`, `blockedMs`, `parallelCompile`, `firstRenderMs`, `lastRenderMs`, `averageRenderMs`, `drawCalls` and `quadCount`. Frame and mask counters are `frameCount`, `rafPending`, `windowCount`, `maskCount` and `maskRefreshCount`; `soundEvents` counts the sounds handed to `DesktopSounds.play` (the sound system decides what is heard). Simulation counters are `dropCount`, `activeDrops`, `sprayCount`, `activeSpray`, `splatCount`, `activeSplats`, `absorbedDrops`, `bubbleCount`, `activeBubbles`, `bubbleBursts`, `bubbleStages` (live per stage), `bubbleStageEntries` (entries per stage since start), `strandCount`, `strandSnaps`, `activeStrands`, `pokeCount`, `eventCount` (bursts and snaps queued for sound), `rippleEnergy`, `surfaceCount`, `surfaceNodeCount` (windows × 32) and `maxDisplacement`.

### Voxel World

Voxel World has two layers: a live 3D island behind the shell, and pixel-art block frames around windows. Its effect is registered as `voxel_world` and declares `staticFx: true`, so reduced motion keeps a still frame (see *Effects API*).

**Island.** With the theme wallpaper, a 3D voxel diorama renders on its own canvas, `.vd-fx-voxel-scene`. The canvas is fixed at z-index -1, on the same layer as the live wallpapers, so it sits behind the whole shell. The island comes from a procedural generator with a fixed seed:

- a snowy stone mountain with pines;
- a lake with a dock and a rowing boat;
- a river that falls over the rim as a waterfall into a sea of voxel clouds, with spray, foam, mist and a daytime rainbow;
- a village of three houses with chimney smoke;
- a windmill with turning sails, a wheat field, flowers, lanterns and hanging vines;
- an underside of stone, ore and glowing crystals.

Two small islets bob nearby.

**Look.** Sun shadows, per-vertex ambient occlusion, subtle texel noise, drifting cloud shadows and depth haze. The top quality level adds a tilt-shift blur and a vignette.

**Day and night.** Dawn, day, golden hour, dusk and night follow the local clock. A pixel sun and moon cross the sky. Stars and shooting stars appear at night, when windows, lanterns and crystals glow and fireflies drift. A bird flock circles by day.

**Interaction.** The pointer turns the camera gently (parallax), and the idle camera breathes. A left click on the empty desktop knocks loose a few grass and dirt blocks with physics and plays `window.impact` (earth), at most once per 350 ms. A mouse or pen click does it on press. A touch does it on release, and only for a tap: it must end within 500 ms, move less than 8 px and not be cancelled, so the start of a pan or a pinch does nothing.

**Window frames.** Each app's frame is built from pixel-art blocks in one of three materials:

| Material | Design |
| --- | --- |
| Oak planks | Log-end corners, with the boards running along the edge |
| Cobblestone | Smooth-stone corners |
| Dirt | A grass top edge |

- Each 8×8 pattern is drawn crisp, with a baked bevel.
- On open, the tiles drop in around the perimeter over 620 ms, with dust at the corners.
- On close, the window crumbles into textured Rapier fragments. They pile up on the taskbar line and fade.
- A fast drag shears tiles off the frame, and the frame repairs itself. The holes stay open while the drag goes on, but at most 3 s after its last drag or resize event.
- A window that leaves the desktop snapshot while it assembles (space switch, minimise) drops its assembly at once, so the overlay loop can go idle.
- Closing a window and shearing tiles play `window.fracture`, as listed under *Effect sounds*.

**Shell.** The shell is styled like an inventory: a hotbar taskbar with bevelled slots and a white frame on the active window, a grass-block start button, oak-plank titlebars with a name plate, stone window buttons with a redstone close button, grey bevelled panels for menus, the start menu, toasts and dialogs, and light parchment window content.

**Wallpaper and fallback.** The image `/img/wallpapers/themes/voxel_world.png` is a render of the island. It shows while the scene loads and wherever WebGL is unavailable. With any other wallpaper, the theme keeps only the block frames and fragments and builds no scene.

**Performance.**

- The ambient loop runs at about 30 fps.
- The quality ladder follows the achieved frame cadence:

| Level | Behaviour |
| --- | --- |
| Q0 | Tilt-shift and a 2048 × 2048 shadow map |
| Q1 | A 1024 × 1024 shadow map |
| Q2 | No shadows, life effects at half scale |
| Q3 | Static, with a probe render every 5 s that can step back up |
| Q4 | Poster only |

- The scene canvas has a pixel budget per level: Q0 3.7 million, Q1 3.0 million, Q2 2.2 million, Q3 1.6 million pixels. The pixel ratio is the lowest of the device ratio, the level's cap (1.5, 1.25, 1, 0.85) and the ratio at which the window fits the budget; CSS scales the canvas up.
- Multisampling is requested only when the window at a pixel ratio of at most 1.5 fits 3.7 million pixels when the scene is built.
- The loop drops to 2 fps while a window covers 96% or more of the screen.
- It pauses while the page is hidden.
- With reduced motion or disabled animations, the scene shows one still frame for the current time of day, refreshed every minute.
- Software GL (SwiftShader, llvmpipe and similar) is refused, and the poster wallpaper shows. The scene needs a hardware WebGL context.

**Commands.** Tests and screenshots drive the scene through `AuraDesktopThemeFx.command(name, payload)`:

- `voxel.time` `{hour: 0..24 | null}` sets or clears a fixed hour.
- `voxel.freeze` `{t: seconds | null}` freezes the life clock.
- `voxel.quality` `{level: 0..4 | null}` locks the quality level.

`AuraDesktopThemeFx.inspect().scene` reports `backend`, `gpu`, `quality`, `pixels` (the size of the canvas backbuffer), `fps`, `frames`, `buildMs`, `hour`, `phase`, `night`, `triangles`, `voxels`, `shadows`, `tiltShift`, `throttled`, `anchors` and the life counts.

**Files.** All in `ui/js/desktop/theme-fx/`:

- `voxel-sky.js`: time-of-day model
- `voxel-island.js`: generator
- `voxel-mesher.js`: ambient-occlusion mesher
- `voxel-blocks.js`: pixel-art atlas and material patches
- `voxel-shaders.js`: sky and tilt-shift GLSL
- `voxel-life.js`: clouds, waterfall, windmill, smoke, birds, fireflies, halos, islets and the boat
- `voxel-scene.js`: renderer, camera, light and quality
- `voxel-graphics.js`: overlay frames, fragments and dust
- `voxel-fragments.js`: the pool of fragment bodies (create, reuse, recycle)
- `voxel-runtime-entry.js`: entry of the vendor bundle that loads Three.js and Rapier
- `voxel_world.js`: orchestrator, registered with fx-core

**Tests.**

- `node scripts/test-ui-regressions.mjs` covers the generator, the mesher and the sky. It needs no browser.
- The browser tests in `ui/desktop_experience_themes_browser_test.go` need `AURAGO_RUN_BROWSER_SMOKE=1`.
- The screenshot run writes day-cycle shots, close-ups and frame, assembly and fracture shots to the ignored `reports/desktop-experience-themes/voxel/` folder:

```bash
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_THEME_RECORDINGS=1 go test ./ui -run '^TestDesktopVoxelWorldCloseUpsBrowser$' -count=1 -timeout 10m
```

### Orbit

Orbit has three layers: a live 3D space scene behind the shell, a Canvas 2D particle layer and 3D window motion. Its effect is registered as `orbit` and declares `staticFx: true`, so reduced motion and disabled animations keep a still frame (see *Effects API*). Windows are flat at rest. The third dimension appears only in motion: while a window opens, closes, moves, minimizes or restores, and while menus and modals open.

**Scene.** The scene draws on its own canvas, `.vd-fx-orbit-scene`, at z-index -1, so it sits behind the whole shell. It is built only with the Orbit wallpaper and fades in when its first frame is drawn. It shows:

- a gas giant with banded clouds, storms, night-side lights, lightning, an atmosphere glow and aurorae;
- a ring system with a shadow and dust, and a moon;
- a nebula sky, three star layers, the sun with lens flares, comets and meteors;
- a space station with a turning habitat ring and beacons;
- a shuttle on a 96 s cycle (docked, undock, cruise, approach and dock), with engine glow and exhaust;
- a satellite that crosses the foreground;
- an asteroid belt drawn as instanced rocks.

Post-processing adds bloom, a vignette, chromatic aberration, film grain and a scanner pulse. The planet's surface and the nebula are baked once into textures. The cloud decks turn with a differential shear that swings back and forth on a 360 s period, so the shear stays bounded (about 7 degrees) and the planet looks the same after hours. The sky sways about half a degree on a 900 s period.

**Models.** Four Blender models load from `ui/3d/desktop-theme-orbit/v1/`: `station.glb`, `shuttle.glb`, `satellite.glb` and `asteroids.glb`. `assets/desktop-theme-orbit/generate.py` (Blender 5.2, deterministic) builds them. `manifest.json` records the SHA-256 hash, size and triangle count of each file. Every model is optional. A model that fails to load is reported as `error`, and the rest of the scene keeps working.

**Window choreography.** `orbit-windows.js` turns the `aurago:desktop-window` events into motion. It never changes application state or intercepts input.

- **Open:** the window flies in from depth over 620 ms. A hologram scan line crosses its content once, and particles converge on its frame.
- **Close:** the window tilts back and falls into depth over 520 ms, and particles burst from its frame.
- **Minimize and restore:** the window folds to its taskbar slot over 460 ms and comes back over 480 ms. Particles stream along the path.
- **Drag:** a spring tilts the window by up to 9° from its drag velocity. The runtime sets `data-orbit-tilt` and `--vd-orbit-tilt-x`, `--vd-orbit-tilt-y` and `--vd-orbit-lift`, and a thruster glows at the trailing edge. After release the window settles flat and the properties are removed.
- **Modals** open like a hatch over 420 ms, above a backdrop that fades in. **Context menus** unfold with their rows in a stagger. The **start menu** appears as a hologram with a light cone over 380 ms.
- A **glint** circles the frame of the focused window every 7 s.
- A silent event, such as a restoration, emits no particles and no sound.

**Particles.** `orbit-particles.js` draws up to 900 particles on a Canvas 2D overlay, `.vd-fx-orbit-particles`, in the effect layer. The pool is a ring buffer: when it is full, the oldest particle is overwritten. The particles are additive and come in cyan, violet and amber. Up to four sonar rings run at once. The emitters are materialize, dissolve, stream, thruster, spark and stardust. Each emitter draws from a seeded generator, so a given sequence repeats.

**Interaction.**

- A click on the empty desktop sends a sonar ping and an `orbit.ping` sound (medium, panned to the click position). Pings are at least 350 ms apart. Clicks on windows, icons, widgets, bars, menus, dialogs and controls do nothing. While the live scene is on screen, the scene's scanner pulse (`inspect().scene.pulse`) is the only ring, and the particle layer adds 24 sparks only (`inspect().particles.rings` stays 0). Without a live scene (the poster, quality 4 or software GL), the particle layer draws the ring too. Motion-off mode has no particle layer.
- A mouse or pen pings on press. A touch pings only as a tap: it must end within 500 ms and move less than 8 px.
- Pointer travel over the empty desktop leaves faint stardust motes, one for each 18 px.
- The pointer moves the camera slightly (parallax). It also sets the pointer sheen through `--vd-orbit-mx` and `--vd-orbit-my` on `<body>`.

**Shell.** The skin is dark glass: deep-blue panels with cyan and violet hairlines, and a HUD taskbar. The signal colour is cyan (`#5CE1FF`), with near-black text on it. Barlow Condensed SemiBold sets the uppercase dialog, popup and panel headings. Geist sets the text.

**Wallpaper and fallback.** The *Orbit* wallpaper is the poster `/img/wallpapers/themes/orbit.png` (set in `css/desktop-wallpaper-themes.css`): a still frame of the scene at t = 44 s, from the neutral camera and without film grain. It shows while the live scene loads, without WebGL, with software GL and at quality 4. The live canvas fades in over it. With any other wallpaper, the theme keeps the particles and the window motion and builds no scene.

**Forest wallpaper.** Organic suggests *Forest*, a misty conifer forest at dusk with a low amber sun right of centre and the left edge kept dark for the desktop icons. It is the vector file `/img/wallpapers/themes/forest.svg` (set in `css/desktop-wallpaper-themes.css`), written by the deterministic generator `assets/desktop-wallpaper-forest/generate.mjs` (`node assets/desktop-wallpaper-forest/generate.mjs`); edit the generator, not the SVG.

**Performance.**

- The scene runs at about 30 fps at Q0 and Q1, and at about 24 fps at Q2.
- The quality ladder follows the frame cadence, as in Voxel World. The scene steps down when frames run slow and steps back up after a sustained run of fast frames. Large screens start at Q1 (above about 4.2 million pixels at the capped pixel ratio).
- The pixel ratio is the lowest of the device ratio, the level's cap and the ratio at which the window fits the level's budget (3.7, 3.0, 2.2 and 1.6 million pixels for Q0 to Q3).
- The loop drops to 2 fps while a window covers 96% or more of the usable desktop, the viewport without the taskbar band.
- The loop pauses while the page is hidden.
- The `<body>` carries `data-orbit-quality` with the current level (see *Body attributes*). From Q2 the window frames drop their backdrop blur and get a more opaque fill.

| Level | Behaviour |
| --- | --- |
| Q0 | Full bloom, two-sample multisampling, a pixel ratio of up to 1.5, 140 asteroids |
| Q1 | Half bloom, a pixel ratio of up to 1.25, 90 asteroids |
| Q2 | No bloom, a pixel ratio of up to 1, 50 asteroids, reduced planet detail and half the stars |
| Q3 | Static, with a pixel ratio of up to 0.85 and a probe render every 5 s that can step back up |
| Q4 | Scene hidden; the poster shows |

**Fallbacks.**

- With reduced motion or disabled animations, the scene shows one still frame (t = 44 s) and refreshes it every 60 s. Without motion there are no particles and no window bridge.
- Software GL is refused: SwiftShader, llvmpipe, lavapipe, softpipe, or a GPU string that contains "software" or "basic render". The scene stays hidden and the poster shows. `window.__auraOrbitSoftwareGL = true` overrides the check, and the browser tests use it.
- If WebGL is missing, the scene fails to build or the vendor bundle does not load, the poster stays. A lost WebGL context hides the scene, and the scene returns when the context is restored.
- The particle layer and the CSS window motion do not need WebGL.

**Commands.** Tests and screenshots drive the effect through `AuraDesktopThemeFx.command(name, payload)`. The scene commands return `null` when the Orbit wallpaper builds no scene.

- `orbit.time` `{t: seconds | null}` freezes the scene clock at `t`, or releases it.
- `orbit.quality` `{level: 0..4 | null}` locks the quality level, or releases the lock.
- `orbit.capture` `{width, height, grain}` returns the scene as a PNG data URL, or `null` before the scene is built. The default size is 1920 × 1080, at most 4096 px per side. `grain` sets the film grain for that capture (the poster uses 0); without it the scene's own grain stays.
- `orbit.ping` `{x, y}` sends a ping at that point, even inside the 350 ms interval. It returns `true`, or `null` without numeric coordinates.
- `orbit.burst` `{kind: 'materialize' | 'dissolve', windowId}` emits the burst over that window's rectangle. It returns `true`, or `null` for an unknown window or kind.

`AuraDesktopThemeFx.inspect()` reports `scene` (backend, GPU, quality, pixels, fps, frames, build time, triangles, models, bloom, throttled and coverage), `particles` (live, emitted, capacity and rings) and `windows` (tracked and tilting).

**Files.** All in `ui/js/desktop/theme-fx/` unless noted. The `desktop-theme-fx` bundle loads the Orbit parts after `voxel_world.js`, in this order: `orbit-choreo`, `orbit-particles`, `orbit-shaders`, `orbit-textures`, `orbit-models`, `orbit-scene`, `orbit-windows` and `orbit`.

- `orbit-choreo.js`: DOM-free schedules (the shuttle cycle, comets, meteors and lightning), the camera composition, the spring and the drag tilt.
- `orbit-particles.js`: the DOM-free particle pool and its Canvas 2D renderer.
- `orbit-shaders.js`: the GLSL sources.
- `orbit-textures.js`: the noise and ring textures computed on the CPU.
- `orbit-models.js`: loads and animates the four models.
- `orbit-scene.js`: the WebGL renderer, camera, post chain and quality ladder. It is about 1,060 lines. Keep it under 1,100 and put new code in a part of its own.
- `orbit-windows.js`: the window bridge (springs, tilt and thrusters).
- `orbit-runtime-entry.js`: entry of the vendor bundle.
- `orbit.js`: the orchestrator, registered as `orbit`.
- `ui/js/vendor/desktop-themes/orbit-runtime.js`: the vendor bundle with Three.js 0.186.1, GLTFLoader and the post passes, built by `scripts/build-desktop-theme-vendor.mjs`. It loads only when the scene starts.
- `ui/css/desktop-theme-orbit.css`: the skin.
- `ui/img/wallpapers/themes/orbit.png`: the poster, regenerated by `TestDesktopOrbitPosterBrowser` (see *Tests*).
- `ui/js/desktop/sound/theme-orbit.js`: the sound bank.
- `assets/desktop-theme-orbit/generate.py` and `ui/3d/desktop-theme-orbit/v1/`: the model source, the GLBs, the manifest and the licence.

**Tests.**

- `node scripts/test-ui-regressions.mjs` covers the choreography and the particle system. It needs no browser.
- `node scripts/test-desktop-theme-orbit-assets.mjs` checks the GLBs against `manifest.json`. `npm run check:desktop-themes` runs it too.
- The browser tests in `ui/desktop_theme_orbit_browser_test.go` need `AURAGO_RUN_BROWSER_SMOKE=1`: `TestDesktopOrbitBrowser`, `TestDesktopOrbitReducedMotionBrowser` and `TestDesktopOrbitSoftwareFallbackBrowser`. The other tests set `window.__auraOrbitSoftwareGL`, so the scene runs on SwiftShader. That is slow, so these tests check behaviour, not frame rate. The software-fallback test does not set the flag and checks that the scene is refused.
- `TestDesktopOrbitCloseUpsBrowser` writes review images to the ignored `reports/desktop-experience-themes/orbit/` folder when `AURAGO_THEME_RECORDINGS=1` is set. Hardware frame-rate acceptance is separate from these tests.
- `TestDesktopOrbitPosterBrowser` regenerates `ui/img/wallpapers/themes/orbit.png` from the live scene at t = 44 s with no grain. It needs `AURAGO_RUN_BROWSER_SMOKE=1` and `AURAGO_ORBIT_POSTER=1`. It re-encodes the PNG with the best compression when the capture is over 2.5 MB, and it fails above 2.8 MB. Run it after visible scene changes.

### Minimal

Minimal is a light or dark theme of paper or graphite, hairlines and typography. It has no effect layer (`fx: false`). Its scheme follows `appearance.minimal_mode`, which is Light by default. Settings shows **Minimal appearance** while the theme is selected. The accent is the ink itself: `#161616` in Light and `#EDEDED` in Dark.

- **Type.** Geist. Window titles are uppercase with wide tracking.
- **Surfaces.** Windows are flat, with no blur and no gradients. One soft shadow lifts the active window and the floating panels. Everything else is a hairline.
- **Icons.** Grayscale at rest. They take their colour under the pointer.
- **Motion.** Calm fades of 100 to 200 ms, with a few pixels of travel.
- **Wallpaper.** *Minimal* is a CSS gradient of paper or graphite with a faint light. It follows the scheme.
- **Cursors and sounds.** System cursors and the Crystal sound set.

The skin file has two root rules. The plain root rule holds the Light palette, and `.desktop-body[data-theme="minimal"][data-scheme="dark"]` holds the Dark palette (see *Token Contract*).

### Add a Theme

1. Add one descriptor line to `DESKTOP_THEMES` in `ui/js/desktop/core/theme-runtime.js`, keys in registry order, with `thumb: '/img/themes/<id>.svg'`.
2. Add the same id at the same position in `DesktopThemeIDs()` in `internal/desktop/themes.go`.
3. Create `ui/css/desktop-theme-<id>.css` with the root rule and the full token set. List it in the `desktop-shell.bundle.css` entry of `cssBundles` in `scripts/build-ui-bundles.js`, after `desktop-wallpaper-live.css` and before `desktop-polish.css`.
4. If `schemeSetting` is set, the skin also needs the dark root rule `.desktop-body[data-theme="<id>"][data-scheme="dark"]` that sets `color-scheme: dark` (see *Token Contract*). A new scheme setting also needs its key in the allowed list of `ui/desktop_theme_registry_test.go`, a light and dark setting in `internal/desktop/types.go` and a label in `SCHEME_MODE_LABELS` in `ui/js/desktop/apps/settings.js`.
5. If `fx: true`, create `ui/js/desktop/theme-fx/<id>.js` and list it in the `desktop-theme-fx` bundle after `fx-core.js`. A theme with several parts lists them in order, with the entry file last (Orbit has eight parts, with `orbit.js` last).
6. Create `ui/img/themes/<id>.svg` (schematic, at most 8192 bytes).
7. Add `desktop.settings_theme_<id>` and `desktop.settings_theme_<id>_desc` to all 16 `ui/lang/desktop/*.json` files.
8. Run `npm run build:ui`, then the tests below.

### Tests and Commands

```bash
npm run build:ui                                   # regenerate the bundles
npm run check:ui                                   # fail when a generated bundle is stale
npm run check:desktop-themes                       # cursors, XP icons, pinned vendor and UI bundles
go test ./ui -run DesktopTheme -count=1            # registry, CSS contract, thumbnails, bundle order, layering
node scripts/test-ui-regressions.mjs               # Node suite, no browser; includes the Green Slime simulation, Orbit choreography and particles
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run DesktopTheme -timeout 30m
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '(TestDesktopGreenSlime|TestDesktopLivingMaterials|TestDesktopExperience)' -count=1 -timeout 30m
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_THEME_RECORDINGS=1 go test ./ui -run '^TestDesktopGreenSlimeCloseUpsBrowser$' -count=1 -timeout 10m
node scripts/test-desktop-theme-orbit-assets.mjs    # Orbit model pack against its manifest (also in check:desktop-themes)
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '(?i)TestDesktopOrbit(Browser|ReducedMotionBrowser|SoftwareFallbackBrowser)$' -count=1 -timeout 20m
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_THEME_RECORDINGS=1 go test ./ui -run '^TestDesktopOrbitCloseUpsBrowser$' -count=1 -timeout 15m
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_ORBIT_POSTER=1 go test ./ui -run '^TestDesktopOrbitPosterBrowser$' -count=1 -timeout 15m    # regenerates ui/img/wallpapers/themes/orbit.png
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_THEME_SWEEP=fruity,crazy go test ./ui -run TestDesktopThemeSweepBrowser -timeout 30m
go test ./ui -run TestDesktopAuditAppearanceBrowser -count=1
```

- The first command needs no browser. The sweep and Crazy reduced-motion tests skip unless `AURAGO_RUN_BROWSER_SMOKE=1` is set. The theme browser tests need Chrome or Edge.
- `AURAGO_THEME_SWEEP` limits the sweep to some registry ids. Sweep screenshots go to `reports/desktop-theme-sweep/`.
- `TestDesktopAuditAppearanceBrowser` writes the Standard and Fruity screenshots to `reports/desktop-audit-browser/`. A change to shared Standard or Fruity tokens must leave them pixel-identical.
- `TestDesktopCrazyThemeHonoursReducedMotionBrowser` checks Crazy's reduced-motion behaviour in a browser.
- The experience tests (`TestDesktopExperience*`) verify close guards, atomic settings, real geometry/rotation/Rapier collisions, pixel occlusion, static fallbacks and cleanup. `TestDesktopLivingMaterialsIdleBrowser` runs Green Slime and Electronic Dreams: their material must keep changing at idle, and Green Slime's drips must detach without input.
- `TestDesktopGreenSlime*` checks the slime springs, the attached membrane (WebGL2 on the green_slime wallpaper, WebGL2 translucent on another wallpaper, and Canvas 2D) and the six-window budget. Pixel checks copy the effect canvas with `drawImage` first, so they read either renderer.
- The WebGL2 subtests of the membrane test and the interaction and close-up tests need a GPU. They skip when the browser reports SwiftShader, llvmpipe, lavapipe, softpipe or another software renderer, because the gel falls back on software GL by default. The Canvas 2D subtest always runs.
- `TestDesktopGreenSlimeInteractionBrowser` runs one WebGL2 stage (settings and files windows at 1920×1080) with five subtests, in order: a forced bubble through all five stages (the stage counters rise, 3–7 micro-droplets per burst, at most 5 bubbles and 2 per window, all healed, and every visible burst asked a stubbed `DesktopSounds.play` for one `slime.pop`); a strand pulled with the real mouse (it snaps and leaves a drop, the window does not move, no marquee appears, one `slime.snap` is requested with a pan to the left); drops that merge into a lower window and splat on the taskbar edge until the splat clears (silent); a burst heard through the real sound system, with desktop sounds on and the sound theme following Green Slime; and a forced WebGL context loss that must end on Canvas 2D with one canvas and one layer. It needs `AURAGO_RUN_BROWSER_SMOKE=1`.
- `TestDesktopGreenSlimeCloseUpsBrowser` writes review images into the ignored `reports/desktop-experience-themes/slime/`: 1× full frames and 2× to 8× crops of the frame, corners, edges, drips and overlaps; one bubble through its stages, with contact sheets; the burst; a strand; drops; a taskbar splat; static crops; and `gl-ref-*` crops of the wallpaper's own gel beads for comparison. It needs `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_THEME_RECORDINGS=1`, a GPU and about a minute. It asserts that every stage is reached and that the page reports no errors, not how the images look. The `TestDesktopGreenSlime` pattern in the command block also matches it; without `AURAGO_THEME_RECORDINGS=1` it skips.
- The membrane and six-window tests write `green-slime-attached-<label>.png` (labels `webgl2`, `webgl2-translucent`, `canvas2d`), `green-slime-corner-<label>.png` and `green-slime-six-windows.png` to the ignored `reports/desktop-experience-themes/` folder. The close-ups go to the ignored `reports/desktop-experience-themes/slime/` folder. `AURAGO_THEME_RECORDINGS=1` gates the close-ups and the motion GIF test (`TestDesktopThemeMotionRecordingsBrowser`, into `reports/desktop-experience-themes/motion/`), not the membrane or six-window screenshots.
- Add `AURAGO_THEME_RECORDINGS=1` and run `TestDesktopThemeMotionRecordingsBrowser` to capture actual pointer-driven opening, dragging and closing as four GIFs in that report directory's `motion/` subdirectory. `AURAGO_BROWSER_MUTE_AUDIO=1` mutes the browser output while preserving Web Audio assertions.
- `AURAGO_RUN_DESKTOP_THEME_NEUTRALITY=1 AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopThemeNeutralityAgainstHeadBrowser -count=1` compares Standard and Fruity pixels with the committed HEAD bundles before committing a theme change. It covers desktop/narrow viewports and both Fruity schemes.
- On Windows, set task-local `GOCACHE`, `GOTMPDIR`, `TEMP` and `TMP` if the default temporary directories are restricted. `AURAGO_DESKTOP_THEME_VENDOR_NODE_MODULES` can point the vendor check at an existing installation of the repository's pinned npm dependencies.

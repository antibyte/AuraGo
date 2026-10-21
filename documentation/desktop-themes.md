# Desktop Themes

The Virtual Desktop has twelve themes. A theme sets the shell layout (taskbar or dock), the colour scheme, the icon set, the look of windows, menus and controls and, for some themes, subtle visual effects. Apps keep their own content and take their colours from the shared theme tokens.

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
| Voxel World | Taskbar | Light | Papirus | Fixed forest green | Grass, earth, stone and timber blocks in warm daylight. |

"Your accent" means the Accent color setting applies. Fruity's scheme follows its own Light/Dark setting.

## Choosing a Theme

- Open Settings, category **Appearance**, and pick a card under **Theme**. A click, Enter or Space selects it.
- A theme also sets the **Icon theme** (Papirus, WhiteSur or Windows XP). You can change the icon set afterwards; picking a theme pairs its icon set again.
- The wallpaper follows the theme while it is still the previous theme's suggestion, or the default *Group Shot*, in every space. Suggestions: Amiga *Slate*, Cyberpunk *City Rain*, Black Gloss *Midnight*, Crazy *Fractal Trip (animated)*, Lollipop *Paper Waves*, Organic *Forest*, Electronic Dreams *Electronic Dreams*, Windows XP *Windows XP Bliss*, Green Slime *Green Slime* and Voxel World *Voxel World*. Standard and Fruity use *Group Shot*. A wallpaper you picked yourself is never changed.
- **Fruity appearance** (Light or Dark) appears only while Fruity is selected. The other themes have a fixed scheme.
- **Accent color** (teal, orange, blue, violet or green) applies to Standard, Fruity and Black Gloss. The other nine themes use their fixed accent and hide the row.
- The settings are `appearance.theme`, `appearance.icon_theme`, `appearance.fruity_mode`, `appearance.accent`, `appearance.wallpaper` and `appearance.wallpaper_by_space`. The UI saves them through `PUT /api/desktop/settings`.

## Accessibility

- **Reduced motion.** When the system requests reduced motion (`prefers-reduced-motion: reduce`), a global override ends every transition and animation at once: durations drop to 0 ms and animations run a single iteration. The shell also skips its window, menu and start-menu transitions, and Crazy's confetti and sparkles stop.
- **Window animations.** Settings, category **Windows**, *Window animations* turns off theme animations and animated effects; themes with a static effect layer retain that layer. This setting also applies when the system asks for reduced motion.
- **Contrast.** Body text on the app and panel backgrounds, and muted text on the app background, are tested to a contrast ratio of at least 4.5:1 for every skinned theme.
- **Focus.** Keyboard focus shows a ring in every theme. Amiga draws a dashed black outline and Crazy a cyan ring.
- **Amiga window gadgets.** The Amiga skin keeps the authentic Workbench gadget layout (close far left, depth and zoom gadgets on the right) purely visually through CSS grid placement, so keyboard Tab order follows the DOM (AI, minimize, maximize, close; AI and maximize are omitted where a window lacks them) rather than the on-screen order, and the dashed black focus ring makes the focused gadget visible.
- **Windows XP Start menu.** The Luna-style Start menu uses two columns: the app list fills the left pane, with system links and categories stacked on the right. The CSS adapts the column widths on narrow screens while keeping each action reachable.
- **Theme motion.** Electronic Dreams' signal light and the Green Slime and Voxel World effects switch to a still rendering when *Window animations* is off or reduced motion is requested. The Windows XP, Green Slime and Voxel World skins keep their gradients, glossy surfaces and block edges when glass polish is off.
- Touch: on a coarse pointer the global menu bar grows to 44 px and every tray control fills it; below 1024 px the window menus move to a second 44 px row and the bar is 88 px (`css/desktop-chrome.css`). Amiga sizes its gadgets from the bar height in its skin and keeps a single row.

## Developer Guide

### Matching cursors and sounds

Amiga, Cyberpunk, Black Gloss, Crazy, Lollipop, Organic, Electronic Dreams,
Windows XP, Green Slime and Voxel World include original cursor artwork and
individual sound sets. In **Settings → Appearance**, **Theme cursors**
switches between the theme artwork and system cursors. There are twelve cursor
roles per theme, including text, links, drag, resize and busy states. High-contrast
mode and touch-only devices retain system cursors. App canvases and remote desktop
surfaces keep their own tool cursors. Standard and Fruity retain system cursors.

In **Settings → Sound**, enable **UI sounds** to hear the feedback. Sounds remain
off by default. **Follow desktop theme** automatically picks the matching sound
set (Crystal for Standard and Fruity); a manually chosen set remains selected
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
| Green Slime | Glossy lime slime pointer | Low-pass wet plips, elastic sweeps and soft squelches |
| Voxel World | Block-edged pointer in earth and green tones | Dry taps and wood, stone and earth impacts |

`cursorTheme` and `soundTheme` on each registry descriptor own the pairing.
`core/cursor-runtime.js` applies versioned, local SVG cursor URLs with native
hotspots and keyword fallbacks. `css/desktop-cursors.css` preserves semantic
cursor roles without a pointer-following overlay or animation loop.
The 120 cursor SVGs, ten cursor preview sheets and ten theme sound-set covers are original AuraGo artwork, generated
by `node scripts/build-desktop-cursors.mjs`; use `--check` to verify reproducibility.
No third-party cursor artwork or sampled operating-system sounds are included.

Electronic Dreams, Windows XP, Green Slime and Voxel World each have a separate
sound bank in `sound/theme-electronic-dreams.js`, `sound/theme-windows-xp.js`,
`sound/theme-green-slime.js` and `sound/theme-voxel-world.js`. Each bank supplies
the shell events through the shared lazy Web Audio synthesizer; the four themes
do not share one generic theme bank. In-flight synthesis is shared; mute, theme
changes and tab hiding cancel pending playback. Audio unlock remains bound to a
user gesture. Turning sound off releases the AudioContext immediately.
Verify `TestDesktopThemeSensoryBrowser`, `TestDesktopSoundBrowser` (all fifteen
sets) and `TestThemeSensorySettingsPersistIndependently`, then rebuild UI bundles.

### Architecture

1. `DESKTOP_THEMES` in `ui/js/desktop/core/theme-runtime.js` describes every theme. `desktop.DesktopThemeIDs()` in `internal/desktop/themes.go` lists the same ids and is the allowed value list of `appearance.theme`.
2. `applyDesktopSettings()` in `ui/js/desktop/core/desktop-foundation.js` calls `applyDesktopThemeAttributes()`, which sets `data-theme`, `data-shell`, `data-scheme` and `data-polish` on `<body>`.
3. CSS keys off these attributes: `[data-theme]` in the skins and the shell-overrides bridge, `[data-shell]` in the layout rules (`desktop-base.css`, `desktop-windows.css`, `desktop-chrome.css`), and `[data-polish]` in `desktop-polish.css`.
4. JavaScript asks the capability helpers (`shellLayout()`, `themeUsesGlobalMenu()`, `themeScheme()`, `themeMotionMs(kind)`, `themeIconFamily()`) and never compares a theme id. Apps read `document.body.dataset.scheme`.
5. `syncDesktopThemeFx()` starts or stops the effects of an `fx` theme.

Layering in `ui/css/desktop-shell.bundle.css`: the shared shell files (`desktop-base.css` carries the Standard and Fruity core tokens), then `desktop-shell-overrides.css` (the `--vd-theme-*` bridge), `desktop-chrome.css`, `desktop-wallpaper-live.css`, `desktop-wallpaper-themes.css` (the four locally packaged theme wallpapers), the skins `desktop-theme-amiga.css`, `-cyberpunk`, `-black-gloss`, `-crazy`, `-lollipop`, `-organic`, `-electronic-dreams`, `-windows-xp`, `-green-slime` and `-voxel-world`, and last `desktop-polish.css`. `scripts/build-ui-bundles.js` assembles this bundle and the `desktop-theme-fx` bundle.

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

### Descriptor Reference

Each `DESKTOP_THEMES` entry is one `Object.freeze({ ... })` line with these keys in this order. The registry test parses the file as text, so keep single quotes and the key order.

| Key | Values | Meaning |
| --- | --- | --- |
| `id` | `^[a-z][a-z0-9_]*$`, not `light` or `dark` | `appearance.theme` value, `data-theme`, skin file name (`_` becomes `-`) and keyframe prefix |
| `labelKey`, `descKey` | desktop locale keys | card title and description |
| `scheme` | `light`, `dark` | colour scheme; must equal the skin's `color-scheme` |
| `schemeSetting` | empty or `appearance.fruity_mode` | the setting that picks the scheme; shows the Fruity appearance row |
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
- `command(name, payload)` forwards to the active effect's `command(name, payload)` and returns its result. It returns `null` when no effect is active, when the effect has no `command` method, when the call throws, or when the result is `undefined`. Tests and debugging use it to trigger effect actions, for example Green Slime's `bubble`, `poke`, `drop` and `quality` commands.
- An effect module `ui/js/desktop/theme-fx/<id>.js` registers `{ start }` under its theme id. `start({ layer, signal, options })` returns `{ pause, resume, stop }` and may add `inspect` and `command`; missing methods are no-ops.
- The host creates `.vd-theme-fx-layer` (fixed, full viewport, `pointer-events: none`, one z-index step below `--vd-z-dock`; the rule lives in `desktop-base.css`), an AbortController and a `visibilitychange` listener. A hidden tab calls `pause()` and a visible tab calls `resume()`.
- `stop()` runs before the signal is aborted and the layer is removed. Bind listeners to `signal` and release everything in `stop()`.
- `options.motion` separates material rendering from movement. Electronic Dreams, Green Slime and Voxel World declare `staticFx: true`: reduced motion and disabled animations keep a still frame with no continuous animation loop. Crazy remains entirely inert in this mode. Mobile desktop mode uses the CSS skins and local wallpaper without loading a material renderer.
- `window.AuraDesktopWindowEffects.snapshot()` exposes visible window geometry. The `aurago:desktop-window` event reports open, focus, drag/resize start/update/end, minimize, restore, maximize, snap and accepted close, with window ID, rectangle, z order, workspace, velocity and restoration-silence information. An `open` precedes focus; a rejected close never emits destruction. Effects must not mutate application state or intercept input.
- Voxel rendering lives in `voxel-graphics.js`; `voxel_world.js` owns physics and lifecycle. The separate `vendor/desktop-themes/voxel-runtime.js` ESM bundle loads Three.js and Rapier only when needed. Frame cubes use instancing and ranked depth occlusion; shell overlays and window contents are stencil masked. WebGL failure retains the Canvas/CSS fallback.
- Voxel physics advances at 60 fixed steps per second with at most 256 pooled fragments. Long frame intervals lower render resolution before reducing scenery and fragment budgets. Hidden tabs suspend work; switching themes releases observers, bodies, render targets and WebGL contexts. Hardware frame-rate acceptance is separate from the software-WebGL browser fixture.
- Budget pattern (`theme-fx/crazy.js`): pools of spans created once, one Web Animation per particle on `transform` and `opacity`, no per-frame loop and no timer. Crazy has 36 confetti and 14 sparkle spans, one confetti burst at a time, at most one sparkle every 33 ms, and no sparkles over app content, the taskbar, menus, dialogs or toasts.

### Green Slime

The Green Slime effect (`ui/js/desktop/theme-fx/green_slime*.js`) draws a translucent lime gel around each window frame, with drips, bubbles, strands and free drops. The simulation is DOM-free. The bridge in `green_slime.js` passes it window geometry, overlay masks and pointer input, and draws the finished scene with one renderer. The gel never paints over window content, higher windows or overlays. The layer has `pointer-events: none`, and the pointer listeners are passive, so the desktop keeps every event.

**Files and bundle order.** The `desktop-theme-fx` bundle loads these parts in this order, after `fx-core.js`, `crazy.js` and `electronic_dreams.js`:

1. `green_slime_geometry.js` (`AuraGreenSlimeGeometry`): constants, seeded RNG, rounded-rectangle perimeter and gel profile.
2. `green_slime_bubbles.js` (`AuraGreenSlimeBubbles`): bubble lifecycle.
3. `green_slime_touch.js` (`AuraGreenSlimeTouch`): hover, wake, poke and strand.
4. `green_slime_sim.js` (`AuraGreenSlimeSim`): surfaces, ripple wave, drops, splats and the scene.
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

`inspect()` returns `ready`, `renderer` (`webgl2` or `canvas2d`), `backdrop` (`texture`, `translucent` or `loading`), `backdropReason`, `qualityScale`, `contextLost` and `error`, which holds the reason for a fallback. WebGL2 timings and counters are `fade`, `gpu`, `compileMs`, `blockedMs`, `parallelCompile`, `firstRenderMs`, `lastRenderMs`, `averageRenderMs`, `drawCalls` and `quadCount`. Frame and mask counters are `frameCount`, `rafPending`, `windowCount`, `maskCount` and `maskRefreshCount`. Simulation counters are `dropCount`, `activeDrops`, `sprayCount`, `activeSpray`, `splatCount`, `activeSplats`, `absorbedDrops`, `bubbleCount`, `activeBubbles`, `bubbleBursts`, `bubbleStages` (live per stage), `bubbleStageEntries` (entries per stage since start), `strandCount`, `strandSnaps`, `activeStrands`, `pokeCount`, `rippleEnergy`, `surfaceCount`, `surfaceNodeCount` (windows × 32) and `maxDisplacement`.

### Add a Theme

1. Add one descriptor line to `DESKTOP_THEMES` in `ui/js/desktop/core/theme-runtime.js`, keys in registry order, with `thumb: '/img/themes/<id>.svg'`.
2. Add the same id at the same position in `DesktopThemeIDs()` in `internal/desktop/themes.go`.
3. Create `ui/css/desktop-theme-<id>.css` with the root rule and the full token set. List it in the `desktop-shell.bundle.css` entry of `cssBundles` in `scripts/build-ui-bundles.js`, after `desktop-wallpaper-live.css` and before `desktop-polish.css`.
4. If `fx: true`, create `ui/js/desktop/theme-fx/<id>.js` and list it in the `desktop-theme-fx` bundle after `fx-core.js`.
5. Create `ui/img/themes/<id>.svg` (schematic, at most 8192 bytes).
6. Add `desktop.settings_theme_<id>` and `desktop.settings_theme_<id>_desc` to all 16 `ui/lang/desktop/*.json` files.
7. Run `npm run build:ui`, then the tests below.

### Tests and Commands

```bash
npm run build:ui                                   # regenerate the bundles
npm run check:ui                                   # fail when a generated bundle is stale
npm run check:desktop-themes                       # cursors, XP icons, pinned vendor and UI bundles
go test ./ui -run DesktopTheme -count=1            # registry, CSS contract, thumbnails, bundle order, layering
node scripts/test-ui-regressions.mjs               # Node suite, no browser; includes the Green Slime simulation
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run DesktopTheme -timeout 30m
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '(TestDesktopGreenSlime|TestDesktopLivingMaterials|TestDesktopExperience)' -count=1 -timeout 30m
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_THEME_RECORDINGS=1 go test ./ui -run '^TestDesktopGreenSlimeCloseUpsBrowser$' -count=1 -timeout 10m
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
- `TestDesktopGreenSlimeInteractionBrowser` runs one WebGL2 stage (settings and files windows at 1920×1080) with four subtests, in order: a forced bubble through all five stages (the stage counters rise, 3–7 micro-droplets per burst, at most 5 bubbles and 2 per window, all healed); a strand pulled with the real mouse (it snaps and leaves a drop, the window does not move, no marquee appears); drops that merge into a lower window and splat on the taskbar edge until the splat clears; and a forced WebGL context loss that must end on Canvas 2D with one canvas and one layer. It needs `AURAGO_RUN_BROWSER_SMOKE=1`.
- `TestDesktopGreenSlimeCloseUpsBrowser` writes review images into the ignored `reports/desktop-experience-themes/slime/`: 1× full frames and 2× to 8× crops of the frame, corners, edges, drips and overlaps; one bubble through its stages, with contact sheets; the burst; a strand; drops; a taskbar splat; static crops; and `gl-ref-*` crops of the wallpaper's own gel beads for comparison. It needs `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_THEME_RECORDINGS=1`, a GPU and about a minute. It asserts that every stage is reached and that the page reports no errors, not how the images look. The `TestDesktopGreenSlime` pattern in the command block also matches it; without `AURAGO_THEME_RECORDINGS=1` it skips.
- The membrane and six-window tests write `green-slime-attached-<label>.png` (labels `webgl2`, `webgl2-translucent`, `canvas2d`), `green-slime-corner-<label>.png` and `green-slime-six-windows.png` to the ignored `reports/desktop-experience-themes/` folder. The close-ups go to the ignored `reports/desktop-experience-themes/slime/` folder. `AURAGO_THEME_RECORDINGS=1` gates the close-ups and the motion GIF test (`TestDesktopThemeMotionRecordingsBrowser`, into `reports/desktop-experience-themes/motion/`), not the membrane or six-window screenshots.
- Add `AURAGO_THEME_RECORDINGS=1` and run `TestDesktopThemeMotionRecordingsBrowser` to capture actual pointer-driven opening, dragging and closing as four GIFs in that report directory's `motion/` subdirectory. `AURAGO_BROWSER_MUTE_AUDIO=1` mutes the browser output while preserving Web Audio assertions.
- `AURAGO_RUN_DESKTOP_THEME_NEUTRALITY=1 AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopThemeNeutralityAgainstHeadBrowser -count=1` compares Standard and Fruity pixels with the committed HEAD bundles before committing a theme change. It covers desktop/narrow viewports and both Fruity schemes.
- On Windows, set task-local `GOCACHE`, `GOTMPDIR`, `TEMP` and `TMP` if the default temporary directories are restricted. `AURAGO_DESKTOP_THEME_VENDOR_NODE_MODULES` can point the vendor check at an existing installation of the repository's pinned npm dependencies.

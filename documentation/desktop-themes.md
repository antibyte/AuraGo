# Desktop Themes

The Virtual Desktop has seven themes. A theme sets the shell layout (taskbar or dock), the colour scheme, the icon set, the look of windows, menus and controls and, for one theme, animated effects. Apps keep their own content and take their colours from the shared theme tokens.

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

"Your accent" means the Accent color setting applies. Fruity's scheme follows its own Light/Dark setting.

## Choosing a Theme

- Open Settings, category **Appearance**, and pick a card under **Theme**. A click, Enter or Space selects it.
- A theme also sets the **Icon theme** (Papirus or WhiteSur). You can change the icon set afterwards; picking a theme pairs its icon set again.
- The wallpaper follows the theme while it is still the previous theme's suggestion, or the default *Group Shot*, in every space. Suggestions: Amiga *Slate*, Cyberpunk *City Rain*, Black Gloss *Midnight*, Crazy *Fractal Trip (animated)*, Lollipop *Paper Waves*. Standard and Fruity use *Group Shot*. A wallpaper you picked yourself is never changed.
- **Fruity appearance** (Light or Dark) appears only while Fruity is selected. The other themes have a fixed scheme.
- **Accent color** (teal, orange, blue, violet or green) applies to Standard, Fruity and Black Gloss. The other four themes use their fixed accent and hide the row.
- The settings are `appearance.theme`, `appearance.icon_theme`, `appearance.fruity_mode`, `appearance.accent`, `appearance.wallpaper` and `appearance.wallpaper_by_space`. The UI saves them through `PUT /api/desktop/settings`.

## Accessibility

- **Reduced motion.** When the system requests reduced motion (`prefers-reduced-motion: reduce`), a global override ends every transition and animation at once: durations drop to 0 ms and animations run a single iteration. The shell also skips its window, menu and start-menu transitions, and Crazy's confetti and sparkles stop.
- **Window animations.** Settings, category **Windows**, *Window animations* turns off every theme animation and the Crazy effects, whether or not the system asks for reduced motion.
- **Contrast.** Body text on the app and panel backgrounds, and muted text on the app background, are tested to a contrast ratio of at least 4.5:1 for each skinned theme (Amiga, Cyberpunk, Black Gloss, Crazy and Lollipop).
- **Focus.** Keyboard focus shows a ring in every theme. Amiga draws a dashed black outline and Crazy a cyan ring.
- **Amiga window gadgets.** The Amiga skin keeps the authentic Workbench gadget layout (close far left, depth and zoom gadgets on the right) purely visually through CSS grid placement, so keyboard Tab order follows the DOM (AI, minimize, maximize, close; AI and maximize are omitted where a window lacks them) rather than the on-screen order, and the dashed black focus ring makes the focused gadget visible.
- Touch: on a coarse pointer the global menu bar grows to 44 px and every tray control fills it (`css/desktop-chrome.css`); Amiga sizes its gadgets from the same bar height in its skin.

## Developer Guide

### Architecture

1. `DESKTOP_THEMES` in `ui/js/desktop/core/theme-runtime.js` describes every theme. `desktop.DesktopThemeIDs()` in `internal/desktop/themes.go` lists the same ids and is the allowed value list of `appearance.theme`.
2. `applyDesktopSettings()` in `ui/js/desktop/core/desktop-foundation.js` calls `applyDesktopThemeAttributes()`, which sets `data-theme`, `data-shell`, `data-scheme` and `data-polish` on `<body>`.
3. CSS keys off these attributes: `[data-theme]` in the skins and the shell-overrides bridge, `[data-shell]` in the layout rules (`desktop-base.css`, `desktop-windows.css`, `desktop-chrome.css`), and `[data-polish]` in `desktop-polish.css`.
4. JavaScript asks the capability helpers (`shellLayout()`, `themeUsesGlobalMenu()`, `themeScheme()`, `themeMotionMs(kind)`, `themeIconFamily()`) and never compares a theme id. Apps read `document.body.dataset.scheme`.
5. `syncDesktopThemeFx()` starts or stops the effects of an `fx` theme.

Layering in `ui/css/desktop-shell.bundle.css`: the shared shell files (`desktop-base.css` carries the Standard and Fruity core tokens), then `desktop-shell-overrides.css` (the `--vd-theme-*` bridge), `desktop-chrome.css`, `desktop-wallpaper-live.css`, the skins `desktop-theme-amiga.css`, `-cyberpunk`, `-black-gloss`, `-crazy` and `-lollipop`, and last `desktop-polish.css`. `scripts/build-ui-bundles.js` assembles this bundle and the `desktop-theme-fx` bundle.

### Body attributes

| Attribute | Values | Used by |
| --- | --- | --- |
| `data-theme` | theme id | skin files and the `.desktop-body[data-theme]` bridge |
| `data-shell` | `taskbar`, `dock` | layout rules and `shellLayout()` |
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
| `globalMenu` | boolean | top menu bar; read by `themeUsesGlobalMenu()` |
| `iconTheme` | `papirus`, `whitesur` | icon set paired on selection (`appearance.icon_theme`) |
| `iconFamily` | `standard`, `fruity` | mini action-icon sheet (`refreshMiniIconTheme()`) |
| `accent` | `user`, `fixed` | `user` shows the accent row and follows the user accent; `fixed` needs the fixed-accent tokens on the root |
| `glassPolish` | boolean | `data-polish` on or off |
| `fx` | boolean | loads the lazy effects bundle for the theme |
| `wallpaper` | an `appearance.wallpaper` value | suggested wallpaper (`followThemeWallpaper()`) |
| `thumb` | `/img/themes/<id>.svg` | picker thumbnail |
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

- The host `ui/js/desktop/theme-fx/fx-core.js` exposes `window.AuraDesktopThemeFx` with `register(id, impl)`, `start(id, options)`, `stop()` and `activeId()`.
- An effect module `ui/js/desktop/theme-fx/<id>.js` registers `{ start }` under its theme id. `start({ layer, signal, options })` returns `{ pause, resume, stop }`; missing methods are no-ops.
- The host creates `.vd-theme-fx-layer` (fixed, full viewport, `pointer-events: none`, one z-index step below `--vd-z-dock`; the rule lives in `desktop-base.css`), an AbortController and a `visibilitychange` listener. A hidden tab calls `pause()` and a visible tab calls `resume()`.
- `stop()` runs before the signal is aborted and the layer is removed. Bind listeners to `signal` and release everything in `stop()`.
- Return an inert object when reduced motion or `data-animations="false"` applies, and check `Element.prototype.animate` before use. Crazy does both.
- Budget pattern (`theme-fx/crazy.js`): pools of spans created once, one Web Animation per particle on `transform` and `opacity`, no per-frame loop and no timer. Crazy has 36 confetti and 14 sparkle spans, one confetti burst at a time, at most one sparkle every 33 ms, and no sparkles over app content, the taskbar, menus, dialogs or toasts.

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
go test ./ui -run DesktopTheme -count=1            # registry, CSS contract, thumbnails, bundle order, layering
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run DesktopTheme -timeout 30m
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_THEME_SWEEP=fruity,crazy go test ./ui -run TestDesktopThemeSweepBrowser -timeout 30m
go test ./ui -run TestDesktopAuditAppearanceBrowser -count=1
```

- The first command needs no browser. The sweep and Crazy reduced-motion tests skip unless `AURAGO_RUN_BROWSER_SMOKE=1` is set. The theme browser tests need Chrome or Edge.
- `AURAGO_THEME_SWEEP` limits the sweep to some registry ids. Sweep screenshots go to `reports/desktop-theme-sweep/`.
- `TestDesktopAuditAppearanceBrowser` writes the Standard and Fruity screenshots to `reports/desktop-audit-browser/`. A change to shared Standard or Fruity tokens must leave them pixel-identical.
- `TestDesktopCrazyThemeHonoursReducedMotionBrowser` checks Crazy's reduced-motion behaviour in a browser.

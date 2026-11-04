# Pages and chat default in the desktop Standard look

Date: 2026-11-04. Status: design approved by the user, written spec awaiting review.

## Goal

User request (German): "alle seiten ausser dem chat sollen den selben look erhalten wie das desktop standard theme. beim chat soll nur das default theme an das desktop standard theme angepasst werden."

In English: all pages except the chat get the same look as the desktop Standard theme. In the chat, only the default theme is adapted to the desktop Standard theme.

## Decisions taken with the user

| Topic | Decision |
| --- | --- |
| Light mode | Stays. The header toggle on the pages stays. Dark equals Standard. Light is a derived variant, since Standard itself is dark only. |
| Accent | Fixed Standard orange `#E95420` everywhere, independent of the desktop's `appearance.accent` setting. No server lookup, so it works on login and setup. |
| Approach | A: tokens plus component surfaces. Rejected: B (tokens only), because radii up to 28 px and component shapes would still look different. Rejected: C (loading the desktop CSS bundle on the pages), because of size and collisions with page CSS. |
| Login effects | The neural net, the flame around the login card and the scan line stay, recoloured to orange. Aurora and orbs are removed. |

## Reference look

The reference is the desktop Settings window in the Standard theme: near-black neutral surfaces, a dark sidebar, cards with a fine gradient and a thin blue-grey border, radii 8-14 px, dark inset fields and orange selection.

Sources:

- `ui/css/desktop-base.css` (`.desktop-body[data-theme="standard"]`)
- `ui/css/desktop-shell-overrides.css`
- `ui/css/desktop-materials.css` (the Standard `--vd-mat-*` set, line 64 on)

The desktop wallpaper is NOT brought to the pages. A page corresponds to window content.

## Pages in scope

404, cheatsheets, config, containers, dashboard, invasion_control, knowledge, login, media, missions_v2, plans, setup, skills, truenas.

All of them carry `.pw-page` and `data-theme-scope="workspace"`.

## Approach A: tokens plus component surfaces

The page tokens change, and the component surfaces are styled centrally. The rejected options are described under Decisions.

### Dark tokens in `.pw-page` (`ui/css/precision-workspace.css`)

| Token | Old | New |
| --- | --- | --- |
| --pw-canvas | #0d1726 | #12161d |
| --pw-surface | #17283c | #1a212b |
| --pw-surface-elevated | #20364e | #222a35 |
| --pw-surface-soft | #273c52 | #1f2a38 |
| --pw-text | #edf3fa | #edf1f7 |
| --pw-muted | #b3c2d2 | #aeb7c6 |
| --pw-subtle | #96a9bc | #a4afc0 |
| --pw-accent | #83baff | #ed764d |
| --pw-accent-strong | #b2d5ff | #f08763 (accent mixed 70 % with white, the top stop of Standard's primary gradient) |
| --pw-on-accent | #0d1726 | #0c1016 |
| --pw-danger | #fb7185 | #ff7a72 |
| --pw-warning | #f6c453 | #f2b84b |
| --pw-success | #4ade80 | #4ad29c |
| --pw-line | #30455e | rgba(191, 210, 235, 0.15) |
| --pw-line-strong | #526a82 | rgba(191, 210, 235, 0.28) |
| --pw-radius-sm / md / lg / xl | 10 / 14 / 20 / 28 px | 8 / 10 / 14 / 14 px |

The font stays Geist. It is identical to Standard's `--vd-font-ui`.

### Accent rule

Raw `#E95420` is too low-contrast as text: about 4.4:1 on the card colour. White on it is 3.65:1.

The desktop's primary button uses the accent mixed 80 % with white (`#ed764d`) with dark ink `#0c1016`. The pages use the same value for `--pw-accent`, so links and accent text pass 4.5:1.

Exact gradients for primary buttons come from Standard's `--vd-mat-primary`.

### Component surfaces

New `--pw-mat-*` tokens in `ui/css/precision-workspace.css` drive the surfaces. Gradient literals appear only in these tokens. Existing rules are edited where they already live (`precision-pages.css`, the entry-page adapters), because `precision-pages.css` loads later and entry pages do not load it. The tokens mirror the Standard materials:

- bar
- side
- card
- well
- field
- btn
- primary
- selected
- hover
- divider

Each token has a dark and a light value.

The central rules cover:

- `.app-header`
- cards
- buttons, including `.btn-primary`
- fields (`.cfg-input` and friends)
- selected navigation
- dialogs (`.modal`)

### Hardcoded colours

Most page CSS is already token based. The following files have 0 hex colours in their page CSS: dashboard, containers, knowledge, media, missions, plans, skills, truenas, invasion and cheatsheets. These need no colour work.

Manual work is needed in these files:

| File | Hardcoded colours |
| --- | --- |
| `ui/css/config.css` | 127 lines with hex |
| `ui/css/config-workspace.css` | 58 hex, 6 rgba |
| `ui/css/setup.css` | 76 hex, 64 rgba |
| `ui/css/login.css` | 2 hex, 47 rgba |
| `ui/css/precision-pages.css` | 10 hex |

`ui/css/config-workspace.css` also redefines the whole `--pw-*` palette and the radii for config. That block is deleted, so config inherits the new tokens.

`ui/css/enhancements.css` is not edited, because the other chat themes load it. Later rules in `precision-workspace.css` and `chat-standard.css` override its `[data-theme="dark"]` rules.

The noise layer and the colour orbs disappear from the pages. On the login page, the aurora and the orbs are removed from the CSS, `ui/login.html` and `ui/js/login/main.js`. The hardcoded blue in `ui/js/login/main.js` and `ui/js/login/flame-wrap.js` becomes orange.

## Light variant of the pages

The light variant is derived from the desktop's Fruity light palette:

- neutral surfaces instead of today's blue-tinted ones
- white cards
- the same shapes and radii as dark

| Token | Old | New |
| --- | --- | --- |
| --pw-canvas | #edf3fa | #f6f7f9 |
| --pw-surface | #ffffff | #ffffff |
| --pw-surface-elevated | #e4ecf5 | #eceef2 |
| --pw-surface-soft | #d7e3ef | #e8ebf0 |
| --pw-text | #162b40 | #1c2430 |
| --pw-muted | #455e76 | #4a5568 |
| --pw-subtle | #546c83 | #5a6577 |
| --pw-accent | #2864b4 | #b93e0b |
| --pw-accent-strong | #1d4e93 | #9a3412 |
| --pw-on-accent | #ffffff | #ffffff |
| --pw-line | #cad6e2 | rgba(40, 52, 76, 0.14) |
| --pw-line-strong | #8098af | rgba(40, 52, 76, 0.38) |

`--pw-danger`, `--pw-warning` and `--pw-success` keep their current light values. The accent is a darker orange so that it reaches at least 4.5:1 as text and carries white ink on primary buttons. The contrast test enforces this; if a value fails, the implementation darkens it and updates this table.

## Chat

Only the default theme (`dark`, label "Standard") changes. All other chat themes stay as they are.

### Scope of the CSS change

- Every selector starts with `:root:is([data-theme="dark"], :not([data-theme]))`. A bare `[data-theme="dark"]` would also match the Standard entry in the theme dropdown while another theme is active.
- The rules live only inside the chat bundle, in a new part `ui/css/chat-standard.css`.
- `scripts/build-ui-bundles.js` adds that part to `ui/css/chat.bundle.css`.
- `ui/shared-variables.css` and `ui/css/tokens.css` stay untouched. The desktop loads them too, and at least the Retro CRT chat theme inherits from their `:root` block.

### Themes that stay pixel-identical

light, retro-crt, 8bit, cyberwar, lollipop, dark-sun, ocean, sandstorm, papyrus, threedee, galaxy, black-matrix

### Bubbles

Bubbles follow the desktop chat app (`ui/css/desktop-app-chat.css`, `.vd-chat-bubble`):

- User bubble: a dark accent tint with an accent border and light text.
- Agent bubble: the card material with border and shadow.

### Hardcoded teal

Active teal gets override rules scoped to the default theme. Most teal lines in `ui/css/chat.css` and `ui/css/chat-modules.css` are inert `var(--accent, #2dd4bf)` fallbacks. Other active teal sits in `ui/css/enhancements.css` (selection, focus, scrollbars, hover sweep) and in the `--vd-mat-*` fallbacks of `ui/css/stt-overlay.css`. These base files stay unchanged.

The chat background logo is removed for the default theme.

## Out of scope

- The desktop shell and the desktop apps.
- The embedded desktop chat app.
- The other chat themes.
- `ui/bookshelf-reader.html` and `ui/game-maker-player.html`. These are standalone full-screen views that do not load the shared page CSS.
- A desktop accent lookup.
- The observation that the default accent setting `teal` has no rule in the desktop CSS. Standard therefore shows orange while "teal" is set. This is a separate possible bug and is not fixed here.

## Constraints

- `ui/shared-components.css`, `ui/shared-variables.css` and `ui/css/tokens.css` are frozen, because chat and desktop load them.
- No external assets. No Internet fonts, libraries or analytics.
- The wave-4 ratchets `ui/testdata/wave4_*.txt` only go down.
- No new page-level `style.display` reveals for `.is-hidden`.

## Verification

- Update the pinned colours in `ui/precision_pages_test.go` and `ui/config_precision_workspace_test.go`.
- New test: contrast of the new page tokens in light and dark.
- New test: the chat rules of the default theme are scoped, so no other theme is affected.
- One round of screenshots of all pages in light and dark, plus the chat.
- Final acceptance is the user's own check on aurago-test.

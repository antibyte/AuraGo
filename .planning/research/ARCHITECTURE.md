# UI Architecture Research

**Project:** AuraGo SPA
**Researched:** 2026-04-03
**Confidence:** HIGH

## Executive Summary

AuraGo uses a vanilla JavaScript SPA architecture embedded in Go via `go:embed`. Pages are server-rendered HTML shells with inline JS for page-specific logic and `shared.js` for cross-cutting concerns. The UI supports 15 languages through a server-side translation system that injects JSON dictionaries into pages at render time. CSS uses a two-tier system: design tokens in `tokens.css` (CSS custom properties) and shared components in `shared.css`, with page-specific stylesheets layered on top.

## Page Structure and Loading

### Multi-Page SPA Pattern
AuraGo is NOT a true SPA that dynamically loads content. Instead, it uses a **multi-page architecture** where:

1. Each route (`/`, `/config`, `/setup`, `/login`, etc.) is a **separate HTML file**
2. The Go server serves different HTML shells based on URL
3. JavaScript is page-specific (`js/chat/main.js`) combined with shared (`shared.js`)
4. CSS is similarly split: shared tokens + page-specific files

### HTML Shell Pattern
Each page follows this structure:

```html
<!DOCTYPE html>
<html lang="{{.Lang}}" data-theme="dark">
<head>
    <!-- Tailwind CDN config (in <script>) -->
    <link rel="stylesheet" href="/shared.css">          <!-- Always -->
    <script src="/shared.js"></script>                   <!-- Always -->
    <link rel="stylesheet" href="/css/tokens.css">     <!-- Always -->
    <link rel="stylesheet" href="/css/[page].css">      <!-- Page-specific -->
</head>
<body>
    <header class="app-header">...</header>
    <main>...</main>
    <footer>...</footer>
    <script src="/js/[page]/main.js"></script>          <!-- Page-specific -->
</body>
</html>
```

### Embedded Files (embed.go)
All UI assets are embedded at compile time:

```
//go:embed index.html config.html dashboard.html plans.html
//missions_v2.html setup.html login.html invasion_control.html
//cheatsheets.html gallery.html media.html knowledge.html
//containers.html truenas.html skills.html config_help.json
//manifest.json sw.js tailwind.min.js chart.min.js
//shared.css shared.js *.png *.ico
//cfg/*.js css/*.css js/*/*.js js/*/*/*.js
//lang/*.json lang/*/*.json lang/*/*/*.json
var Content embed.FS
```

This single `Content` filesystem serves all pages and assets.

## CSS Organization

### Two-Tier System

| Layer | File | Purpose |
|-------|------|---------|
| **Design Tokens** | `css/tokens.css` | CSS custom properties (colors, spacing, typography, animations) |
| **Shared Components** | `shared.css` | Cross-page components (buttons, cards, modals, badges, forms) |
| **Page-Specific** | `css/[page].css` | Chat, setup, login, config, etc. |
| **Enhancements** | `css/enhancements.css` | Additional polish layer |

### Design Tokens (tokens.css)
Centralized CSS custom properties:

```css
:root {
    /* Colors */
    --accent: #2dd4bf;
    --bg-primary: #0b0f1a;
    --text-primary: #f1f5f9;
    /* ... */

    /* Typography */
    --font-sans: 'Inter', system-ui, sans-serif;
    --text-sm: 0.875rem;

    /* Spacing */
    --space-1: 0.25rem;
    --space-4: 1rem;

    /* Borders & Shadows */
    --radius-lg: 8px;
    --shadow-md: 0 4px 6px rgba(0, 0, 0, 0.4);

    /* Animation Keyframes */
    --transition-fast: 150ms ease;
}
```

### Dark/Light Theme Implementation
Themes are implemented via `[data-theme="dark"]` and `[data-theme="light"]` selectors on `:root` in `shared.css`:

```css
:root,
[data-theme="dark"] {
    --bg-primary: #0b0f1a;
    --accent: #2dd4bf;
    /* dark theme vars */
}

[data-theme="light"] {
    --bg-primary: #d8e4df;
    --accent: #0f766e;
    /* light theme vars */
}
```

Theme is toggled by setting `data-theme` attribute on `<html>` and persisted to `localStorage`.

### Shared CSS Components
Key shared components in `shared.css`:

| Component | Classes |
|-----------|---------|
| **Headers** | `.app-header`, `.cfg-header`, `.page-header` |
| **Buttons** | `.btn`, `.btn-primary`, `.btn-secondary`, `.btn-header` |
| **Cards** | `.card`, `.card-compact`, `.glass-card` |
| **Badges/Pills** | `.badge`, `.pill`, `.badge-active`, `.badge-running` |
| **Forms** | `.form-group`, `select`, `.field-select`, `.toggle` |
| **Modals** | `.modal-overlay`, `.modal-card` |
| **Toasts** | `.toast-container`, `.toast` |
| **Navigation** | `.radial-menu`, `.tabs`, `.tab` |
| **Utilities** | `.is-hidden`, `.empty-state`, `.skeleton` |

### Page-Specific CSS Files
| File | Purpose |
|------|---------|
| `css/chat.css` | Chat interface bubbles, composer |
| `css/chat-modules.css` | Chat module-specific styles |
| `css/setup.css` | Setup wizard steps and forms |
| `css/login.css` | Login page with WebGL background |
| `css/config.css` | Configuration UI |
| `css/dashboard.css` | Dashboard cards and stats |
| `css/missions.css` | Mission control cards |
| `css/media.css` | Media gallery |
| `css/knowledge.css` | Knowledge base |
| `css/containers.css` | Docker container management |
| `css/invasion.css` | Invasion control UI |
| `css/skills.css` | Skills management |
| `css/plans.css` | Plans/subscriptions |
| `css/cheatsheets.css` | Cheatsheet viewer |
| `css/gallery.css` | Image gallery |
| `css/truenas.css` | TrueNAS specific |
| `css/stt-overlay.css` | Speech-to-text overlay |
| `css/enhancements.css` | Additional polish |

## JavaScript Organization

### Shared JS (shared.js)
Provides cross-cutting functionality (~1280 lines):

| Module | Functions |
|--------|-----------|
| **I18N** | `t(key)`, `applyI18n()` |
| **Modals** | `showModal()`, `showConfirm()`, `showAlert()` |
| **Theme** | `toggleTheme()`, `initTheme()` |
| **Auth** | `checkAuth()`, `performLogout()` |
| **Navigation** | `injectRadialMenu()`, `initRadialMenu()` |
| **Toast** | `showToast()` |
| **SSE** | `AuraSSE` (EventSource manager) |
| **PWA** | `initPWA()` (service worker, push) |
| **Tailscale** | `initTsnetLoginWatcher()` |
| **Utilities** | `debounce()`, `copyToClipboard()`, `api()` |

### Page-Specific JS
| Page | Main Script |
|------|-------------|
| Chat | `js/chat/main.js` |
| Setup | `js/setup/main.js` |
| Login | `js/login/main.js` |
| Dashboard | `js/dashboard/main.js` |
| Config | `js/config/main.js` |
| Missions | `js/missions/main.js` |
| Containers | `js/containers/main.js` |
| Media | `js/media/main.js` |
| Knowledge | `js/knowledge/main.js` |
| Gallery | `js/gallery/main.js` |
| Skills | `js/skills/main.js` |
| Plans | `js/plans/main.js` |
| Cheatsheets | `js/cheatsheets/main.js` |
| Invasion | `js/invasion/main.js` |

### Chat Modules (js/chat/modules/)
Modular chat components:
- `smart-scroller.js` - Auto-scroll logic
- `code-blocks.js` - Syntax highlighting
- `voice-recorder.js` - Audio recording
- `speech-to-text.js` - Voice input
- `drag-drop.js` - File drag-drop
- `mermaid-loader.js` - Mermaid diagram loading

### Script Loading Order
1. `tailwind.min.js` (CDN)
2. Tailwind config (inline `<script>`)
3. `shared.css`
4. `shared.js` (defines `t()`, `showModal()`, etc.)
5. Page-specific CSS
6. `js/[page]/main.js`

## Translation System (I18N)

### Server-Side Translation Injection
Translations are injected by the Go server at render time:

```go
// In HTML template
<script>
    const I18N = {{.I18N}};  // Server renders JSON dict
</script>
```

### Translation File Structure
```
ui/lang/
├── setup/
│   ├── de.json    (150+ keys)
│   ├── en.json
│   └── ...
├── config/
│   ├── auth/
│   │   ├── de.json
│   │   └── en.json
│   ├── danger/
│   ├── indexing/
│   ├── mcp/
│   ├── prompts/
│   ├── secrets/
│   ├── tokens/
│   └── updates/
└── shared/
    └── ... (shared translations)
```

### Translation Key Format
Keys use dot notation for namespacing:

```json
{
    "setup.page_title": "AuraGo – Quick Setup",
    "setup.nav_next": "Next →",
    "setup.step0_title": "LLM Connection",
    "common.btn_cancel": "Cancel",
    "common.toggle_theme": "Toggle theme"
}
```

### I18N Function (shared.js)
```javascript
function t(k, p) {
    const dict = typeof I18N !== 'undefined' ? I18N : null;
    let s = (dict && dict[k]) || k;
    if (p) Object.entries(p).forEach(([a, b]) => s = s.replaceAll('{{' + a + '}}', b));
    return s;
}
```

### Data Attributes for I18N
Elements can declare translations via attributes:

| Attribute | Effect |
|-----------|--------|
| `data-i18n="key"` | Sets `textContent` |
| `data-i18n-html="key"` | Sets `innerHTML` (with `\n` to `<br>` conversion) |
| `data-i18n-placeholder="key"` | Sets `placeholder` |
| `data-i18n-title="key"` | Sets `title` attribute |
| `data-i18n-aria-label="key"` | Sets `aria-label` |

Example:
```html
<button data-i18n="setup.nav_next">Next</button>
<input data-i18n-placeholder="setup.search_placeholder" />
```

### I18N Application
`applyI18n()` in `shared.js` scans for all `data-i18n*` attributes on page load. Pages can also call `applyI18N()` manually after dynamic content changes.

### Language Switcher
The UI language switcher (bottom-left corner on login/config pages):
- Fetches available languages from hardcoded list
- Calls `PUT /api/ui-language` to persist selection
- Reloads page to get new translations

## Page Structure Patterns

### App Shell Pages (index.html)
```html
<body class="h-screen h-[100dvh] flex flex-col">
    <header class="app-header">           <!-- Fixed top -->
        <a href="/" class="logo">...</a>
        <div class="header-actions">...</div>
    </header>
    <main id="chat-box">...</main>        <!-- Scrollable content -->
    <footer class="app-footer">...</footer> <!-- Fixed bottom -->
</body>
```

### Setup Page Pattern (setup.html)
```html
<body>
    <header class="cfg-header setup-header">
        <div class="logo">...</div>
        <nav id="header-nav">...</nav>
        <div class="header-actions">...</div>
    </header>
    <div class="setup-container">
        <div class="setup-card">
            <div class="step-indicator">...</div>
            <div class="setup-section active" id="step-0">...</div>
            <div class="setup-section" id="step-1">...</div>
            ...
            <div class="success-screen" id="success-screen">...</div>
        </div>
    </div>
    <script src="/js/setup/main.js"></script>
</body>
```

### Login Page Pattern (login.html)
```html
<body>
    <canvas id="bg-canvas"></canvas>      <!-- WebGL background -->
    <div id="css-bg" class="css-fallback-bg">...</div>
    <button id="theme-toggle" class="corner-theme">...</button>
    <div class="login-card">
        <div class="login-logo">...</div>
        <div class="login-field">...</div>
        <button class="btn-login">...</button>
    </div>
    <script src="/js/login/main.js"></script>
</body>
```

### Config Page Pattern
Uses collapsible sections with form groups:
```html
<div class="collapsible-group">
    <div class="collapsible-header">
        <span class="collapsible-title">...</span>
        <span class="collapsible-arrow">▼</span>
    </div>
    <div class="collapsible-content">
        <div class="form-group">...</div>
    </div>
</div>
```

## Common UI Patterns

### Header Pattern
```html
<header class="page-header">
    <h1><span class="page-title-icon">🎯</span> Page Title</h1>
    <div class="header-actions">
        <button class="btn-primary">Action</button>
    </div>
</header>
```

### Card Pattern
```html
<div class="card">
    <div class="card-title">Card Title</div>
    <p>Content...</p>
</div>
```

### Form Field Pattern
```html
<div class="form-group">
    <label for="field-id">Label</label>
    <input type="text" id="field-id" class="field-input">
</div>
```

### Toggle Switch Pattern
```html
<label class="toggle">
    <input type="checkbox" id="toggle-id">
    <span class="toggle-slider"></span>
</label>
```

### Badge Pattern
```html
<span class="badge badge-active">Active</span>
<span class="badge badge-running">Running</span>
```

### Tab Navigation
```html
<div class="tabs">
    <button class="tab active">Tab 1</button>
    <button class="tab">Tab 2</button>
</div>
```

## Responsive Breakpoints

| Breakpoint | Target |
|-------------|--------|
| `< 640px` | Mobile - modal stacked, form rows collapse |
| `640-768px` | Small tablet |
| `768px+` | Desktop - full layout |
| `hover: none` | Touch devices - always show card actions |

## Key Architecture Decisions

### Why Vanilla JS?
- Zero runtime dependencies
- Small binary size (embedded assets)
- Full control over bundle
- Go server handles routing and initial render

### Why go:embed?
- Single binary deployment
- No separate asset packaging
- Compile-time asset bundling
- Simple deployment story

### Why CSS Custom Properties?
- Runtime theming without rebuild
- Dark/light theme switching
- Consistent design tokens
- No CSS-in-JS overhead

### Why Server-Side I18N?
- No translation loading race conditions
- SEO friendly (content in HTML)
- Simpler caching strategy
- All 15 languages available at render time

## Scalability Considerations

| Concern | At 100 users | At 10K users | At 1M users |
|---------|--------------|--------------|-------------|
| **CSS size** | ~200KB total | Same (cached) | Same (cached) |
| **JS per page** | ~50-100KB | Same | Same |
| **I18N dicts** | 15 files, loaded per page | Same | CDN edge caching |
| **Assets** | Embedded in binary | Same | CDN caching |
| **SSE connections** | 1 per client | 1 per client | WebSocket scaling needed |

## Gaps and Issues

| Issue | Severity | Notes |
|-------|----------|-------|
| Translation audit needed | HIGH | Some texts untranslated, inconsistent keys |
| Mission Control layout issues | MEDIUM | Pills placement, header overflow |
| Config page inconsistencies | MEDIUM | Layout varies across sections |
| No shared toast function | LOW | Each page implements its own |
| Chat modules loaded globally | LOW | Even when not on chat page |

## Sources

- **embed.go** - File embedding configuration
- **shared.js** - Cross-page functionality
- **shared.css** - Theme and component styles
- **tokens.css** - Design token definitions
- **ui/index.html** - Main app shell
- **ui/setup.html** - Setup wizard structure
- **ui/login.html** - Login page
- **ui/lang/** - Translation files

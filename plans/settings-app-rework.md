# Settings App Rework Plan

## Ziel
Die Einstellungen-App auf dem virtuellen Desktop komplett überarbeiten: UX verbessern, in eigenes Modul extrahieren, Sidebar-Navigation mit Hamburger-Menü auf Mobile.

## Design-Entscheidungen (User-Approved)

- **Modul**: Eigenes standalone `settings.js` mit `window.SettingsApp = { render, dispose }`
- **Split**: `settings-calculator.js` wird in 3 Dateien aufgeteilt:
  - `editor-filemenu.js` — File-Manager Hilfsfunktionen (renderFiles, renderEditor, setEditorMenus, openMediaPreview etc.)
  - `settings.js` — Neue standalone Settings App
  - `calculator.js` — Bestehender Calculator Code
- **Navigation**: Vertikale Sidebar links mit Icons + Text (macOS-Stil)
- **Search**: Global Search bleibt erhalten (durchsucht alle Kategorien)
- **Mobile**: Sidebar wird zu Hamburger-Menü (nicht horizontaler Scroll)
- **Data Model**: Settings-Definitionen bleiben Frontend-seitig (wie aktuell)

## UX-Verbesserungen

### Aktuelle Probleme
- Sidebar fühlt sich flach an — minimales visuelles Feedback
- Keine Subtext/Differenzierung zwischen Kategorien
- Setting-Rows sind standardisiert ohne visuelle Hierarchie
- Kein Breadcrumb oder Kontext-Indikator
- Search ist "versteckt" im Sidebar-Layout ohne klare Auffindbarkeit

### Neue UX

**Sidebar Neugestaltung:**
- Jede Kategorie bekommt einen größeren Hit-Bereich (48px height)
- Active State mit prominenterem Accent-Indikator (dickerer linker Rand + hellerer Hintergrund)
- Kategorie-Button hat Icon + Titel + subtile Beschreibung darunter
- Hover-State mit sanftem Glow-Effekt
- Sidebar bekommt einen dezenten Gradient-Hintergrund

**Pane Neugestaltung:**
- Kategorie-Header größer, mit prominentem Icon + Title + Description
- Setting-Rows als Karten mit leichtem Hover-Effekt
- Bessere visuelle Trennung zwischen verschiedenen Setting-Typen

**Hamburger-Menü (Mobile):**
- Hamburger Button oben links im Pane-Header
- Sidebar wird als Overlay von links eingeblendet
- Smooth Slide-In Animation
- Backdrop-Hintergrund

## Änderungsübersicht

### Neue Dateien

| Datei | Beschreibung |
|-------|-------------|
| `ui/js/desktop/apps/settings.js` | Neue standalone Settings App IIFE — `window.SettingsApp` |
| `ui/js/desktop/apps/calculator.js` | Ausgelagerter Calculator Code |
| `ui/js/desktop/apps/editor-filemenu.js` | Ausgelagerter File-Editor/Menü Code |

### Geänderte Dateien

| Datei | Änderung |
|-------|---------|
| `scripts/build-ui-bundles.js` | `desktopMainParts` — `settings-calculator.js` → 3 Dateien |
| `ui/js/desktop/core/module-loader.js` | `settings` Eintrag: +`scripts: ['/js/desktop/apps/settings.js']` |
| `ui/js/desktop/core/menus-and-routing.js` | `renderSettings(id)` → `window.SettingsApp.render(...)` |
| `ui/css/desktop-app-settings.css` | Komplett neues CSS für Sidebar + Hamburger Mobile Layout |
| `ui/desktop_js_line_budget_test.go` | `settings-calculator.js` → `settings.js` + `calculator.js` |
| `ui/desktop_settings_icons_test.go` | Pfad `settings-calculator.js` → `settings.js` |
| `ui/desktop_mobile_layout_test.go` | `settings-calculator.js` → `settings.js` |
| `ui/desktop_office_apps_test.go` | `settings-calculator.js` → `editor-filemenu.js` |
| `ui/desktop_file_manager_test.go` | Referenz `settings-calculator.js` → `editor-filemenu.js` |
| `ui/desktop_shell_apps_cleanup_test.go` | `settings-calculator.js` → `calculator.js` |
| `ui/desktop_main_loader_test.go` | `settings-calculator.js` → 3 neue Dateien |
| `ui/lang/desktop/*.json` | (16 Sprachen) Neue Übersetzungsschlüssel für Sidebar-Design |
| `ui/js/desktop/apps/AGENTS.md` | Neue Einträge für `settings.js`, `calculator.js`, `editor-filemenu.js` |

### Gelöscht

| Datei | Grund |
|-------|-------|
| `ui/js/desktop/apps/settings-calculator.js` | Ersetzt durch 3 Dateien |

## Task 1: Code-Split — settings-calculator.js aufteilen

### 1a: editor-filemenu.js erstellen
- Extrahiere Code von Zeile 1 bis Zeile 382 aus `settings-calculator.js`
- Enthält: `renderFiles`, `renderEditor`, `setEditorMenus`, `openMediaPreview`, Hilfsfunktionen

### 1b: calculator.js erstellen
- Extrahiere Code ab `tokenizeCalculatorExpression` (ca. Zeile 642) bis Ende
- Enthält: `renderCalculator`, Calculator-Logik, `registerWindowCleanup`

### 1c: settings.js erstellen (Rohfassung)
- Extrahiere Settings Code (Zeilen 384-612)
- Verpacke in IIFE mit `window.SettingsApp = { render, dispose }`
- Übernehme Abhängigkeiten: `esc`, `t`, `iconMarkup` sind aus Foundation Scope verfügbar via closure injection → brauchen wir einen Context-Parameter-Wrapper, ähnlich wie bei anderen lazy gest App-Modulen die aus menus-and-routing.js aufgerufen werden.).

   => SettingsApp.render(contentEl(id), { esc, t, iconMarkup, api, state, applyDesktopSettings })

## Task 2 Build-Konfiguration aktualisieren in scripts/build-ui-bundles.js desktopMainParts ändern settings-calculator.js -> editor-filemenu.js entfernen settings.js UND calculator.js als separate Teile hinzufügen NICHT als Hauptbundles, sondern mit neuem Eintrag! WICHTIG: Die neuen Dateien werden DEM module-loader.js-Konfig überlassen, also NICHT in desktopMainParts.

- `settings-calculator.js` aus `desktopMainParts` entfernen
- `editor-filemenu.js` zu `desktopMainParts` hinzufügen
- `settings.js` und `calculator.js` NICHT in `desktopMainParts` — werden lazy via `module-loader.js` geladen

### 2b: module-loader.js anpassen
- `settings` Eintrag: `scripts: ['/js/desktop/apps/settings.js']`
- `calculator` Eintrag: `scripts: ['/js/desktop/apps/calculator.js']`

### 2c: menus-and-routing.js anpassen
- `settings` Aufruf: `window.SettingsApp.render(...)` mit Context-Wrapper
- `calculator` Aufruf: `window.CalculatorApp.render(...)` mit Context-Wrapper

### 2d: settings-calculator.js löschen
- Datei löschen (nachdem alle 3 neuen Dateien in 1a-1c erstellt wurden)

## Task 3: CSS — Settings App UI Rework

Vollständiges neues CSS für `desktop-app-settings.css`:

### Desktop Layout (Standard)
**Sidebar (links, 260px):**
- Dunklerer Hintergrund als aktuell, subtiler Gradient
- Jeder Nav-Button: 48px Höhe, 10px Padding horiz., 3-spaltiges Grid: [28px icon] [auto titel] [auto beschreibung]
- Active State: 4px linker Accent-Border, hellerer Hintergrund (rgba(255,255,255,0.1))
- Hover State: Transparenter Hintergrund-Wechsel
- Search Input: bleibt, aber optisch verbessert (bessere Icons/Platzierung)
- "Settings" Titel: bleibt oben, aber dezenter

**Pane (rechts):**
- Pane-Head: 48px Icon in farbigem Circle, größerer Title, Description darunter
- Setting-Rows: Karten-Layout mit Hover-Effekt
- Toggle/Select: Immer rechtsbündig, klare visuelle Struktur

**Spezifische CSS-Klassen:**
- `.vd-settings-sidebar` — Basis Sidebar
- `.vd-settings-nav` — Nav-Button (komplett neues Grid)
  - `.vd-settings-nav-icon` — Icon (18-20px)
  - `.vd-settings-nav-title` — Kategorie-Name
  - `.vd-settings-nav-desc` — Subtitle Beschreibung (klein, muted)
- `.vd-settings-nav.active` — Active State mit 4px Accent-Border left
- `.vd-settings-pane` — Rechter Bereich
- `.vd-settings-pane-head` — Kategorie-Header
- `.vd-settings-pane-icon` — Großes Icon (48px Container)
- `.vd-settings-pane-title` — Titel
- `.vd-settings-pane-desc` — Beschreibung
- `.vd-settings-list` — Grid für Setting-Rows
- `.vd-setting-row` — Karten-Row
- `.vd-setting-select` — Select Input
- `.vd-switch` — Toggle Switch

### Mobile Layout (<768px)
✅ Hamburger-Menü statt Sidebar:
- `vd-settings-app`: `grid-template-columns: 1fr` (sidebar versteckt)
- Hamburger Button im Pane-Head: `.vd-settings-hamburger` (24x24, links oben)
- Sidebar: Als Overlay overlay `.vd-settings-sidebar` als fixed Panel
- Slide-In Animation: `transform: translateX(-100%) → translateX(0)`
- `.vd-settings-sidebar.open` Klasse triggert Einblendung
- `.vd-settings-backdrop` — Halbtransparenter Hintergrund
- Pane-Head bekommt Hamburger Button + Titel

### Transitionen
- Sidebar Nav-Buttons: `background 0.15s, box-shadow 0.15s`
- Sidebar Overlay (mobile): `transform 0.25s ease, opacity 0.25s`
- Setting-Rows Hover: `background 0.12s`
- Checkbox/Switch: bestehende Transition beibehalten

## Task 4: Settings Page — UX-Verbesserungen

### settingsSections() Datenmodell (unverändert)
- `settingsSections()` bleibt als Array von Kategorien
- Jede Kategorie: `{ id, icon, fallback, title, desc, items }`
- Jedes Setting: `{ type: 'select'|'toggle'|'info', key, label, desc, options }`

### renderSettingsShell(host) — Neues HTML
```html
<div class="vd-settings-app">
  <button class="vd-settings-hamburger" data-toggle-sidebar aria-label="Menü">
    ${iconMarkup('menu-symbolic', 'M', 'vd-settings-hamburger-icon', 20)}
  </button>
  <aside class="vd-settings-sidebar">
    <div class="vd-settings-sidebar-header">
      <div class="vd-settings-sidebar-title">Einstellungen</div>
    </div>
    <div class="vd-settings-search">
      <input type="search" class="vd-settings-search-input" placeholder="Suchen…">
    </div>
    <nav class="vd-settings-nav-list">
      ${sections.map(section => `
        <button class="vd-settings-nav ${section.id === active.id ? 'active' : ''}"
                data-section="${section.id}">
          <span class="vd-settings-nav-icon">${iconMarkup(section.icon, ...)}</span>
          <span class="vd-settings-nav-title">${t(section.title)}</span>
          <span class="vd-settings-nav-desc">${t(section.desc)}</span>
        </button>
      `).join('')}
    </nav>
  </aside>
  <div class="vd-settings-backdrop" data-toggle-sidebar></div>
  <section class="vd-settings-pane">
    <!-- bestehende Pane-Struktur -->
  </section>
</div>
```

### Event Handling (in renderSettingsShell)
- `.vd-settings-nav` Klick → category wechseln + re-rendern
- `.vd-settings-search-input` Input → globale Filterung
- `[data-toggle-sidebar]` → Sidebar toggle class + backdrop
- **Kein Re-Rendering beim Sidebar-Toggle** — nur CSS-Klasse umschalten

### window.SettingsApp.render(host, ctx) — Interface
```js
window.SettingsApp = {
  render(host, ctx) {
    // ctx: { contentEl, esc, t, iconMarkup, api, state, applyDesktopSettings, ... }
    // ... rendering logic
  },
  dispose(windowId) {
    // cleanup
  }
};
```

## Task 5: Tests aktualisieren

### 5a: `ui/desktop_js_line_budget_test.go`
- `settings-calculator.js` aus `knownOversizedContinuations` entfernen
- Prüfen ob `settings.js` und `calculator.js` unter 1100 Lines bleiben

### 5b: `ui/desktop_settings_icons_test.go`
- Suchstring `settings-calculator.js` → `settings.js`

### 5c: `ui/desktop_mobile_layout_test.go`
- Array `settings-calculator.js` → `settings.js`
- Neue Tests: inputmode/enterkeyhint in settings.js prüfen

### 5d: `ui/desktop_office_apps_test.go`
- `settings-calculator.js` → `editor-filemenu.js` (der Editor-Menü-Test prüft `setEditorMenus`)

### 5e: `ui/desktop_file_manager_test.go`
- `settings-calculator.js` → `editor-filemenu.js`

### 5f: `ui/desktop_shell_apps_cleanup_test.go`
- `TestDesktopCalculatorRegistersWindowCleanup` → `calculator.js`

### 5g: `ui/desktop_main_loader_test.go`
- `settings-calculator.js` → `editor-filemenu.js`, `settings.js`, `calculator.js`

## Task 6: Build & Bundle aktualisieren

### 6a: Neues Bundle generieren
```bash
node scripts/build-ui-bundles.js
```

### 6b: Build testen
```bash
go build ./cmd/aurago
```

## Task 7: Lang Chain — AGENTS.md aktualisieren

### ui/js/desktop/apps/AGENTS.md
- `settings.js` zum Child DOX Index hinzufügen
- `calculator.js` zum Child DOX Index hinzufügen
- `editor-filemenu.js` zum Child DOX Index hinzufügen
- Eintrag für alten `settings-calculator.js` entfernen
- Hinweis: Settings wird lazy geladen (module-loader), Editor-Filemenu ist im Main Bundle

## Notes
- Die Settings App ist die einzige App, die globalen Desktop-State ändert — `saveDesktopSetting` triggert `applyDesktopSettings()` plus Re-Rendering vom ganzen Shell (Icons, Widgets, Start Menu, Start Button). Das bleibt erhalten.
- Die Kategorie `appearance` bleibt die Standard-Kategorie beim Öffnen.
- Der `host.dataset.activeSettings` Mechanismus für Session-Persistenz bleibt erhalten.
- Foundation-Funktionen (`esc`, `t`, `iconMarkup`, `settingValue`, `settingBool`, `applyDesktopSettings`, `renderStartButtonIcon`, `renderIcons`, `renderWidgets`, `renderStartApps`, `desktopSettings`) werden per `ctx`-Wrapper injected.
- Calculator wird ebenfalls in ein lazy-geladenes Modul umgewandelt mit `window.CalculatorApp`.
- `editor-filemenu.js` bleibt im main.bundle (inlined), da es von mehreren Stellen referenziert wird.
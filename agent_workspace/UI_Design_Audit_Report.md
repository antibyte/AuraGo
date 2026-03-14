# AuraGo Web UI – Design & UX Audit Report

**Datum:** 14.03.2026  
**Scope:** Gesamtes Web UI (`ui/` Verzeichnis)  
**Fokus:** Design-Konsistenz, UX-Patterns, Code-Qualität

---

## Executive Summary

Das AuraGo Web UI zeigt eine gemischte Qualität in Bezug auf Design-Konsistenz. Während die Hauptseiten (Chat, Dashboard, Config) einen hohen Grad an visueller Einheitlichkeit aufweisen, gibt es bei neueren/experimentellen Seiten (Cheatsheets, Invasion, Missions, Gallery) erhebliche Inkonsistenzen. Die Verwendung von Tailwind CSS auf einigen Seiten und reinem Custom-CSS auf anderen schafft eine fragmentierte Entwicklungserfahrung.

**Kritisch:** 14 Issues  
**Mittel:** 12 Issues  
**Niedrig:** 8 Issues

---

## 1. CSS-Architektur & Variablen

### 🔴 Kritisch: Duplizierte CSS-Variablen

**Problem:** Mehrere Seiten definieren lokale CSS-Variablen, die bereits in `shared.css` vorhanden sind.

**Betroffene Dateien:**
| Datei | Duplizierte Variablen |
|-------|----------------------|
| `css/missions.css` | `--bg-tertiary`, `--accent-primary`, `--accent-secondary`, `--accent-success`, `--accent-warning`, `--accent-danger`, `--border-color`, `--shadow`, `--card-radius` |
| `css/cheatsheets.css` | Dasselbe Set wie missions.css |
| `css/invasion.css` | Dasselbe Set wie missions.css |
| `css/gallery.css` | `--border` (nicht in shared.css definiert!) |

**Impact:** 
- Wartungs-Overhead
- Inkonsistentes Verhalten beim Theme-Switching
- Größere Bundle-Size

**Empfohlene Lösung:**
```css
/* Alle Seiten sollten NUR shared.css Variablen verwenden */
/* Falls Variablen fehlen, diese in shared.css ergänzen */
```

### 🟡 Mittel: Fehlende CSS-Variablen in shared.css

**Fehlende Variablen, die in mehreren Dateien lokal definiert werden:**
- `--bg-tertiary` → Sollte zu shared.css hinzugefügt werden
- `--accent-primary` → Ist eigentlich `--accent` in shared.css
- `--border-color` → Sollte zu shared.css hinzugefügt werden
- `--card-radius` → Sollte zu shared.css hinzugefügt werden

---

## 2. HTML-Struktur & Meta-Tags

### 🔴 Kritisch: Inkonsistente Theme-Attribute

| Datei | Theme-Attribut | Status |
|-------|---------------|--------|
| `index.html` | `data-theme="dark"` | ✅ OK |
| `login.html` | `data-theme="dark"` | ✅ OK |
| `config.html` | `data-theme="dark"` | ✅ OK |
| `setup.html` | `data-theme="dark"` | ✅ OK |
| `dashboard.html` | ❌ FEHLT | ⚠️ Problem |
| `missions_v2.html` | `data-theme="dark"` | ✅ OK |
| `gallery.html` | `data-theme="dark"` | ✅ OK |
| `cheatsheets.html` | `lang="de"`, aber kein data-theme in `<html>` | ⚠️ Problem |
| `invasion_control.html` | `lang="de"`, aber kein data-theme in `<html>` | ⚠️ Problem |

**Empfohlene Lösung:**
```html
<!-- Alle HTML-Dateien sollten folgende Struktur haben -->
<html lang="{{.Lang}}" data-theme="dark">
```

### 🔴 Kritisch: Inkonsistente Viewport-Meta-Tags

| Datei | Viewport-Content |
|-------|-----------------|
| `index.html` | `width=device-width, initial-scale=1.0, viewport-fit=cover, interactive-widget=resizes-content` |
| `login.html` | `width=device-width, initial-scale=1.0` (fehlt: viewport-fit) |
| `config.html` | `width=device-width, initial-scale=1.0, viewport-fit=cover` |
| `setup.html` | `width=device-width, initial-scale=1.0, viewport-fit=cover` |
| `dashboard.html` | `width=device-width, initial-scale=1.0` (fehlt: viewport-fit) |
| `missions_v2.html` | `width=device-width, initial-scale=1.0` (fehlt: viewport-fit) |
| `gallery.html` | `width=device-width, initial-scale=1.0` (fehlt: viewport-fit) |
| `cheatsheets.html` | `width=device-width, initial-scale=1.0` (fehlt: viewport-fit) |
| `invasion_control.html` | `width=device-width, initial-scale=1.0` (fehlt: viewport-fit) |

**Impact:** Mobile UX-Probleme, besonders auf iOS mit Notch/Dynamic Island.

---

## 3. Header-Struktur

### 🔴 Kritisch: Inkonsistente Header-Implementierung

**Vergleich der Header-Strukturen:**

```html
<!-- index.html - Korrekt -->
<header class="app-header">
    <a href="/" class="logo">
        <div class="logo-icon">🤖</div>
        <span>AURA</span><span>GO</span>
    </a>
    <div class="header-actions">...</div>
</header>

<!-- dashboard.html - Inkonsistent -->
<header class="app-header">
    <a href="/" class="logo">
        <div class="logo-icon">📊</div>
        <span>AURA</span><span>GO</span>
        <span class="logo-subtitle">DASHBOARD</span>
    </a>
    <div class="header-actions">
        <button id="theme-toggle" class="btn-theme" data-i18n-title="common.toggle_theme">
    </div>
</header>

<!-- config.html - Anders -->
<div class="cfg-header">
    <div style="display:flex;align-items:center;gap:0.5rem;">
        <button id="cfg-hamburger" class="hamburger-btn">☰</button>
        <a href="/" class="logo">...</a>
    </div>
    <div class="header-actions">...</div>
</div>

<!-- setup.html - Wieder anders -->
<header class="cfg-header">
    <div class="logo">
        <div class="logo-icon">✦</div>
        <span>AURAGO</span>
    </div>
</header>
```

**Probleme:**
1. `app-header` vs `cfg-header` vs keine Klasse
2. Logo-Struktur variiert (manchmal `<a>`, manchmal `<div>`)
3. Logo-Icons sind inkonsistent (🤖, 📊, ⚡, 🥚, 🚀, ✦)
4. Logo-Text-Styling (manchmal inline, manchmal via CSS)

---

## 4. Formular-Elemente

### 🔴 Kritisch: Toggle-Switch-Inkonsistenzen

**Vier verschiedene Toggle-Implementierungen:**

```css
/* 1. shared.css - .toggle input:checked + .slider */
/* 2. css/config.css - .toggle.on + .toggle::after */
/* 3. css/setup.css - .toggle-switch + .toggle-slider */
/* 4. css/cheatsheets.css - .toggle + .slider */
```

**Unterschiede:**
- Größen: 48px×26px, 44px×24px, 44px×24px
- Farben: Manche verwenden `--success`, andere `--accent`
- Animationen: Unterschiedliche Timing-Funktionen
- HTML-Struktur: Verschiedene Klassen-Namen

### 🟡 Mittel: Input-Feld-Inkonsistenzen

| Datei | Input-Klasse | Border-Radius | Fokus-Shadow |
|-------|--------------|---------------|--------------|
| shared.css | `.form-group input` | 10px | `0 0 0 3px rgba(45,212,191,0.15)` |
| css/config.css | `.field-input` | 10px | `0 0 0 3px rgba(45,212,191,0.15)` |
| css/setup.css | `.field-input` | 10px | `0 0 0 3px var(--accent-dim)` |
| css/gallery.css | `.gallery-search` | 8px | ❌ Kein Fokus-Styling |
| css/dashboard.css | `.profile-search` | 0.5rem | `0 0 0 3px rgba(45,212,191,0.15)` |

---

## 5. Button-Styling

### 🟡 Mittel: Button-Klassen-Fragmentierung

**Verschiedene Button-Typen über das UI:**

```css
/* shared.css */
.btn, .btn-primary, .btn-secondary, .btn-danger, .btn-success
.btn-header, .btn-theme, .btn-sm

/* config.css */
.btn-save, .wh-btn, .wh-btn-primary, .wh-btn-sm, .wh-btn-icon

/* dashboard.css */
.log-btn, .mood-btn

/* missions.css */
.icon-btn

/* setup.css */
.btn-setup, .btn-next, .btn-back, .btn-skip

/* gallery.css */
.btn-gallery-nav, .btn-gallery-action
```

**Empfohlene Lösung:**
Alle Buttons sollten auf Basis-Klassen aus `shared.css` aufbauen:
```css
/* Einheitliche Struktur */
.btn { /* Basis-Styling */ }
.btn-primary { /* Primary-Variante */ }
.btn-sm { /* Size-Modifier */ }
```

---

## 6. Modal-Dialoge

### 🔴 Kritisch: Drei verschiedene Modal-Systeme

**System 1: Legacy (chat.css)**
```html
<div id="modal-overlay" class="modal-overlay">
    <div class="modal-card">
        <div class="modal-title"></div>
        <div class="modal-body"></div>
        <div class="modal-actions">...</div>
    </div>
</div>
```

**System 2: shared.css (Modern)**
```html
<div class="modal-overlay active">
    <div class="modal">
        <h2>...</h2>
        <div class="modal-actions">...</div>
    </div>
</div>
```

**System 3: missions.css / cheatsheets.css / invasion.css (Individual)**
```html
<div class="modal-overlay" id="modal">
    <div class="modal">
        <div class="modal-header">
            <h2 class="modal-title">...</h2>
            <button class="modal-close">...</button>
        </div>
        <div class="modal-body">...</div>
    </div>
</div>
```

**Probleme:**
- Unterschiedliche Animationen
- Unterschiedliche Z-Index-Werte
- Unterschiedliche Schließ-Mechanismen
- Unterschiedliche Backdrop-Styles

---

## 7. Tailwind CSS vs Custom CSS

### 🔴 Kritisch: Gemischte Styling-Strategien

**Tailwind wird verwendet in:**
- `index.html` (Chat)
- `login.html`
- `config.html`
- `setup.html`

**Reines Custom CSS wird verwendet in:**
- `dashboard.html`
- `missions_v2.html`
- `gallery.html`
- `cheatsheets.html`
- `invasion_control.html`

**Impact:**
- Entwickler müssen zwei Systeme verstehen
- Inkonsistente Utility-Klassen vs Custom-Klassen
- Größere CSS-Bundles (Tailwind CDN + Custom CSS)

---

## 8. i18n (Internationalisierung)

### 🟡 Mittel: Inkonsistente Übersetzungs-Implementierung

**Drei verschiedene Muster:**

```html
<!-- Pattern 1: data-i18n Attribute -->
<span data-i18n="dashboard.budget_title">Budget & Tokens</span>

<!-- Pattern 2: JavaScript Injection -->
<script>document.write(t('missions.page_title'))</script>

<!-- Pattern 3: data-i18n-placeholder -->
<input data-i18n-placeholder="gallery.search_placeholder">
```

**Probleme:**
- Pattern 2 (document.write) ist veraltet und blockiert Rendering
- Manche Seiten haben hardcoded deutsche Texte (`lang="de"`)
- Manche Attribute werden nicht übersetzt (`title`, `aria-label`)

---

## 9. Responsive Design

### 🟡 Mittel: Inkonsistente Breakpoints

| Datei | Mobile Breakpoint | Tablet Breakpoint |
|-------|------------------|-------------------|
| shared.css | 767px | - |
| css/config.css | 767px | 768px-1023px |
| css/dashboard.css | 900px | - |
| css/missions.css | 768px | - |
| css/gallery.css | 640px | - |
| css/cheatsheets.css | 768px | - |
| css/invasion.css | 600px | - |

**Empfohlung:** Standardisierte Breakpoints:
- Mobile: `< 768px`
- Tablet: `768px - 1023px`
- Desktop: `≥ 1024px`

---

## 10. Spezifische UX-Probleme

### 🔴 Kritisch: Fehlende Ladestates

**Seiten ohne Skeleton-Screens oder Loading-Indikatoren:**
- `cheatsheets.html` - Kein Initial-Loading-State
- `invasion_control.html` - Kein Initial-Loading-State
- `gallery.html` - Nur Text "Loading..."

### 🟡 Mittel: Inkonsistente Empty-States

**dashboard.css:**
```css
.empty-state { padding: 80px 20px; border: 1px dashed var(--border-subtle); }
```

**cheatsheets.css:**
```css
.empty-state { text-align: center; padding: 60px 20px; }
```

**gallery.css:**
```css
.gallery-empty { grid-column: 1 / -1; text-align: center; padding: 4rem 2rem; }
```

### 🟡 Mittel: Inkonsistente Card-Designs

| Aspekt | dashboard.css | missions.css | cheatsheets.css |
|--------|---------------|--------------|-----------------|
| Border-Radius | `var(--radius)` (20px) | `var(--card-radius)` (12px) | `var(--card-radius)` (12px) |
| Hover-Effect | translateY(-4px) | translateY(-4px) | translateY(-2px) |
| Box-Shadow | Komplex | Komplex | Einfach |
| Backdrop-Filter | blur(10px) | ❌ Nein | blur(10px) |

---

## 11. Z-Index Management

### 🟡 Mittel: Inkonsistente Z-Index-Werte

```css
/* shared.css */
.toast-container { z-index: 10000; }
.radial-menu { z-index: 9999; }
.modal-overlay { z-index: 200; }
.toast { z-index: 300; }

/* missions.css */
.modal-overlay { /* kein z-index definiert */ }

/* config.css */
.sidebar-backdrop { z-index: 19; }
.cfg-sidebar { z-index: 20; }
```

**Fehlt:** Einheitliches Z-Index-System (z.B. mit CSS-Variablen)

---

## 12. Empfohlene Prioritäten

### Sofort (Sprint 1)
1. ✅ Vereinheitlichung der Theme-Attribute in allen HTML-Dateien
2. ✅ Vereinheitlichung der Viewport-Meta-Tags
3. ✅ Entfernung duplizierter CSS-Variablen

### Kurzfristig (Sprint 2)
4. Refactoring der Toggle-Switches auf ein einheitliches System
5. Vereinheitlichung der Modal-Systeme
6. Standardisierung der Button-Klassen

### Mittelfristig (Sprint 3)
7. Entscheidung: Tailwind vs Custom CSS Strategie
8. Vereinheitlichung der Card-Designs
9. Standardisierung der Breakpoints

### Langfristig
10. Migration aller `document.write` i18n zu `data-i18n`
11. Implementierung eines einheitlichen Z-Index-Systems
12. Design-System-Dokumentation

---

## Anhang: Code-Beispiele

### Empfohlene Header-Struktur
```html
<header class="app-header">
    <div class="header-left">
        <a href="/" class="logo">
            <div class="logo-icon">{{PAGE_ICON}}</div>
            <span class="logo-text">
                <span class="logo-accent">AURA</span>GO
            </span>
            <span class="logo-subtitle" data-i18n="{{page.subtitle}}">Subtitle</span>
        </a>
    </div>
    <div class="header-actions">
        <button id="theme-toggle" class="btn-theme" 
                data-i18n-title="common.toggle_theme" 
                aria-label="Toggle theme">
        </button>
        <!-- Page-specific actions -->
    </div>
</header>
```

### Empfohlene CSS-Variablen-Struktur (shared.css Ergänzung)
```css
:root {
    /* Bereits vorhanden... */
    
    /* Zusätzlich empfohlen: */
    --bg-tertiary: #334155;
    --border-color: rgba(148, 163, 184, 0.15);
    --card-radius: 16px;
    
    /* Z-Index Scale */
    --z-base: 1;
    --z-dropdown: 100;
    --z-sticky: 200;
    --z-modal: 300;
    --z-popover: 400;
    --z-tooltip: 500;
    --z-toast: 600;
    --z-radial-menu: 9999;
}
```

---

*Report erstellt durch UI/UX Audit - AuraGo Development Team*

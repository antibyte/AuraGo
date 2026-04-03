# UI/UX Stack Research

**Project:** AuraGo Web UI
**Researched:** 2026-04-03
**Confidence:** HIGH

## Executive Summary

AuraGo uses a **vanilla CSS** architecture with CSS custom properties (variables) for theming. There is no CSS framework -- all styling is hand-crafted. The design follows a **glassmorphism aesthetic** with backdrop blur effects, subtle gradients, and a unified teal accent color. The system supports dark/light themes via `[data-theme]` attribute selectors. Key technical debt includes duplicate keyframe definitions, inconsistent naming conventions, hardcoded color values, and massive CSS files that violate single-responsibility.

## Technology Stack

### Core Styling
| Technology | Purpose | Notes |
|------------|---------|-------|
| Vanilla CSS | All styling | No Tailwind, Bootstrap, or other framework |
| CSS Custom Properties | Theming system | Variables in `:root` and `[data-theme]` |
| Glassmorphism | Visual design | `backdrop-filter: blur()`, semi-transparent backgrounds |

### CDN Resources (Referenced but Minimal Use)
| Resource | Purpose | Notes |
|----------|---------|-------|
| Tailwind CDN | Mentioned in HTML | Not actively used in CSS |
| Chart.js | Dashboard charts | UI charting only |
| CodeMirror 6 | Code editing | JSON/ YAML editing in config |

### File Organization
```
ui/
  shared.css          # ~3000+ lines, base theme + shared components
  css/
    tokens.css        # Design tokens (spacing, typography, z-index, shadows)
    config.css        # Config page styles (~3000+ lines)
    dashboard.css     # Dashboard page styles (~3000+ lines)
    missions.css      # Missions page styles (~1000+ lines)
    setup.css         # Setup wizard styles (not reviewed)
```

## CSS Variables Architecture

### Theme Variables (in shared.css)

**Dark Theme (default):**
```css
:root,
[data-theme="dark"] {
    --bg-primary: #0b0f1a;
    --bg-secondary: #111827;
    --bg-tertiary: #0f172a;
    --bg-glass: rgba(255, 255, 255, 0.04);
    --accent: #2dd4bf;           /* Teal primary */
    --accent-dim: rgba(45, 212, 191, 0.15);
    --accent-glow: rgba(45, 212, 191, 0.3);
    --text-primary: #f1f5f9;
    --text-secondary: #94a3b8;
    --border-subtle: rgba(148, 163, 184, 0.08);
    --border-accent: rgba(45, 212, 191, 0.2);
    --card-bg: rgba(30, 41, 59, 0.6);
    --success: #22c55e;
    --warning: #f59e0b;
    --danger: #ef4444;
    /* ... extensive list */
}
```

**Light Theme (override):**
```css
[data-theme="light"] {
    --bg-primary: #d8e4df;
    --accent: #0f766e;           /* Darker teal for contrast */
    --text-primary: #102032;
    /* ... extensive overrides */
}
```

### Design Tokens (in tokens.css)
```css
:root {
    /* Typography Scale */
    --text-xs: 0.75rem;
    --text-sm: 0.875rem;
    --text-base: 1rem;
    --text-lg: 1.125rem;
    --text-xl: 1.25rem;
    --text-2xl: 1.5rem;

    /* Spacing */
    --space-1: 0.25rem;
    --space-2: 0.5rem;
    /* ... through --space-12 */

    /* Border Radius */
    --radius-sm: 4px;
    --radius-md: 6px;
    --radius-lg: 8px;
    --radius-xl: 12px;
    --radius-2xl: 16px;
    --radius-full: 9999px;

    /* Shadows */
    --shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.3);
    --shadow-md: 0 4px 6px rgba(0, 0, 0, 0.4);
    --shadow-lg: 0 10px 15px rgba(0, 0, 0, 0.5);

    /* Z-Index Scale */
    --z-base: 0;
    --z-dropdown: 100;
    --z-modal: 1000;
    --z-toast: 1100;
    --z-tooltip: 1200;

    /* Transitions */
    --transition-fast: 150ms ease;
    --transition-base: 200ms ease;
    --transition-slow: 300ms ease;
}
```

### Extra Tokens (in tokens.css)
```css
/* Accent variants */
--accent-hover: #5eead4;
--accent-active: #14b8a6;

/* Status dim variants */
--success-dim: rgba(34, 197, 94, 0.15);
--warning-dim: rgba(234, 179, 8, 0.15);
--danger-dim: rgba(239, 68, 68, 0.15);
--info: #3b82f6;
--info-dim: rgba(59, 130, 246, 0.15);

/* Text variants */
--text-muted: #71717a;
--text-inverse: #0f0f0f;
```

## CSS Methodology

### Naming Conventions
- **BEM-inspired:** `.card`, `.card-title`, `.card--active`
- **Block naming:** `.badge`, `.btn`, `.modal`, `.tabs`
- **Modifier pattern:** `.badge-active`, `.badge-running`, `.btn-primary`
- **Utility classes:** `.is-hidden`, `.hidden`, `.fade-in`
- **Page-specific:** `.dash-card`, `.cfg-sidebar`, `.mission-card`

### Component Patterns

**Cards (Glassmorphism):**
```css
.card {
    background: var(--card-bg);
    border: 1px solid var(--border-subtle);
    border-radius: 16px;
    backdrop-filter: blur(10px);
    box-shadow: var(--surface-shadow), inset 0 1px 0 var(--surface-highlight);
    transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}
.card:hover {
    border-color: var(--accent);
    transform: translateY(-4px);
}
```

**Buttons:**
```css
.btn {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 10px 20px;
    border-radius: 8px;
    font-size: 0.9rem;
    font-weight: 500;
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    position: relative;
    overflow: hidden;
}
.btn::before {
    content: '';
    position: absolute;
    top: 0;
    left: -100%;
    width: 100%;
    height: 100%;
    background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.2), transparent);
    transition: left 0.5s;
}
.btn:hover::before {
    left: 100%;
}
```

**Forms:**
```css
.form-group input,
.form-group select,
.form-group textarea {
    width: 100%;
    padding: 12px 16px;
    border: 1px solid var(--border-subtle);
    border-radius: 10px;
    background: var(--input-bg);
    color: var(--text-primary);
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
.form-group input:focus,
.form-group select:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 3px rgba(45, 212, 191, 0.15);
}
```

## Theme Implementation

### Theme Switching
```javascript
// In shared.js or theme toggle handler
document.documentElement.setAttribute('data-theme', 'light'); // or 'dark'
```

### Light Theme Overrides Pattern
```css
[data-theme="light"] .btn-header,
[data-theme="light"] .btn-theme {
    border-color: rgba(71, 85, 105, 0.16);
    background: rgba(234, 242, 237, 0.92);
}

[data-theme="light"] .card:hover {
    box-shadow: 0 10px 24px rgba(15, 118, 110, 0.24);
}
```

### Color Scheme Declaration
```css
:root,
[data-theme="dark"] {
    color-scheme: dark;
    /* ... */
}

[data-theme="light"] {
    color-scheme: light;
    /* ... */
}
```

## Keyframe Animations (Centralized in tokens.css)
```css
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
@keyframes fadeInUp { from { opacity: 0; transform: translateY(20px); } to { opacity: 1; } }
@keyframes card-enter { /* staggered card entrance */ }
@keyframes pulse { /* loading indicator */ }
@keyframes spin { /* spinner */ }
@keyframes shimmer { /* skeleton loading */ }
```

## Critical Inconsistencies and Technical Debt

### 1. Duplicate Keyframe Definitions
| Animation | Files | Issue |
|-----------|-------|-------|
| `fadeIn` | tokens.css, missions.css, shared.css | Defined 3+ times |
| `card-enter` | tokens.css, missions.css | Defined 2+ times |
| `pulse` | shared.css, dashboard.css, missions.css | Redundant |
| `shimmer` | shared.css, dashboard.css | Identical copies |

### 2. Inconsistent Color Hardcoding
Some components use hardcoded values instead of CSS variables:
```css
/* Bad - hardcoded */
color: #fbbf24;
background: rgba(99, 102, 241, 0.12);
border-color: rgba(245, 158, 11, 0.3);

/* Good - uses variables */
color: var(--warning);
background: var(--warning-dim);
border-color: var(--warning);
```

### 3. Duplicated Status Badge Classes
```css
/* In shared.css */
.badge-running { /* ... */ }
.badge-warning { /* ... */ }
.badge-active { /* ... */ }

/* In dashboard.css */
.pill-running { /* similar */ }
.pill-warning { /* similar */ }
.pill-completed { /* similar */ }

/* In missions.css */
.badge-priority-low { /* similar */ }
.badge-priority-medium { /* similar */ }
```

### 4. Massive Single-Responsibility Violations
- `shared.css`: 3000+ lines containing reset, themes, ALL shared components, animations, utilities
- `dashboard.css`: 3000+ lines with page-specific AND shared-like styles
- `config.css`: 3000+ lines with config-specific AND duplicate patterns
- `missions.css`: 1000+ lines with page-specific styles

### 5. Missing Responsive Grid System
No unified grid utilities. Each page defines its own:
```css
/* dashboard.css */
.stats-grid { display: grid; grid-template-columns: repeat(3, 1fr); }

/* missions.css */
.missions-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(350px, 1fr)); }

/* config.css */
.hp-grid-2col { display: grid; grid-template-columns: 1fr 1fr; }
```

### 6. Inconsistent Selector Patterns
```css
/* Sometimes class-only */
.card { }

/* Sometimes attribute */
[data-theme="light"] .card { }

/* Sometimes specificity battles */
:root .card { }  /* unnecessary specificity */
```

### 7. Unused or Redundant Properties
```css
/* dashboard.css has full duplicate of shared.css animations */
@keyframes card-enter { /* identical */ }
@keyframes pulse-glow { /* duplicate of shared.css pulse */ }
```

## Responsive Breakpoints

| Breakpoint | Purpose | Pattern |
|------------|---------|---------|
| 480px | Mobile small | `.container { padding: 12px; }` |
| 640px | Mobile large | `.form-row { grid-template-columns: 1fr; }` |
| 768px | Tablet | `.page-header { flex-wrap: wrap; }` |
| 900px | Tablet large | `.dash-grid { grid-template-columns: 1fr; }` |
| 1100px | Desktop | Knowledge grid collapse |

## Component Inventory

| Component | States | Notes |
|-----------|--------|-------|
| `.btn` | default, hover, active, disabled | Primary/secondary/danger/success variants |
| `.card` | default, hover, expanded | Glassmorphism with accent glow on hover |
| `.badge` | active, inactive, running, warning | Pill and badge shapes |
| `.pill` | active, disconnected, reconnecting | Connection status indicators |
| `.tabs` | default, active, hover | Sticky with blur backdrop |
| `.modal` | closed, open | Scale + fade animation |
| `.form-group` | default, focus, error | Label color change on focus |
| `.toggle` | off, on | Custom checkbox replacement |
| `.empty-state` | default | Floating icon animation |
| `.toast` | success, error | Slide-in animation |

## Recommendations

1. **Extract duplicate keyframes** to a single `animations.css` file
2. **Consolidate status badge classes** into unified `.status-badge` with modifier classes
3. **Replace hardcoded colors** with CSS variable references
4. **Split massive files** by responsibility (reset, themes, components, utilities, pages)
5. **Create unified grid/spacing system** via CSS custom properties
6. **Audit unused selectors** -- many dashboard.css patterns are duplicates of shared.css
7. **Add CSS linting** to catch hardcoded colors and duplicate definitions

## Sources

- `ui/shared.css` (lines 1-3042)
- `ui/css/tokens.css` (lines 1-198)
- `ui/css/dashboard.css` (lines 1-3293)
- `ui/css/config.css` (lines 1-300+)
- `ui/css/missions.css` (lines 1-1021)
- `.planning/codebase/STACK.md` (existing tech stack)
- `.planning/PROJECT.md` (project context)

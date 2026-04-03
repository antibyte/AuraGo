# Feature Research

**Domain:** UI Component Library Analysis (AuraGo SPA)
**Researched:** 2026-04-03
**Confidence:** HIGH

## Executive Summary

AuraGo's embedded SPA UI uses a glassmorphism design language with CSS custom properties for dark/light theming. The codebase has a well-established set of shared components in `shared.css` but suffers from inconsistent application across 15+ pages, leading to layout breakages (notably Mission Control) and translation gaps. The component library is mature but needs systematic application and documentation.

## Feature Landscape

### Table Stakes (UI Components Users Expect)

These are the core UI building blocks that power all pages.

| Component | Why Expected | Complexity | Notes |
|-----------|--------------|------------|-------|
| **Header** | Navigation and branding on every page | LOW | Two variants: `app-header` (chat), `cfg-header` (config) |
| **Cards** | Primary content containers | LOW | Glassmorphism with hover lift effect, accent border glow |
| **Buttons** | Primary actions | LOW | 6 variants: primary, secondary, danger, success, header, icon-only |
| **Forms** | User input (config, setup) | MEDIUM | Inputs, selects, textareas with focus glow; select dropdowns use custom SVG arrow |
| **Modal** | Confirmations, alerts | LOW | Scale + opacity animation, glassmorphism surface |
| **Tabs** | Section navigation within pages | LOW | Sticky positioning, gradient active state |
| **Toast** | Transient feedback | LOW | Success/error variants, positioned bottom-right |
| **Toggle Switch** | Binary settings | LOW | Animated slider with success gradient when on |
| **Badge/Pill** | Status indicators | LOW | Active (green pulse), disconnected (red shake), reconnecting (amber blink) |
| **Empty State** | Zero-data handling | LOW | Centered with floating icon animation |
| **Language Switcher** | i18n access | LOW | Fixed bottom-left, all 15 languages |

### Differentiators (Unique UI Features)

These set AuraGo's UI apart from typical agent dashboards.

| Component | Value Proposition | Complexity | Notes |
|-----------|-------------------|------------|-------|
| **Radial Menu** | Quick-access actions without leaving current page | MEDIUM | Morphing hamburger-to-X animation, staggered item reveals, icon+label tooltips |
| **Mood Widget** | Emotional feedback for personality system | MEDIUM | Emoji panel with emotion display and trait visualization |
| **Personality Selector** | Personality switching | LOW | Dropdown in header with chevron indicator |
| **Connection State Pills** | Real-time connection status | LOW | Animated states: active (pulse), disconnected (shake+glow), reconnecting (blink) |
| **Token/Budget Pills** | Cost tracking visibility | LOW | Always-visible spending indicator in header |
| **Voice Input** | Voice-to-text in chat | MEDIUM | Voice recorder module with visual feedback |
| **Composer Panel** | Extended chat input tools | LOW | File upload, push notifications, mood feedback, stop button |
| **Glass Card Utility** | Semi-transparent containers | LOW | Reusable `glass-card` class with blur backdrop |

### Anti-Features (Components That Seem Good but Create Problems)

These patterns exist in the codebase but cause issues.

| Pattern | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **Dual modal systems** | Flexibility for different use cases | `modal-overlay` + `modal-card` + `vault-modal-*` create confusion, code duplication | Unify to single modal component with variants |
| **Page-specific CSS** | Targeted styling per page | Leads to inconsistency (Mission Control overflow), hard to maintain | Extend shared components with modifier classes |
| **Hamburger + Radial Menu** | Mobile navigation options | Redundant on mobile (hamburger hidden but radial menu overlaps) | Single navigation pattern for mobile |
| **Heavy animations** | Visual polish appeal | `prefers-reduced-motion` partial support; animation complexity bloats CSS | Selective animation (only on interaction) |
| **Inline `<script>` translations** | Dynamic i18n | `document.write(t(...))` in HTML causes FOUC and translation gaps | Data attributes or JS initialization |

## Component Inventory

### Core Shared Components (shared.css)

**Status:** Well-established, used consistently

| Component | Classes | States |
|-----------|---------|--------|
| Cards | `.card`, `.card-compact`, `.card-expanded`, `.glass-card` | Default, hover (lift+glow), active |
| Buttons | `.btn`, `.btn-primary`, `.btn-secondary`, `.btn-danger`, `.btn-success`, `.btn-sm` | Default, hover (shine), active, disabled |
| Header Buttons | `.btn-header`, `.btn-theme`, `.btn-speaker` | Default, hover, active |
| Forms | `.form-group`, `.form-row`, `select`, `.form-select`, `.field-select` | Default, hover, focus (glow), disabled |
| Toggles | `.toggle`, `.toggle-wrap` | Off, on (green gradient), disabled |
| Badges | `.badge`, `.badge-active`, `.badge-inactive`, `.badge-running`, `.badge-warning`, `.badge-idle`, `.badge-ssh` | Static with optional pulse-glow animation |
| Pills | `.pill`, `.pill-active`, `.pill-disconnected`, `.pill-reconnecting` | Active (pulse-dot), disconnected (shake+blink), reconnecting (blink) |
| Tabs | `.tabs`, `.tab` | Default, hover, active (gradient fill) |
| Modal | `.modal-overlay`, `.modal`, `.modal-card`, `.modal-header`, `.modal-actions` | Open/closed animation, scale transform |
| Toast | `.toast`, `.toast-container`, `.toast.success`, `.toast.error` | Enter (slide+scale), exit animation |
| Empty State | `.empty-state` | Floating icon animation |
| Collapsible | `.collapsible-group`, `.collapsible-header`, `.collapsible-content` | Open/closed with arrow rotation |
| View Toggle | `.view-toggle` | Grid/list mode buttons with active state |
| Radial Menu | `.radial-menu`, `.radial-trigger`, `.radial-item`, `.radial-item-icon`, `.radial-item-label` | Closed, open (morphs to X), staggered item reveals |
| Tooltip | `.tooltip` | Hover reveal with translate animation |
| Loading | `.spinner`, `.skeleton` | Spinner rotation, skeleton shimmer |
| Language Switcher | `.ui-lang-switcher`, `.ui-lang-btn`, `.ui-lang-menu`, `.ui-lang-option` | Closed, open dropdown |

### Page-Specific Components

**Status:** Inconsistent, needs standardization

| Page | Component | Issue |
|------|-----------|-------|
| Mission Control | Pills, mission cards | Overflow container, title truncation, button cutoff |
| Config | Form sections, save bar | Inconsistent spacing, save bar not sticky on mobile |
| Dashboard | Cards grid | Varies from other pages |
| Chat (index.html) | Composer panel, mood feedback, attachment chip | Well-designed, chat-specific |
| Setup | Wizard steps | Language-specific inline scripts |

## Feature Dependencies

```
[Theme System (CSS Variables)]
    └──supports──> [Dark/Light Mode Toggle]
    └──supports──> [All Glassmorphism Components]

[Toast Notifications]
    └──used-by──> [Modal Confirmations]
    └──used-by──> [Form Validation Feedback]

[Radial Menu]
    └──requires──> [Header Actions Container]

[Language Switcher]
    └──requires──> [I18N Data Structure]
    └──used-by──> [All Pages]

[Personality Selector]
    └──requires──> [Mood Widget Panel]

[Composer Panel]
    └──used-by──> [Chat Input (index.html)]
```

## MVP Definition

### Launch With (v1 UI Overhaul)

Core components that must work flawlessly across all pages.

- [ ] **Unified Card System** - Consistent card styling using shared `.card` classes with modifier variants for page-specific needs
- [ ] **Theme System** - Complete dark/light mode with CSS variables, no flash on load
- [ ] **Responsive Layout** - Consistent breakpoints across all pages, no overflow/clipping
- [ ] **Translation Coverage** - All 15 languages with no missing keys, no inline script translation
- [ ] **Toast Notification** - Unified toast system for all feedback
- [ ] **Modal System** - Single modal component with variants (confirm, alert, vault-delete)
- [ ] **Form Components** - Consistent inputs, selects, toggles across config and setup pages

### Add After Validation (v1.x)

Polish and advanced features.

- [ ] **Radial Menu Refinement** - Consistent item rendering across pages, mobile optimization
- [ ] **Mood Widget** - Complete emotion visualization
- [ ] **Loading States** - Skeleton loaders for data-heavy pages (config, dashboard)
- [ ] **Accessibility Audit** - ARIA labels, keyboard navigation, focus management
- [ ] **Animation Reduction** - Respect `prefers-reduced-motion`, remove gratuitous animations

### Future Consideration (v2+)

Advanced UI features.

- [ ] **Command Palette** - Cmd+K quick navigation (alternative to radial menu)
- [ ] **Multi-step Wizard** - For complex setup flows
- [ ] **Drag-and-Drop** - Already exists in chat modules, extend to config
- [ ] **Real-time Collaboration Indicators** - If multi-user support added

## Component Quality Assessment

### Well-Designed Components

| Component | Why It Works |
|-----------|--------------|
| **Theme System** | Clean CSS variable architecture, easy to maintain both dark and light |
| **Toggle Switch** | Smooth animation, clear visual feedback for on/off states |
| **Modal Animation** | Scale + opacity creates polished open/close feel |
| **Card Hover Effect** | Subtle lift + glow creates interactivity without distraction |
| **Tabs** | Gradient active state ties into accent color, sticky positioning useful |
| **Toast Notifications** | Non-blocking, auto-dismiss, success/error variants |
| **Logo Animation** | Pulse animation is subtle and not distracting |
| **Connection State Pills** | Excellent state communication through animation |

### Problematic Components

| Component | Issue | Fix Required |
|-----------|-------|--------------|
| **Dual Modal Systems** | `modal-overlay` + `modal-card` + `vault-modal-*` are separate code paths | Unify into single `.modal` component with `data-variant` attribute |
| **Inline Translation Scripts** | `document.write(t(...))` in HTML causes FOUC | Move all text to data attributes or JS-init |
| **Mission Control Layout** | Pills overflow, buttons clip | Review flexbox wrapping and overflow handling |
| **Config Page Save Bar** | Not sticky on mobile, users lose context | Add `position: sticky` with mobile-safe inset |
| **Hamburger Hidden on Mobile** | Duplicate navigation pattern with radial menu | Remove hamburger, rely solely on radial menu |
| **Animation Bloat** | Many components have unnecessary animations | Audit and remove animations not triggered by interaction |
| **Inconsistent Card Usage** | Some pages use `.card`, others have custom styling | Enforce `.card` system with modifier classes |

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Unified Card System | HIGH | LOW | P1 |
| Responsive Layout Fix | HIGH | MEDIUM | P1 |
| Translation Audit | HIGH | MEDIUM | P1 |
| Modal Unification | MEDIUM | LOW | P1 |
| Theme System Completion | MEDIUM | LOW | P1 |
| Toast System | MEDIUM | LOW | P2 |
| Radial Menu Polish | MEDIUM | MEDIUM | P2 |
| Loading States | MEDIUM | MEDIUM | P2 |
| Accessibility Audit | MEDIUM | HIGH | P3 |
| Animation Audit | LOW | MEDIUM | P3 |

**Priority key:**
- P1: Must have for launch
- P2: Should have, add when possible
- P3: Nice to have, future consideration

## Design System Foundation (What Exists)

### CSS Variable Architecture

**Dark Theme Defaults:**
- `--bg-primary`: #0b0f1a
- `--bg-secondary`: #111827
- `--accent`: #2dd4bf (teal)
- `--text-primary`: #f1f5f9
- `--text-secondary`: #94a3b8
- `--border-subtle`: rgba(148, 163, 184, 0.08)
- `--success`: #22c55e
- `--warning`: #f59e0b
- `--danger`: #ef4444

**Light Theme Overrides:**
- `--bg-primary`: #d8e4df (muted sage)
- `--accent`: #0f766e (darker teal for contrast)
- `--text-primary`: #102032

### Typography

- **Font Family:** Inter (Google Fonts) with system-ui fallback
- **Mono Font:** SF Mono, Fira Code, Cascadia Code
- **Fluid Typography:** Uses `clamp()` for responsive text sizing
- **Text Scale:** xs (0.65-0.75rem), sm (0.75-0.85rem), base (0.875-1rem), lg (1.1-1.3rem)

### Spacing System

- **Margin utilities:** xs (0.35rem), sm (0.75rem), md (1rem), lg (1.5rem)
- **Border radius:** 8px (buttons), 10px (cards), 12px (inputs), 16px (modals), 20px (pills)
- **Shadow system:** `surface-shadow` and `surface-shadow-strong` for elevation

### Animation

- **Default timing:** `cubic-bezier(0.4, 0, 0.2, 1)` for smooth transitions
- **Emphasis timing:** `cubic-bezier(0.34, 1.56, 0.64, 1)` for modal/toast entrances
- **Duration:** 0.2s (micro), 0.3s (standard), 0.4s (emphasis), 0.5s (slow)

### Responsive Breakpoints

- **Mobile:** max-width 767px
- **Tablet:** min-width 768px
- **Desktop:** min-width 1024px (implied)
- **Mobile modal:** max-width 640px

## Sources

- `ui/shared.css` - Complete component library (3042 lines)
- `ui/index.html` - Chat UI implementation
- `ui/config.html` - Config UI implementation
- `ui/js/shared.js` - Shared UI JavaScript functions
- `.planning/PROJECT.md` - AuraGo UI/UX Overhaul project context

---
*Feature research for: AuraGo UI Component Library*
*Researched: 2026-04-03*

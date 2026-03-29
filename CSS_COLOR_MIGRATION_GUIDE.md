# CSS Color Migration Guide

## Overview
This document maps the most common hardcoded colors in AuraGo's CSS to their semantic CSS variable equivalents.

## Top Hardcoded Colors → CSS Variables

| Hardcoded Color | CSS Variable | Usage |
|-----------------|--------------|-------|
| `#ef4444` | `var(--danger)` | Error states, delete buttons, warnings |
| `#2dd4bf` | `var(--accent)` | Primary accent, buttons, highlights |
| `#22c55e` | `var(--success)` | Success states, confirmations |
| `#f59e0b` | `var(--warning)` | Warnings, cautions |
| `#3b82f6` | `var(--info)` | Info states, links |
| `#fff`, `#ffffff` | `var(--text-primary)` | Primary text on dark bg |
| `#94a3b8` | `var(--text-secondary)` | Secondary/muted text |
| `#1a1a1a`, `#171c24` | `var(--bg-secondary)`, `var(--bg-elevated)` | Card backgrounds |
| `#2a2a2a`, `#2a2a3e` | `var(--border)` | Borders |
| `#000`, `#000000` | `var(--bg-primary)` | Darkest backgrounds |

## Semantic Variables Available in tokens.css

```css
/* Accent / Brand */
--accent: #2dd4bf;
--accent-dim: rgba(45, 212, 191, 0.15);
--accent-glow: rgba(45, 212, 191, 0.3);
--accent-hover: #5eead4;
--accent-active: #14b8a6;

/* Backgrounds */
--bg-primary: #0f0f0f;
--bg-secondary: #1a1a1a;
--bg-tertiary: #232323;
--bg-elevated: #171c24;
--bg-overlay: rgba(0, 0, 0, 0.7);

/* Text */
--text-primary: #e5e5e5;
--text-secondary: #a1a1aa;
--text-muted: #71717a;
--text-inverse: #0f0f0f;

/* Borders */
--border: #2a2a2a;
--border-subtle: rgba(255, 255, 255, 0.08);

/* Status / Semantic */
--success: #22c55e;
--success-dim: rgba(34, 197, 94, 0.15);
--warning: #eab308;
--warning-dim: rgba(234, 179, 8, 0.15);
--danger: #ef4444;
--danger-dim: rgba(239, 68, 68, 0.15);
--info: #3b82f6;
--info-dim: rgba(59, 130, 246, 0.15);
```

## Migration Priority

### High Priority (User-Facing)
1. **config.css** - 271 hardcoded colors (most visible)
2. **chat.css** - 186 hardcoded colors (main interface)
3. **dashboard.css** - 148 hardcoded colors (dashboard UI)

### Medium Priority
4. **setup.css** - 94 hardcoded colors
5. **login.css** - 86 hardcoded colors
6. **chat-modules.css** - 91 hardcoded colors

### Lower Priority
7. Other smaller CSS files

## Recommended Migration Strategy

### Phase 1: Critical Colors Only
Replace only the top 5 most common colors:
- `#ef4444` → `var(--danger)`
- `#2dd4bf` → `var(--accent)`
- `#22c55e` → `var(--success)`
- `#f59e0b` → `var(--warning)`
- `#3b82f6` → `var(--info)`

### Phase 2: Backgrounds & Text
- Background grays → `--bg-*` variables
- Text colors → `--text-*` variables
- Borders → `--border` variables

### Phase 3: Complete Migration
- All remaining colors
- Ensure dark/light theme compatibility

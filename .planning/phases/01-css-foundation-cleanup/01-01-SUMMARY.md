---
phase: 01-css-foundation-cleanup
plan: 01
subsystem: UI
tags: [css, keyframes, deduplication, animations]
dependency_graph:
  requires: []
  provides: [CSS-01]
  affects: [ui/css/animations.css, ui/css/tokens.css, ui/css/missions.css, ui/css/invasion.css, ui/css/cheatsheets.css]
tech_stack:
  added: [ui/css/animations.css]
  patterns: [centralized keyframe library, @keyframes deduplication]
key_files:
  created:
    - path: ui/css/animations.css
      lines: 531
      description: New centralized @keyframes library with 60 keyframe definitions
  modified:
    - path: ui/css/tokens.css
      description: Removed 88-line ANIMATION KEYFRAMES block, replaced with reference comment
    - path: ui/css/missions.css
      description: Removed duplicate pulse, card-enter, fadeIn @keyframes definitions
    - path: ui/css/invasion.css
      description: Removed duplicate card-enter @keyframes definition
    - path: ui/css/cheatsheets.css
      description: Removed duplicate fadeIn @keyframes definition
decisions:
  - id: CSS-01-D1
    decision: Extract all @keyframes into ui/css/animations.css rather than leaving them scattered
    rationale: "Duplicate keyframes (fadeIn, card-enter, pulse) were defined in 3+ files each, violating CSS-01"
    outcome: "Single source of truth for all shared animations; page-specific keyframes also consolidated for discoverability"
  - id: CSS-01-D2
    decision: Keep page-specific keyframes (msgIn, aurora-move, skToastIn, etc.) in animations.css alongside shared ones
    rationale: "Plan listed all keyframes to be included in animations.css; consolidating here makes the library complete and self-contained"
    outcome: "animations.css now serves as the complete animation dictionary for the entire UI"
metrics:
  duration: ~3 minutes
  completed: "2026-04-03T15:36:00Z"
  tasks_completed: 3
  files_created: 1
  files_modified: 4
  commits: 3
---

# Phase 1 Plan 1: CSS Keyframe Consolidation Summary

## One-liner

Consolidated 61 @keyframes definitions into `ui/css/animations.css`, eliminating duplicate fadeIn, card-enter, and pulse keyframes from tokens.css, missions.css, invasion.css, and cheatsheets.css per CSS-01.

## What Was Done

### Task 1: Create ui/css/animations.css
Created a new 531-line CSS file containing all 60 @keyframes definitions extracted from every CSS file in `ui/css/`. Organized by source page with clear section comments.

Included keyframes from:
- **tokens.css** (shared): fadeIn, fadeInUp, fadeInScale, card-enter, slideInRight, pulse, spin, bounce, float, voicePulse, shimmer, progress-stripes
- **chat.css**: msgIn, queueSlideIn, recorderSlideIn, recorderPulse, pulse-new, floatUp, statusIn, flashGreen, composerPopoverIn, typing, scaleIn, glowPulse, bounceSoft, gradientShift
- **config.css**: pulse-glow-success, cfgRestartSpin
- **containers.css**: ctPulse, ctSlideIn, ctSlideOut
- **dashboard.css**: dashTabIn, pulse-glow, valueUpdate
- **enhancements.css**: typingBounce, statusPulse, messageSlideIn, panelFadeIn, valuePop, moodPulse, connectionPulse, toastSlideIn, bgShift, gentleFloat
- **login.css**: aurora-move, orb-float, grid-pulse, cardIn, glow-in, logo-pulse, scan
- **skills.css**: skToastIn
- **stt-overlay.css**: stt-ring-pulse, stt-mic-glow, stt-status-pulse, stt-blink
- **knowledge.css**: kcFadeIn
- **truenas.css**: tn-spin
- **chat-modules.css**: pulse-dot, vr-pulse

### Task 2: Remove keyframes from tokens.css
Removed the 88-line ANIMATION KEYFRAMES section from `tokens.css`. Replaced with a comment directing developers to `animations.css`. tokens.css now contains only CSS custom properties (design tokens).

### Task 3: Remove duplicate keyframes from missions.css, invasion.css, cheatsheets.css
Removed duplicate @keyframes definitions:
- **missions.css**: pulse (line 98), card-enter (line 370), fadeIn (line 425)
- **invasion.css**: card-enter (line 37)
- **cheatsheets.css**: fadeIn (line 121)

## Verification Results

| Check | Expected | Actual | Status |
|-------|----------|--------|--------|
| animations.css @keyframes count | > 25 | 61 | PASS |
| fadeIn @keyframes location | only animations.css | only animations.css | PASS |
| card-enter @keyframes location | only animations.css | only animations.css | PASS |
| tokens.css @keyframes | 0 | 0 | PASS |
| missions.css duplicate keyframes | 0 | 0 | PASS |
| invasion.css duplicate keyframes | 0 | 0 | PASS |
| cheatsheets.css duplicate keyframes | 0 | 0 | PASS |

## Commits

| Commit | Description |
|--------|-------------|
| 3855f11 | feat(css): create animations.css with all centralized @keyframes definitions |
| 82578dd | refactor(css): remove @keyframes block from tokens.css |
| 4997df1 | refactor(css): remove duplicate @keyframes from missions, invasion, cheatsheets |

## Deviations from Plan

**None** - plan executed exactly as written.

## Notes

- The `animation: fadeIn 0.2s ease` references in missions.css and cheatsheets.css still work because the fadeIn keyframe now lives in `animations.css` which is loaded via the standard CSS import order
- Page-specific keyframes with similar names (pulse-new in chat.css, pulse-dot in chat-modules.css, pulse-glow in dashboard.css, pulse-glow-success in config.css) are intentionally different animations and remain in their respective files alongside the consolidated definitions in animations.css
- tokens.css comment replacement ensures developers know where to find the keyframes without adding an @import (the HTML pages include CSS files in a specific order that already loads animations.css)

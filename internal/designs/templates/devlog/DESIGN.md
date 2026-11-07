# DevLog

Technical, monospaced, terminal-inspired.

## Overview

DevLog is a dark-mode design system built for developer blogs and technical writing platforms. It channels the aesthetic of a well-configured terminal — green on dark, monospaced headings, and soft glowing accents that feel like phosphor on glass. Code blocks are first-class citizens here, and every typographic choice serves readability under sustained technical reading. The density is standard, giving code snippets and prose equal breathing room.

## Colors

### Brand Palette

| Token     | Hex       | Role                                        |
|-----------|-----------|----------------------------------------------|
| Primary   | `#4ADE80` | Green — links, active states, primary actions |
| Secondary | `#FBBF24` | Amber — warnings, highlights, annotations    |
| Tertiary  | `#22D3EE` | Cyan — info accents, secondary links          |

### Surface Palette

| Token          | Hex       | Role                                    |
|----------------|-----------|------------------------------------------|
| Background     | `#0F172A` | Navy-black base                          |
| Surface        | `#1E293B` | Cards, code blocks, panels               |
| Surface Raised | `#334155` | Hover states, elevated elements          |

### Content Palette

| Token          | Hex       | Role                                   |
|----------------|-----------|----------------------------------------|
| Text Primary   | `#F1F5F9` | Body text, headings                    |
| Text Secondary | `#94A3B8` | Comments, metadata, secondary copy     |
| Text Tertiary  | `#64748B` | Placeholders, disabled labels          |

### Border Palette

| Token         | Hex       |
|---------------|-----------|
| Border Subtle | `#1E293B` |
| Border Medium | `#334155` |
| Border Strong | `#475569` |

### Semantic Colors

| Token   | Hex       |
|---------|-----------|
| Success | `#4ADE80` |
| Warning | `#FBBF24` |
| Error   | `#F87171` |
| Info    | `#22D3EE` |

## Typography

### Font Stack

| Role             | Font                                                     |
|------------------|----------------------------------------------------------|
| Display/Headings | JetBrains Mono, 'Fira Code', 'Cascadia Code', monospace |
| UI/Body          | Inter, -apple-system, 'Segoe UI', Helvetica, sans-serif |
| Mono/Code        | JetBrains Mono, 'Fira Code', Consolas, monospace        |

### Type Scale

| Level        | Font           | Size   | Weight | Line Height | Letter Spacing | Usage                        |
|--------------|----------------|--------|--------|-------------|----------------|------------------------------|
| Display      | JetBrains Mono | 36px   | 700    | 1.2         | -0.02em        | Hero titles, landing headers |
| Headline     | JetBrains Mono | 28px   | 700    | 1.3         | -0.01em        | Post titles                  |
| Subhead      | JetBrains Mono | 20px   | 600    | 1.4         | 0              | Section headings             |
| Body Large   | Inter          | 18px   | 400    | 1.7         | 0              | Lead paragraphs, intros      |
| Body         | Inter          | 16px   | 400    | 1.7         | 0              | Default reading text         |
| Body Small   | Inter          | 14px   | 400    | 1.6         | 0              | Sidebar, supplementary text  |
| Caption      | Inter          | 12px   | 500    | 1.5         | 0.01em         | Timestamps, file paths       |
| Overline     | JetBrains Mono | 11px   | 600    | 1.4         | 0.1em          | Category tags, labels        |
| Code         | JetBrains Mono | 14px   | 400    | 1.65        | 0              | Inline code, code blocks     |

## Spacing

| Property                    | Value   |
|-----------------------------|---------|
| Base unit                   | 8px     |
| Scale                       | 4, 8, 16, 24, 32, 48, 64, 96 |
| Component padding — small   | 8px     |
| Component padding — medium  | 16px    |
| Component padding — large   | 32px    |
| Section spacing — mobile    | 48px    |
| Section spacing — tablet    | 64px    |
| Section spacing — desktop   | 96px    |

## Border Radius

| Token  | Value | Usage                              |
|--------|-------|------------------------------------|
| None   | 0px   | Dividers, rules                    |
| Small  | 4px   | Buttons, inputs, chips, cards      |
| Medium | 6px   | Code blocks, panels                |
| Large  | 8px   | Modals, dialogs                    |
| XL     | 12px  | Large containers                   |
| Full   | 9999px| Status dots, avatars               |

The default radius is 4px — subtle enough to feel terminal-native while softening raw rectangles.

## Shadows

**Philosophy:** DevLog uses a dark-mode glow system instead of traditional drop shadows. Accents emit soft colored light against the dark background, evoking a monitor glow.

| Level   | CSS Value                                             | Usage                    |
|---------|-------------------------------------------------------|--------------------------|
| Subtle  | `0 0 8px rgba(74, 222, 128, 0.08)`                   | Hover hints on cards     |
| Medium  | `0 0 16px rgba(74, 222, 128, 0.12)`                  | Active cards, focus rings|
| Large   | `0 0 32px rgba(74, 222, 128, 0.16)`                  | Modals, hero elements    |
| Overlay | `0 0 0 1px rgba(241, 245, 249, 0.06), 0 24px 48px rgba(0, 0, 0, 0.5)` | Dropdown menus, overlays |

**Special — Cyan Glow:** `0 0 20px rgba(34, 211, 238, 0.15)` — used for info elements and secondary focus states.

**Special — Amber Glow:** `0 0 16px rgba(251, 191, 36, 0.12)` — used for warning highlights and annotation markers.

## Components

### Buttons

**Primary**
- Background: `#4ADE80`
- Text: `#0F172A`
- Border: `1px solid #4ADE80`
- Padding: 8px 20px
- Font: Inter, 14px, weight 600
- Radius: 4px
- Hover: Background `#22C55E`, shadow `0 0 16px rgba(74, 222, 128, 0.2)`
- Active: Background `#16A34A`

**Secondary**
- Background: `#1E293B`
- Text: `#F1F5F9`
- Border: `1px solid #334155`
- Padding: 8px 20px
- Font: Inter, 14px, weight 600
- Radius: 4px
- Hover: Background `#334155`, border `#475569`
- Active: Background `#475569`

**Ghost**
- Background: transparent
- Text: `#94A3B8`
- Border: none
- Padding: 8px 20px
- Font: Inter, 14px, weight 600
- Radius: 4px
- Hover: Background `#1E293B`, text `#F1F5F9`
- Active: Background `#334155`

**Destructive**
- Background: `#F87171`
- Text: `#0F172A`
- Border: `1px solid #F87171`
- Padding: 8px 20px
- Font: Inter, 14px, weight 600
- Radius: 4px
- Hover: Background `#EF4444`, shadow `0 0 16px rgba(248, 113, 113, 0.2)`
- Active: Background `#DC2626`

**Sizes:** Small 6px 14px / 12px, Medium 8px 20px / 14px, Large 12px 28px / 16px

**Disabled:** Opacity 0.35, cursor not-allowed, no glow.

### Cards

**Default**
- Background: `#1E293B`
- Border: `1px solid #334155`
- Radius: 4px
- Padding: 24px
- Shadow: none
- Hover: Border `#475569`, shadow `0 0 8px rgba(74, 222, 128, 0.08)`

**Elevated**
- Background: `#1E293B`
- Border: `1px solid #475569`
- Radius: 4px
- Padding: 24px
- Shadow: `0 0 16px rgba(74, 222, 128, 0.12)`

### Inputs

**Text Input**
- Height: 40px
- Background: `#0F172A`
- Border: `1px solid #334155`
- Radius: 4px
- Padding: 8px 14px
- Font: Inter, 14px, weight 400
- Text color: `#F1F5F9`
- Placeholder color: `#64748B`
- Focus: Border `#4ADE80`, shadow `0 0 12px rgba(74, 222, 128, 0.15)`
- Error: Border `#F87171`, shadow `0 0 12px rgba(248, 113, 113, 0.15)`
- Disabled: Background `#1E293B`, opacity 0.4

**Label:** Inter, 12px, weight 600, color `#94A3B8`, margin-bottom 6px.

**Helper Text:** Inter, 12px, weight 400, color `#64748B`, margin-top 4px. Error helper color `#F87171`.

### Chips

**Filter Chip**
- Background: `#1E293B`
- Border: `1px solid #334155`
- Radius: 4px
- Padding: 4px 12px
- Font: JetBrains Mono, 12px, weight 500
- Text: `#94A3B8`
- Selected: Background `#4ADE80`, text `#0F172A`, border `#4ADE80`

**Status Chip**
- Padding: 4px 10px
- Font: JetBrains Mono, 11px, weight 600, uppercase
- Radius: 4px
- Success: Background `rgba(74, 222, 128, 0.12)`, text `#4ADE80`, border `1px solid rgba(74, 222, 128, 0.25)`
- Warning: Background `rgba(251, 191, 36, 0.12)`, text `#FBBF24`, border `1px solid rgba(251, 191, 36, 0.25)`
- Error: Background `rgba(248, 113, 113, 0.12)`, text `#F87171`, border `1px solid rgba(248, 113, 113, 0.25)`

### Lists

**Default List Item**
- Padding: 12px 16px
- Border bottom: `1px solid #1E293B`
- Font: Inter, 14px, weight 400
- Text: `#F1F5F9`
- Secondary text: `#64748B`, 12px
- Hover: Background `#1E293B`
- Active: Background `#334155`
- Leading element: 18px icon, color `#4ADE80`

### Checkboxes

- Size: 16px
- Border: `1.5px solid #475569`
- Radius: 3px
- Background: `#0F172A`
- Checked: Background `#4ADE80`, border `#4ADE80`, checkmark `#0F172A`
- Indeterminate: Background `#4ADE80`, dash `#0F172A`
- Hover: Border `#4ADE80`, shadow `0 0 8px rgba(74, 222, 128, 0.1)`
- Focus: Shadow `0 0 0 2px #0F172A, 0 0 0 4px #4ADE80`
- Disabled: Opacity 0.35
- Label: Inter, 14px, weight 400, color `#F1F5F9`, margin-left 8px

### Radio Buttons

- Size: 16px
- Border: `1.5px solid #475569`
- Radius: 9999px
- Background: `#0F172A`
- Selected: Border `#4ADE80`, inner dot `#4ADE80` (6px)
- Hover: Border `#4ADE80`, shadow `0 0 8px rgba(74, 222, 128, 0.1)`
- Focus: Shadow `0 0 0 2px #0F172A, 0 0 0 4px #4ADE80`
- Disabled: Opacity 0.35
- Label: Inter, 14px, weight 400, color `#F1F5F9`, margin-left 8px

### Tooltips

- Background: `#F1F5F9`
- Text: `#0F172A`
- Font: Inter, 12px, weight 500
- Padding: 6px 12px
- Radius: 4px
- Max width: 260px
- Arrow: 6px, same background
- Delay: 200ms enter, 0ms leave
- Shadow: `0 0 12px rgba(74, 222, 128, 0.1)`

## Do's and Don'ts

1. **Do** use JetBrains Mono for all headings and code — consistency with the terminal aesthetic is key.
2. **Do** give code blocks generous padding (24px) and distinguish them clearly with `#1E293B` backgrounds.
3. **Do** use the green glow sparingly — reserve it for hover, focus, and active states only.
4. **Don't** use light backgrounds; the dark navy-black `#0F172A` is the foundation. Never invert it.
5. **Don't** mix Amber and Cyan accents in the same component — one accent per element.
6. **Do** ensure all text meets WCAG AA contrast against `#0F172A` (minimum 4.5:1 for body).
7. **Don't** use heavy box shadows — glow effects should be subtle and atmospheric, not prominent.
8. **Do** support syntax highlighting themes that respect the green/amber/cyan palette.
9. **Don't** round corners beyond 4px for standard elements — keep the terminal sharpness intact.
10. **Do** provide a visible focus ring on all interactive elements using the green glow for keyboard navigation.

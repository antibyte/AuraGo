# StudioBlank Design System

## Overview

StudioBlank is an ultra-minimal design system where whitespace is the primary design feature. Built for photographer and visual artist portfolios, every UI element recedes so the work itself commands attention. The system uses no shadows, no border radius, and a monochromatic palette with weight contrast in a single type family. Geometry is pure, grids are strict, and ornamentation is entirely absent.

---

## Colors

- **Color Primary** (#0A0A0A): Primary text, actions
- **Color Secondary** (#FAFAFA): Backgrounds, inverse surfaces
- **Color Tertiary** (#D4D4D8): Subtle dividers, borders
- **Surface Base** (#FAFAFA): Page background
- **Surface Inverse** (#0A0A0A): Dark sections, footer
- **Color Success** (#16A34A): Upload complete
- **Color Warning** (#CA8A04): Storage warnings
- **Color Error** (#DC2626): Validation errors
- **Color Info** (#71717A): Informational notes

## Typography

- **Headline Font**: Inter
- **Body Font**: Inter
- **Mono Font**: IBM Plex Mono

- **text-hero**: Inter 64px bold, 1.05 line height
- **text-h1**: Inter 40px bold, 1.1 line height
- **text-h2**: Inter 28px semibold, 1.2 line height
- **text-h3**: Inter 20px semibold, 1.3 line height
- **text-body**: Inter 16px light, 1.65 line height
- **text-body-sm**: Inter 14px regular, 1.6 line height
- **text-caption**: Inter 12px regular, 1.5 line height
- **text-mono**: IBM Plex Mono 13px regular, 1.5 line height

---

## Spacing

Base unit: **16px**. Very generous spacing creates openness and directs focus to imagery.
- **space-1**: 4px — Tight inline gaps
- **space-2**: 8px — Icon-to-label spacing
- **space-3**: 16px — Standard element gap
- **space-4**: 32px — Between grouped elements
- **space-5**: 48px — Section inner padding
- **space-6**: 64px — Between sections
- **space-8**: 96px — Major page-level divisions
- **space-10**: 128px — Hero top/bottom margins

## Border Radius

- **radius-none** (0px): All elements (default)
All components use `border-radius: 0px`. No rounding is applied anywhere. Pure geometric edges define every surface, button, card, and input.

## Elevation

No shadows are used in StudioBlank. The system is completely flat. Depth is communicated exclusively through layering, spacing, and border contrast.
- **shadow-none**: None. Applied to all elements.
Focus states use a 2px border offset rather than a box-shadow ring.

## Components

### Buttons
All buttons are sharp-edged rectangles with no border-radius. Hover is communicated through background inversion.
#### Variants
| Variant     | Background    | Text Color  | Border              |
| ----------- | ------------- | ----------- | ------------------- |
| Primary     | #0A0A0A     | #FAFAFA   | none                |
| Secondary   | transparent   | #0A0A0A   | 1px #0A0A0A |
| Ghost       | transparent   | #0A0A0A   | none                |
| Destructive | #DC2626     | #FAFAFA   | none                |
#### Sizes
| Size   | Height | Padding (h)  | Font Size | Min Width |
| ------ | ------ | ------------ | --------- | --------- |
| Small  | 32px   | 16px         | 12px      | 64px      |
| Medium | 40px   | 24px         | 14px      | 96px      |
| Large  | 48px   | 32px         | 16px      | 128px     |
#### Disabled State
0.3 opacity, `not-allowed` cursor.
- No hover transitions

### Cards
| Property        | Value                                |
| --------------- | ------------------------------------ |
| Background      | #FFFFFF                            |
| Border          | 1px #E5E5E5                 |
| Border Radius   | 0px                                 |
| Padding         | 0px (image bleeds to edge)          |
| Shadow          | none                                |
| Hover           | Border shifts to #0A0A0A          |
Image cards have zero padding; the image is the card. Caption metadata sits below with space-3 gap.

### Inputs
| State    | Border Color  | Background  | Shadow |
| -------- | ------------- | ----------- | ------ |
| Default  | #D4D4D8     | #FFFFFF   | none   |
| Hover    | #A1A1AA     | #FFFFFF   | none   |
| Focus    | #0A0A0A     | #FFFFFF   | none   |
| Error    | #DC2626     | #FFFFFF   | none   |
| Disabled | #E5E5E5     | #F4F4F5   | none   |
1px (bottom border only variant available) border, 0px border radius. 40px tall, 14px Inter 400 font size. Focus: 2px #0A0A0A border.

### Chips
#### Filter Chips
| State    | Background    | Text Color | Border              |
| -------- | ------------- | ---------- | ------------------- |
| Default  | transparent   | #71717A  | 1px #D4D4D8 |
| Selected | #0A0A0A     | #FAFAFA  | 1px #0A0A0A |
| Hover    | #F4F4F5     | #0A0A0A  | 1px #A1A1AA |
#### Status Chips
| Type      | Background    | Text Color | Border              |
| --------- | ------------- | ---------- | ------------------- |
| Published | #0A0A0A     | #FAFAFA  | none                |
| Draft     | transparent   | #71717A  | 1px #D4D4D8 |
| Archived  | #F4F4F5     | #A1A1AA  | none                |
| Featured  | transparent   | #0A0A0A  | 1px #0A0A0A |
0px border radius. 12px Inter 400 uppercase tracking 0.05em. 28px tall.

### Lists
| Property          | Value                          |
| ----------------- | ------------------------------ |
| Row height        | 48px                           |
| Padding           | 16px horizontal                |
| Divider           | 1px #F4F4F5           |
| Hover background  | #F4F4F5                      |
| Active background | #0A0A0A with white text     |
| Border radius     | 0px                            |
| Font              | 14px Inter 400                 |

### Checkboxes
| State     | Fill          | Border            | Check Color |
| --------- | ------------- | ----------------- | ----------- |
| Unchecked | #FFFFFF     | 1px #D4D4D8 | --        |
| Checked   | #0A0A0A     | 1px #0A0A0A | #FAFAFA |
| Disabled  | #F4F4F5     | 1px #E5E5E5 | #A1A1AA |
18px, 0px border radius. Focus: 2px #0A0A0A offset 2px.

### Radio Buttons
| State      | Fill        | Border              | Dot Color   |
| ---------- | ----------- | ------------------- | ----------- |
| Unselected | #FFFFFF   | 1px #D4D4D8 | --          |
| Selected   | #FFFFFF   | 1px #0A0A0A | #0A0A0A   |
| Disabled   | #F4F4F5   | 1px #E5E5E5 | #A1A1AA   |
18px. 8px dot diameter. Focus: 2px #0A0A0A offset 2px.

### Tooltips
| Property     | Value                            |
| ------------ | -------------------------------- |
| Background   | #0A0A0A                        |
| Text color   | #FAFAFA                        |
| Font size    | 12px Inter 400                   |
| Padding      | 8px 12px                         |
| Border radius| 0px                              |
| Max width    | 200px                            |
| Arrow        | 6px triangle               |
| Delay        | 200ms show, 0ms hide             |
| Shadow       | none                             |
---

## Do's and Don'ts

1. **Do** let images speak -- the portfolio work is the design, not the interface.
2. **Don't** add decorative elements such as gradients, patterns, or ornamental shapes.
3. **Do** use generous margins (minimum 64px between major sections) to create visual breathing room.
4. **Don't** introduce more than one accent color across the entire site. Monochrome is the identity.
5. **Do** keep UI chrome to an absolute minimum -- navigation should nearly disappear.
6. **Don't** use rounded corners, shadows, or any depth effect. The system is strictly flat and geometric.
7. **Do** use weight contrast within Inter (300 vs 700) to establish hierarchy instead of size alone.
8. **Don't** overlay text on images unless absolutely necessary -- image integrity is paramount.
9. **Do** prioritize loading performance; lazy-load gallery images with simple fade-in transitions.
10. **Don't** use animations or transitions longer than 200ms. Movement should be barely perceptible.
# TypeGallery Design System

## Overview

TypeGallery is a typography-forward, editorial, and refined design system built for graphic designer and typographer portfolios. Type hierarchy is the primary design element -- every layout decision serves the letterforms. The system uses warm cream surfaces, deep brown and rust tones, and flat, shadowless surfaces so that nothing competes with the typography. Layouts draw from specimen sheets and editorial print design with generous leading, deliberate column rhythm, and sharp geometric edges.

---

## Colors

- **Color Primary** (#3C1518): Primary text, key actions
- **Color Secondary** (#F5F0E8): Background, surfaces
- **Color Tertiary** (#A44A3F): Accent links, highlights
- **Surface Base** (#F5F0E8): Page background (cream)
- **Surface Inverse** (#3C1518): Dark sections, footers
- **Color Success** (#3D7A4A): Save confirmation
- **Color Warning** (#B45309): Unsaved changes
- **Color Error** (#A44A3F): Validation errors
- **Color Info** (#8B7E74): Informational notes

## Typography

- **Headline Font**: EB Garamond
- **Body Font**: Manrope
- **Mono Font**: JetBrains Mono

- **text-hero**: EB Garamond 80px regular, 1.0 line height
- **text-h1**: EB Garamond 56px regular, 1.05 line height
- **text-h2**: EB Garamond 36px medium, 1.15 line height
- **text-h3**: EB Garamond 24px medium, 1.25 line height
- **text-body-lg**: Manrope 18px light, 1.75 line height
- **text-body**: Manrope 16px regular, 1.75 line height
- **text-caption**: Manrope 12px medium, 1.5 line height
- **text-mono**: JetBrains Mono 13px regular, 1.5 line height

Note the generous line heights (1.75) on body text to achieve editorial-grade leading.

---

## Spacing

Base unit: **12px**. Spacious, editorial rhythm.
- **space-1**: 4px — Tight inline elements
- **space-2**: 12px — Icon-to-label, small gaps
- **space-3**: 24px — Within component groups
- **space-4**: 36px — Card inner padding
- **space-5**: 48px — Between components
- **space-6**: 72px — Section padding
- **space-8**: 96px — Between major sections
- **space-10**: 120px — Page-level vertical rhythm

## Border Radius

- **radius-none** (0px): All elements (default)
All components use 0px corners. Sharp, editorial edges reinforce the print-design heritage. No rounding is applied to any surface, button, input, or card.

## Elevation

No shadows are used. TypeGallery is entirely flat. Visual hierarchy is established through typographic scale, weight, color, and spatial rhythm rather than elevation.
- **shadow-none**: None. Applied globally.
Focus states use a 2px border in color-tertiary rather than a box-shadow ring.

## Components

### Buttons
Buttons are sharp-edged with refined, understated styling. Hover states use color inversion. Text is set in Manrope 500.
#### Variants
- **Primary**: #3C1518 fill, #F5F0E8 text, no border.
- **Secondary**: transparent fill, #3C1518 text, 1px #3C1518 border.
- **Ghost**: transparent fill, #A44A3F text, no border.
- **Destructive**: #A44A3F fill, #F5F0E8 text, no border.
#### Sizes
Sizes: Small (32px, 16px, 12px, 72px), Medium (40px, 24px, 14px, 100px), Large (48px, 32px, 16px, 128px).
#### Disabled State
0.35 opacity, disabled cursor.
- No hover transitions

### Cards
#FFFFFF fill, 1px border-default border, square, 36px padding, no shadow, Hover: Border shifts to #3C1518.
Cards use generous internal padding and rely on typographic hierarchy to structure content.

### Inputs
- **Default**: #8B7E74 border color, #FFFFFF fill, no shadow.
- **Hover**: #3C1518 border color, #FFFFFF fill, no shadow.
- **Focus**: #A44A3F border color, #FFFFFF fill, no shadow.
- **Error**: #A44A3F border color, #FDF6F5 fill, no shadow.
- **Disabled**: #D6CFC4 border color, #EDE8DE fill, no shadow.
1px (underline variant also available) border, 0px border radius. 40px tall, 16px Manrope 400 font size. Focus: 2px #A44A3F border.

### Chips
#### Filter Chips
- **Default**: transparent fill, #8B7E74 text, 1px #D6CFC4 border.
- **Selected**: #3C1518 fill, #F5F0E8 text, 1px #3C1518 border.
- **Hover**: #EDE8DE fill, #3C1518 text, 1px #8B7E74 border.
#### Status Chips
- **Published**: #3C1518 fill, #F5F0E8 text, Filled circle indicator.
- **Draft**: transparent fill, #8B7E74 text, 1px border indicator.
- **Featured**: #A44A3F fill, #F5F0E8 text, Star indicator.
- **Archived**: #EDE8DE fill, #B5A99A text.
0px border radius. 12px Manrope 500 uppercase tracking 0.08em. 28px tall.

### Lists
48px row height, 24px horizontal padding, 1px #D6CFC4 divider, #EDE8DE hover background, #3C1518 with cream text active background, square, 16px Manrope 400 font.

### Checkboxes
- **Unchecked**: #FFFFFF fill, 1px #8B7E74 border.
- **Checked**: #3C1518 fill, 1px #3C1518 border, #F5F0E8 checkmark.
- **Disabled**: #EDE8DE fill, 1px #D6CFC4 border, #B5A99A checkmark.
18px, 0px border radius. Focus: 2px #A44A3F offset 2px.

### Radio Buttons
- **Unselected**: #FFFFFF fill, 1px #8B7E74 border.
- **Selected**: #FFFFFF fill, 1px #3C1518 border, #3C1518 dot.
- **Disabled**: #EDE8DE fill, 1px #D6CFC4 border, #B5A99A dot.
18px. 8px dot diameter. Focus: 2px #A44A3F offset 2px.

### Tooltips
#3C1518 fill, #F5F0E8 text, 13px Manrope 400 font size, 8px 14px padding, square, 220px max width, 6px triangle arrow, 200ms show, 50ms hide delay, no shadow.
---

## Do's and Don'ts

1. **Do** treat type hierarchy as the primary design element -- scale, weight, and spacing should carry the entire visual structure.
2. **Don't** introduce decorative elements that compete with letterforms. No gradients, patterns, or ornamental graphics.
3. **Do** use generous leading (1.75x on body text) to create an open, editorial reading rhythm.
4. **Do** establish column rhythm using a consistent baseline grid; all text blocks should align across columns.
5. **Don't** use more than two typefaces in a single layout. The serif/sans pairing is deliberate; adding a third breaks coherence.
6. **Do** design specimen-style layouts for showcasing type work: large glyph displays, character sets, and weight ramps.
7. **Don't** use rounded corners or drop shadows anywhere. The sharp, flat aesthetic reflects print precision.
8. **Do** use the warm cream background consistently -- pure white should only appear on raised card surfaces.
9. **Don't** set body text smaller than 16px. Readability and typographic craft must be evident at every scale.
10. **Do** leverage italic and small-caps variants of EB Garamond for elegant inline emphasis rather than color or weight alone.
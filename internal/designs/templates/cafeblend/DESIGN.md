# CafeBlend Design System

## Overview

CafeBlend is a cozy, coffee-toned design system for coffee shop and small cafe websites. The neumorphism-inspired elevation and warm browns create a tactile, inviting atmosphere that mirrors the comfort of a neighborhood cafe. Rounded forms and soft gradients make every element feel warm to the touch.

---

## Colors

- **Primary** (#8B5E3C): CTAs, nav accents, headings
- **Secondary** (#C4956A): Tags, decorative accents
- **Tertiary** (#FDF6EC): Background tint, card wash
- **Background** (#FDF6EC): Page background (warm cream)
- **Surface** (#FFFFFF): Cards, modals
- **Surface Inset** (#F0E6D6): Neumorphic inset fields
- **Success** (#059669): Order ready, available
- **Warning** (#D97706): Low stock, wait times
- **Error** (#EF4444): Out of stock, error
- **Info** (#6366F1): New item, seasonal

## Typography

- **Headline Font**: DM Serif Display
- **Body Font**: Work Sans
- **Mono Font**: IBM Plex Mono

- **h1**: 34px regular, 42px line height. Cafe name, hero text.
- **h2**: 26px regular, 34px line height. Section headings.
- **h3**: 20px regular, 28px line height. Menu category titles.
- **h4**: 16px regular, 24px line height. Item names.
- **body**: 14px regular, 22px line height. Descriptions.
- **small**: 12px regular, 18px line height. Prices, meta info.
- **mono**: 13px regular, 20px line height. Order numbers, prices.

---

## Spacing

Base unit: **8px**
- **xs**: 4px — Tight gaps
- **sm**: 8px — Inline spacing
- **md**: 16px — Component padding
- **lg**: 24px — Card padding
- **xl**: 32px — Section gaps
- **2xl**: 48px — Page section spacing
- **3xl**: 64px — Layout margins

## Border Radius

- **None** (0px): —
- **sm** (6px): Tags, small elements
- **md** (12px): Buttons, cards, inputs
- **lg** (16px): Modals, panels
- **full** (9999px): Avatars, pills, toggles

## Elevation

Neumorphism-inspired with warm tones for raised and inset states.
- **raised**: 6px 6px 12px #D5C9B8, -6px -6px 12px #FFFFFF. Raised elements.
- **inset**: inset 4px 4px 8px #D5C9B8, inset -4px -4px 8px #FFFFFF. Inset inputs.
- **sm**: 2px offset, 4px blur, #000000 at 6%. Subtle lift.
- **md**: 4px offset, 12px blur, #000000 at 8%. Cards, dropdowns.
- **lg**: 8px offset, 24px blur, #000000 at 12%. Modals.
- **focus**: 3px ring #8B5E3C at 25%. Focus ring.

## Components

### Buttons
#### Variants
- ****Primary****: #8B5E3C fill, #FFFFFF text, no border. Hover: #6B4226 + raised.
- ****Secondary****: #FAF0E1 fill, #8B5E3C text, no border. Hover: raised` intensified.
- ****Ghost****: Transparent fill, #6D4C41 text, no border. Hover: bg #FAF0E1.
- ****Destructive****: #EF4444 fill, #FFFFFF text, no border. Hover: #DC2626.
- Neumorphic buttons use `raised` shadow in default state, flatten on press
#### Sizes
Sizes: sm (6px 14px, 12px, 34px), md (8px 18px, 14px, 42px), lg (10px 24px, 16px, 50px).
#### Disabled State
none (flat) shadow. 0.4 opacity, disabled cursor.

### Cards
- ****Default****: #FFFFFF fill, no border, raised shadow, 12px radius.
- ****Elevated****: #FFFFFF fill, no border, md shadow, 12px radius.
24px padding.
- Neumorphic cards sit on `background` surface for full shadow effect
top with `12px 12px 0 0` radius image slot.

### Inputs
- **Default**: no border, #F0E6D6 fill, inset shadow.
- **Hover**: no border, #EBE0D0 fill, inset shadow.
- **Focus**: no border, #F0E6D6 fill, inset` + `focus shadow.
- **Error**: #EF4444 border, #FEF2F2 fill, no shadow.
- **Disabled**: no border, #F5EDE3 fill, no shadow.
42px, padding: 10px 14px, radius: 12px tall.
- Neumorphic inset style for default/focus states
Work Sans 500, 14px, `text-primary`, 6px bottom margin **label**, Work Sans 400, 12px, `text-tertiary`, 4px margin-top; errors use `error` color **helper text**.

### Chips
- ****Filter****: #FAF0E1 fill, #8B5E3C text, no border, pill shape.
- ****Status****: varies fill, varies text, no border, pill shape.
- Filter chips use `raised` shadow (mini neumorphic)
Status chip semantic mapping:
bg #DCFCE7, text #059669 available, bg #FEF3C7, text #D97706 limited, bg #FEE2E2, text #EF4444 sold out, bg #EDE9FE, text #6366F1 seasonal.

### Lists
Work Sans 400 14px. 52px row height, 12px/16px padding, 1px #E8DDD0 divider. Hover: background #FAF0E1. Selected: background #F0E6D6, left border 3px #8B5E3C.

### Checkboxes
18px square, radius: 6px. Unchecked: background #F0E6D6, shadow `inset` (mini). Checked: background #8B5E3C, white checkmark, shadow `raised` (mini). Indeterminate: background #8B5E3C, white dash. Disabled: 40% opacity. Labels in 8px gap Work Sans 400 14px.

### Radio Buttons
18px circle. Unchecked: background #F0E6D6, shadow `inset` (mini). Selected: border 2px #8B5E3C, inner dot 10px #8B5E3C, shadow `raised`. Disabled: 40% opacity. Labels in 8px gap Work Sans 400 14px.

### Tooltips
#3E2723 fill, #FFFFFF, Work Sans 400, 12px text, 8px corners, `md` shadow. 8px/12px padding, 6px arrow, 220px max width.
---

## Do's and Don'ts

1. **Do** use the neumorphic `raised`/`inset` shadows only on the warm cream background — they break on pure white.
2. **Do** keep the coffee-toned palette consistent; avoid introducing cool blues or grays.
3. **Don't** stack multiple neumorphic elements close together — they need spacing to maintain the soft 3D illusion.
4. **Do** use DM Serif Display sparingly for headlines only; never for body text.
5. **Don't** apply `raised` shadow to small elements under 24px — the effect is lost and looks muddy.
6. **Do** include high-quality product photography with warm white balance matching the palette.
7. **Don't** use flat error states with neumorphic inputs; switch to bordered style for error/focus clarity.
8. **Do** maintain a cozy, neighborhood feel — avoid corporate language or dense data tables.
9. **Don't** use more than two headline sizes on a single page to keep the intimate cafe scale.
10. **Do** test the neumorphic shadows in both light and dark ambient conditions for readability.
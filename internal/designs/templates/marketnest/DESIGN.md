# MarketNest

Warm, community-driven, handmade feel.

## Overview

MarketNest is a design system for handmade goods and artisan marketplaces where every product tells a story. The spacious layout gives makers room to showcase their craft through rich imagery and narrative descriptions. Warm terracotta, earthy sand, and forest green evoke natural materials and handcrafted authenticity. A flat, border-driven visual style avoids the corporate polish of mainstream e-commerce, instead embracing the warmth and imperfection that buyers of artisan goods cherish.

## Colors

- **Primary** (#C2410C): Primary CTAs, seller highlights, active navigation
- **Secondary** (#D4A373): Warm accents, featured badges, decorative borders
- **Tertiary** (#365314): Category tags, eco-friendly badges, secondary actions
- **Background** (#FFFBF5): Global page background, warm cream base
- **Surface** (#FFFFFF): Product cards, modals, seller profiles
- **Success** (#22C55E)
- **Warning** (#F59E0B)
- **Error** (#DC2626)
- **Info** (#3B82F6)

## Typography

- **Headline Font**: Lora
- **Body Font**: Open Sans
- **Mono Font**: Source Code Pro

- **Display**: Lora 48px bold, 1.15 line height, 0.01em tracking. Marketplace hero headlines.
- **Headline**: Lora 36px semibold, 1.25 line height, 0.005em tracking. Category pages, collection titles.
- **Subhead**: Lora 26px semibold, 1.3 line height. Section headings, seller names.
- **Body Large**: Open Sans 18px regular, 1.6 line height. Product story introductions.
- **Body**: Open Sans 16px regular, 1.6 line height. Default body text.
- **Body Small**: Open Sans 14px regular, 1.5 line height. Reviews, material details, specs.
- **Caption**: Open Sans 12px medium, 1.4 line height, 0.02em tracking. Shipping info, stock labels.
- **Overline**: Open Sans 11px bold, 1.2 line height, 0.09em tracking. Category tags, "handmade" badges (uppercase).
- **Code**: Source Code Pro 14px regular, 1.5 line height. Order numbers, tracking codes.

## Spacing

- **Base unit:** 8px
- **Scale:** 4, 8, 12, 16, 24, 32, 48, 64, 96, 128
- **Component padding:** 8px (small), 16px (medium), 24px (large)
- **Section spacing:** 56px (mobile), 80px (tablet), 112px (desktop)

## Border Radius

- **None:** 0px — Full-bleed hero images, dividers
- **Small:** 4px — Badges, inline tags, small chips
- **Medium:** 8px — Cards, product images, modals
- **Large:** 12px — Featured seller cards, collection banners
- **XL:** 20px — Hero sections, promotional callouts
- **Full:** 9999px — Avatars, seller profile pictures, dot indicators

## Elevation

MarketNest follows a flat design philosophy with no shadows. Depth is achieved through warm background layering and deliberate border usage.
- **Subtle:** none — Use `border: 1px #E7E5E4` instead
- **Medium:** none — Use `border: 1px #D6D3D1` with `bg: #FFFBF5` layering
- **Large:** none — Use `border: 2px #D6D3D1` for elevated importance
- **Overlay:** 8px offset, 24px blur, #1C1917 at 10% — Exception: only for modals and popovers
- **Focus Ring:** 3px ring #C2410C at 20% — Terracotta-tinted focus indicator

## Components

### Buttons
**Primary (Filled)** — `bg: #C2410C`, `text: #FFFFFF`, `font: Open Sans 15px/600`, `padding: 12px 24px`, `radius: 4px`, `hover: #9A3412`, `active: #7C2D12`
**Secondary (Outline)** — `bg: transparent`, `text: #C2410C`, `border: 1.5px #C2410C`, `radius: 4px`, `hover: bg #FDF6EC`
**Ghost** — `bg: transparent`, `text: #57534E`, `hover: bg #FDF6EC`
**Destructive** — `bg: #DC2626`, `text: #FFFFFF`, `radius: 4px`, `hover: #B91C1C`
- **Sizes**: Small `34px h / 10px 16px`, Medium `42px h / 12px 24px`, Large `50px h / 14px 32px`
- **Disabled**: 40% opacity, disabled cursor

### Cards
**Default** — `bg: #FFFFFF`, `border: 1px #E7E5E4`, `radius: 8px`, `padding: 0 (image flush top) / 20px (content area)`, `hover: border #D6D3D1`
**Elevated** — `border: 2px #D6D3D1`, `bg: #FFFBF5`, `hover: border #A8A29E`

### Inputs
**Text Input** — `bg: #FFFFFF`, `border: 1.5px #D6D3D1`, `text: #1C1917`, `placeholder: #A8A29E`, `radius: 4px`, `padding: 0 14px`, `height: 42px`, `font: Open Sans 16px/400`, `focus: border #C2410C, ring 3px ring #C2410C at 15%, `error: border #DC2626, ring #DC2626 at 15%, `disabled: bg #F5F5F4, 50% opacity`
- **Label**: top, Open Sans, 13px, 600, #1C1917
- **Helper text**: 12px, #57534E

### Chips
**Filter Chip** — `height: 32px`, `padding: 0 14px`, `radius: 4px`, `border: 1px #D6D3D1`, `selected: bg #C2410C, text #FFFFFF, border #C2410C`, `hover: bg #FDF6EC`
**Status Chip** — Success: `bg #DCFCE7, text #166534` / Warning: `bg #FEF3C7, text #92400E` / Error: `bg #FEE2E2, text #991B1B`

### Lists
**Default List Item** — `height: 48px`, `padding: 0 16px`, `font: Open Sans 16px/400`, `divider: 1px #E7E5E4`, `hover: bg #FFFBF5`, `selected: bg #FDF6EC, text #C2410C`, `icon variant: 20px icon, 12px gap`

### Checkboxes
18px, border: 1.5px #D6D3D1, radius: 3px, checked: bg #C2410C border #C2410C with white checkmark, indeterminate: bg #C2410C with white dash, disabled: 40% opacity, label: Open Sans 14px/400 with 10px gap.

### Radio Buttons
18px, border: 1.5px #D6D3D1, selected: border #C2410C with 6px inner dot #C2410C, disabled: 40% opacity, label: Open Sans 14px/400 with 10px gap.

### Tooltips
#1C1917, text: #FFFFFF, font: Open Sans 12px/400, padding: 8px 12px, radius: 4px, max-width: 220px, arrow: 6px, delay: 300ms, position: top preferred fill.

## Do's and Don'ts
- **Do** feature the maker's story alongside their product; include a seller avatar and short bio on every product card.
- **Do** use warm, natural photography with soft lighting; avoid stark white studio backgrounds.
- **Do** highlight materials and process (e.g., "hand-thrown stoneware") with the tertiary forest green tags.
- **Do** use the serif heading font (Lora) to reinforce the handcrafted, editorial feel of the marketplace.
- **Do** show real customer photos in reviews to build community trust.
- **Don't** use aggressive urgency tactics (countdown timers, "only 1 left!"); artisan buyers value authenticity over pressure.
- **Don't** apply drop shadows to product cards; the flat, bordered style is intentional and must remain consistent.
- **Don't** use the terracotta primary for decorative borders; reserve it for interactive, actionable elements.
- **Don't** crop product images to squares if the item is tall or wide; use natural aspect ratios.
- **Don't** hide the seller's location or shipping origin; transparency about provenance is core to the marketplace trust model.
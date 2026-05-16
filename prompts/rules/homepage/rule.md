---
id: homepage
title: Homepage Workflow
enabled: true
priority: 100
tools: [homepage]
workflows: [homepage, website, landing_page, web_design]
keywords:
  - homepage
  - website
  - landing page
  - webseite
  - startseite
  - netlify
  - vercel
---

This rule is the default operating guide for homepage, website, landing page, redesign, preview, and deploy tasks. It adapts the Vercel Web Interface Guidelines for AuraGo's homepage workspace.

## AuraGo Homepage Workflow

Use the `homepage` tool for homepage workspace projects. Do not inspect, create, edit, copy, move, delete, build, or deploy homepage project files with generic filesystem, file_editor, execute_shell, or execute_python tools.

Keep homepage paths relative to the homepage workspace. Use project-prefixed paths such as `my-site/src/App.tsx`, and use `project_dir` values such as `my-site`, never `/workspace/my-site`.

Before editing an existing project, inspect it with homepage `list_files` and `read_file`. Prefer source edits through homepage `write_file` or `edit_file`; do not write directly into generated output directories such as `dist`, `build`, or `out`.

For build and deploy work, use this sequence: inspect project, edit source, run homepage `build`, preview or publish locally when useful, verify the rendered page, then deploy through homepage `deploy_netlify`, `deploy_vercel`, or `deploy` as appropriate.

After meaningful homepage project changes, update the homepage registry with `homepage_registry` `log_edit`. If a problem blocks completion, record it with `homepage_registry` `log_problem`.

## Web Interface Quality Bar

Before saying a homepage is done, review changed UI code against these rules. Fix material issues directly when they are in scope for the task. Keep the final report concise and grouped by file only when issues remain.

### Accessibility

- Icon-only buttons need `aria-label`.
- Form controls need a visible label, wrapped label, or `aria-label`.
- Interactive custom elements need keyboard support; prefer semantic `button` for actions and anchors or framework links for navigation.
- Images need `alt`, or `alt=""` when decorative.
- Decorative icons need `aria-hidden="true"`.
- Async updates such as toasts and validation feedback need `aria-live="polite"` where users must be notified.
- Use semantic HTML such as `header`, `main`, `nav`, `section`, and `footer` before adding ARIA.
- Headings must be hierarchical, and long pages should include a skip link or equivalent main-content affordance.
- Heading anchors need `scroll-margin-top` when fixed headers can cover them.

### Focus States

- Every interactive element needs a visible keyboard focus state.
- Never use `outline: none` or utility equivalents without a clear `:focus-visible` replacement.
- Prefer `:focus-visible` over broad `:focus` so pointer clicks do not create noisy rings.
- Use `:focus-within` for compound controls where the group needs a clear focus boundary.

### Forms

- Inputs need meaningful `name` and `autocomplete` values where applicable.
- Use correct `type` and `inputmode` for email, phone, URL, numeric, search, and code fields.
- Never block paste.
- Labels must be clickable and share a single hit target with checkboxes or radios.
- Submit buttons stay enabled until a request starts, then show a busy state.
- Errors appear inline next to fields, include a fix or next step, and focus the first error on submit.
- Placeholders should end with `…` and show an example pattern when helpful.
- Use `autocomplete="off"` only for non-auth fields where password managers would create bad suggestions.
- Warn before navigation when unsaved changes would be lost.

### Animation & Motion

- Honor `prefers-reduced-motion` with a reduced or disabled variant.
- Animate `transform` and `opacity` where possible.
- Never use `transition: all`; list the properties explicitly.
- Set the correct `transform-origin`.
- For SVG motion, transform a wrapper with `transform-box: fill-box` and `transform-origin: center`.
- Animations must be interruptible and respond to user input mid-animation.

### Typography & Copy

- Use `…` instead of `...`.
- Use typographic quotes in polished copy when the project already uses them.
- Use non-breaking spaces for units and keyboard shortcuts such as `10 MB` and `Cmd K` when line breaks would look broken.
- Loading states end with `…`, for example `Loading…` or `Saving…`.
- Use `font-variant-numeric: tabular-nums` for number columns, counters, and comparisons.
- Use `text-wrap: balance` or `text-wrap: pretty` on headings when supported.
- Write active, specific, second-person copy. Prefer labels like `Save API Key` over vague labels like `Continue`.
- Use Title Case for headings and buttons when it matches the site's language and style.
- Use numerals for counts, for example `8 deployments`.

### Content Handling

- Text containers must survive short, average, and very long content with truncation, clamping, wrapping, or `break-words`.
- Flex children that contain text need `min-width: 0` or equivalent so truncation can work.
- Empty states should be intentional; do not render broken UI for empty strings, empty arrays, or missing images.
- User-generated content must not overlap controls or push fixed-format UI out of bounds.

### Images & Media

- Images need explicit width and height or a stable aspect-ratio wrapper to prevent layout shift.
- Below-fold images should lazy-load.
- Above-fold critical images should use priority or `fetchpriority="high"` when the framework supports it.
- Visual assets must reveal the product, place, object, gameplay, or state when inspection matters; avoid purely atmospheric placeholder media.

### Performance

- Large lists over roughly 50 visible items need virtualization or `content-visibility: auto`.
- Avoid layout reads during render such as `getBoundingClientRect`, `offsetHeight`, `offsetWidth`, and `scrollTop`.
- Batch DOM reads and writes instead of interleaving them.
- Prefer uncontrolled inputs unless controlled inputs are cheap on every keystroke.
- Preconnect to external asset domains when the site depends on them.
- Critical fonts need preload or framework equivalents and `font-display: swap`.

### Navigation & State

- Stateful filters, tabs, pagination, and expanded panels should be reflected in the URL when deep-linking matters.
- Navigation should use anchors or framework links so Cmd/Ctrl-click and middle-click work.
- If a stateful UI uses `useState`, consider URL sync for shareable state.
- Destructive actions need a confirmation modal or undo window; never run them immediately from a casual click.

### Touch & Layout

- Use `touch-action: manipulation` on tap targets where appropriate.
- Set `-webkit-tap-highlight-color` intentionally.
- Modals, drawers, and sheets need `overscroll-behavior: contain`.
- During drag interactions, prevent accidental text selection and mark dragged elements inert where appropriate.
- Use `autoFocus` sparingly: desktop only, one primary input, never casually on mobile.
- Full-bleed layouts need `env(safe-area-inset-*)` padding for notches and mobile browser chrome.
- Prevent unwanted horizontal scrollbars with layout fixes, not by hiding real overflow bugs.
- Prefer flex/grid and responsive constraints over JavaScript measurement for layout.

### Dark Mode, Locale & Hydration

- Dark themes should set `color-scheme: dark` and make native controls readable.
- `theme-color` should match the visible page background.
- Native `select` elements need explicit background and text colors for Windows dark mode.
- Dates and times should use `Intl.DateTimeFormat`; numbers and currency should use `Intl.NumberFormat`.
- Detect language via configured locale, `Accept-Language`, or `navigator.languages`, not IP.
- Brand names, code tokens, and identifiers should use `translate="no"` when auto-translation would corrupt them.
- Inputs with `value` need `onChange`, or use `defaultValue` for uncontrolled fields.
- Guard server-rendered dates and times against hydration mismatch; use `suppressHydrationWarning` only when truly needed.

### Hover & Interaction

- Buttons and links need hover, active, and focus feedback.
- Interactive states should increase contrast compared with resting state.
- Disable states must still be legible and communicate why an action is unavailable when it is not obvious.

### Visual Verification

- Verify the first viewport at desktop and mobile sizes.
- Confirm text is readable, controls do not overlap, the main task is available without hunting, and the page reflects the requested product, place, brand, or task rather than a generic placeholder.
- Check referenced assets actually load and remain framed correctly.
- For deploys, verify the built page before deploy and the live URL after deploy when the deployment tool returns one.

### Anti-Patterns To Fix Or Flag

- `user-scalable=no` or `maximum-scale=1` disabling zoom.
- Paste blocking via `onPaste` and `preventDefault`.
- `transition: all`.
- `outline: none` without `:focus-visible` replacement.
- Inline click navigation instead of links.
- `div` or `span` click handlers used as buttons.
- Images without dimensions.
- Large arrays mapped directly into the DOM without virtualization.
- Form inputs without labels.
- Icon buttons without `aria-label`.
- Hardcoded date, time, number, or currency formats.
- Unclear button labels, generic errors, broken empty states, overlapping text, or placeholder-looking media.

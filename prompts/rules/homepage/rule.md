---
id: homepage
title: Homepage Workflow
enabled: true
priority: 100
tools: [homepage, homepage_project, homepage_file, homepage_quality, homepage_deploy, homepage_git]
workflows: [homepage, website, landing_page, web_design]
keywords:
  - homepage
  - website
  - landing page
  - webseite
  - seite
  - site
  - startseite
  - netlify
  - vercel
---

This rule is the default operating guide for homepage, website, landing page, redesign, preview, and deploy tasks. It adapts the Vercel Web Interface Guidelines for AuraGo's homepage workspace.

## AuraGo Homepage Workflow

Use focused homepage tools for homepage workspace projects: `homepage_project` for workspace/project lifecycle and diagnostics, `homepage_file` for project files, `homepage_quality` for checks, `homepage_deploy` for preview/publish/deploy, and `homepage_git` for repository actions. Do not inspect, create, edit, copy, move, delete, build, or deploy homepage project files with generic filesystem, file_editor, execute_shell, or execute_python tools.

When the task is to create, recreate, delete, rebuild, redesign, publish, or deploy a website/page/site, treat it as a homepage workflow even if the user says only "Seite" or "site". Load and follow the `HOMEPAGE DESIGN SYSTEM` section before choosing colors, layout, cards, spacing, typography, or visual effects.

The default visual target is Atmospheric Glass. Unless the user supplied a different brand or project `DESIGN.md`, the resulting page should visibly use the Atmospheric Glass system: translucent glass surfaces, luminous layered gradients, soft borders, blur/backdrop-filter depth, restrained white/silver/blue accents, Inter-based type hierarchy, and readable contrast. When implementing, strictly enforce WCAG AA text contrast. Pair `backdrop-filter: blur(...)` with a fallback `background-color` (for reduced-transparency) and a subtle semi-transparent inner border (e.g., `border: 1px solid rgba(255,255,255,0.1)`) to define physical edges. A generic dark purple/blue card UI without glass physics does not satisfy the default design system.

Keep homepage paths relative to the homepage workspace. Use project-prefixed paths such as `my-site/src/App.tsx`, and use `project_dir` values such as `my-site`, never `/workspace/my-site`.

## Local Asset References

When adding local images, videos, fonts, downloads, or other static files, place them inside the project's public/static asset directory and reference them with project-relative web URLs, not host filesystem paths or homepage workspace paths.

- For Vite, React, and plain HTML projects, prefer `public/assets/...` or the framework-equivalent static directory.
- Example: write the file as `my-site/public/assets/hero.jpg`, then reference it in HTML, CSS, or JavaScript as `/assets/hero.jpg`.
- Never use `C:\...`, `/workspace/...`, `data/homepage/...`, `agent_workspace/...`, or `file://...` in page markup; browsers cannot fetch those from the served site.
- If importing assets from source code, keep the asset within the project and let the framework emit the served URL; verify the build or preview after moving or generating assets.

Before editing an existing project, inspect it with `homepage_file` `list_files` and `read_file`. Prefer source edits through `homepage_file` `write_file` or `edit_file`; do not write directly into generated output directories such as `dist`, `build`, or `out`.

For build and deploy work, use this sequence: inspect project, edit source, run `homepage_deploy` `build`, preview or publish locally when useful, verify the rendered page, then deploy through `homepage_deploy` `deploy_netlify`, `deploy_vercel`, or `deploy` as appropriate.

For Netlify static exports, pass the directory that actually contains the deployable `index.html`. If a framework project cannot build but a sibling static export such as `my-site-static/` exists, deploy that static directory with `build_dir: "."` or rely on the Netlify fallback result when it reports `fallback_project_dir`.

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
- Avoid generic AI marketing jargon (`Elevate`, `Seamless`, `Unleash`, `Next-Gen`). Use concrete, descriptive language.
- Do not use generic placeholder names like `Acme` or `John Doe`; invent realistic context-appropriate names.
- Avoid using the em-dash (`—`) as a stylistic crutch.
- Use Title Case for headings and buttons when it matches the site's language and style.
- Use numerals for counts, for example `8 deployments`.

### Content Handling

- Text containers must survive short, average, and very long content with truncation, clamping, wrapping, or `break-words`.
- Flex children that contain text need `min-width: 0` or equivalent so truncation can work.
- Empty states should be intentional; do not render broken UI for empty strings, empty arrays, or missing images.
- Prefer skeleton loaders that match the final layout shape over generic circular spinners to minimize Cumulative Layout Shift (CLS) during data fetching.
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
- Stateful UI that dictates the current view (e.g., active tabs, filters, pagination) SHOULD use URL query parameters instead of isolated local state to ensure deep-linkability.
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
- Always design mobile-first. Ensure complex layouts (grids, sidebars, multi-column heroes) gracefully collapse to a single column on mobile screens (`< 768px`). Do not ship desktop-only interfaces.
- Maintain strict z-index discipline. Avoid arbitrary high values like `z-50` or `z-[9999]` for regular content. Keep stacking contexts flat and predictable.

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

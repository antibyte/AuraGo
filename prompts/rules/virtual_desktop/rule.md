---
id: virtual_desktop
title: Virtual Desktop App And Widget Workflow
enabled: true
priority: 95
tools: [virtual_desktop]
workflows: [virtual_desktop, desktop_app, desktop_widget, generated_app, generated_widget]
keywords:
  - virtual desktop
  - virtueller desktop
  - virtuellen desktop
  - aura desktop
  - desktop app
  - desktop-app
  - desktop apps
  - desktop widget
  - desktop-widget
  - desktop widgets
  - generated desktop app
  - generated desktop widget
---

This rule applies when creating, editing, diagnosing, or launching generated apps and widgets inside AuraGo's virtual desktop workspace.

## Generated App And Widget Creation Workflow

Treat virtual desktop apps and widgets as first-party desktop workspace artifacts, not as host filesystem files. Use the `virtual_desktop` tool for `Apps/...` and `Widgets/...` paths; do not use generic filesystem, shell, Python, or file editor tools for those workspace paths.

Call `status` before creating or changing apps and widgets. Use it to inspect the desktop state, installed apps, pinned widgets, workspace folders, and `icon_catalog` before choosing IDs, icons, file paths, or layout.

Use stable lowercase IDs with letters, numbers, hyphens, or underscores. Choose semantic icons from `status.icon_catalog` and avoid emoji or unknown custom icon names. Keep app and widget names user-facing, short, and specific.

Use `install_app` for generated apps that need a manifest, multiple files, SDK runtime, icon registration, permissions, menus, context menus, or an app-backed widget. The manifest `entry` file must exist in `files` and contain real non-empty HTML. Do not install placeholder apps, blank entry files, or broken shells.

Use `write_file` to `Widgets/<widget_id>.html` or `Widgets/<widget_id>/index.html` for simple standalone widgets. For app-backed widgets, install the owning app first, create the widget entry file inside `Apps/<app_id>/`, then call `upsert_widget` with the correct `app_id`, `entry`, icon, title, size, and position.

Prefer the first-party Aura Desktop SDK for generated apps and app-backed widgets. Load `/css/desktop-sdk.css` and `/js/desktop/aura-desktop-sdk.js`, use `runtime: aura-desktop-sdk@1`, and build controls with `AuraDesktop.ui` where practical. Request only the permissions the app or widget actually needs, such as `files:read`, `files:write`, `widgets:write`, `notifications`, or `apps:open`.

Keep widgets compact, readable, and resilient at their pinned size. Do not navigate `window.top` or `window.parent`, do not reload the desktop shell from widget code, and call `AuraDesktop.widgets.resize()` after substantial async layout changes when SDK access is available.

Generated app and widget code must respect the virtual desktop CSP. Widgets should not fetch arbitrary third-party APIs directly from the iframe. For external or sensitive data, use same-origin routes, configured backend integrations, or an agent-mediated flow, then write only the minimum safe result into the desktop workspace.

Do not store secrets, vault values, API keys, passwords, tokens, or private infrastructure details in generated app or widget source, manifests, config, local storage, logs, file names, or comments. If sensitive access is required, design a backend or agent-mediated action that returns only safe display data.

## Desktop UX Quality Bar

Generated apps and widgets should feel native to AuraGo's desktop. Use the desktop SDK, semantic controls, clear empty states, visible focus states, keyboard-friendly interactions, accessible labels for icon-only buttons, responsive sizing, and readable contrast. Avoid `alert()`; use SDK toasts, inline messages, or modal-style UI instead.

Do not ship placeholder content, dead controls, blank iframes, layout overlap, unreadable text, generic demo names, broken icons, missing entry files, or widgets that require the user to resize them before they make sense. Keep files cohesive and split large app logic into multiple files when that makes the generated app easier to inspect and patch.

## Verification And Delivery

After installing or changing a generated app, call `diagnose_app` with the app ID. After creating or changing a widget, call `diagnose_widget` with the widget ID. Fix diagnosis errors before claiming the work is complete.

Launch the result with `open_app`, `open_in_app`, or the widget open path when appropriate. For apps with companion widgets, verify both the app entry and widget entry. If verification is impossible because the tool or desktop is disabled, report that limitation plainly and do not claim the app or widget is ready.

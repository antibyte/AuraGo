---
name: modern-web-guidance
description: Modern HTML, CSS and browser JavaScript guidance from GoogleChrome. Use when building or improving web interfaces, layouts, dialogs, forms, animations, accessibility, performance or browser APIs, including Homepage and Game Maker projects. Consult relevant local guides before implementing the feature. Not for backend-only work, infrastructure or trivial text edits.
license: Apache-2.0
compatibility: AuraGo Agent Skills with native file-reading tools; no Node.js, npm or network required.
metadata:
  author: GoogleChrome and contributors; AuraGo adapter
  source: https://github.com/GoogleChrome/modern-web-guidance-src
  distribution: https://github.com/GoogleChrome/modern-web-guidance
  revision: 4dd968349e54ca45a05dd5c818fb035c9054b022
  version: "1.0.0"
---

# Modern Web Guidance

AuraGo adaptation of GoogleChrome Modern Web Guidance, modified to use the
complete bundled guide corpus offline instead of the upstream npm search CLI.
The reference guides themselves are unchanged. Attribution and the pinned
revision are in `references/UPSTREAM.md`; license: `references/LICENSE.txt`.

## Find and read relevant guidance

1. Translate the requested feature into a concrete web capability, such as
   "confirmation dialog", "responsive table" or "animate intrinsic size".
2. Read `references/INDEX.md` and select the relevant guide paths. It groups all
   bundled guides by topic. Read the matching feature guide and, when useful,
   the corresponding general accessibility, HTML, CSS or performance guide.
3. Read those files completely before applying their recommendations. Follow
   their cross-references only when relevant to the task. Do not load the entire
   corpus or invent guide IDs.

Use AuraGo's `file_reader_advanced` with `operation: "read_lines"` (or
`search_context` for discovery). Paths are relative to the installed skill
directory, normally `../agent_skills/modern-web-guidance` from the agent workdir.
For example, read
`../agent_skills/modern-web-guidance/references/guides/ui-components/dialog.md`.
Use successive line ranges if a result is truncated. These are skill-reference
reads; keep project edits in the project's own Homepage/Game Maker tools.
If a custom skill directory is outside the permitted file roots, report the
unavailable references rather than bypassing the filesystem boundary.

The upstream guides sometimes refer to `npx ... retrieve "guide-id"`. In this
package, resolve that ID through the local index and read its Markdown file;
do not execute the npm command. The upstream search and legacy-scan executables
are not included. For a requested broad modernization review, inspect the
specified code and relevant guides with the existing tools and state the scope
actually reviewed; do not claim an automated legacy scan ran.

## Apply and verify

- Prefer the browser's native capabilities when they fit the task, adapting the
  vanilla examples to the project's framework and existing components.
- Respect the user's browser targets and project contracts. Baseline Widely
  available features need no default fallback; newer or limited features need
  the guide's feature detection and fallback unless the project's policy says
  otherwise. Compatibility tables describe the pinned snapshot, not live data.
- Preserve semantics, keyboard access, focus management, reduced-motion support
  and necessary fallbacks. Do not add unrelated features or dependencies.
- Check the changed behavior against the applicable guidance and existing tests.
  Briefly name the guides that materially affected the result and any remaining
  compatibility limits, without imposing a special response format.

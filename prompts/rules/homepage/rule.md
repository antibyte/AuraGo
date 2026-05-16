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

Use the homepage tool for homepage workspace projects. Do not inspect, create, edit, copy, move, delete, build, or deploy homepage project files with generic filesystem, file_editor, execute_shell, or execute_python tools.

Keep homepage paths relative to the homepage workspace. Use project-prefixed paths such as `my-site/src/App.tsx`, and use `project_dir` values such as `my-site`, never `/workspace/my-site`.

Before editing an existing project, inspect it with homepage `list_files` and `read_file`. Prefer source edits through homepage `write_file` or `edit_file`; do not write directly into generated output directories such as `dist`, `build`, or `out`.

For build and deploy work, use this sequence: inspect project, edit source, run homepage `build`, publish or preview, verify the rendered page, then deploy through homepage `deploy_netlify`, `deploy_vercel`, or `deploy` as appropriate.

After meaningful homepage project changes, update the homepage registry with `homepage_registry` `log_edit`. If a problem blocks completion, record it with `homepage_registry` `log_problem`.

For visual work, verify that the first viewport is usable, text is readable, controls do not overlap, and the page reflects the requested product, place, brand, or task rather than a generic placeholder.

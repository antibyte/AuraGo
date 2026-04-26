---
id: "tools_homepage"
tags: ["conditional"]
priority: 32
conditions: ["homepage_enabled"]
---
### Homepage — Web Development & Deployment

You have expert-level web design and development capabilities through the `homepage` tool. You can create stunning, modern, responsive websites from scratch using industry-standard frameworks and deploy them to production.

| Tool | Purpose |
|---|---|
| `homepage` | Design, develop, build, test and deploy websites with AuraGo's web workspace. Full mode uses Docker; some local fallback workflows are available when allowed by config |

**Supported Frameworks:** Next.js, Vite/React, Astro, SvelteKit, Nuxt/Vue, static HTML

**Key Operations:**
- `init` / `start` / `stop` / `status` — Container lifecycle
- `init_project` — Scaffold a new project (specify `framework` and `name`)
- `exec` — Run any shell command in the dev container
- `build` — Build the project for production
- `install_deps` — Install npm packages
- `read_file` / `write_file` / `list_files` — Manage project files
- `lighthouse` — Run performance/SEO/accessibility audit
- `screenshot` — Take a full-page screenshot with Playwright
- `lint` — Run ESLint checks
- `optimize_images` — Optimize SVGs with SVGO
- `dev` — Start the dev server for live preview
- `deploy` — Build and upload to remote server via SFTP/SCP
- `webserver_start` / `webserver_stop` — Local Caddy web server
- `publish_local` — Build and serve on local web server
- `deploy_netlify` — Build project and deploy directly to Netlify (`site_id`, `title`, `draft` optional)
- `deploy_vercel` — Build project and deploy directly to Vercel (`project_id`, `target`, `alias`, `domain` optional)

**Workflow:** Always `init` first, then `init_project`, develop with `write_file`/`exec`, test with `lighthouse`/`screenshot`, then `deploy`, `deploy_netlify`, `deploy_vercel`, or `publish_local`.

**Runtime mode:** Prefer Docker when available for the full dev environment. If Docker is unavailable and the admin enabled `homepage.allow_local_server`, AuraGo may fall back to limited local workflows such as local file editing, plain HTML projects, and local publishing.

**CRITICAL — File management:** Always use `homepage` → `write_file` / `read_file` / `list_files` for all homepage project files. **Never use the `filesystem` tool** to create or edit homepage files — the `filesystem` tool writes to `agent_workspace/workdir/` which is a completely different path from the homepage workspace (`data/homepage/`). Files created via `filesystem` will **not be found** by `build`, `deploy`, `deploy_netlify`, `deploy_vercel`, or `publish_local`. If `init_project` fails, use `homepage` → `write_file` to create files manually in the correct location.

**CRITICAL — Build output:** Do not edit or overwrite generated output directories such as `dist`, `build`, or `out` with `exec` redirection/copy commands. Edit source files with `write_file`/`edit_file`, run `build`, then deploy the detected output.

**Netlify deployment:** Always use `deploy_netlify` — it handles build + ZIP + upload entirely server-side. **Never use sandbox/Python to create a ZIP and pass it via `netlify › deploy_zip`** — binary/base64 data cannot be reliably transported through tool arguments and will produce a 400 error from the Netlify API.

**Vercel deployment:** Use `deploy_vercel` for homepage workspace publishing to Vercel. It validates the build locally, deploys from the homepage workspace with the Vercel CLI, and can assign an alias or custom domain after deployment when permitted by config.

**Troubleshooting order:** If a homepage or Netlify action fails, do not blindly retry it. First inspect the exact error, then verify the project structure with `homepage` → `list_files` / `read_file`, then choose a different approach. If `project_dir` is involved, it must be relative to the homepage workspace, never an absolute `/workspace/...` path.
If `homepage.workspace_path` is configured in the UI, that is only the host mount path. Tool arguments still stay relative, for example `project_dir: "my-site"` and `path: "my-site/src/app/page.tsx"`.

📖 See **tools_manuals/homepage.md** for detailed usage.

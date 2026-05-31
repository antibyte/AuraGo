# Homepage (`homepage`)

Design, develop, build, test and deploy professional websites using AuraGo's web workspace with Docker-based full mode and limited local fallback support.

## Required Task Rule

The `homepage` tool has a required active task rule: `homepage`.

Before planning, deleting, creating, editing, building, previewing, publishing, or deploying a homepage/website/page project, read and follow the current `# TASK RULES` section for `Homepage Workflow` and the `# HOMEPAGE DESIGN SYSTEM` section when it is present. The design system is not optional styling advice; it is the default visual contract for homepage work unless the user or project supplies a more specific `DESIGN.md`.

If those sections are not visible yet, do not improvise with generic filesystem, shell, or ad-hoc design choices. Use the `homepage` tool path; the runtime will pause the first matching tool call, inject the required rule/design context, and ask you to retry only after reading it. Intent matching is only a convenience fallback, not the authority.

## Prerequisites

- **Required**: Homepage integration enabled (`homepage.enabled: true`)
- **Recommended**: Docker integration enabled for the full dev environment
- For deployment: SFTP/SCP credentials must be stored in the vault

## Operations

| Operation | Description |
|-----------|-------------|
| `init` | Initialize the dev environment (run first) |
| `start` | Start the dev container |
| `stop` | Stop the dev container |
| `status` | Get environment status |
| `rebuild` | Rebuild the dev container from scratch |
| `destroy` | Remove everything; requires explicit `force: true` |
| `init_project` | Create a new web project |
| `exec` | Run a diagnostic shell command in the container |
| `write_file` | Write/create a file |
| `read_file` | Read a file |
| `list_files` | List project files |
| `install_deps` | Install npm packages |
| `build` | Build for production |
| `dev` | Start the dev server |
| `lint` | Run TypeScript & ESLint checks |
| `check_js` | Check for JavaScript runtime errors |
| `optimize_images` | Optimize SVGs with SVGO |
| `lighthouse` | Run Lighthouse performance audit |
| `screenshot` | Take a full-page screenshot |
| `deploy` | Build and upload to remote server via SFTP |
| `test_connection` | Test SFTP/SCP connection |
| `webserver_start` | Start local Caddy web server |
| `webserver_stop` | Stop the web server |
| `webserver_status` | Check web server status |
| `publish_local` | Build and serve locally |
| `deploy_netlify` | Build and deploy directly to Netlify |
| `deploy_vercel` | Build and deploy directly to Vercel |
| `git_init` | Initialize a git repository |
| `git_commit` | Commit all changes |
| `git_status` | View changed files |
| `git_diff` | View current changes |
| `git_log` | View commit history |
| `git_rollback` | Revert recent commits |
| `tunnel` | Create a public URL for sharing |

## Existing Project Fast Path

When updating or publishing an existing homepage project, use this direct path:

1. `homepage` → `list_files` with `path: "."` to identify the project directory.
2. `homepage` → `read_file` / `write_file` with a project-prefixed `path`, for example `my-site/index.html`.
3. `homepage` → `build` with `project_dir: "my-site"`; missing dependencies are installed automatically.
4. `homepage` → `publish_local` for browser checks, then `deploy_netlify` or `deploy_vercel`.
4. Verify the returned deployment URL or the live page before reporting success.

Do not use the generic `filesystem` tool to inspect, copy, or edit homepage project files. It writes to `agent_workspace/workdir/`, not the homepage workspace. Do not use generic `execute_shell` for `/workspace/...` commands either; `/workspace` is the homepage container path, so use `homepage` operations instead.

## Runtime Modes

### Docker Mode

Full dev environment with Node.js, Playwright, Lighthouse and Caddy web server with automatic HTTPS.
Supports all frameworks: Next.js, Vite, Astro, SvelteKit, Vue, static HTML.

## Container Lifecycle

### init — Initialize the dev environment
Creates the Docker image and container. **Run this first** before any other operation.
```json
{"action": "homepage", "operation": "init"}
```

**Note:** Docker should be running for the full toolset. If Docker is unavailable, start it with `sudo systemctl start docker`.

### Local Fallback Mode
If Docker is unavailable and `homepage.allow_local_server` is enabled, AuraGo can still handle limited local workflows:
- local/plain HTML project creation
- homepage workspace file reads and writes
- local publishing via the Python fallback server
- status checks for the fallback server

### start — Start the dev container
```json
{"action": "homepage", "operation": "start"}
```

### stop — Stop the dev container
```json
{"action": "homepage", "operation": "stop"}
```

### status — Get environment status
Returns status of Docker containers or Python fallback server.
```json
{"action": "homepage", "operation": "status"}
```

**Response fields:**
- `docker_available`: true/false - whether Docker is accessible
- `mode`: "docker" or "python_fallback"
- `python_server`: Status of Python HTTP server (in fallback mode)

### rebuild — Rebuild the dev container from scratch
Removes container and image, then rebuilds. Use when you need a fresh environment.
```json
{"action": "homepage", "operation": "rebuild"}
```

### destroy — Remove everything
Removes both containers and the dev image.
Only use this when the user explicitly asks to destroy/reset the homepage environment. For routine recovery prefer `status`, `start`, `init`, or `rebuild`.
```json
{"action": "homepage", "operation": "destroy", "force": true}
```

## Project Scaffolding

### init_project — Create a new web project

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `framework` | string | yes | `next`, `vite`, `astro`, `svelte`, `vue`, or `html` |
| `name` | string | no | Project name (default: "my-site") |

```json
{"action": "homepage", "operation": "init_project", "framework": "next", "name": "my-portfolio"}
```
```json
{"action": "homepage", "operation": "init_project", "framework": "astro", "name": "blog"}
```
```json
{"action": "homepage", "operation": "init_project", "framework": "html", "name": "landing-page"}
```

## Development

`write_file`, `read_file`, `edit_file`, `list_files`, and other project-file operations work against the **homepage dev workspace**, not the published web root.
Keep `project_dir`/`path` values relative to the homepage workspace.

If an existing project directory is owned by root or otherwise not writable by the homepage container user, `write_file`, `init_project`, and write-oriented `exec` commands will attempt an automatic project-scoped permission repair before failing. Do not retry with manual `chmod` from the unprivileged container user; inspect the returned repair error instead.

### exec — Run a shell command in the container
```json
{"action": "homepage", "operation": "exec", "command": "cd /workspace/my-site && npm run dev"}
```

Use `exec` for diagnostics such as reading package metadata, checking logs, or running one-off inspection commands. Do not use it as the normal build/deploy path, and do not use shell redirects, `cp`, `mv`, `rm`, or similar commands to write directly into generated output directories like `/workspace/my-site/dist`, `/workspace/my-site/build`, or `/workspace/my-site/out`. Edit source files with `write_file`/`edit_file`, run `build`, then deploy through `deploy_netlify` or `deploy_vercel`.

### write_file — Write/create a file
Content is safely base64-encoded internally. Parent directories are created automatically.
```json
{"action": "homepage", "operation": "write_file", "path": "my-site/src/components/Hero.tsx", "content": "export default function Hero() { return <section>...</section> }"}
```

### read_file — Read a file
```json
{"action": "homepage", "operation": "read_file", "path": "my-site/src/app/page.tsx"}
```

### list_files — List project files
Returns up to 200 files, excluding node_modules, .next, and .git.
```json
{"action": "homepage", "operation": "list_files", "path": "my-site/src"}
```

### install_deps — Install npm packages

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `packages` | string[] | no | Specific packages (omit for `npm install`) |
| `project_dir` | string | no | Project subdirectory |

```json
{"action": "homepage", "operation": "install_deps", "project_dir": "my-site", "packages": ["tailwindcss", "@headlessui/react", "framer-motion"]}
```

### build — Build for production
```json
{"action": "homepage", "operation": "build", "project_dir": "my-site"}
```

### dev — Start the dev server
Starts the dev server (e.g. `npm run dev`) in the background. When the Caddy web container is running and a `project_dir` is specified, a reverse_proxy route is automatically registered so the app is accessible through the Caddy web server at `/<project_dir>/`.

The dev container and Caddy web container are connected via a shared Docker network (`aurago-homepage-net`), allowing Caddy to forward requests to the dev server. Proxy routes are persisted in the workspace (`.aurago-proxy-routes.json`) and survive container restarts.

```json
{"action": "homepage", "operation": "dev", "project_dir": "my-site"}
```

### lint — Run TypeScript & ESLint checks
Runs `tsc --noEmit` (if `tsconfig.json` exists) followed by ESLint. Returns combined output.
```json
{"action": "homepage", "operation": "lint", "project_dir": "my-site"}
```

### check_js — Check for JavaScript runtime errors
Uses Playwright to load a page in headless Chromium and captures JavaScript errors and console errors. Useful for detecting runtime issues that linting alone cannot catch.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | yes | URL to check (e.g. `http://localhost:3000`) |

```json
{"action": "homepage", "operation": "check_js", "url": "http://localhost:3000"}
```

**Response fields:**
- `errorCount`: Number of JS errors detected
- `errors`: Array of error messages (page errors and console errors)

### optimize_images — Optimize SVGs with SVGO
Optimizes **SVG files only** (not PNG/JPEG). Uses SVGO with multipass optimization.
```json
{"action": "homepage", "operation": "optimize_images", "project_dir": "my-site"}
```

## Testing & Quality

### lighthouse — Run Lighthouse performance audit
Returns compact scores for performance, accessibility, best practices and SEO.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | yes | URL to audit (e.g. `http://localhost:3000`) |

```json
{"action": "homepage", "operation": "lighthouse", "url": "http://localhost:3000"}
```

### screenshot — Take a full-page screenshot

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | yes | URL to screenshot |
| `viewport` | string | no | Viewport size (default: "1280x720") |

```json
{"action": "homepage", "operation": "screenshot", "url": "http://localhost:3000", "viewport": "1920x1080"}
```

## Deployment

### deploy — Build and upload to remote server via SFTP
Builds the project, then uploads the build output to the configured remote server.
```json
{"action": "homepage", "operation": "deploy", "project_dir": "my-site"}
```

### test_connection — Test SFTP/SCP connection
Tests connectivity to the configured deployment target.
```json
{"action": "homepage", "operation": "test_connection"}
```

## Local Web Server (Caddy)

AuraGo uses two different concepts here:
- **Dev workspace**: where homepage project files are edited inside the homepage tool workflow
- **Published local site**: served by the separate Caddy container `aurago-homepage-web`

For local publishing, the Caddy container serves files from:
- container name: `aurago-homepage-web`
- document root: `/srv`
- config path: `/etc/caddy/Caddyfile`

### Shared Docker Network

Both the dev container (`aurago-homepage`) and the Caddy web container (`aurago-homepage-web`) are connected to a shared Docker network (`aurago-homepage-net`). This enables:

1. **Static file serving** — Caddy serves build output from `/srv` (default)
2. **Reverse proxy to dev servers** — When a dev server is started via the `dev` operation, Caddy automatically proxies `/<project_dir>/*` to the dev container. This makes Next.js, Vite, and other framework dev servers accessible through the same Caddy URL without needing a separate port.

Proxy routes are persisted in the workspace as `.aurago-proxy-routes.json` and are automatically included when Caddy starts or reloads.

### Caddy Config with Proxy Routes

The generated Caddyfile uses `handle` blocks for proxy routes (matched first) and a default `handle` for static file serving:

```
:80 {
    root * /srv
    encode gzip

    handle /phaser-demo* {
        reverse_proxy aurago-homepage:3000
    }

    handle {
        file_server
        try_files {path} /index.html
    }
}
```

Do **not** assume `/var/www/html`. Use `publish_local` or `webserver_start` instead of manual `docker cp` to guessed paths.

### webserver_start — Start local Caddy web server
Serves the build output via Caddy (Docker required). Always pass `project_dir` when you know which project should be public at `/`. If omitted, AuraGo restores the last published project or auto-detects a single servable project, including plain HTML projects with `index.html` at the project root.
```json
{"action": "homepage", "operation": "webserver_start", "project_dir": "my-site"}
```

**Response:**
- `url`: The URL where the site is accessible
- `served_url`: Canonical served URL
- `mode`: "docker"
- `container_name`: `aurago-homepage-web`
- `document_root`: `/srv`
- `source_path`: Host-side build or project directory mounted into Caddy
- `project_dir` / `build_dir`: The resolved workspace source mounted as `/srv`; for plain HTML projects, `build_dir` is `.`

### webserver_stop — Stop the web server
```json
{"action": "homepage", "operation": "webserver_stop"}
```

### webserver_status — Check web server status
```json
{"action": "homepage", "operation": "webserver_status"}
```

### publish_local — Build and serve locally
Combines `build` + `webserver_start` in one step.
For plain HTML projects (no `package.json`), the build step is automatically skipped and the project directory is served directly.
Referenced assets (`/files/generated_images/*`, `/files/audio/*`, `/files/documents/*`) are automatically copied into the build directory so the Caddy container can serve them.
```json
{"action": "homepage", "operation": "publish_local", "project_dir": "my-site"}
```

## Recommended Workflow

1. **Initialize:** `init` → creates the Docker dev environment
2. **Scaffold:** `init_project` → creates a new project with chosen framework (optionally with a template)
3. **Version Control:** `git_init` → initialize git repository for the project
4. **Develop:** Use `write_file` to create/edit files, `install_deps` for packages
5. **Commit:** `git_commit` → save progress with a meaningful message
6. **Preview:** `dev` to start the dev server, `tunnel` to share externally, `screenshot` to capture preview
7. **Test:** `lighthouse` for performance audit, `lint` for code quality, `check_js` for runtime JS errors
8. **Optimize:** `optimize_images` for SVG optimization
9. **Build:** `build` (with `auto_fix: true` for automatic error recovery)
10. **Deploy:** Choose based on target:
    - `publish_local` — serve locally with Caddy (DEFAULT for most cases, no config flags needed)
    - `deploy` — upload to remote server (requires `homepage.allow_deploy=true`)
    - `deploy_netlify` — deploy to Netlify (requires `netlify.allow_deploy=true`)
    - `deploy_vercel` — deploy to Vercel (requires `vercel.allow_deploy=true`)

## Version Control (Git)

Git operations run inside the Docker container. Initialize a repository to track changes, rollback mistakes, and maintain project history.

### git_init — Initialize a git repository
Creates a repository with an initial commit.
```json
{"action": "homepage", "operation": "git_init", "project_dir": "my-site"}
```

### git_commit — Commit all changes
Stages all modifications and commits.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `git_message` | string | no | Commit message (default: "Update") |
| `project_dir` | string | no | Project subdirectory |

```json
{"action": "homepage", "operation": "git_commit", "project_dir": "my-site", "git_message": "Add hero section and navigation"}
```

### git_status — View changed files
```json
{"action": "homepage", "operation": "git_status", "project_dir": "my-site"}
```

### git_diff — View current changes
```json
{"action": "homepage", "operation": "git_diff", "project_dir": "my-site"}
```

### git_log — View commit history

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `count` | integer | no | Number of commits to show (default: 10, max: 100) |

```json
{"action": "homepage", "operation": "git_log", "project_dir": "my-site", "count": 5}
```

### git_rollback — Revert recent commits
Creates new revert commits (safe — never rewrites history).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `count` | integer | no | Number of commits to revert (default: 1, max: 10) |

```json
{"action": "homepage", "operation": "git_rollback", "project_dir": "my-site", "count": 1}
```

## Build with Auto-Fix

The `build` operation supports automatic error recovery when `auto_fix` is set to `true`.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `auto_fix` | boolean | no | If true, attempt to fix common build errors and retry once |

```json
{"action": "homepage", "operation": "build", "project_dir": "my-site", "auto_fix": true}
```

**Auto-fixable errors:**
- Missing npm modules → auto-installs the package
- Webpack module resolution failures → auto-installs the module
- ESLint fixable errors → runs `eslint --fix`
- Missing dependencies / node_modules → runs `npm install`

The response includes `auto_fix_applied: true` and `auto_fix_pattern` when a fix was applied.

## Project Templates

When using `init_project`, you can specify a `template` to get pre-built CSS layouts and a README:

| Template | Description |
|----------|-------------|
| `portfolio` | Dark-theme portfolio with hero, project cards, skills grid |
| `blog` | Clean serif blog with article list, tags, metadata |
| `landing` | Marketing landing page with hero, features, testimonials, CTA |
| `dashboard` | Admin dashboard with sidebar, stats cards, data table |

```json
{"action": "homepage", "operation": "init_project", "framework": "next", "name": "my-portfolio", "template": "portfolio"}
```

Templates write CSS files into `src/styles/` and a `TEMPLATE_README.md`. Import the CSS in your layout and use the documented class names.

## Sharing & Tunnels

### tunnel — Create a public URL for sharing
Starts a Cloudflare quick tunnel to expose a local port to the internet via a temporary `*.trycloudflare.com` URL.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `port` | integer | no | Local port to expose (default: 3000) |

```json
{"action": "homepage", "operation": "tunnel", "port": 3000}
```

**Notes:**
- Requires `cloudflared` in the Docker container (included since container rebuild)
- The URL is temporary and changes each time
- Also returns a `lan_url` for accessing from other devices on the same network
- The `webserver_start` response now automatically includes a `lan_url` when binding to all interfaces

## Important Notes

- All file paths are relative to `/workspace` inside the container
- `project_dir` must always be relative to the homepage workspace. Use `ki-news`, not `/workspace/ki-news`.
- `homepage.workspace_path` in the config UI is only the host mount path. Tool arguments still stay relative, e.g. `project_dir: "my-site"` and `path: "my-site/src/app/page.tsx"`.
- The container persists between sessions (uses `unless-stopped` restart policy)
- Build output directory is auto-detected: checks `out`, `dist`, `build`, `.next`, `public`. If none exist (plain HTML), serves the project root directly.
- Plain HTML projects (no `package.json`) skip the build step entirely — no Docker dev container needed for deployment.
- For deployment, store credentials in the vault: `homepage_deploy_password` or `homepage_deploy_key`
- The Caddy web server can serve with automatic HTTPS if a domain is configured (Docker mode only)
- Use compound operations (`init_project`, `build`, `publish_local`, `deploy_netlify`, `deploy_vercel`) to save tokens and keep the pipeline recoverable — avoid running many individual `exec` calls
- Use the homepage workspace operations from the Required Task Rule for project files; do not fall back to generic workspace tools.
- **NEVER use generic `execute_shell` for `/workspace/...` commands.** `/workspace` exists inside the homepage container; use `homepage` → `exec`, `list_files`, `read_file`, `write_file`, or `build`.
- **Do not directly edit generated output** (`dist`, `build`, `out`) with shell redirection or copy commands. These directories are deployment artifacts; change source files and rebuild.

### Using Generated Images in Netlify Deployments

**IMPORTANT:** Images generated with `generate_image` or found through `media_registry` are AuraGo-local assets first. For homepage source code, prefer copying the image into the project's deployable static directory and referencing that project asset.

For Vite, React, and plain HTML projects, put local images under `public/assets/...` or the framework-equivalent static directory, then reference them with a project-relative web URL:
```html
<img src="/assets/banner.jpeg" alt="Banner">
```

If an existing page already references generated AuraGo assets such as `/files/generated_images/<filename>` or legacy `/img_<id>.jpeg` paths, `deploy_netlify` tries to bundle those files into the ZIP. Do not rely on that as the primary workflow for new edits: make the image a deployable project asset in `public/assets` and update the markup to `/assets/<filename>`.

Never use `file://`, host filesystem paths, `/workspace/...`, `data/homepage/...`, or `agent_workspace/...` in page markup. Those paths are not fetchable from a deployed browser.

The `deploy_netlify` operation installs missing dependencies, builds, validates that the selected output contains `index.html`, scans HTML/CSS/JS for generated asset references, uploads a ZIP, waits for Netlify to report the deploy as ready, and verifies the public URL.

If a framework build fails but a finished static sibling directory exists, such as `my-site-static/index.html`, `deploy_netlify` may fall back to that static project and returns `fallback_project_dir` in the result. For an explicitly static export, you can also call `deploy_netlify` with `project_dir: "my-site-static"` and `build_dir: "."`.

### Using Vercel Deployments

Use `deploy_vercel` when the homepage project should be published to Vercel from AuraGo's managed workspace.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project_dir` | string | yes | Homepage workspace directory to deploy |
| `project_id` | string | no | Vercel project name or ID to link before deploying; falls back to `vercel.default_project_id` |
| `build_dir` | string | no | Explicit directory to upload; otherwise auto-detected after build |
| `target` | string | no | `preview` or `production` (default: `preview`) |
| `alias` | string | no | Alias or domain to assign after a successful deployment |
| `domain` | string | no | Custom domain to add/verify before alias assignment |

```json
{"action": "homepage", "operation": "deploy_vercel", "project_dir": "my-site", "project_id": "my-site", "target": "preview"}
```

```json
{"action": "homepage", "operation": "deploy_vercel", "project_dir": "my-site", "project_id": "my-site", "target": "production", "domain": "www.example.com"}
```

Notes:
- `deploy_vercel` is designed for homepage workspace projects including HTML, Vite/React, Astro, and Next.js.
- Projects with `package.json` deploy Vercel-native from the project root after a local build check, even when `build_dir` is provided. Explicit static `build_dir` values are only for plain static projects without package metadata and must contain a valid `index.html`.
- Do not patch Next.js into static export for Vercel. Let Vercel build and route the framework project from source.
- AuraGo validates the build first, links the Vercel project when a project reference is available, then deploys from the homepage workspace via the Vercel CLI.
- If the Vercel project does not exist yet, AuraGo can create it automatically only when `vercel.allow_project_management=true`.
- Alias or custom domain assignment requires `vercel.allow_domain_management=true`.

## Troubleshooting

### "Docker not available" Error

**Problem:** Docker daemon is not running or not accessible.

**Solutions:**
1. **Start Docker:** `sudo systemctl start docker` (recommended)
2. **Enable Docker:** `sudo systemctl enable docker`

### Web Server Not Accessible

**Problem:** `publish_local` or `webserver_start` reports success but site not reachable.

**Check:**
1. Verify port is not in use: `netstat -tlnp | grep 8080`
2. Check firewall rules: `sudo ufw allow 8080`
3. Try different port in config: `homepage.webserver_port: 8081`

### Dev Server Returns 404 via Caddy

**Problem:** The dev server runs inside the container but Caddy returns 404 for `/<project_dir>/` paths.

**Cause:** The Caddy and dev containers must be on the same Docker network (`aurago-homepage-net`) for Caddy to reach the dev server by container name.

**Solutions:**
1. Restart the dev container (`homepage stop` + `homepage start`) — it auto-connects to the shared network
2. Restart the Caddy web server (`webserver_stop` + `webserver_start`) — it also auto-connects
3. Verify both containers are on the network: `docker network inspect aurago-homepage-net`

### Status Shows "running": false

**Problem:** Container appears stopped.

**Solutions:**
1. Check Docker status: `docker ps -a`
2. Start manually: `homepage start`
3. If the user explicitly approves a destructive reset: `homepage destroy` with `force: true`, then `homepage init`

### init_project fails or install_deps can't find directory

**Problem:** `init_project` appears to succeed but `install_deps` or `build` reports directory not found. This typically means the Node.js version in the container is too old for the requested framework (npm `EBADENGINE` warnings).

**Solutions:**
1. Use `framework: "html"` for plain HTML projects (no Node.js required)
2. Rebuild the homepage container to get a newer Node.js: `homepage rebuild`
3. Create files manually with `homepage write_file`

### deploy_netlify: "Deploy path does not exist"

**Problem:** Files were created outside the homepage workspace, commonly through a generic file tool, placing them in `agent_workspace/workdir/` instead of the homepage workspace.

**Solution:** Use `homepage write_file` to create all project files. The `deploy_netlify` operation only finds files in the homepage workspace (`data/homepage/`).

### deploy_netlify: "Missing script: build"

**Problem:** `package.json` exists, but it does not define a `build` script.

**Solution:** Do not keep retrying the same deploy call. Either:
1. Treat the project as static HTML and deploy the project root directly, or
2. Add a valid `build` script to `package.json`, then retry.

If the project is plain HTML, you usually do not need a build step at all.

### deploy_vercel: "project was not found"

**Problem:** The requested Vercel project does not exist and automatic project creation is blocked.

**Solution:** Either:
1. Set `vercel.default_project_id` or pass `project_id` explicitly, or
2. Enable `vercel.allow_project_management` so AuraGo may create the Vercel project automatically.

### deploy_vercel: alias or domain assignment failed

**Problem:** The deployment succeeded, but the requested alias or custom domain could not be attached.

**Solution:** Check:
1. `vercel.allow_domain_management=true`
2. The domain is already configured for the same Vercel team/account
3. DNS verification is complete if you are using a custom domain

## Configuration

```yaml
homepage:
  enabled: true
  allow_local_server: false  # Enable Python fallback when Docker unavailable
  workspace_path: "data/homepage"  # Path for homepage projects
  # For deployment:
  # SFTP/SCP credentials stored in vault:
  # - homepage_deploy_host: deployment server hostname
  # - homepage_deploy_user: deployment username
  # - homepage_deploy_password OR homepage_deploy_key: authentication
```

## Notes

- **File paths**: All file paths are relative to `/workspace` inside the container
- **project_dir**: Must always be relative to the homepage workspace (e.g., `my-site`, not `/workspace/my-site`)
- Use homepage workspace operations for homepage project files; generic workspace tools use a different location
- **Container persistence**: The container persists between sessions (uses `unless-stopped` restart policy)
- **Build output**: Auto-detected: `out`, `dist`, `build`, `.next`, `public`
- **Plain HTML projects**: Skip the build step entirely — no Docker dev container needed for deployment
- **Auto-fix**: The `build` operation can automatically fix common errors when `auto_fix: true`

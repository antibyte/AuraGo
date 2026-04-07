# Homepage (`homepage`)

Design, develop, build, test and deploy professional websites using AuraGo's web workspace with Docker-based full mode and limited local fallback support.

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
| `destroy` | Remove everything |
| `init_project` | Create a new web project |
| `exec` | Run a shell command in the container |
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
| `git_init` | Initialize a git repository |
| `git_commit` | Commit all changes |
| `git_status` | View changed files |
| `git_diff` | View current changes |
| `git_log` | View commit history |
| `git_rollback` | Revert recent commits |
| `tunnel` | Create a public URL for sharing |

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
```json
{"action": "homepage", "operation": "destroy"}
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

### exec — Run a shell command in the container
```json
{"action": "homepage", "operation": "exec", "command": "cd /workspace/my-site && npm run dev"}
```

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

Do **not** assume `/var/www/html`. Use `publish_local` or `webserver_start` instead of manual `docker cp` to guessed paths.

### webserver_start — Start local Caddy web server
Serves the build output via Caddy (Docker required).
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
- Use compound operations (`init_project`, `build`, `deploy`) to save tokens — avoid running many individual `exec` calls
- **NEVER use the `filesystem` tool for homepage project files.** The filesystem tool writes to `agent_workspace/workdir/` — a completely different location from the homepage workspace. Files created there will NOT be found by `build`, `deploy`, `deploy_netlify`, or `publish_local`. Always use `homepage` → `write_file` instead.

### Using Generated Images in Netlify Deployments

**IMPORTANT:** Images generated with `generate_image` are served by AuraGo at `/files/generated_images/<filename>`. When deploying to Netlify, these images are **automatically bundled** into the ZIP — you do NOT need to manually copy them.

Simply embed the image in your HTML using the exact URL path from `media_registry`:
```html
<img src="/files/generated_images/img_20260316_114059_254b5b21fe9f.jpeg" alt="Banner">
```

The `deploy_netlify` operation scans all HTML and CSS files, detects `/files/generated_images/` references, and includes those image files in the deployment package automatically. After deploying, the image will be live at the same URL path on Netlify.

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

### Status Shows "running": false

**Problem:** Container appears stopped.

**Solutions:**
1. Check Docker status: `docker ps -a`
2. Start manually: `homepage start`
3. If stuck: `homepage destroy` then `homepage init`

### init_project fails or install_deps can't find directory

**Problem:** `init_project` appears to succeed but `install_deps` or `build` reports directory not found. This typically means the Node.js version in the container is too old for the requested framework (npm `EBADENGINE` warnings).

**Solutions:**
1. Use `framework: "html"` for plain HTML projects (no Node.js required)
2. Rebuild the homepage container to get a newer Node.js: `homepage rebuild`
3. Create files manually with `homepage write_file` — do NOT use the `filesystem` tool

### deploy_netlify: "Deploy path does not exist"

**Problem:** Files were created with the `filesystem` tool instead of `homepage write_file`, placing them in `agent_workspace/workdir/` instead of the homepage workspace.

**Solution:** Use `homepage write_file` to create all project files. The `deploy_netlify` operation only finds files in the homepage workspace (`data/homepage/`).

### deploy_netlify: "Missing script: build"

**Problem:** `package.json` exists, but it does not define a `build` script.

**Solution:** Do not keep retrying the same deploy call. Either:
1. Treat the project as static HTML and deploy the project root directly, or
2. Add a valid `build` script to `package.json`, then retry.

If the project is plain HTML, you usually do not need a build step at all.

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
- **NEVER use `filesystem` tool** for homepage project files — it writes to `agent_workspace/workdir/` instead of the homepage workspace
- **Container persistence**: The container persists between sessions (uses `unless-stopped` restart policy)
- **Build output**: Auto-detected: `out`, `dist`, `build`, `.next`, `public`
- **Plain HTML projects**: Skip the build step entirely — no Docker dev container needed for deployment
- **Auto-fix**: The `build` operation can automatically fix common errors when `auto_fix: true`

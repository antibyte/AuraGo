# Homepage — Web Development & Deployment Tool

Design, develop, build, test and deploy professional websites using a Docker-based dev environment with Node.js, Playwright, Lighthouse, SVGO and more.

## Prerequisites

- **Required**: Docker integration enabled (`docker.enabled: true`)
- Homepage tool must be enabled (`homepage.enabled: true`)
- For deployment: SFTP/SCP credentials must be stored in the vault

## Docker Mode

Full dev environment with Node.js, Playwright, Lighthouse and Caddy web server with automatic HTTPS.
Supports all frameworks: Next.js, Vite, Astro, SvelteKit, Vue, static HTML.

## Container Lifecycle

### init — Initialize the dev environment
Creates the Docker image and container. **Run this first** before any other operation.
```json
{"action": "homepage", "operation": "init"}
```

**Note:** Docker must be running. If Docker is unavailable, start it with `sudo systemctl start docker`.)

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

### lint — Run ESLint
```json
{"action": "homepage", "operation": "lint", "project_dir": "my-site"}
```

### optimize_images — Optimize SVGs with SVGO
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

### webserver_start — Start local Caddy web server
Serves the build output via Caddy (Docker required).
```json
{"action": "homepage", "operation": "webserver_start", "project_dir": "my-site"}
```

**Response:**
- `url`: The URL where the site is accessible
- `mode`: "docker"

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
```json
{"action": "homepage", "operation": "publish_local", "project_dir": "my-site"}
```

## Recommended Workflow

1. **Initialize:** `init` → creates the Docker dev environment
2. **Scaffold:** `init_project` → creates a new project with chosen framework
3. **Develop:** Use `write_file` to create/edit files, `install_deps` for packages
4. **Preview:** `dev` to start the dev server, `screenshot` to capture preview
5. **Test:** `lighthouse` for performance audit, `lint` for code quality
6. **Optimize:** `optimize_images` for SVG optimization
7. **Build:** `build` to create production output
8. **Deploy:** Either `deploy` (upload to remote server) or `publish_local` (serve locally with Caddy)

## Important Notes

- All file paths are relative to `/workspace` inside the container
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

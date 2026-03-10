# Homepage — Web Development & Deployment Tool

Design, develop, build, test and deploy professional websites using a Docker-based development environment. The container includes Node.js, Playwright, Lighthouse, SVGO and popular web frameworks.

## Prerequisites

- Docker integration must be enabled (`docker.enabled: true`)
- Homepage tool must be enabled (`homepage.enabled: true`)
- For deployment: SFTP/SCP credentials must be stored in the vault
- For local web server: Caddy container image will be auto-pulled

## Container Lifecycle

### init — Initialize the dev environment
Creates the Docker image and container. **Run this first** before any other operation.
```json
{"action": "homepage", "operation": "init"}
```

### start — Start the dev container
```json
{"action": "homepage", "operation": "start"}
```

### stop — Stop the dev container
```json
{"action": "homepage", "operation": "stop"}
```

### status — Get container status
Returns status of both dev container and web server container.
```json
{"action": "homepage", "operation": "status"}
```

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
Serves the build output on the configured port. If a domain is configured, Caddy provides automatic HTTPS via Let's Encrypt.
```json
{"action": "homepage", "operation": "webserver_start", "project_dir": "my-site"}
```

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
- Build output directory is auto-detected: checks `out`, `dist`, `build`, `.next`, `public`
- For deployment, store credentials in the vault: `homepage_deploy_password` or `homepage_deploy_key`
- The Caddy web server can serve with automatic HTTPS if a domain is configured
- Use compound operations (`init_project`, `build`, `deploy`) to save tokens — avoid running many individual `exec` calls

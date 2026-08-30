---
id: docker
title: Docker Workflow
enabled: true
priority: 92
tools: [docker]
workflows: [docker, container, compose, deployment, dockerfile]
keywords:
  - docker
  - container
  - containers
  - compose
  - docker-compose
  - dockerfile
  - image
  - deployment
  - port mapping
  - volume
  - registry
---

This rule applies whenever creating, modifying, inspecting, or managing Docker containers, images, networks, volumes, or Docker Compose stacks.

## Docker Workflow

Treat Docker as a production infrastructure layer, not a local convenience. Every container creation or modification must follow security, observability, and reproducibility principles.

### Security-First Defaults

When creating or running containers, apply secure defaults unless the user explicitly overrides them:

1. **Non-root execution.** AuraGo's `docker` tool has no `user`/`cap_drop`/`security_opt` parameters for `create`/`run` today, so per-container hardening cannot be requested through this tool. Prefer images that already run as non-root by default (a `USER` directive baked into the image); if the image only supports root, say so plainly instead of claiming the container was hardened.
2. **Capability dropping.** For the same reason, capability dropping and `no-new-privileges` are not configurable per-container through this tool. Do not tell the user a container was hardened with `CapDrop`/`SecurityOpt` unless that hardening is already baked into the image or applied outside AuraGo; treat this as a known tool limitation, not something to silently skip while implying it happened.
3. **Port exposure minimization.** Only bind required ports with the `ports` parameter. Never expose the Docker daemon socket (`/var/run/docker.sock`) into a container unless absolutely necessary and the user explicitly requests it; the tool already blocks binding known system-critical host paths.
4. **Image provenance.** Prefer official or verified images. Avoid mutable tags like `:latest` for production workloads; use explicit version tags (e.g., `nginx:1.25.3`).

### Volume and Bind Mount Safety

- Validate host paths before mounting. Do not mount system-critical directories (`/`, `/etc`, `/var`, `/boot`, `/proc`, `/sys`) into containers.
- Use named volumes for persistent data instead of bind mounts when possible.
- Ensure bind mount sources exist on the host before container creation.

### Network and Restart Policy

- Use explicit Docker networks for multi-container applications instead of the default `bridge` network where possible.
- Set restart policies intentionally: `unless-stopped` for services, `no` for one-off tasks, `on-failure` for batch jobs.
- Avoid `always` unless the user explicitly requests it, because it can mask startup failures.

### Image Lifecycle

- Before pulling an image, check if it already exists locally to avoid unnecessary network operations.
- When building images, use multi-stage builds to minimize final image size.
- The `docker` tool has no bulk prune operation. Clean up dangling images and stopped containers individually with `remove_image`/`remove` after reviewing `list_images`/`list_containers` (`all: true` to include stopped containers), and never remove volumes or running containers without explicit user confirmation.

### Compose Stacks

- For multi-container workloads, prefer a `docker-compose.yml` (or compose-compatible YAML) over imperative `docker run` commands.
- Store compose files in version control when they represent persistent infrastructure.
- Use environment variable substitution (`${VAR}`) in compose files for secrets and environment-specific values, but never commit actual secrets.

### Operational Discipline

- **Inspect before mutate.** Call `inspect` or `list_containers` before stopping, removing, or modifying a container.
- **Logs for diagnostics.** Use `logs` with a reasonable `tail` (default 100, increase to 500 for troubleshooting) before escalating to shell exec.
- **Health awareness.** Check container health status in `list_containers` output when available.
- **No blind prune.** There is no `system_prune` operation exposed to the agent; do not tell the user one was run. Remove unused images/containers one at a time with `remove_image`/`remove`, and confirm with the user before removing anything that might hold state.

### Secrets and Configuration

- Never pass secrets via environment variables in `env` if they can be avoided. Prefer Docker secrets (swarm mode) or mounted secret files.
- If secrets must be passed as env vars, register them with the security vault and reference them indirectly.

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

1. **Non-root execution.** Prefer running containers as non-root (`User`). If the image runs as root by default and no user is specified, document this choice.
2. **Capability dropping.** Default to dropping all capabilities and adding back only what is required (`CapDrop: ["ALL"]`). 
3. **Read-only root filesystem.** Use `SecurityOpt: ["no-new-privileges:true"]` where supported.
4. **Port exposure minimization.** Only bind required ports. Never expose the Docker daemon socket (`/var/run/docker.sock`) into a container unless absolutely necessary and the user explicitly requests it.
5. **Image provenance.** Prefer official or verified images. Avoid mutable tags like `:latest` for production workloads; use explicit version tags (e.g., `nginx:1.25.3`).

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
- Clean up dangling images and stopped containers periodically, but never auto-prune volumes or running containers without explicit user confirmation.

### Compose Stacks

- For multi-container workloads, prefer a `docker-compose.yml` (or compose-compatible YAML) over imperative `docker run` commands.
- Store compose files in version control when they represent persistent infrastructure.
- Use environment variable substitution (`${VAR}`) in compose files for secrets and environment-specific values, but never commit actual secrets.

### Operational Discipline

- **Inspect before mutate.** Call `inspect` or `list_containers` before stopping, removing, or modifying a container.
- **Logs for diagnostics.** Use `logs` with a reasonable `tail` (default 100, increase to 500 for troubleshooting) before escalating to shell exec.
- **Health awareness.** Check container health status in `list_containers` output when available.
- **No blind prune.** `system_prune` is destructive. Confirm with the user before running it, and never use `all=true` and `volumes=true` together without explicit approval.

### Secrets and Configuration

- Never pass secrets via environment variables in `env` if they can be avoided. Prefer Docker secrets (swarm mode) or mounted secret files.
- If secrets must be passed as env vars, register them with the security vault and reference them indirectly.

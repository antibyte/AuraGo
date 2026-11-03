# Auxiliary Docker build inputs

## Purpose

Versioned Dockerfiles and supporting inputs for AuraGo-managed containers and the main image's entrypoint.

## Ownership

This folder owns the Ansible, Browser Automation and browser-egress Dockerfiles, `docker-entrypoint.sh` and the other maintained Docker build inputs. Root `Dockerfile` and `docker-compose.yml` retain the conventional main-image entrypoints. Feature guides own deeper runtime and builder contracts.

## Local Contracts

- Ansible and browser Dockerfiles use the repository root as their build context; moving a Dockerfile does not change its COPY sources.
- Compose names these Dockerfiles explicitly. Native auto-builds also find them under `deploy/docker/`, retaining context-root Dockerfiles for existing custom build contexts.
- Root `Dockerfile` copies `docker-entrypoint.sh` to `/app/docker-entrypoint.sh`; `.dockerignore` must include that source despite the general shell-script exclusion.
- Release cleanup must preserve this entire source folder.

## Work Guidance

Update Compose, native build resolution, documentation and audit contracts together when an input moves. Keep main-image installed paths stable.

## Verification

`bash -n deploy/docker/docker-entrypoint.sh`; relevant `go test ./internal/audit` Docker/build/install contracts and `go test ./internal/tools` sidecar build tests. Use `docker compose config --quiet` when Docker is available.

## Child DOX Index

None. CYD Creator and Kaggle image contracts are routed through the root feature index.

# Auxiliary Docker build inputs

## Purpose

Versioned Dockerfiles and supporting inputs for AuraGo-managed containers and the main image's entrypoint.

## Ownership

This folder owns the Ansible, Browser Automation and browser-egress Dockerfiles, `docker-entrypoint.sh` and the other maintained Docker build inputs. Root `Dockerfile` and `docker-compose.yml` retain the conventional main-image entrypoints. Feature guides own deeper runtime and builder contracts.

## Local Contracts

- Ansible and browser Dockerfiles use the repository root as their build context; moving a Dockerfile does not change its COPY sources.
- Ansible leaves `DEFAULT_INVENTORY` empty unless explicitly configured; the sidecar then uses a private localhost snapshot. Only an explicit default may permit an inventory outside the playbook root. Runtime policy: `../../ansible_api/AGENTS.md`.
- Ansible images carry `io.aurago.ansible.security-policy=4`. Keep the label aligned with the Python implementation and Go admission before managed container replacement.
- Compose names these Dockerfiles explicitly. Native auto-builds also find them under `deploy/docker/`, retaining context-root Dockerfiles for existing custom build contexts.
- Root `Dockerfile` copies `docker-entrypoint.sh` to `/app/docker-entrypoint.sh`; `.dockerignore` must include that source despite the general shell-script exclusion.
- Release cleanup must preserve this entire source folder.
- The base Compose file starts without Docker API access. `compose.docker-access.yml`
  provides the opt-in socket proxy, private control network and AuraGo dependency.
  Keep proxy permissions, image pins and the operator migration guide in sync.
- Publish fresh container configs and fallback keys by same-directory replacement.
  Recover empty configs with container HTTPS port 8443, preserving explicit
  settings in existing deployments. Read external key files as the container UID
  and report access errors without printing the key.
- The entrypoint preserves /app/data/.env and queries the backend loader through `config-merger -data-dir <config>` before Vault setup. Check vault.bin in that resolved directory; a missing original key refuses startup, including nested/custom Egg data directories. The query performs no state initialization or writes.

## Work Guidance

Update Compose, native build resolution, documentation and audit contracts together when an input moves. Keep main-image installed paths stable.

## Verification

`bash -n deploy/docker/docker-entrypoint.sh`; relevant `go test ./internal/audit` Docker/build/install contracts and `go test ./internal/tools` sidecar build tests. Use `docker compose config --quiet` when Docker is available.

## Child DOX Index

- `kicad-export/AGENTS.md` owns the versioned KiCad worker and image smoke contracts.
- CYD Creator and Kaggle image contracts are routed through the root feature index.

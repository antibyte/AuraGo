# AuraGo Docker Installation Guide

AuraGo uses persistent Docker volumes for its data and master key. Host Docker access and a host-managed key file are optional.

## Hardware Requirements

| | Minimum | Recommended |
|---|---------|-------------|
| CPU | 2 cores | 4 cores |
| RAM | 4 GB | 8 GB |
| Disk | 10 GB | 20 GB+ |
| Platform | Linux amd64/arm64 | Linux amd64/arm64 |

> Docker images are built for **Linux only** (amd64 and arm64). macOS and Windows are not supported as Docker hosts — use the bare-metal installer instead.

---

## 1. Standard Docker Compose Installation

Follow these steps to deploy AuraGo on any Linux server with Docker Compose.

### Step 1: Create a directory
```bash
mkdir -p ~/aurago-docker
cd ~/aurago-docker
```

### Step 2: Download docker-compose.yml
```bash
curl -fLO https://raw.githubusercontent.com/antibyte/AuraGo/main/docker-compose.yml
```

### Step 3: Choose where to keep the master key

Start with the automatic key on a new installation. AuraGo stores it in
`/app/data/.env` in the persistent data volume. Keep a protected backup of that
file with the Vault and its data volume. Existing installations must keep
their original key.

For an optional host-managed key, place the original key (or a new 64-digit
hexadecimal key for a fresh install) in `secrets/aurago_master.key`. Compose
mounts `./secrets` read-only. The file must be readable by the container's UID
1001, including traversal of the mounted directory. For regular rootful Linux
without user-namespace remapping, grant ownership to UID 1001 and keep mode
`600`. For remapped or rootless engines use the mapped UID. The automatic volume
key works without host permission setup.

### Step 4: Optional host config override

If you want to pre-seed a custom config before the first start, create it as:

```bash
mkdir -p config
nano config/config.yaml
```

If `config/config.yaml` is missing, AuraGo simply uses the built-in template and writes the active config into the persistent Docker volume.

> This directory-based layout is intentional. Docker bind mounts create missing file paths as directories, which easily leads to broken first starts if a compose file mounts `./config.yaml` directly.

### Step 5: Start the Container
```bash
docker compose up -d
```

That's it!
- If `secrets/aurago_master.key` exists, AuraGo loads it as the master key.
- Otherwise AuraGo auto-generates a key and stores it in `/app/data/.env` inside the persistent volume.
- A default config is generated inside the container from the built-in template.
- The default stack starts without access to the host Docker daemon. See [Docker Socket Security](#4-docker-socket-security) to enable Docker management.
- The Web UI is now available at `https://<your-server-ip>:8443`.

Open the Web UI to finish setting up your LLM Provider and API keys!

Fresh configs use an automatically generated local certificate on **container
port 8443**. Verify the certificate and configure browser/device trust if
needed. Compose publishes **8443** by default. Set
`AURAGO_HTTPS_PORT=443` in the Compose `.env` file only when host port 443 is
free, then open `https://<your-server-ip>`. Otherwise open
`https://<your-server-ip>:8443`. Compose does not choose a different host port
automatically.

Existing or user-supplied configurations retain their transport and ports;
the `8088:8088` mapping supports existing HTTP installs. In fresh HTTPS mode,
8088 is only the container's internal loopback HTTP listener. If you supply a
custom HTTPS config, align its internal port with the Compose mapping (8443).
The Docker healthcheck uses AuraGo's CLI and supports both HTTPS with local
certificates and HTTP Egg instances on their configured port.

When changing transport or ports later, adjust Docker mappings as well. A
mapped public port is retained in the browser; AuraGo cannot infer arbitrary
host mappings. See [HTTPS configuration](https.md).


The container runs in UTC unless you set `TZ`. Schedules that follow the server
clock, such as the person detection arming schedule of the Network Cameras app,
use that zone. For local time, uncomment `- TZ=Europe/Berlin` (or your zone) in
the `aurago` service's `environment` block and run `docker compose up -d` again;
the image ships the time zone database.

Quick Connect can also use serial hardware attached to the AuraGo server. The
Docker service does not receive device access by default. To pass one through,
add a `devices` entry and the host device group's numeric GID to `group_add` in
your Compose override; see [Quick Connect serial access](quick-connect-serial.md).

> [!IMPORTANT]
> Until setup is complete, the setup wizard asks for a one-time **setup token**. This proves that you, and not someone scanning the network, own the new instance. AuraGo prints the token to its log on startup:
>
> ```bash
> docker logs aurago 2>&1 | grep bootstrap_token
> ```
>
> The log line also contains a path like `/setup#bootstrap=<token>`. Open `https://<your-server-ip>:8443` (or your configured published HTTPS port) followed by that path and the wizard fills in the token for you. Each restart creates a new token, and it stops working once setup is finished.

---

## 2. Image Tags

The AuraGo Docker image is published to GitHub Container Registry (`ghcr.io/antibyte/aurago`).

| Tag | Description | Use Case |
|-----|-------------|----------|
| `latest` | Always points to the latest stable release | Default for release installs |
| `v1.2.3` | Pinned to a specific release | Reproducible deployments |

> **Recommendation:** For production, pin to a specific version tag. For example:
> ```yaml
> image: ghcr.io/antibyte/aurago:v1.2.3
> ```

To deploy a specific build, choose a published release tag or pin its
registry digest in `image:`. The publishing workflow signs AuraGo images with
Cosign. A normal Compose pull checks image integrity against the registry
digest. For signature verification, verify the exact digest against the AuraGo
`docker-publish.yml` GitHub Actions identity and GitHub OIDC issuer before
deployment.

---

## 3. Installation via Dockge / Portainer

If you use a Docker management stack like **Dockge** or **Portainer**, deployment is just a copy-paste away.

### Step 1: Create the Stack
1. Open Dockge (or Portainer).
2. Create a new stack named `aurago`.
3. Paste the contents of the `docker-compose.yml` from the AuraGo repository into the editor.

### Step 2: Keep the persistent key

The stack generates a master key on first boot. Back up the persistent data
volume with its original key. To supply an external key, follow the
[host-file permission requirements](#step-3-choose-where-to-keep-the-master-key).

### Step 3: Optional pre-filled config

If you want to provide your own config before first start:

```bash
mkdir -p config
nano config/config.yaml
```

If you do nothing here, AuraGo will start from the built-in defaults and store the active config in its persistent data volume.

### Step 4: Deploy
Deploy the stack.
- Dockge/Portainer will pull the image and start three containers: `aurago`, `aurago_docker_proxy`, and `aurago_gotenberg`.
- On first start, the container automatically generates the config from the built-in template.
- Persistent volumes for `/app/data` and `/app/agent_workspace/workdir` are automatically created.

### Step 5: Configure via Web UI
Access the Web UI at `https://<your-server-ip>` and navigate to the **CONFIG** tab to finish setting up your AI agent. The setup wizard asks for the one-time setup token from the container log (entry `bootstrap_token`, visible in the container's log view in Dockge/Portainer).

> [!NOTE]
> Your `AURAGO_MASTER_KEY` is either stored in `secrets/aurago_master.key` on the host or auto-generated into `data/.env` inside the Docker volume. THIS KEY ENCRYPTS THE AGENT'S SECRET VAULT. BACK IT UP OR YOU WILL NOT BE ABLE TO MOVE THE VAULT TO ANOTHER SERVER!

---

## 4. Docker Socket Security

The base stack has no Docker socket, proxy, or `DOCKER_HOST`. To enable
Docker Manager, local Invasion deployments or AuraGo-managed containers:

```bash
curl -fLO https://raw.githubusercontent.com/antibyte/AuraGo/main/deploy/docker/compose.docker-access.yml
docker compose -f docker-compose.yml -f compose.docker-access.yml up -d
```

From a source checkout, use `-f deploy/docker/compose.docker-access.yml` for
the second file. Use the same file list and Compose project for later operations.

**Upgrading from the former default proxy:** Keep the same Compose project,
named volumes, and key. Save your current Compose file before replacing it.
To disable Docker access, stop its `docker-proxy` service while the old
definition is still available, then recreate AuraGo with the new base file.
To retain Docker integrations, start with both files and carry over any prior
proxy overrides to the extension.

| | Socket Proxy (default) | Direct Socket Mount |
|---|---|---|
| Host access | Filtered API subset; build, commit and swarm calls are blocked | Full Docker API |
| Risk if compromised | Host root is still reachable: container create with host binds is allowed | Complete host takeover |
| Container management | Create (including host binds), start, stop, remove, inspect, exec, image pull | Everything |

The proxy reduces attack surface but does not contain a compromised AuraGo: container create with host binds is allowed.

The opt-in proxy is attached to an internal `docker-control` network that is shared only with the AuraGo container. Keep the `docker-control` network private and do not attach unrelated sidecars to it. The enabled permissions map to these features:

Optional [managed RustFS storage](rustfs.md) uses the existing container/image/volume/network and information/version permissions with `POST=1`. It works with `BUILD=0` and `EXEC=0`; its S3 traffic stays on the shared application network, outside `docker-control`. Enable and operate it from **Configuration → S3**.

- `CONTAINERS=1` with `POST=1`: create, start, stop and remove managed sidecars, Software Store apps, Invasion Control eggs, the CYD Creator builder and the security proxy, and copy files into and out of containers (the CYD Creator builder exchanges build trees and artefacts this way).
- `IMAGES=1` with `POST=1`: pull and tag published images such as Code Studio, go2rtc, the local LLM runtime, Software Store apps, the CYD Creator toolchain and the security proxy.
- `VOLUMES=1` and `NETWORKS=1`: model and app volumes, the CYD Creator build cache and work volumes, private Software Store networks, the browser automation network check and the security proxy's lookup of the network it shares with AuraGo.
- `INFO=1` and `VERSION=1`: runtime inventory (including NVIDIA container runtime detection), Engine API version negotiation and the security proxy's Engine version check (Docker Engine 26 or newer for volume subpaths).
- `EXEC=1`: AuraGo runs commands inside containers through the Docker exec API. It is used by:
  - the agent's Docker `exec` operation;
  - the Homepage tool: scaffolding, installs, builds, file access, editors, Git, deploys and permission repair in the `aurago-homepage` dev container, and `caddy reload` in the `aurago-homepage-web` web container;
  - Code Studio: runtime probes at startup, commands and terminals;
  - the built-in OpenSCAD compiler container used by the OpenSCAD desktop app: runtime check, renders and exports, and diagnostics;
  - container terminals in the Web UI, including the CommandCode Store terminal;
  - the code sandbox (`execute_sandbox`), whose llm-sandbox Docker backend runs code with Docker exec;
  - the CYD Creator builder container: firmware builds, backtrace decoding and work directory cleanup;
  - the security proxy reload.
- `BUILD=0`: Docker build API access stays disabled. Code Studio and most managed sidecars pull published images. Images that AuraGo builds through the Engine API need `BUILD=1` or must already exist on the host:
  - the Homepage dev container image `aurago-homepage:latest`, which the Homepage tool builds on first use. Without `BUILD=1`, build it once on the Docker host: `docker exec aurago /app/aurago --print-homepage-dockerfile | docker build -t aurago-homepage:latest -` (native installs run the AuraGo binary with `--print-homepage-dockerfile`). `homepage rebuild` deletes this image before it builds, so run the command again afterwards;
  - the bundled CommandCode Store image, built only when its pull fails;
  - local Code Studio runtime images, used instead of the default published image;
  - the security proxy's rate-limit image `aurago-proxy:ratelimit-<version>` (Caddy with the `caddy-ratelimit` module), built once when `security_proxy.rate_limiting.enabled` is on; see the Security Proxy section of the integrations manual.

  The optional browser automation auto-build calls the Docker CLI, which the AuraGo image does not include, so in this setup it never reaches the build API. Build that image on the host if you need it (`docker compose --profile browser-automation build browser-automation`). The Ansible sidecar auto-build (`ansible.auto_build`) also calls the Docker CLI and fails the same way; build `aurago-ansible:latest` on the host from a source checkout (`docker build -f deploy/docker/Dockerfile.ansible -t aurago-ansible:latest .`). On native installs the browser automation, Ansible and Space Agent auto-builds run on the engine in `docker.host`, the engine that also runs the sidecar; when that engine refuses builds (a socket proxy with `BUILD=0`), AuraGo retries once on the Docker CLI's default engine and then needs the image on `docker.host`.

`EXEC=1` can be set to `0` only if you use none of these features: the agent's Docker `exec` operation, the Homepage tool, Code Studio, the built-in OpenSCAD compiler container, container terminals (including the CommandCode Store terminal), the code sandbox, CYD Creator builds and the security proxy reload stop working without it.

The `:ro` flag on the socket mount (`/var/run/docker.sock:/var/run/docker.sock:ro`) is no security gain: it does not stop API calls through the socket. Access control comes only from the proxy's filters and the private `docker-control` network.

The proxy image is pinned to a release tag plus its multi-arch index digest (`tecnativa/docker-socket-proxy:<tag>@sha256:<digest>`, the version pinned in `compose.docker-access.yml`); tag and digest always change together. The first `docker compose up -d` after an AuraGo upgrade that changes this image string recreates the `aurago_docker_proxy` container once; that is expected.

To refresh the pin (maintainers):

1. Pick the release tag (`vX.Y.Z`; never the moving `latest`, `master` or `nightly` tags) and read its index digest with `docker buildx imagetools inspect tecnativa/docker-socket-proxy:vX.Y.Z` (the `Digest:` line; no running Docker daemon is needed).
2. Put `tecnativa/docker-socket-proxy:vX.Y.Z@sha256:<digest>` into the `docker-proxy` service in `compose.docker-access.yml`, then validate the two-file Compose configuration with `config --quiet`.

If the pinned digest ever disappears from Docker Hub, new installs fail to pull it with `manifest unknown`, while existing installs keep running from their cached image (only `docker compose pull` fails) until the pin is refreshed.

The AuraGo container receives `DOCKER_HOST=tcp://docker-proxy:2375` for its Docker integrations. With `docker.readonly: true`, AuraGo withholds Docker client variables and the Unix socket grant from general host shell commands and local Ansible probes. Use the Docker integration for read operations.

Host processes can also have direct access through their OS permissions, a mounted
socket or the `docker-control` network. That access follows the OS or proxy policy.
Use the isolated executor for shell commands that must run without those permissions.
Trusted setup and installed compatibility skills keep their own Docker access paths.

---

## 5. Upgrading

### Standard Upgrade

```bash
docker compose pull
docker compose up -d
```

Before upgrading, record the current image digest and make a consistent backup
of the stopped data volume and original key. Use the same Compose files and
project name as your current deployment. Check container health after starting
the new image. To recover from a failed migration, stop the new image and
restore the matching pre-upgrade volume and image digest. Your data is preserved in Docker volumes:
- `aurago_data` — config, memory, chat history, vault, vector DB
- `aurago_workdir` — Python venv and generated tools

Keep `aurago_data` on a local Docker volume or local disk bind mount. AuraGo stores SQLite databases there, and SQLite locking is not reliable on many NFS, CIFS, SMB, or other network filesystem mounts.

### Python Venv Considerations

The `aurago_workdir` volume preserves the Python virtual environment across restarts. This is intentional for performance, but requires attention during upgrades:

**When upgrading Python versions** (e.g., from Python 3.11 to 3.12):
```bash
# Stop container
docker compose down

# Remove the old venv (it will be recreated on next start)
project=aurago-docker  # Use your actual Compose project name.
docker volume rm "${project}_aurago_workdir"

# Restart with new image
docker compose up -d
```

**If you encounter pip package issues** after an upgrade:
```bash
# Enter the container
docker compose exec aurago bash

# Remove and recreate venv
rm -rf /app/agent_workspace/workdir/venv
exit

# Restart container to trigger venv recreation
docker compose restart aurago
```

### Clean Reinstall (Preserving Data)

```bash
# Stop everything
docker compose down

# Volumes are preserved by default. To verify:
docker volume ls | grep aurago

# Pull and restart
docker compose pull
docker compose up -d
```

### Nuclear Option (DESTROYS ALL DATA)

```bash
docker compose down -v   # -v removes volumes
docker compose up -d     # start fresh
```

---

## 6. Migrating an Existing Installation to a Host-Managed Secret File

Retain a protected backup of the original master key and Vault. Copy the same
key from the volume's `/app/data/.env` to `secrets/aurago_master.key` without
printing it to the terminal. Use the host-file ownership and permissions from
[step 3](#step-3-choose-where-to-keep-the-master-key). Recreate AuraGo with the
existing Compose project and verify it can open the original Vault. Keep the
original key backup.

---

## 7. Troubleshooting

### Container exits immediately
Check the logs:
```bash
docker compose logs aurago
```
Common causes: corrupt `config.yaml`, missing master key permissions.

### config.yaml is a directory (not a file)
Docker auto-creates a directory when the host file does not exist at mount time. Fix:
```bash
docker compose down
rmdir config.yaml          # remove the auto-created directory
touch config.yaml          # create an empty file
docker compose up -d
```

### Healthcheck fails during first start
The healthcheck has a 5-minute startup window (`start-period: 300s`) to allow for VectorDB initialization. On very slow systems this may not be enough. You can override it in `docker-compose.yml`:
```yaml
healthcheck:
  start_period: 420s
```

---

## 6. Logging Configuration

### Default Logging

By default, AuraGo uses Docker's `json-file` logging driver with rotation:
- Maximum file size: 10 MB
- Maximum files: 3
- Total maximum log size: ~30 MB per container

### Alternative: Journald Logging (Linux)

For better integration with system logging on Linux:

```yaml
    logging:
      driver: journald
      options:
        tag: "aurago"
```

### Alternative: Syslog Logging

For centralized log management:

```yaml
    logging:
      driver: syslog
      options:
        syslog-address: "udp://localhost:514"
        tag: "aurago"
```

### Viewing Logs

```bash
# Real-time logs
docker compose logs -f aurago

# Last 100 lines
docker compose logs --tail=100 aurago

# Logs from specific time
docker compose logs --since="2024-01-01T00:00:00" aurago

# Save logs to file
docker compose logs aurago > aurago_logs.txt
```

---

## 7. Network Configuration

### Internal Network Isolation (Advanced)

For enhanced security, you can isolate sidecars on an internal network:

```yaml
networks:
  aurago_internal:
    driver: bridge
    internal: true
  aurago_public:
    driver: bridge

services:
  aurago:
    networks:
      - aurago_public
      - aurago_internal
  
  gotenberg:
    networks:
      - aurago_internal
    # Remove ports section - only accessible from aurago
```

This prevents external access to Gotenberg while allowing AuraGo to communicate with it.
The repository's default Compose bridge permits egress and shares API access with
its other services; its loopback host binding does not isolate those services.
Keep Gotenberg's unauthenticated API and `/debug` accessible only to AuraGo and
trusted operators. Preserve the pinned image, resource limits and all four
private-IP filters in `docker-compose.yml`. AuraGo verifies the running flags
before every upload; `/health` alone is insufficient. After updating the Compose
profile, recreate only that service with `docker compose up -d gotenberg` during
an operator maintenance window. Native auto-migration never adopts Compose
containers. See [Document Creator](document-creator.md) for external instances,
offline rendering and migration recovery.

### Custom Port Configuration

To change the default port (8088):

1. Edit `docker-compose.yml`:
```yaml
    ports:
      - "9090:8088"  # Host port 9090, container port stays 8088
```

2. Update environment variable if needed:
```yaml
    environment:
      - AURAGO_SERVER_HOST=0.0.0.0
```

The container's internal port (8088) should not be changed.

---

## 8. Troubleshooting

### Container Won't Start

**Check logs:**
```bash
docker compose logs aurago
```

**Common issues:**

1. **Master key unavailable**: Check the entrypoint logs for the exact path.
   For the optional `secrets/aurago_master.key`, verify that UID 1001 (or its
   mapped host UID) can read the private file. Restore the original key for an
   existing Vault.
2. **Port already in use**: Set `AURAGO_HTTPS_PORT` to a free host port.
3. **Volume permission issues**: Find the resolved volume name in the
   `volumes` section of `docker compose config`, then inspect its ownership and mount
   options. Back up the volume before repairing permissions.

### Healthcheck Fails

If the container is marked as unhealthy:

1. **Wait longer**: Initial startup can take 3-5 minutes for VectorDB initialization
2. **Check resource limits**: Increase memory if OOM kills occur
3. **Verify config**: Invalid config.yaml can prevent startup

```bash
# Check health status
docker compose ps

# Inspect healthcheck
docker inspect aurago | grep -A 10 Health
```

For the Gotenberg sidecar, AuraGo's compose file probes `GET /health` using
`curl`, `wget`, or a bash TCP fallback. If you maintain a custom compose file,
avoid healthchecks that assume only `wget` is installed; some Gotenberg image
variants do not include it.

### Docker Proxy Issues

If container management fails:

1. **Verify proxy is running**:
   ```bash
   docker compose ps docker-proxy
   ```

2. **Test proxy health**:
   ```bash
   docker compose exec docker-proxy wget -q --spider http://localhost:2375/_ping
   ```

3. **Check permissions**: Ensure `/var/run/docker.sock` is accessible

### Reset to Factory Defaults

```bash
# Stop and remove everything
docker compose down
project=aurago-docker  # Use your actual Compose project name.
docker volume rm "${project}_aurago_data" "${project}_aurago_workdir"

# Remove master key (if you want to regenerate)
rm secrets/aurago_master.key

# Start fresh
docker compose up -d
```

> **Warning**: This deletes all data including memories, chat history, and vault secrets!

---

## 9. Security Checklist

- [ ] External key file, if supplied, is private and readable by the container UID
- [ ] Master key backed up securely (password manager, encrypted storage)
- [ ] Host Docker access enabled only for deployments that need it
- [ ] Docker socket proxy reachable only through the internal `docker-control` network
- [ ] `IMAGES=1`, `POST=1`, and `EXEC=1` kept enabled if you use managed Docker features; keep `BUILD=0` unless you explicitly need local Docker image builds
- [ ] Resource limits configured appropriately for your hardware
- [ ] SSH keys for Ansible are dedicated keys, not your personal keys
- [ ] Regular security updates applied to host system
- [ ] Firewall rules limit access to the published HTTPS port and any required legacy HTTP port
- [ ] Logs reviewed periodically for suspicious activity

---

## 10. Production Deployment Recommendations

For production environments:

1. **Pin image digest**: Record the verified digest used by the deployment.
   ```yaml
   image: ghcr.io/antibyte/aurago:v1.2.3
   ```

2. **Keep the Docker socket proxy isolated**:
   ```yaml
   networks:
     docker-control:
       internal: true
   ```

3. **Configure backup strategy**:
   - Backup `aurago_master.key` securely
   - Regular backups of `aurago_data` volume
   - Test restoration procedure

4. **Monitor resource usage**:
   ```bash
   docker stats aurago
   ```

5. **Set up log aggregation**: Use journald or syslog driver

6. **Network isolation**: Use internal networks for sidecars

7. **Regular updates**: Subscribe to release notifications for security patches

8. **Audit configuration**: Periodically review config.yaml for security settings

Full browser resources are supplied by the version-bound local asset set. The installer and Docker image include it; bare executables retain a small recovery/login page. See [resource packaging and offline installation](web-assets.md).

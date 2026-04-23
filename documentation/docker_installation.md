# AuraGo Docker Installation Guide

AuraGo provides a fully automated Docker deployment with a secure default configuration. The setup uses a Docker socket proxy for safe container management, persistent volumes for your data, and an optional host-managed secret file for the vault master key.

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
curl -O https://raw.githubusercontent.com/antibyte/AuraGo/main/docker-compose.yml
```

### Step 3: Create the Master Key (recommended, optional)

Generate the encryption key **before** the first start so it is stored outside the container:

```bash
mkdir -p secrets
openssl rand -hex 32 > secrets/aurago_master.key
chmod 600 secrets/aurago_master.key
```

The `docker-compose.yml` mounts `./secrets` read-only into the container. If `secrets/aurago_master.key` exists, AuraGo loads it on startup without exposing it in the normal config.

> If you skip this step, AuraGo auto-generates a master key on first start and stores it in `/app/data/.env` inside the persistent Docker volume. That works out of the box, but a host-managed key file is better for backup and migration.

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
- The Docker socket proxy (`docker-proxy`) starts automatically and provides safe, filtered access to the host Docker daemon.
- The Web UI is now available at `http://<your-server-ip>:8088`.

Open the Web UI to finish setting up your LLM Provider and API keys!

---

## 2. Image Tags

The AuraGo Docker image is published to GitHub Container Registry (`ghcr.io/antibyte/aurago`).

| Tag | Description | Use Case |
|-----|-------------|----------|
| `latest` | Always points to the latest stable release | Default for release installs |
| `main` | Current main branch build | Testing / unreleased fixes |
| `v1.2.3` | Pinned to a specific release | Reproducible deployments |

> **Recommendation:** For production, pin to a specific version tag. For example:
> ```yaml
> image: ghcr.io/antibyte/aurago:v1.2.3
> ```

If you want the current branch build instead of the latest release, override the tag without editing the compose file:

```bash
AURAGO_IMAGE_TAG=main docker compose up -d
```

---

## 3. Installation via Dockge / Portainer

If you use a Docker management stack like **Dockge** or **Portainer**, deployment is just a copy-paste away.

### Step 1: Create the Stack
1. Open Dockge (or Portainer).
2. Create a new stack named `aurago`.
3. Paste the contents of the `docker-compose.yml` from the AuraGo repository into the editor.

### Step 2: Create the Master Key
On the host where Docker runs, create the secret file in the stack directory:
```bash
mkdir -p secrets
openssl rand -hex 32 > secrets/aurago_master.key
chmod 600 secrets/aurago_master.key
```

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
Access the Web UI at `http://<your-server-ip>:8088` and navigate to the **CONFIG** tab to finish setting up your AI agent.

> [!NOTE]
> Your `AURAGO_MASTER_KEY` is either stored in `secrets/aurago_master.key` on the host or auto-generated into `data/.env` inside the Docker volume. THIS KEY ENCRYPTS THE AGENT'S SECRET VAULT. BACK IT UP OR YOU WILL NOT BE ABLE TO MOVE THE VAULT TO ANOTHER SERVER!

---

## 4. Docker Socket Security

By default, the `docker-compose.yml` uses a **Docker socket proxy** (`tecnativa/docker-socket-proxy`) instead of mounting the Docker socket directly into the AuraGo container. This is a significant security improvement:

| | Socket Proxy (default) | Direct Socket Mount |
|---|---|---|
| Host access | Filtered API subset only | Full root-equivalent access |
| Risk if compromised | Limited to allowed operations | Complete host takeover |
| Container management | Start, stop, inspect, exec | Everything (including privilege escalation) |

### Switching to Direct Socket Access (NOT recommended)

If you have a specific reason to bypass the proxy:

1. In `docker-compose.yml`, comment out the `docker-proxy` service and the `depends_on` block.
2. Uncomment the `docker.sock` volume mount in the `aurago` service.
3. Change `DOCKER_HOST` in the `aurago` environment to `unix:///var/run/docker.sock`.
4. Remove or comment out the `docker-proxy` service block.

> **Warning:** Direct socket access grants root-equivalent access to the host machine. Only use this if you fully trust the AuraGo agent and all its tool calls.

---

## 5. Upgrading

### Standard Upgrade

```bash
docker compose pull
docker compose up -d
```

This pulls the latest image and restarts the container. Your data is preserved in Docker volumes:
- `aurago_data` — config, memory, chat history, vault, vector DB
- `aurago_workdir` — Python venv and generated tools

### Python Venv Considerations

The `aurago_workdir` volume preserves the Python virtual environment across restarts. This is intentional for performance, but requires attention during upgrades:

**When upgrading Python versions** (e.g., from Python 3.11 to 3.12):
```bash
# Stop container
docker compose down

# Remove the old venv (it will be recreated on next start)
docker volume rm aurago_aurago_workdir

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

If you already have a running AuraGo container that stores the key in `data/.env`:

```bash
# 1. Extract the existing key
mkdir -p secrets
docker compose exec aurago cat /app/data/.env | grep AURAGO_MASTER_KEY | cut -d= -f2 | tr -d '"' > secrets/aurago_master.key
chmod 600 secrets/aurago_master.key

# 2. Restart — the entrypoint now picks up /run/optional-secrets/aurago_master.key
docker compose down && docker compose up -d

# 3. (Optional) Remove the old .env from the volume
docker compose exec aurago rm -f /app/data/.env
```

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
The healthcheck has a 3-minute startup window (`start-period: 180s`) to allow for VectorDB initialization. On very slow systems this may not be enough. You can override it in `docker-compose.yml`:
```yaml
healthcheck:
  start_period: 300s
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

1. **Master key not found**: Check if `aurago_master.key` exists and has correct permissions:
   ```bash
   ls -la aurago_master.key
   # Should show: -rw------- (600)
   ```

2. **Port already in use**: Change host port in `docker-compose.yml`

3. **Volume permission issues**:
   ```bash
   docker volume rm aurago_aurago_data
   docker compose up -d
   ```

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
docker volume rm aurago_aurago_data aurago_aurago_workdir

# Remove master key (if you want to regenerate)
rm aurago_master.key

# Start fresh
docker compose up -d
```

> **Warning**: This deletes all data including memories, chat history, and vault secrets!

---

## 9. Security Checklist

- [ ] Master key created with `chmod 600` permissions
- [ ] Master key backed up securely (password manager, encrypted storage)
- [ ] Docker socket proxy enabled (not direct socket mount)
- [ ] EXEC=0 in docker-proxy if DockerExec tool not needed
- [ ] Resource limits configured appropriately for your hardware
- [ ] SSH keys for Ansible are dedicated keys, not your personal keys
- [ ] Regular security updates applied to host system
- [ ] Firewall rules restrict access to port 8088 if needed
- [ ] Logs reviewed periodically for suspicious activity

---

## 10. Production Deployment Recommendations

For production environments:

1. **Pin image version**: Don't use `latest`, use specific version tag
   ```yaml
   image: ghcr.io/antibyte/aurago:v1.2.3
   ```

2. **Enable Docker socket proxy restrictions**:
   ```yaml
   environment:
     - EXEC=0  # Disable if not needed
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

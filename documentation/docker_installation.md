# AuraGo Docker Installation Guide

AuraGo provides a fully automated Docker deployment with a secure default configuration. The setup uses a Docker socket proxy for safe container management, persistent volumes for your data, and Docker Compose secrets for encryption keys.

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

### Step 3: Create the Master Key (recommended)

Generate the encryption key **before** the first start so it is stored outside the container:

```bash
openssl rand -hex 32 > aurago_master.key
chmod 600 aurago_master.key
```

The `docker-compose.yml` references this file as a Docker Compose secret. The key is mounted read-only into `/run/secrets/aurago_master_key` — it never appears in `docker inspect` or process listings.

> If you skip this step the container auto-generates a key on first start and saves it inside the Docker volume (`data/.env`). This works but is less secure — extracting the key later for backup requires `docker compose exec`.

### Step 4: Start the Container
```bash
docker compose up -d
```

That's it!
- The `AURAGO_MASTER_KEY` is loaded from your `aurago_master.key` file via Docker Compose secrets.
- A default config is generated inside the container from the built-in template.
- The Docker socket proxy (`docker-proxy`) starts automatically and provides safe, filtered access to the host Docker daemon.
- The Web UI is now available at `http://<your-server-ip>:8088`.

Open the Web UI to finish setting up your LLM Provider and API keys!

---

## 2. Image Tags

The AuraGo Docker image is published to GitHub Container Registry (`ghcr.io/antibyte/aurago`).

| Tag | Description | Use Case |
|-----|-------------|----------|
| `latest` | Always points to the latest stable release | Production (default) |
| `v1.2.3` | Pinned to a specific release | Reproducible deployments |
| `main` | Current main branch build | Testing / pre-release |

> **Recommendation:** For production, pin to a specific version tag. For example:
> ```yaml
> image: ghcr.io/antibyte/aurago:v1.2.3
> ```

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
openssl rand -hex 32 > aurago_master.key
chmod 600 aurago_master.key
```

### Step 3: Deploy
Deploy the stack.
- Dockge/Portainer will pull the image and start three containers: `aurago`, `aurago_docker_proxy`, and `aurago_gotenberg`.
- On first start, the container automatically generates the config from the built-in template.
- Persistent volumes for `/app/data` and `/app/agent_workspace/workdir` are automatically created.

### Step 4: Configure via Web UI
Access the Web UI at `http://<your-server-ip>:8088` and navigate to the **CONFIG** tab to finish setting up your AI agent.

> [!NOTE]
> Your `AURAGO_MASTER_KEY` is stored in `aurago_master.key` on the host (or inside `data/.env` in the Docker volume if you skipped the secret file). THIS KEY ENCRYPTS THE AGENT'S SECRET VAULT. BACK IT UP OR YOU WILL NOT BE ABLE TO MOVE THE VAULT TO ANOTHER SERVER!

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

## 6. Migrating an Existing Installation to Docker Secrets

If you already have a running AuraGo container that stores the key in `data/.env`:

```bash
# 1. Extract the existing key
docker compose exec aurago cat /app/data/.env | grep AURAGO_MASTER_KEY | cut -d= -f2 | tr -d '"' > aurago_master.key
chmod 600 aurago_master.key

# 2. Restart — the entrypoint now picks up /run/secrets/aurago_master_key
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

# AuraGo Docker Installation Guide

AuraGo provides a fully automated Docker deployment. You don't need to manually create config files or generate encryption keys — the container does it all for you on the first run.

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
- A default `config.yaml` is created in your directory.
- The Web UI is now available at `http://<your-server-ip>:8088`. 

Open the Web UI to finish setting up your LLM Provider and API keys!

---

## 2. Installation via Dockge / Portainer

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
- Dockge will pull the latest `aurago:latest` image.
- On startup, the container automatically generates the `config.yaml` file (fixing Docker's default behavior of creating directories) and loads the master key from the Docker secret.
- Persistent volumes for `/app/data` and `/app/agent_workspace/workdir` are automatically created.

### Step 4: Configure via Web UI
Access the Web UI at `http://<your-server-ip>:8088` and navigate to the **CONFIG** tab to finish setting up your AI agent.

> [!NOTE]
> Your `AURAGO_MASTER_KEY` is stored in `aurago_master.key` on the host (or inside `data/.env` in the Docker volume if you skipped the secret file). THIS KEY ENCRYPTS THE AGENT'S SECRET VAULT. BACK IT UP OR YOU WILL NOT BE ABLE TO MOVE THE VAULT TO ANOTHER SERVER!

---

## 3. Migrating an Existing Installation to Docker Secrets

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

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

### Step 3: Start the Container
```bash
docker compose up -d
```

That's it! 
- A secure `AURAGO_MASTER_KEY` is automatically generated and saved in `data/.env`.
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

### Step 2: Deploy
Deploy the stack. 
- Dockge will pull the latest `aurago:latest` image.
- On startup, the container automatically generates the `config.yaml` file (fixing Docker's default behavior of creating directories) and a secure Master Key.
- Persistent volumes for `/app/data` and `/app/agent_workspace/workdir` are automatically created.

### Step 3: Configure via Web UI
Access the Web UI at `http://<your-server-ip>:8088` and navigate to the **CONFIG** tab to finish setting up your AI agent.

> [!NOTE]
> If you ever need your `AURAGO_MASTER_KEY` (e.g. for manual database decryption), you can find it inside the generated `data/.env` file within the `aurago_data` volume. THIS KEY ENCRYPTS THE AGENTS SECRET VAULT. MAKE A BACKUP OF IT OR YOU WILL NOT BE ABLE TO MOVE THE VAULT TO ANOTHER SERVER IF NEEDED!    

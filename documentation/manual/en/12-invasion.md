# Chapter 12: Invasion Control

> ⚠️ **Important:** Invasion Control is available exclusively via **Web-UI** and **REST API**. CLI commands for Invasion Control are not implemented.

Deploy and manage AuraGo agents across your infrastructure with Invasion Control. From single-server setups to multi-cloud deployments, manage remote execution with ease.

## Concept: Nests & Eggs

Invasion Control shares the **Nests** and **Eggs** concept with Mission Control but focuses on **deployment and lifecycle management** rather than scheduling.

### Nests (Target Servers)

A **Nest** represents a target server or environment where you deploy agents or run missions:

| Nest Type | Description | Use Case |
|-----------|-------------|----------|
| **Local** | The AuraGo host itself | Local development, testing |
| **SSH** | Remote servers via SSH | Production servers, VMs |
| **Docker** | Docker containers | Containerized applications |
| **Docker API** | Remote Docker daemon | Docker Swarm, remote hosts |

### Eggs (Configurations)

An **Egg** in Invasion Control is a deployment configuration that defines:
- **Connection parameters** (SSH keys, API endpoints)
- **Environment setup** (dependencies, configs)
- **Deployment scripts** (install, update, remove)
- **Agent configuration** (what the remote agent should do)

```
┌─────────────────────────────────────────────────────────┐
│  Nest Registry                                          │
│  ├─ 🏠 local (localhost)                                │
│  ├─ 🖥️ web-server-01 (SSH: 192.168.1.10)               │
│  ├─ 🐳 db-container (Docker: postgres)                 │
│  └─ ☁️ cloud-worker (Tailscale: 100.x.x.x)             │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│  Egg Templates                                          │
│  ├─ 📦 standard-agent (full AuraGo binary)             │
│  ├─ 📦 minimal-agent (lightweight version)             │
│  ├─ 📦 monitoring-only (metrics collector)             │
│  └─ 📦 custom-worker (specialized tasks)               │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│  Deployment (Hatch)                                     │
│  ├─ Nest: web-server-01                                 │
│  ├─ Egg: standard-agent                                 │
│  ├─ Status: ✅ Healthy                                  │
│  └─ Last Ping: 2 seconds ago                            │
└─────────────────────────────────────────────────────────┘
```

## SSH Connections Setup

SSH is the most common way to connect to remote Nests. AuraGo supports multiple authentication methods.

### Authentication Methods

| Method | Security | Use Case |
|--------|----------|----------|
| **SSH Key** | ⭐⭐⭐ High | Production servers, automated deployments |
| **Password** | ⭐⭐ Medium | Quick testing, legacy systems |
| **SSH Agent** | ⭐⭐⭐ High | Desktop environments with ssh-agent |
| **Vault Reference** | ⭐⭐⭐ High | Storing credentials in encrypted vault |

### Setting Up SSH Key Authentication

**Step 1: Generate SSH Key Pair**

```bash
# On your AuraGo host
ssh-keygen -t ed25519 -C "aurago-deployment" -f ~/.ssh/aurago_deploy
```

**Step 2: Add Public Key to Target Server**

```bash
# Copy public key to remote server
ssh-copy-id -i ~/.ssh/aurago_deploy.pub user@remote-server

# Or manually add to ~/.ssh/authorized_keys on remote
```

**Step 3: Configure Nest in AuraGo**

Navigate to **Invasion Control → Nests → New Nest**:

```yaml
Name: production-web-01
Type: SSH
Host: 192.168.1.10
Port: 22
Username: aurago
Authentication: SSH Key
SSH Key Path: /home/aurago/.ssh/aurago_deploy
# Or use vault: ${vault.ssh.production_key}

Advanced Options:
  Timeout: 30
  Keep Alive: true
  Strict Host Key Checking: yes
```

> 💡 **Tip:** Use different SSH keys for different environments (dev, staging, production) to limit blast radius if a key is compromised.

### Testing SSH Connection

Use the Web-UI or REST API to test the connection:

**Web-UI:** Navigate to **Invasion Control → Nests → production-web-01 → Test Connection**

**REST API:**
```bash
curl -X POST http://localhost:8088/api/invasion/nests/production-web-01/validate \
  -H "Authorization: Bearer ${API_TOKEN}"
```

Expected response:
```json
{
  "success": true,
  "host": "192.168.1.10",
  "response_time_ms": 45,
  "os": "Ubuntu 22.04 LTS",
  "architecture": "x86_64"
}
```

## Docker Deployment

Deploy agents as Docker containers for maximum isolation and portability.

### Local Docker (Daemon Socket)

For Docker on the AuraGo host:

```yaml
Name: local-docker
Type: Docker
Connection: unix:///var/run/docker.sock

Authentication: None (local socket)
```

> ⚠️ **Warning:** Mounting the Docker socket gives full container access. Use only for trusted local deployments.

### Remote Docker (TCP/TLS)

For remote Docker daemons:

```yaml
Name: remote-docker-host
Type: Docker API
Host: tcp://docker-worker-01:2376
TLS: true
CA Certificate: /certs/ca.pem
Client Certificate: /certs/client-cert.pem
Client Key: /certs/client-key.pem
```

### Creating a Docker Egg

Navigate to **Invasion Control → Eggs → New Egg**:

```yaml
Name: containerized-agent
Type: docker

Image: aurago/agent:latest
Pull Policy: Always

Container Configuration:
  Name: aurago-agent-${nest.name}
  Restart Policy: unless-stopped
  
Environment Variables:
  AURAGO_MODE: remote
  AURAGO_MASTER: ${aurago.host}
  AURAGO_TOKEN: ${vault.agent.token}
  
Volumes:
  - /var/run/docker.sock:/var/run/docker.sock:ro
  - agent_data:/data
  
Network: bridge
Ports:
  - "8080:8080"
  
Resources:
  CPU Limit: 1.0
  Memory Limit: 512M
```

### Deploying to Docker via REST API

```bash
# Deploy containerized-agent to local-docker
curl -X POST http://localhost:8088/api/invasion/nests/local-docker/hatch \
  -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "egg": "containerized-agent",
    "nest": "local-docker",
    "name": "web-agent-01"
  }'
```

Expected response:
```json
{
  "deployment_id": "hatch-001",
  "status": "pending",
  "steps": [
    {"step": "pull_image", "status": "completed"},
    {"step": "create_container", "status": "completed"},
    {"step": "start_container", "status": "completed"},
    {"step": "health_check", "status": "completed"}
  ],
  "result": "success"
}
```

## Deploying Remote Agents

Remote agents extend AuraGo's capabilities to other servers, forming a distributed agent network.

### Deployment Process

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  AuraGo      │────▶│  Target Nest │────▶│  Remote      │
│  Master      │ SSH │  (Server)    │     │  Agent       │
└──────────────┘     └──────────────┘     └──────────────┘
      │                                               │
      │◀──────────────────────────────────────────────┘
      │              Heartbeat & Status
```

### Standard Agent Deployment

**Egg: standard-remote-agent**

Navigate to **Invasion Control → Eggs → New Egg**:

```yaml
Name: standard-remote-agent
Type: deployment

Prerequisites:
  Commands:
    - "which curl || apt-get install -y curl"
    - "which docker || echo 'Docker not found - will use binary'"

Installation:
  Method: binary
  Source: https://releases.aurago.io/agent/latest
  Destination: /opt/aurago/agent
  
Configuration:
  Config File: /opt/aurago/config.yaml
  Template: |
    server:
      port: 8080
      bind: 0.0.0.0
    
    agent:
      mode: remote
      master_url: ${aurago.master_url}
      api_token: ${vault.agent.token}
      
    capabilities:
      allowed_tools:
        - filesystem
        - shell
        - docker
      
Service Setup:
  Systemd: true
  Service Name: aurago-agent
  Auto Start: true
  Restart: always
```

### Deployment via Web-UI

Navigate to **Invasion Control → Deployments → New Deployment**:

1. Select an **Egg** (deployment template)
2. Select a **Nest** (target server)
3. Configure deployment name
4. Click **Deploy**

### Deployment via REST API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/invasion/nests` | GET | List all nests |
| `/api/invasion/nests` | POST | Create a new nest |
| `/api/invasion/nests/{id}` | GET | Get nest details |
| `/api/invasion/nests/{id}` | PUT | Update nest |
| `/api/invasion/nests/{id}` | DELETE | Delete nest |
| `/api/invasion/nests/{id}/toggle` | POST | Enable/disable nest |
| `/api/invasion/nests/{id}/validate` | POST | Test nest connection |
| `/api/invasion/eggs` | GET | List all eggs |
| `/api/invasion/eggs` | POST | Create a new egg |
| `/api/invasion/eggs/{id}` | GET/PUT/DELETE | Manage egg |
| `/api/invasion/eggs/{id}/toggle` | POST | Enable/disable egg |
| `/api/invasion/nests/{id}/hatch` | POST | Deploy (hatch) egg on nest |
| `/api/invasion/ws` | WS | WebSocket for real-time events |

**Example: Deploy via REST API**

```bash
curl -X POST http://localhost:8088/api/invasion/nests/production-web-01/hatch \
  -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "egg": "standard-remote-agent",
    "nest": "production-web-01",
    "name": "web-agent-01"
  }'
```

Expected response:
```json
{
  "deployment_id": "hatch-001",
  "name": "web-agent-01",
  "egg": "standard-remote-agent",
  "nest": "production-web-01",
  "status": "healthy",
  "phases": [
    {"phase": "connection_check", "status": "completed"},
    {"phase": "prerequisites", "status": "completed"},
    {"phase": "installation", "status": "completed"},
    {"phase": "service_setup", "status": "completed"},
    {"phase": "verification", "status": "completed"}
  ],
  "agent_version": "1.4.2",
  "created_at": "2024-01-15T10:23:45Z"
}
```

## Lifecycle Management

### Deployment States

```
┌─────────┐    ┌──────────┐    ┌──────────┐
│ PENDING │───▶│ INSTALL  │───▶│ STARTING │
└─────────┘    └──────────┘    └──────────┘
                                    │
     ┌──────────────────────────────┘
     ▼
┌──────────┐    ┌──────────┐    ┌──────────┐
│ UPDATING │◀───│ HEALTHY  │───▶│ DEGRADED │
└──────────┘    └──────────┘    └──────────┘
                     │
     ┌───────────────┼───────────────┐
     ▼               ▼               ▼
┌──────────┐    ┌──────────┐    ┌──────────┐
│ STOPPED  │    │  ERROR   │    │ REMOVING │
└──────────┘    └──────────┘    └──────────┘
```

### Managing Deployments via Web-UI

**View All Deployments:**

Navigate to **Invasion Control → Deployments**

The dashboard shows:

| Deployment | Nest | Status | Version |
|------------|------|--------|---------|
| web-agent-01 | web-server | ✅ Healthy | 1.4.2 |
| db-monitor | db-server | ✅ Healthy | 1.4.2 |
| backup-worker | backup-nas | ⚠️ Degraded | 1.4.1 |
| cloud-proxy | aws-instance | ✅ Healthy | 1.4.2 |

**Updating an Agent:**

Navigate to **Invasion Control → Deployments → web-agent-01 → Update**

Or use REST API:
```bash
curl -X PUT http://localhost:8088/api/invasion/eggs/web-agent-01 \
  -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "action": "update",
    "target_version": "1.4.2"
  }'
```

**Removing a Deployment:**

Navigate to **Invasion Control → Deployments → web-agent-01 → Remove**

Or use REST API:
```bash
curl -X DELETE http://localhost:8088/api/invasion/eggs/backup-worker \
  -H "Authorization: Bearer ${API_TOKEN}"
```

## Connection Types

### SSH (Secure Shell)

Most versatile connection type for Linux/Unix servers.

Configure via Web-UI at **Invasion Control → Nests → New Nest**:

```yaml
Type: SSH
Host: 192.168.1.10
Port: 22
Username: deploy
Auth: SSH Key
Key Path: ~/.ssh/id_ed25519

Jump Host:
  Enabled: false
  # For bastion host setups:
  # Host: bastion.company.com
  # User: jumphost
```

### Docker API

Connect to Docker daemon directly for container management.

```yaml
Type: Docker API
Host: tcp://192.168.1.20:2376
TLS: true

# Certificates for TLS verification
CA: /certs/ca.pem
Cert: /certs/client.pem
Key: /certs/client-key.pem

Context: default  # Docker context name (optional)
```

### Local

The AuraGo host itself.

```yaml
Type: Local
Path: /var/lib/aurago/local-agents

# No authentication needed
# Direct filesystem access
```

### Kubernetes (Optional)

For deploying to K8s clusters:

```yaml
Type: Kubernetes
Context: production-cluster
Namespace: aurago-agents

Authentication:
  Method: kubeconfig
  Path: ~/.kube/config
  # Or use service account token
```

## Routing Options

Control how AuraGo connects to remote Nests.

### Direct

Direct network connection – fastest, but requires network accessibility.

```
AuraGo ──────▶ Remote Server
        (Direct)
```

Configure via Web-UI:

```yaml
Routing: Direct
Host: 192.168.1.10
Port: 22
```

**Requirements:**
- Remote server must be directly reachable
- Firewall must allow the connection
- Static IP or DNS name recommended

### SSH Tunnel

Route traffic through an intermediate SSH server (bastion/jump host).

```
AuraGo ──────▶ Bastion Host ──────▶ Remote Server
        (SSH)              (SSH)
```

Configure via Web-UI:

```yaml
Routing: SSH Tunnel
Target:
  Host: 10.0.1.50  # Internal IP, not directly reachable
  Port: 22

Jump Host:
  Host: bastion.company.com
  Port: 22
  User: jumphost
  Key: ~/.ssh/bastion_key

Local Port Forwarding:
  - "8080:localhost:8080"  # Access remote service locally
```

> 💡 **Tip:** SSH tunnels are excellent for accessing internal servers without VPN. All traffic is encrypted end-to-end.

### Tailscale

Zero-config VPN using Tailscale mesh networking.

```
AuraGo ──────▶ Tailscale Mesh ──────▶ Remote Server
        (Encrypted WireGuard)
```

Configure via Web-UI:

```yaml
Routing: Tailscale
Tailscale IP: 100.x.x.x

# Tailscale authentication
# (handled by Tailscale daemon)
Auth Key: ${vault.tailscale.auth_key}

# Optional: Use Tailscale SSH
Tailscale SSH: true
```

**Advantages:**
- Works across NAT and firewalls
- Automatic encryption (WireGuard)
- No port forwarding needed
- Works anywhere with internet

### Routing Comparison

| Method | Setup | Security | Speed | Best For |
|--------|-------|----------|-------|----------|
| **Direct** | Easy | Depends on network | ⭐⭐⭐ Fastest | Same network, static IPs |
| **SSH Tunnel** | Medium | ⭐⭐⭐ Excellent | ⭐⭐ Good | Bastion setups, secure access |
| **Tailscale** | Easy | ⭐⭐⭐ Excellent | ⭐⭐⭐ Fast | Remote workers, dynamic IPs |
| **VPN** | Complex | ⭐⭐⭐ Excellent | ⭐⭐ Good | Enterprise, complex networks |

## Hatch Status Monitoring

Monitor the health and status of all your deployments.

### Health Metrics

| Metric | Description | Warning Threshold |
|--------|-------------|-------------------|
| **Status** | Current state (Healthy/Degraded/Error) | - |
| **Last Ping** | Time since last heartbeat | > 60 seconds |
| **CPU Usage** | Remote agent CPU consumption | > 80% |
| **Memory Usage** | Remote agent RAM usage | > 90% |
| **Disk Space** | Available space on remote | < 10% |
| **Version** | Running agent version | Mismatch with master |

### Dashboard View

Navigate to **Invasion Control → Dashboard** to view:

```
┌─────────────────────────────────────────────────────────┐
│ 🥚 Invasion Control Dashboard                           │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Overall Health: 4/5 Healthy                            │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │ web-agent-01                                    │   │
│  │ ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ ✅ Healthy     │   │
│  │ Nest: web-server │ Version: 1.4.2 │ Uptime: 15d │   │
│  │ Last Ping: 2s ago │ CPU: 12% │ RAM: 256MB       │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │ backup-worker                                   │   │
│  │ ━━━━━━━━━━━━━━━━━━━━━━╺━━━━━━━━ ⚠️ Degraded     │   │
│  │ Nest: backup-nas │ Version: 1.4.1 (outdated)    │   │
│  │ Warning: High disk usage (92%)                  │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  [View Logs] [Update] [Restart] [Remove]               │
└─────────────────────────────────────────────────────────┘
```

### Alert Configuration

Navigate to **Invasion Control → Settings → Alerts**:

```yaml
Monitoring:
  Health Check Interval: 30s
  
  Alerts:
    Agent Offline:
      Condition: last_ping > 2m
      Action: notify.telegram
      
    High Resource Usage:
      Condition: cpu > 80% OR memory > 90%
      Action: notify.email
      
    Version Mismatch:
      Condition: version != master.version
      Action: notify.webui
```

Or via REST API:
```bash
curl -X PUT http://localhost:8088/api/invasion/nests/production-web-01 \
  -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "health_check_interval": "30s",
    "alerts": [
      {
        "name": "Agent Offline",
        "condition": "last_ping > 2m",
        "action": "notify.telegram"
      }
    ]
  }'
```

## Troubleshooting Deployments

### Common Issues

#### Connection Refused

```
❌ Error: Connection refused (port 22)
```

**Solutions:**
1. Verify target server is running
2. Check firewall rules: `sudo ufw allow 22`
3. Verify SSH service: `sudo systemctl status sshd`
4. Check port number (may not be default 22)

#### Authentication Failed

```
❌ Error: Authentication failed for user 'deploy'
```

**Solutions:**
1. Verify username is correct
2. Check SSH key permissions (should be 600)
3. Ensure public key is in `~/.ssh/authorized_keys` on target
4. Try manual SSH: `ssh -i key user@host`

#### Permission Denied (Installation)

```
❌ Error: Cannot write to /opt/aurago
```

**Solutions:**
1. Ensure user has sudo privileges
2. Create directory with correct permissions first:
   ```bash
   sudo mkdir -p /opt/aurago
   sudo chown deploy:deploy /opt/aurago
   ```
3. Or choose different install path (e.g., `~/aurago`)

#### Agent Won't Start

```
❌ Error: Service failed to start
```

**Solutions:**
1. Check logs: `journalctl -u aurago-agent -n 50`
2. Verify config file syntax
3. Check port conflicts: `netstat -tlnp | grep 8080`
4. Ensure binary has execute permission: `chmod +x aurago-agent`

### Diagnostic via REST API

```bash
curl -X POST http://localhost:8088/api/invasion/nests/web-agent-01/validate \
  -H "Authorization: Bearer ${API_TOKEN}"
```

Expected response:
```json
{
  "deployment": "web-agent-01",
  "checks": {
    "connection": {
      "status": "passed",
      "response_time_ms": 45,
      "message": "SSH reachable"
    },
    "authentication": {
      "status": "passed",
      "message": "Authentication successful"
    },
    "installation": {
      "status": "passed",
      "message": "Binary exists, config readable"
    },
    "service": {
      "status": "failed",
      "message": "Port 8080 already in use"
    }
  },
  "recommendation": "Port 8080 is occupied. Options: 1) Change agent port, 2) Stop other service, 3) Use Docker deployment"
}
```

### Log Access via REST API

View remote agent logs:

```bash
curl "http://localhost:8088/api/invasion/nests/web-agent-01" \
  -H "Authorization: Bearer ${API_TOKEN}"
```

Expected response:
```json
{
  "deployment": "web-agent-01",
  "lines": [
    "[2024-01-15 10:23:45] INFO: Agent starting (v1.4.2)",
    "[2024-01-15 10:23:45] INFO: Connecting to master...",
    "[2024-01-15 10:23:46] INFO: Connected successfully",
    "[2024-01-15 10:23:46] INFO: Health server listening on :8080",
    "[2024-01-15 10:24:15] INFO: Heartbeat sent"
  ]
}
```

### Emergency Recovery via REST API

If a deployment becomes unresponsive:

```bash
curl -X POST http://localhost:8088/api/invasion/nests/web-agent-01/hatch \
  -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "force": true
  }'
```

Expected response:
```json
{
  "deployment": "web-agent-01",
  "action": "force_restart",
  "steps": [
    {"step": "ssh_connect", "status": "completed"},
    {"step": "kill_process", "status": "completed"},
    {"step": "start_fresh", "status": "completed"},
    {"step": "verify", "status": "completed"}
  ],
  "result": "success"
}
```

---

> 💡 **Tip:** Always test deployments on a staging environment before deploying to production. Use the same Egg configuration to ensure consistency.

> ⚠️ **Warning:** Keep SSH keys and API tokens secure. Use AuraGo's vault system to store sensitive credentials instead of hardcoding them in configurations.

---

## Next Steps

- **[Chapter 11: Mission Control](11-missions.md)** – Run scheduled tasks on remote Nests
- **[Chapter 13: Dashboard](13-dashboard.md)** – Monitor all deployments
- **[Chapter 14: Security](14-security.md)** – Secure your agent deployments

---
id: "docker_mode"
tags: ["conditional"]
priority: "45"
conditions: ["is_docker"]
---
# DOCKER ENVIRONMENT NOTICE

You are running inside a Docker container. Keep these constraints in mind:

- **No host firewall access.** `iptables`/`ufw` commands will fail. Do not attempt firewall operations.
- **No sudo.** The container runs as a non-root user or root without sudo. Avoid sudo-prefixed commands.
- **Docker socket may be unavailable.** Container management tools (docker, sandbox, homepage dev containers) only work if the host socket is mounted. If a Docker tool call fails, inform the user that the socket is not mounted.
- **Network limitations.** UDP broadcast (Wake-on-LAN) and mDNS discovery (Chromecast) require `network_mode: host`. If those tools are unavailable, suggest the user add `network_mode: host` to their docker-compose or use manual IP addresses for Chromecast.
- **File paths are container-internal.** Paths you see are inside the container, not on the host.

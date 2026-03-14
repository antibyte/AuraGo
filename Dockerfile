# ============================================================
# Stage 1: Build
# ============================================================
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS builder

# Injected by docker buildx for cross-compilation
ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

# Download dependencies first (better layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build the production binaries
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /aurago ./cmd/aurago
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /lifeboat ./cmd/lifeboat
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /config-merger ./cmd/config-merger
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /aurago-remote ./cmd/remote

# ============================================================
# Stage 2: Runtime
# ============================================================
# python:3.12-slim already ships Python 3 + pip.
# We add ffmpeg (needed for Telegram voice conversion).
FROM python:3.12-slim-bookworm AS runtime

# ----- system dependencies -----
RUN apt-get update && apt-get install -y --no-install-recommends \
        ffmpeg \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# ----- app layout -----
WORKDIR /app

# Binaries from builder stage
COPY --from=builder /aurago /app/aurago
COPY --from=builder /lifeboat /app/lifeboat
COPY --from=builder /config-merger /app/config-merger
COPY --from=builder /aurago-remote /app/aurago-remote

# Static resources that the agent needs at runtime.
# config.yaml is intentionally NOT baked in – users must supply it via volume.
COPY prompts                        /app/prompts
COPY agent_workspace/skills         /app/agent_workspace/skills
COPY documentation                  /app/documentation

# Create writable runtime directories.
# agent_workspace/workdir  – Python venv, generated tools, scratch files
# data/                    – memory, chat history, state
RUN mkdir -p \
        /app/agent_workspace/workdir \
        /app/agent_workspace/tools \
        /app/data \
        /app/log

# The venv lives inside workdir and is created automatically by AuraGo
# on first Python execution.  Mount workdir as a named volume so the venv
# (and installed pip packages) survive container restarts.

# ----- copy entrypoint & default config -----
# Must be done as root (before USER directive) because Docker COPY always
# creates files with root ownership regardless of the USER setting.
# The chown -R below then hands everything over to aurago.
# In Docker the server must always bind to 0.0.0.0 so it's reachable from
# the host.  Setting this env var lets config.Load() override server.host
# without any YAML manipulation in the entrypoint script.
ENV AURAGO_SERVER_HOST=0.0.0.0

COPY docker-entrypoint.sh /app/docker-entrypoint.sh
COPY config_template.yaml /app/config.yaml.default
# Normalize CRLF -> LF in case the file was committed from a Windows machine.
RUN sed -i 's/\r$//' /app/docker-entrypoint.sh /app/config.yaml.default
RUN chmod +x /app/docker-entrypoint.sh

# ----- runtime user (non-root) -----
RUN useradd -m -u 1001 aurago \
    && chown -R aurago:aurago /app
USER aurago

# ----- exposed ports -----
# 8088 – Web UI + REST API  (matches config.yaml server.port default)
# 8089 – Internal TCP bridge (accessed only by the agent itself)
EXPOSE 8088

# ----- volumes -----
# Mount these from outside to persist state across container restarts:
#   /app/config.yaml              – your filled-in config (required)
#   /app/data                     – memory, chat history, master key, state
#   /app/agent_workspace/workdir  – Python venv + generated tools
VOLUME ["/app/data", "/app/agent_workspace/workdir"]

# ----- entrypoint -----
ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["/app/aurago", "--config", "/app/data/config.yaml"]

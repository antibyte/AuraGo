# Code Studio package 2a: image and LSP bridge — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to work through this plan one task at a time. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The Code Studio image gets four language servers: gopls, typescript-language-server, pyright and clangd. Go gets a WebSocket-to-stdio bridge at `GET /api/code-studio/lsp?lang=go|typescript|python|cpp`. The bridge pipes one JSON-RPC message per text frame to one language server in the `aurago-code-studio` container. The image is built and checked locally as `aurago-code-studio:lsp-dev`. Nothing is pushed: no git push and no image push.

**Architecture:**

- **Image:** `deploy/docker/Dockerfile.code-studio` pins the three downloadable servers in `ARG` lines and adds Debian's `clangd`. It removes the Go build caches in the same layer that creates them. The build stops if the base image's Node is older than 22.22.2.
- **Bridge:** everything lives in one new file, `internal/server/code_studio_lsp.go`, in three layers:
  - Protocol (Task 2): a Content-Length reader and writer, a Docker attach-stream demultiplexer, and a capped stderr tail.
  - Runner (Task 3): a fixed command table, a process-wide session counter, and a small streaming-exec seam `codeStudioLSPRunner` (`StartLSP`, `EndLSP`). `codeStudioDockerAdapter` implements the seam, so tests can use an in-memory fake.
  - Handler (Task 4): `handleLSP` runs the terminal's gate chain. `serveCodeStudioLSP` runs one session: slot, `command -v` probe, start, the two pipes, ping/pong, and cleanup.
- **Changes to existing code:** exactly one route line in `registerCodeStudioRoutes`.

**Tech Stack:**

- Go `net/http` and `github.com/gorilla/websocket`
- the Docker Engine API, through the existing `tools.DockerRequestContext` and `openCodeStudioExecStdinStream`
- Go `testing` with `httptest`, a gorilla client and an `io.Pipe` fake
- an opt-in test against the real image on local Docker Desktop

---

## Notes for the executor

### Repo rules (mandatory)

**Shared checkout.** Other agent sessions build and commit in this same checkout at the same time.
- Never run `git add`, `git commit`, `git stash`, `git reset --hard` or `git checkout -- <path>` in the shared index or working tree.
- Commit each task through the temp-index script onto `main`:
  `bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh <message-file> <repo-relative paths...>`
- Write the message file with the Write tool into that scratchpad. Every message ends with the line `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- The script prints `committed <new> (parent <old>)`, then the short status of the paths. That status is empty when they are clean.
- If it prints `WARNING: .git/index.lock present`, the commit landed but the shared index was not synced. Do not delete the lock. Tell the user.

**GitNexus.**
- Before editing an existing function, run impact upstream. Report the callers, processes and risk. Warn on HIGH or CRITICAL. Treat `UNKNOWN` as unresolved and confirm it with a text search.
  - MCP: `impact({target: "<symbol>", direction: "upstream"})`
  - CLI from the repo root: `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<symbol>" --direction upstream --repo .`
- Before each commit, run detect_changes:
  - MCP: `detect_changes({scope: "all"})`
  - CLI: `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo .`
  - The result also lists other sessions' dirty files. Judge only this plan's paths.
  - `partial: true` or `truncated: true` is not a clean result. Run it again.
- In this plan, the only existing function that is edited is `registerCodeStudioRoutes` (Task 4). Everything else is new code.

**Go on this Windows box.**
- Prefix every Go command with `GOTOOLCHAIN=go1.27.4`.
- `GOTMPDIR` must be a short path with backslashes: `C:\tmp\go-build-lsp`. Create it with `mkdir -p /c/tmp/go-build-lsp`.
- Run all commands from Git Bash (the Bash tool). Each Bash call starts in the repo root, so commands begin with `cd /c/Users/Andi/Documents/repo/AuraGo`.
- The test command pipes through `grep`. If you see `FAIL ... [build failed]`, run it again without the `| grep ...` part to read the compiler errors.
- If a build error comes from another session's in-progress Go file, see "Overlay fallback" below. Never edit or move their files.

**Write-tool trap.** The Write tool decodes `\u` escapes. The code in this plan contains no `\u` sequence. If you add one, write `\x5c` for the backslash in Go source.

**Lean tests.** Write exactly these nine test functions and no other suites:
- eight unit tests in `code_studio_lsp_test.go`
- one opt-in real-image test in `code_studio_lsp_image_test.go`

Run only the patterns given in each task.

**Scope.**
- Do not touch any frontend file. Plan 2b owns them.
- Do not change behaviour in `code_studio_terminal.go` or `containers_terminal.go`. This plan only calls their helpers.
- Line numbers in `code_studio_handlers.go` may have shifted. Find the anchor by its quoted text.

**Docker.**
- Docker Desktop must be running.
- Never stop or remove a container or image you did not create.
- Never push the image. The user publishes it later through `.github/workflows/docker-publish.yml`.
- Keep `aurago-code-studio:lsp-dev` after the plan. It is the local dev image for the manual checks in 2b and 2c.

### Deliberate design choices (do not "fix" these)

1. **Gate 1 is `requireDesktopOperation(h.server, w, r, desktopScopeWrite, desktopExecute)`.**
   - `requirePermission(desktopScopeWrite)` is `requireDesktopPermission`, which takes the desktop operation from the path. Only GET paths that end in `/terminal` (and a few others) count as execute.
   - For `/api/code-studio/lsp` the path-derived operation would be "read". "Read" skips `beginDesktopRun`, so a switch to read-only would not revoke a live LSP session. The terminal's session would be revoked.
   - Naming the execute operation explicitly is the same check the terminal gets. It keeps the spec's order and its intent ("same rules as the terminal").
2. **The session cap is a package-level guarded counter (`codeStudioLSPSessions`), not a field on `Server`.**
   - One AuraGo process serves one `Server`.
   - It avoids editing the `Server` struct in `server.go`, a hot file in the shared checkout.
   - It matches the existing package-level `activeContainerTerminalBackend`.
3. **The cleanup always runs `EndLSP`, even when the server exited in time.** Children such as tsserver or `go list` can outlive their parent. `EndLSP` also runs after a failed start, because a late start failure may have started the process.
4. **Extra close codes:**
   - Invalid JSON closes with 1007, a binary frame with 1003, and an oversize page frame with 1009 (sent by gorilla).
   - These happen only on client bugs. Plan 2b treats them as "any other close".
   - A server stream that breaks the LSP framing closes with 1011, like a process exit.
5. **The cap is checked before the `command -v` probe,** so a flood of sockets never runs Docker execs.

### Overlay fallback (only when another session's files break the build)

Write this script with the Write tool to `C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/lsp-overlay.mjs`:

```js
// Writes lsp-overlay.json next to this script: a go -overlay that hides other
// sessions' in-progress Go edits from this plan's builds without touching them.
// Untracked foreign .go files become empty (dropped); modified, deleted or
// newly staged tracked foreign .go files read as their HEAD version (or empty
// when HEAD has no such file).
import { execFileSync } from 'node:child_process';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repo = 'C:/Users/Andi/Documents/repo/AuraGo';
const here = dirname(fileURLToPath(import.meta.url));
const own = new Set([
  'internal/server/code_studio_handlers.go',
  'internal/server/code_studio_lsp.go',
  'internal/server/code_studio_lsp_test.go',
  'internal/server/code_studio_lsp_image_test.go',
]);
const git = (...args) => execFileSync('git', ['-C', repo, ...args], { encoding: 'utf8', maxBuffer: 256 * 1024 * 1024 });
const list = (...args) => git(...args).split('\n').map((s) => s.trim()).filter((p) => p.endsWith('.go') && !own.has(p));
const replace = {};
for (const p of list('ls-files', '--others', '--exclude-standard', '--', '*.go')) {
  replace[resolve(repo, p)] = '';
}
const headDir = join(here, 'lsp-overlay-head');
mkdirSync(headDir, { recursive: true });
for (const p of list('diff', '--name-only', 'HEAD', '--', '*.go')) {
  let content;
  try {
    content = git('show', `HEAD:${p}`);
  } catch {
    replace[resolve(repo, p)] = '';
    continue;
  }
  const copy = join(headDir, p.replaceAll('/', '__'));
  writeFileSync(copy, content);
  replace[resolve(repo, p)] = copy;
}
writeFileSync(join(here, 'lsp-overlay.json'), JSON.stringify({ Replace: replace }, null, 1));
console.log(`${Object.keys(replace).length} foreign Go files overlaid`);
```

Regenerate the overlay right before each test run, because the set of foreign files changes:
`node C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/lsp-overlay.mjs`

Then add `-overlay C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/lsp-overlay.json` right after `go test` in the task's command. Do not commit the script or the JSON.

## File Structure

| File | Change | Responsibility |
| --- | --- | --- |
| `deploy/docker/Dockerfile.code-studio` | Modify (Task 1) | Pinned gopls, typescript-language-server, pyright; Debian clangd; Node ≥ 22.22.2 build check |
| `internal/server/code_studio_lsp.go` | Create (Task 2), extend (Tasks 3, 4) | Framing, demux, stderr tail; command table, session cap, runner seam, Docker adapter `StartLSP`/`EndLSP`; handler, session, pipes, cleanup |
| `internal/server/code_studio_lsp_test.go` | Create (Task 2), extend (Task 4) | The eight lean unit tests with an in-memory exec fake |
| `internal/server/code_studio_lsp_image_test.go` | Create (Task 5) | Opt-in check of the four real servers through the real bridge (`AURAGO_LSP_IMAGE`) |
| `internal/server/code_studio_handlers.go` | Modify (Task 4) | One route line in `registerCodeStudioRoutes` |

## Spec coverage map

| Spec item | Task |
| --- | --- |
| Image: gopls `v0.24.0` via `go install` with the image's Go, `/usr/local/bin`, caches removed | 1 |
| Image: typescript-language-server `6.1.0`, pyright `1.1.414` (npm), Node ≥ 22.22.2 guaranteed | 1 |
| Image: Debian bookworm `clangd` | 1 |
| Docker demux, Content-Length reader/writer, stderr discarded with a capped debug tail | 2 |
| Limits: header block ≤ 8 KiB, server message ≤ 32 MiB | 2 |
| Fixed command table (`gopls`, `typescript-language-server --stdio`, `pyright-langserver --stdio`, `clangd --background-index=false`) | 3 |
| Non-TTY exec with stdin/stdout/stderr, user `developer`, `/workspace`, `AURAGO_LSP_SESSION`, `GOTOOLCHAIN=local` | 3 |
| Cleanup script: `/proc` scan by nonce, TERM then KILL | 3 |
| At most 8 sessions server-wide | 3 (counter), 4 (4429) |
| Route, gate chain in the spec's order, unknown `lang` → 400 before the upgrade | 4 |
| Page frame ≤ max file size + 64 KiB (`SetReadLimit`), `json.Valid` | 4 |
| Close codes 4404 (`command -v`), 4429, 4500, 1011 | 4 |
| Ping/pong as in the terminal | 4 |
| Cleanup sequence: close stdin, wait ≤ 2 s, end leftovers | 4 |
| Go tests (framing both ways, limits, invalid JSON, 400, 403, 4404, 4429, cleanup) | 2, 4 |
| Manual check: each server answers `initialize` and a hover through the real bridge on the local image | 5 |

## Contract delivered to plan 2b

These come from the spec and the contract, section "2a -> 2b".

- **Socket:** WebSocket `GET /api/code-studio/lsp?lang=go|typescript|python|cpp`, same origin, session cookie.
- **Framing:** one JSON-RPC message per text frame in both directions. There are no Content-Length headers on the socket. The bridge does not inspect or rewrite messages.
- **Close codes after the upgrade:**
  - 4404: language server not installed
  - 4429: too many sessions
  - 4500: start failed
  - 1011: the server process exited, or its stream broke
  - Client-bug closes, all "any other close" for 2b: 1007 (invalid JSON), 1003 (binary frame), 1009 (frame over max file size + 64 KiB)
- **HTTP errors before the upgrade:**
  - 400: unknown `lang`
  - 403: read-only desktop, forbidden origin, or Docker read-only
  - 503: Docker disabled, or the container is unavailable
  - The browser cannot read these statuses. It sees a failed handshake (close 1006). 2b already knows read-only from `/api/code-studio/status` (`read_only`).
- **Session lifetime:**
  - Ping every 30 s. The session ends after 75 s without any frame from the page.
  - An open socket holds the container (`HoldTerminal`). Closing the socket releases it.

---

### Task 1: Language servers in the Code Studio image

**Files:**
- Modify: `deploy/docker/Dockerfile.code-studio`

- [ ] **Step 1: Check for parallel work and for Docker.**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && git status --short -- deploy/docker/Dockerfile.code-studio internal/server/code_studio_handlers.go internal/server/code_studio_lsp.go internal/server/code_studio_lsp_test.go internal/server/code_studio_lsp_image_test.go`
Expected: no output. If any path is listed, another session is working on it. Stop and ask the user.

Run: `docker version --format '{{.Server.Version}}'`
Expected: a version number (Docker Desktop is running).

- [ ] **Step 2: Impact check.** The Dockerfile has no graph symbols, so there is no impact run. Note this in the report.

- [ ] **Step 3: Replace `deploy/docker/Dockerfile.code-studio` with this complete content.** The changes are: three new `ARG` lines, `clangd` in the apt list, a new gopls layer after the Go `PATH` line, and a new npm layer after the existing npm line. Everything else is unchanged.

```dockerfile
FROM node:22-bookworm-slim

ARG TARGETARCH
ARG GO_VERSION=1.27.4
# Language servers for Code Studio code intelligence (LSP). Change a version
# only together with a local build and TestCodeStudioLSPRealImage.
ARG GOPLS_VERSION=v0.24.0
ARG TYPESCRIPT_LANGUAGE_SERVER_VERSION=6.1.0
ARG PYRIGHT_VERSION=1.1.414

ENV DEBIAN_FRONTEND=noninteractive
ENV WORKSPACE=/workspace

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash \
        ca-certificates \
        clangd \
        curl \
        gcc \
        git \
        grep \
        libc6-dev \
        make \
        openssh-client \
        python3 \
        python3-pip \
        python3-venv \
        ripgrep \
        sudo \
        tini \
        unzip \
    && rm -rf /var/lib/apt/lists/*

RUN set -eux; \
    case "$TARGETARCH" in \
        amd64) go_arch="amd64" ;; \
        arm64) go_arch="arm64" ;; \
        *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${go_arch}.tar.gz" -o /tmp/go.tgz; \
    rm -rf /usr/local/go; \
    tar -C /usr/local -xzf /tmp/go.tgz; \
    rm -f /tmp/go.tgz

ENV PATH=/usr/local/go/bin:$PATH

# gopls is built with the image's own Go: GOTOOLCHAIN=local never fetches
# another toolchain. Build and module caches stay in /tmp/gopls-build and are
# removed in the same layer.
RUN set -eux; \
    GOTOOLCHAIN=local CGO_ENABLED=0 GOBIN=/usr/local/bin \
    GOPATH=/tmp/gopls-build/gopath GOCACHE=/tmp/gopls-build/cache \
        go install -trimpath -ldflags="-s -w" "golang.org/x/tools/gopls@${GOPLS_VERSION}"; \
    chmod -R u+w /tmp/gopls-build; \
    rm -rf /tmp/gopls-build; \
    gopls version

RUN npm install -g --force pnpm yarn typescript tsx

# typescript-language-server 6.x needs Node 22.22.2 or newer. The base tag
# floats, so the build stops here when it resolved to an older Node.
RUN node -e 'const want=[22,22,2];const have=process.versions.node.split(".").map(Number);for(let i=0;i<3;i++){if(have[i]>want[i])process.exit(0);if(have[i]<want[i]){console.error("Node "+process.versions.node+" is older than 22.22.2, which typescript-language-server needs: run docker pull node:22-bookworm-slim, or pin FROM node:22.22.2-bookworm-slim (or a newer 22.x tag)");process.exit(1)}}' \
    && npm install -g "typescript-language-server@${TYPESCRIPT_LANGUAGE_SERVER_VERSION}" "pyright@${PYRIGHT_VERSION}" \
    && npm cache clean --force \
    && typescript-language-server --version \
    && pyright --version

RUN if getent passwd 1000 >/dev/null; then \
        existing_user="$(getent passwd 1000 | cut -d: -f1)"; \
        if [ "$existing_user" != "developer" ]; then usermod -l developer "$existing_user"; fi; \
        existing_group="$(id -gn developer)"; \
        if [ "$existing_group" != "developer" ] && ! getent group developer >/dev/null; then groupmod -n developer "$existing_group"; fi; \
        usermod -d /home/developer -m -s /bin/bash developer; \
    else \
        groupadd -g 1000 developer; \
        useradd -m -u 1000 -g developer -s /bin/bash developer; \
    fi \
    && mkdir -p "$WORKSPACE" /home/developer/.npm /home/developer/.cache \
    && chown -R developer:developer "$WORKSPACE" /home/developer \
    && echo "developer ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/developer \
    && chmod 0440 /etc/sudoers.d/developer

USER developer
WORKDIR /workspace

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["bash", "-lc", "sleep infinity"]
```

- [ ] **Step 4: The existing Dockerfile contract tests still pass.**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && mkdir -p /c/tmp/go-build-lsp && GOTOOLCHAIN=go1.27.4 GOTMPDIR='C:\tmp\go-build-lsp' go test ./internal/server -run '(?i)(CodeStudioDockerfile|DockerImageIncludesCodeStudioDockerfile)' -count=1 -v 2>&1 | grep -E -- '--- (PASS|FAIL|SKIP)|^(ok|FAIL|panic)'`
Expected: `--- PASS` for these four tests, then `ok  	aurago/internal/server`:
- `TestCodeStudioDockerfileDoesNotSelfUpdateNPM`
- `TestCodeStudioDockerfileIncludesRunButtonRuntimes`
- `TestCodeStudioDockerfileHandlesExistingUID1000User`
- `TestDockerImageIncludesCodeStudioDockerfile`

- [ ] **Step 5: Refresh the base image and check its Node.**

Run: `docker pull node:22-bookworm-slim`
Expected: `Status: Image is up to date for node:22-bookworm-slim` or `Status: Downloaded newer image for node:22-bookworm-slim`.

Run: `docker run --rm node:22-bookworm-slim node --version`
Expected: `v22.22.2` or a newer `v22.x` patch.

If the version is older even after the pull:
1. Run `docker pull node:22.22.2-bookworm-slim`.
2. If that tag exists, change the first line to `FROM node:22.22.2-bookworm-slim`, or a newer `22.x` patch tag that exists. Say so in the report and in the Task 1 commit message.
3. If no 22.x tag of at least 22.22.2 exists, stop and ask the user. Moving to Node 24 changes the runtime the image ships, and that is the user's decision.

- [ ] **Step 6: Build the image in the background.** The build takes several minutes (apt, Go download, gopls compile, npm). Start it with the Bash tool's `run_in_background` option:

`cd /c/Users/Andi/Documents/repo/AuraGo && { docker build -f deploy/docker/Dockerfile.code-studio -t aurago-code-studio:lsp-dev . ; echo "exit=$?"; } > C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/lsp-image-build.log 2>&1`

Poll about once a minute until the last line is `exit=<n>`:
`tail -n 8 C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/lsp-image-build.log`

Expected: the log ends with `naming to docker.io/library/aurago-code-studio:lsp-dev` (BuildKit) and then `exit=0`. Earlier in the log, the gopls layer prints `golang.org/x/tools/gopls v0.24.0`, and the npm layer prints `6.1.0` and `pyright 1.1.414`.

If the build fails, find the matching message and act:

| Message in the log | Action |
| --- | --- |
| `is older than 22.22.2` | Do Step 5's pin, then build again. |
| `requires go >= 1.27.N (running go 1.27.4; GOTOOLCHAIN=local)` | The spec's premise (gopls v0.24.0 builds with Go 1.27.4) is wrong. Stop and ask the user. Raising `GO_VERSION` is a decision, and it also requires updating `TestCodeStudioDockerfileIncludesRunButtonRuntimes`, which asserts `ARG GO_VERSION=1.27.4`. |
| `unsupported TARGETARCH:` | The legacy builder is active. Build again with `--build-arg TARGETARCH=amd64` added after `-t aurago-code-studio:lsp-dev`. |

- [ ] **Step 7: Verify the image.**

Run: `MSYS_NO_PATHCONV=1 docker run --rm --network none aurago-code-studio:lsp-dev sh -c 'node --version; gopls version; typescript-language-server --version; pyright --version; clangd --version | head -n 1; command -v gopls typescript-language-server pyright-langserver clangd'`

Expected, in this order (`gopls version` may add an indented build-info line after its first line):
```
v22.22.2            (or a newer v22 patch)
golang.org/x/tools/gopls v0.24.0
6.1.0
pyright 1.1.414
Debian clangd version 14.0.6
/usr/local/bin/gopls
/usr/local/bin/typescript-language-server
/usr/local/bin/pyright-langserver
/usr/bin/clangd
```

Run: `MSYS_NO_PATHCONV=1 docker run --rm --user root aurago-code-studio:lsp-dev sh -c 'for p in /tmp/gopls-build /root/go /root/.cache/go-build; do [ -e "$p" ] && echo "left: $p"; done; true'`
Expected: no output. The gopls caches are gone.

Run: `docker image inspect aurago-code-studio:lsp-dev --format '{{.Size}}'` and `docker history aurago-code-studio:lsp-dev --format '{{.Size}}\t{{.CreatedBy}}' | head -n 12`
Report:
- the total size
- the sizes of the apt layer (with clangd), the gopls layer and the npm LSP layer

If `ghcr.io/antibyte/aurago-code-studio:latest` exists locally, also report its size:
- Run `docker image inspect ghcr.io/antibyte/aurago-code-studio:latest --format '{{.Size}}'`.
- If it prints `No such image`, skip this. Do not pull it.

The expected growth is about 200 to 250 MB. Flag it in the report if the growth exceeds 400 MB.

- [ ] **Step 8: Graph check.** Run detect_changes (see Notes). Expected: no symbol changes on this plan's paths, because a Dockerfile has no symbols.

- [ ] **Step 9: Commit.** Write `C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/msg-2a-1.txt` with the Write tool:

```
feat(code-studio-image): language servers for code intelligence

Adds pinned gopls v0.24.0 (built with the image's Go 1.27.4, caches
removed), typescript-language-server 6.1.0 and pyright 1.1.414 (npm) and
Debian's clangd to the Code Studio image. The build stops when the base
image's Node is older than 22.22.2. Built and checked locally as
aurago-code-studio:lsp-dev; not published.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

If Step 5 pinned the base tag, add one sentence naming the tag before the `Co-Authored-By` line.

Run: `bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/msg-2a-1.txt deploy/docker/Dockerfile.code-studio`
Expected: `committed <sha> (parent <sha>)` and no status line for the path.

---

### Task 2: LSP framing, Docker demux and stderr tail

**Files:**
- Create: `internal/server/code_studio_lsp.go`
- Create: `internal/server/code_studio_lsp_test.go`

- [ ] **Step 1: Impact check.** This task adds only new symbols in a new file, so no existing function is edited and there is no impact run.

- [ ] **Step 2: Create `internal/server/code_studio_lsp.go`** with this complete content:

```go
package server

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Code Studio LSP bridge. GET /api/code-studio/lsp?lang=go|typescript|python|cpp
// upgrades to a WebSocket and pipes it to one language server, a non-TTY exec
// in the Code Studio container. The page only names a language; the command
// comes from a fixed table. Every text frame carries one JSON-RPC message in
// both directions: the bridge strips the LSP Content-Length headers from the
// server's stdout and adds them on the way to its stdin.

const (
	// codeStudioLSPSessionEnv tags every process of one session: the server and
	// the children that inherit its environment, so the cleanup finds them all.
	codeStudioLSPSessionEnv = "AURAGO_LSP_SESSION"

	// codeStudioLSPMaxSessions bounds the language servers of the process.
	codeStudioLSPMaxSessions = 8

	// codeStudioLSPHeaderLimit bounds the header block of one server message.
	codeStudioLSPHeaderLimit = 8 << 10
	// codeStudioLSPMessageLimit bounds the body of one server message.
	codeStudioLSPMessageLimit = 32 << 20
	// codeStudioLSPReadBuffer is larger than the header limit, so a header
	// line that fits the limit always fits the read buffer.
	codeStudioLSPReadBuffer = codeStudioLSPHeaderLimit + 1024
	// codeStudioLSPFrameSlack is added to the max file size for page frames:
	// a didOpen carries a whole file plus its JSON envelope.
	codeStudioLSPFrameSlack = 64 << 10
	// codeStudioLSPStderrTail is how much stderr the debug log keeps.
	codeStudioLSPStderrTail = 2 << 10

	// codeStudioLSPExitWait is how long the cleanup waits for the server to
	// exit after its stdin closed.
	codeStudioLSPExitWait = 2 * time.Second
	// codeStudioLSPCheckTimeout bounds the `command -v` probe.
	codeStudioLSPCheckTimeout = 15 * time.Second
	// codeStudioLSPCloseWriteWait bounds the write of one close frame.
	codeStudioLSPCloseWriteWait = time.Second

	// Close codes of the browser contract (plan 2b). A server process that
	// exits closes with websocket.CloseInternalServerErr (1011).
	codeStudioLSPCloseNotInstalled = 4404
	codeStudioLSPCloseTooMany      = 4429
	codeStudioLSPCloseStartFailed  = 4500
)

// readCodeStudioLSPMessage reads one base-protocol message: header lines up
// to an empty line (at most codeStudioLSPHeaderLimit bytes in total), then
// exactly Content-Length body bytes (at most codeStudioLSPMessageLimit). It
// returns io.EOF only when the stream ends between two messages. r must
// buffer at least codeStudioLSPReadBuffer bytes.
func readCodeStudioLSPMessage(r *bufio.Reader) ([]byte, error) {
	length := -1
	headerBytes := 0
	for {
		line, err := r.ReadSlice('\n')
		headerBytes += len(line)
		if headerBytes > codeStudioLSPHeaderLimit || errors.Is(err, bufio.ErrBufferFull) {
			return nil, fmt.Errorf("language server header exceeds %d bytes", codeStudioLSPHeaderLimit)
		}
		if err != nil {
			if errors.Is(err, io.EOF) && headerBytes > 0 {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}
		text := strings.TrimRight(string(line), "\r\n")
		if text == "" {
			break
		}
		name, value, found := strings.Cut(text, ":")
		if !found {
			return nil, fmt.Errorf("malformed language server header %q", text)
		}
		if !strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("invalid language server Content-Length %q", strings.TrimSpace(value))
		}
		if n > codeStudioLSPMessageLimit {
			return nil, fmt.Errorf("language server message exceeds %d bytes", codeStudioLSPMessageLimit)
		}
		length = n
	}
	if length < 0 {
		return nil, errors.New("language server message has no Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, codeStudioLSPUnexpectedEOF(err)
	}
	return body, nil
}

// writeCodeStudioLSPMessage writes body with its Content-Length header in one write.
func writeCodeStudioLSPMessage(w io.Writer, body []byte) error {
	frame := make([]byte, 0, len(body)+32)
	frame = append(frame, "Content-Length: "...)
	frame = strconv.AppendInt(frame, int64(len(body)), 10)
	frame = append(frame, "\r\n\r\n"...)
	frame = append(frame, body...)
	_, err := w.Write(frame)
	return err
}

// codeStudioLSPUnexpectedEOF reports an end of stream inside a message as
// io.ErrUnexpectedEOF.
func codeStudioLSPUnexpectedEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return err
}

// codeStudioLSPDemux turns a non-TTY Docker attach stream (8-byte frame
// headers: stream type, three zero bytes, big-endian payload size) into the
// server's stdout. Payloads of any other stream go to stderr.
type codeStudioLSPDemux struct {
	src       io.Reader
	stderr    io.Writer
	header    [8]byte
	remaining uint32
}

func (d *codeStudioLSPDemux) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for d.remaining == 0 {
		if _, err := io.ReadFull(d.src, d.header[:]); err != nil {
			return 0, err
		}
		size := binary.BigEndian.Uint32(d.header[4:8])
		if d.header[0] == 1 {
			d.remaining = size
			continue
		}
		dst := d.stderr
		if dst == nil {
			dst = io.Discard
		}
		if _, err := io.CopyN(dst, d.src, int64(size)); err != nil {
			return 0, codeStudioLSPUnexpectedEOF(err)
		}
	}
	if uint32(len(p)) > d.remaining {
		p = p[:d.remaining]
	}
	n, err := d.src.Read(p)
	d.remaining -= uint32(n)
	if errors.Is(err, io.EOF) && d.remaining > 0 {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}

// codeStudioLSPTail keeps the last codeStudioLSPStderrTail bytes written to
// it. The bridge discards stderr and logs only this tail at debug level.
type codeStudioLSPTail struct {
	mu  sync.Mutex
	buf []byte
}

func (t *codeStudioLSPTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if over := len(t.buf) - codeStudioLSPStderrTail; over > 0 {
		t.buf = append(t.buf[:0], t.buf[over:]...)
	}
	return len(p), nil
}

func (t *codeStudioLSPTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.ToValidUTF8(string(t.buf), "")
}
```

- [ ] **Step 3: Create `internal/server/code_studio_lsp_test.go`** with this complete content:

```go
package server

import (
	"bufio"
	"fmt"
	"strings"
	"testing"
)

func TestCodeStudioLSPRejectsOversizeMessages(t *testing.T) {
	read := func(raw string) ([]byte, error) {
		return readCodeStudioLSPMessage(bufio.NewReaderSize(strings.NewReader(raw), codeStudioLSPReadBuffer))
	}
	if body, err := read("Content-Length: 2\r\n\r\n{}"); err != nil || string(body) != "{}" {
		t.Fatalf("plain message = %q, %v", body, err)
	}
	// A header block over 8 KiB is refused, even when it ends properly.
	header := "X-Padding: " + strings.Repeat("a", codeStudioLSPHeaderLimit) + "\r\nContent-Length: 2\r\n\r\n{}"
	if _, err := read(header); err == nil || !strings.Contains(err.Error(), "header exceeds") {
		t.Fatalf("oversize header: err = %v", err)
	}
	// A body over 32 MiB is refused from its Content-Length, before any body byte is read.
	body := fmt.Sprintf("Content-Length: %d\r\n\r\n{}", codeStudioLSPMessageLimit+1)
	if _, err := read(body); err == nil || !strings.Contains(err.Error(), "message exceeds") {
		t.Fatalf("oversize body: err = %v", err)
	}
}
```

- [ ] **Step 4: Format.**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && gofmt -l internal/server/code_studio_lsp.go internal/server/code_studio_lsp_test.go`
Expected: no output. If a file is listed, run `gofmt -w` on that file only.

- [ ] **Step 5: Test.**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && mkdir -p /c/tmp/go-build-lsp && GOTOOLCHAIN=go1.27.4 GOTMPDIR='C:\tmp\go-build-lsp' go test ./internal/server -run '(?i)(CodeStudioLSP)' -count=1 -v 2>&1 | grep -E -- '--- (PASS|FAIL|SKIP)|^(ok|FAIL|panic)'`
Expected:
```
--- PASS: TestCodeStudioLSPRejectsOversizeMessages (0.00s)
ok  	aurago/internal/server	<time>
```

- [ ] **Step 6: Graph check.** Run detect_changes. Expected: only new symbols in `code_studio_lsp.go` and `code_studio_lsp_test.go`, and no HIGH or CRITICAL risk on existing symbols.

- [ ] **Step 7: Commit.** Write `C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/msg-2a-2.txt`:

```
feat(code-studio-api): LSP framing and Docker stream demux

Content-Length reader and writer for the Code Studio LSP bridge (header
block at most 8 KiB, body at most 32 MiB), a demultiplexer for non-TTY
Docker attach streams and a capped stderr tail for debug logs.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

Run: `bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/msg-2a-2.txt internal/server/code_studio_lsp.go internal/server/code_studio_lsp_test.go`
Expected: `committed <sha> (parent <sha>)` and no status lines.

---

### Task 3: Command table, session cap and the Docker runner

**Files:**
- Modify: `internal/server/code_studio_lsp.go` (import block and append)

- [ ] **Step 1: Impact check.** This task edits no existing function. It adds two new methods (`StartLSP`, `EndLSP`) to the existing type `codeStudioDockerAdapter`, which cannot change the behaviour of its callers. It only calls the existing helpers `openCodeStudioExecStdinStream`, `bufferedDockerRawConn` and `tools.DockerRequestContext`, without changing them.

- [ ] **Step 2: Replace the import block** of `internal/server/code_studio_lsp.go` with:

```go
import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/tools"
)
```

- [ ] **Step 3: Append to the end of `internal/server/code_studio_lsp.go`:**

```go
// codeStudioLSPServer is one row of the fixed command table.
type codeStudioLSPServer struct {
	Lang    string
	Binary  string
	Command []string
}

// codeStudioLSPServerFor maps the page's lang parameter to a command. The
// page can only name a language; it never chooses the command.
func codeStudioLSPServerFor(lang string) (codeStudioLSPServer, bool) {
	switch lang {
	case "go":
		return codeStudioLSPServer{Lang: lang, Binary: "gopls", Command: []string{"gopls"}}, true
	case "typescript":
		return codeStudioLSPServer{Lang: lang, Binary: "typescript-language-server", Command: []string{"typescript-language-server", "--stdio"}}, true
	case "python":
		return codeStudioLSPServer{Lang: lang, Binary: "pyright-langserver", Command: []string{"pyright-langserver", "--stdio"}}, true
	case "cpp":
		// No background index: clangd never writes .cache/clangd into the project.
		return codeStudioLSPServer{Lang: lang, Binary: "clangd", Command: []string{"clangd", "--background-index=false"}}, true
	}
	return codeStudioLSPServer{}, false
}

// codeStudioLSPSessionCounter bounds the LSP sessions of the process. One
// AuraGo process serves one Server, so a package-level counter is server-wide
// and needs no new Server field.
type codeStudioLSPSessionCounter struct {
	mu     sync.Mutex
	active int
}

var codeStudioLSPSessions codeStudioLSPSessionCounter

// acquire takes a session slot. It returns false when all slots are taken;
// the release function is safe to call more than once.
func (c *codeStudioLSPSessionCounter) acquire() (func(), bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active >= codeStudioLSPMaxSessions {
		return nil, false
	}
	c.active++
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			c.active--
			c.mu.Unlock()
		})
	}, true
}

func (c *codeStudioLSPSessionCounter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.active
}

// codeStudioLSPStream is the attach stream of one language-server exec:
// reads return the multiplexed stdout and stderr frames, writes go to stdin,
// and CloseWrite closes stdin only.
type codeStudioLSPStream interface {
	io.ReadWriteCloser
	CloseWrite() error
}

// codeStudioLSPRunner is the streaming-exec seam of the bridge. The Docker
// adapter implements it; tests use an in-memory fake.
type codeStudioLSPRunner interface {
	// StartLSP starts cmd as a non-TTY exec with stdin, stdout and stderr
	// attached and the session tag nonce in its environment.
	StartLSP(ctx context.Context, containerID string, cmd []string, nonce string) (codeStudioLSPStream, error)
	// EndLSP ends every process of the container that carries the tag nonce.
	EndLSP(ctx context.Context, containerID, nonce string) error
}

var (
	_ codeStudioLSPRunner = codeStudioDockerAdapter{}
	_ codeStudioLSPStream = codeStudioLSPDockerStream{}
)

// codeStudioLSPExecPayload is the exec create body of one language server.
// GOTOOLCHAIN=local keeps gopls from fetching a toolchain while the network
// is off.
func codeStudioLSPExecPayload(cmd []string, nonce string) map[string]interface{} {
	return map[string]interface{}{
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          cmd,
		"Env":          []string{codeStudioLSPSessionEnv + "=" + nonce, "GOTOOLCHAIN=local"},
		"Tty":          false,
		"User":         "developer",
		"WorkingDir":   codeStudioWorkspaceRoot,
	}
}

// StartLSP creates the exec and attaches its raw stream. The stream outlives
// ctx; the bridge closes it.
func (a codeStudioDockerAdapter) StartLSP(ctx context.Context, containerID string, cmd []string, nonce string) (codeStudioLSPStream, error) {
	body, err := json.Marshal(codeStudioLSPExecPayload(cmd, nonce))
	if err != nil {
		return nil, fmt.Errorf("encode language server exec: %w", err)
	}
	data, code, err := tools.DockerRequestContext(ctx, a.cfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/exec", string(body))
	if err != nil {
		return nil, err
	}
	if code != http.StatusCreated {
		return nil, fmt.Errorf("docker language server exec create failed: %s", strings.TrimSpace(string(data)))
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &created); err != nil || created.ID == "" {
		return nil, fmt.Errorf("parse docker language server exec id: %v", err)
	}
	stream, err := openCodeStudioExecStdinStream(ctx, a.cfg, created.ID)
	if err != nil {
		return nil, err
	}
	return codeStudioLSPDockerStream{stream}, nil
}

// codeStudioLSPDockerStream adds CloseWrite to the hijacked exec connection.
// openCodeStudioExecStdinStream already refuses connections that cannot
// half-close.
type codeStudioLSPDockerStream struct{ *bufferedDockerRawConn }

func (s codeStudioLSPDockerStream) CloseWrite() error {
	if closer, ok := s.Conn.(interface{ CloseWrite() error }); ok {
		return closer.CloseWrite()
	}
	return errors.New("docker connection cannot close stdin")
}

// codeStudioLSPEndScript ends every process whose environment carries the
// session tag passed as $1: the language server and the children it started
// (tsserver, go list, ...). TERM first, KILL one second later for what is
// left. The script's own processes carry no tag. It runs as developer, the
// user of the servers, and needs tr and grep like containerTerminalEndScript.
const codeStudioLSPEndScript = `tag="` + codeStudioLSPSessionEnv + `=$1"
end_tagged() {
  for d in /proc/[0-9]*; do
    tr '\000' '\n' 2>/dev/null < "$d/environ" | grep -qxF "$tag" || continue
    kill -s "$1" "${d#/proc/}" 2>/dev/null
  done
}
end_tagged TERM
sleep 1
end_tagged KILL`

// EndLSP runs codeStudioLSPEndScript as a detached helper exec.
func (a codeStudioDockerAdapter) EndLSP(ctx context.Context, containerID, nonce string) error {
	if containerID == "" || nonce == "" {
		return errors.New("language server session has no end tag")
	}
	body, err := json.Marshal(map[string]interface{}{
		"AttachStdout": false,
		"AttachStderr": false,
		"Cmd":          []string{"/bin/sh", "-c", codeStudioLSPEndScript, "aurago-end-lsp", nonce},
		"Tty":          false,
		"User":         "developer",
	})
	if err != nil {
		return fmt.Errorf("encode language server end exec: %w", err)
	}
	data, code, err := tools.DockerRequestContext(ctx, a.cfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/exec", string(body))
	if err != nil {
		return err
	}
	if code != http.StatusCreated {
		return fmt.Errorf("docker language server end failed: %s", strings.TrimSpace(string(data)))
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &created); err != nil || created.ID == "" {
		return fmt.Errorf("parse docker language server end exec id: %v", err)
	}
	data, code, err = tools.DockerRequestContext(ctx, a.cfg, http.MethodPost, "/exec/"+url.PathEscape(created.ID)+"/start", `{"Detach":true,"Tty":false}`)
	if err != nil {
		return err
	}
	if code != http.StatusOK && code != http.StatusCreated && code != http.StatusNoContent {
		return fmt.Errorf("docker language server end start failed: %s", strings.TrimSpace(string(data)))
	}
	return nil
}
```

- [ ] **Step 4: Format.**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && gofmt -l internal/server/code_studio_lsp.go`
Expected: no output.

- [ ] **Step 5: Compile and test.** The new code compiles with the package's test binary.

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && mkdir -p /c/tmp/go-build-lsp && GOTOOLCHAIN=go1.27.4 GOTMPDIR='C:\tmp\go-build-lsp' go test ./internal/server -run '(?i)(CodeStudioLSP)' -count=1 -v 2>&1 | grep -E -- '--- (PASS|FAIL|SKIP)|^(ok|FAIL|panic)'`
Expected: `--- PASS: TestCodeStudioLSPRejectsOversizeMessages`, then `ok  	aurago/internal/server`.

- [ ] **Step 6: Graph check.** Run detect_changes. Expected: new symbols only (`codeStudioLSPServerFor`, `codeStudioLSPSessionCounter`, `StartLSP`, `EndLSP`, …), and no HIGH or CRITICAL risk.

- [ ] **Step 7: Commit.** Write `C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/msg-2a-3.txt`:

```
feat(code-studio-api): LSP command table, session cap and Docker runner

Fixed lang-to-command table, a process-wide cap of 8 sessions, and the
streaming-exec seam: the Docker adapter starts a language server as a
non-TTY exec (developer, /workspace, AURAGO_LSP_SESSION tag,
GOTOOLCHAIN=local) and ends tagged processes through a /proc scan (TERM,
then KILL).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

Run: `bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/msg-2a-3.txt internal/server/code_studio_lsp.go`
Expected: `committed <sha> (parent <sha>)` and no status line.

---

### Task 4: The WebSocket bridge, the route and the unit tests

**Files:**
- Modify: `internal/server/code_studio_lsp.go` (import block and append)
- Modify: `internal/server/code_studio_handlers.go` (`registerCodeStudioRoutes`, one line)
- Modify: `internal/server/code_studio_lsp_test.go` (import block and append)

- [ ] **Step 1: Impact check.** Run impact upstream on `registerCodeStudioRoutes`:
`cd /c/Users/Andi/Documents/repo/AuraGo && node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "registerCodeStudioRoutes" --direction upstream --repo .`

- A text search for this plan found one caller: `internal/server/server_routes.go` (`registerCodeStudioRoutes(mux, s)`).
- Report the callers, processes and risk.
- If the risk is HIGH or CRITICAL, warn before editing. The change only adds a route.
- If the result is `UNKNOWN`, confirm the single caller with `grep -rn "registerCodeStudioRoutes(" internal/server --include=*.go`.

- [ ] **Step 2: Replace the import block** of `internal/server/code_studio_lsp.go` with:

```go
import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)
```

- [ ] **Step 3: Append to the end of `internal/server/code_studio_lsp.go`:**

```go
// handleLSP is GET /api/code-studio/lsp?lang=go|typescript|python|cpp, a
// WebSocket to one language server in the Code Studio container. The gate
// chain is handleTerminal's, in the same order. Step 1 names the execute
// operation explicitly: requirePermission derives the operation from the
// path, and only paths that end in /terminal count as execute there, so a
// plain requirePermission would let a live session survive a switch to
// read-only.
func (h codeStudioHandlers) handleLSP(w http.ResponseWriter, r *http.Request) {
	if !requireDesktopOperation(h.server, w, r, desktopScopeWrite, desktopExecute) {
		return
	}
	if h.rejectReadOnly(w) {
		return
	}
	done, admitted := h.server.admitDockerTerminal(w, r)
	if !admitted {
		return
	}
	defer done()
	if !h.isAuthenticated(r) {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !desktop.SameHostWebSocketOrigin(r) {
		jsonError(w, "forbidden websocket origin", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lsp, ok := codeStudioLSPServerFor(r.URL.Query().Get("lang"))
	if !ok {
		jsonError(w, "unknown language server", http.StatusBadRequest)
		return
	}
	container, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	release, err := container.HoldTerminal(containerID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer release()
	conn, err := codeStudioWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	h.serveCodeStudioLSP(r.Context(), conn, containerID, lsp)
}

// serveCodeStudioLSP runs one language-server session on an upgraded socket:
// session slot, install check, start, the two pipes, then the cleanup.
func (h codeStudioHandlers) serveCodeStudioLSP(ctx context.Context, conn *websocket.Conn, containerID string, lsp codeStudioLSPServer) {
	logger := h.codeStudioLogger()
	release, ok := codeStudioLSPSessions.acquire()
	if !ok {
		closeCodeStudioLSP(conn, codeStudioLSPCloseTooMany, "too many language servers")
		return
	}
	defer release()
	runner, ok := h.docker.(codeStudioLSPRunner)
	if !ok {
		closeCodeStudioLSP(conn, codeStudioLSPCloseStartFailed, "language server runner is unavailable")
		return
	}
	installed, err := h.codeStudioLSPInstalled(ctx, containerID, lsp.Binary)
	if err != nil {
		logger.Warn("code studio language server check failed", "lang", lsp.Lang, "error", err)
		closeCodeStudioLSP(conn, codeStudioLSPCloseStartFailed, "language server check failed")
		return
	}
	if !installed {
		closeCodeStudioLSP(conn, codeStudioLSPCloseNotInstalled, lsp.Binary+" is not installed")
		return
	}
	nonce, err := newContainerTerminalNonce()
	if err != nil {
		closeCodeStudioLSP(conn, codeStudioLSPCloseStartFailed, "language server start failed")
		return
	}
	stream, err := runner.StartLSP(ctx, containerID, lsp.Command, nonce)
	if err != nil {
		logger.Warn("code studio language server start failed", "lang", lsp.Lang, "error", err)
		closeCodeStudioLSP(conn, codeStudioLSPCloseStartFailed, "language server start failed")
		// A start request that failed late may still have started the process.
		h.endCodeStudioLSP(runner, containerID, nonce, lsp.Lang)
		return
	}
	tail := &codeStudioLSPTail{}
	serverDone := make(chan struct{})
	h.pipeCodeStudioLSP(ctx, conn, stream, tail, serverDone)
	// The page is gone: close stdin, give the server time to exit, then end
	// whatever still carries the session tag.
	_ = stream.CloseWrite()
	select {
	case <-serverDone:
	case <-time.After(codeStudioLSPExitWait):
	}
	_ = stream.Close()
	<-serverDone
	h.endCodeStudioLSP(runner, containerID, nonce, lsp.Lang)
	logger.Debug("code studio language server ended", "lang", lsp.Lang, "stderr_tail", tail.String())
}

// pipeCodeStudioLSP copies server messages to the page and page frames to the
// server until the page leaves, the server exits or ctx ends. The server side
// closes serverDone when it stops; the function returns when the page side
// stops, with the socket closed.
func (h codeStudioHandlers) pipeCodeStudioLSP(ctx context.Context, conn *websocket.Conn, stream codeStudioLSPStream, tail *codeStudioLSPTail, serverDone chan<- struct{}) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	conn.SetReadLimit(h.maxFileSizeBytes() + codeStudioLSPFrameSlack)
	pongWait := dockerManagerTerminalServeOptions.PongWait
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(pongWait)) })
	go func() {
		ticker := time.NewTicker(dockerManagerTerminalServeOptions.PingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(containerTerminalPingWriteWait)); err != nil {
					cancel()
					return
				}
			}
		}
	}()

	var pageDone atomic.Bool
	go func() {
		defer close(serverDone)
		reader := bufio.NewReaderSize(&codeStudioLSPDemux{src: stream, stderr: tail}, codeStudioLSPReadBuffer)
		for {
			body, err := readCodeStudioLSPMessage(reader)
			if err != nil {
				if !pageDone.Load() {
					reason := "language server exited"
					if !errors.Is(err, io.EOF) {
						reason = "language server stream failed"
					}
					closeCodeStudioLSP(conn, websocket.CloseInternalServerErr, reason)
				}
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, body); err != nil {
				_ = conn.Close()
				return
			}
		}
	}()

	for {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if messageType != websocket.TextMessage {
			closeCodeStudioLSP(conn, websocket.CloseUnsupportedData, "text frames only")
			break
		}
		if !json.Valid(payload) {
			closeCodeStudioLSP(conn, websocket.CloseInvalidFramePayloadData, "invalid JSON")
			break
		}
		if deadliner, ok := stream.(containerTerminalWriteDeadliner); ok {
			_ = deadliner.SetWriteDeadline(time.Now().Add(containerTerminalWriteTimeout))
		}
		if err := writeCodeStudioLSPMessage(stream, payload); err != nil {
			closeCodeStudioLSP(conn, websocket.CloseInternalServerErr, "language server input failed")
			break
		}
	}
	pageDone.Store(true)
	_ = conn.Close()
}

// codeStudioLSPInstalled runs `command -v` for the server binary with the
// image PATH. Exit 0 means installed; another exit code means missing.
func (h codeStudioHandlers) codeStudioLSPInstalled(ctx context.Context, containerID, name string) (bool, error) {
	result, err := h.docker.Exec(ctx, containerID, []string{"sh", "-c", `command -v "$1" >/dev/null`, "sh", name}, codeStudioLSPCheckTimeout)
	if err != nil {
		return false, err
	}
	if result.ExitCode < 0 {
		return false, errors.New("the exit code of the language server probe is unknown")
	}
	return result.ExitCode == 0, nil
}

// endCodeStudioLSP ends the tagged processes. It uses its own context: the
// request context may already be canceled when the session ends.
func (h codeStudioHandlers) endCodeStudioLSP(runner codeStudioLSPRunner, containerID, nonce, lang string) {
	ctx, cancel := context.WithTimeout(context.Background(), containerTerminalEndTimeout)
	defer cancel()
	if err := runner.EndLSP(ctx, containerID, nonce); err != nil {
		h.codeStudioLogger().Warn("code studio language server cleanup failed", "lang", lang, "error", err)
	}
}

// closeCodeStudioLSP sends a close frame with code and reason, then closes the socket.
func closeCodeStudioLSP(conn *websocket.Conn, code int, reason string) {
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(codeStudioLSPCloseWriteWait))
	_ = conn.Close()
}
```

- [ ] **Step 4: Register the route.** In `internal/server/code_studio_handlers.go`, inside `registerCodeStudioRoutes`, insert this line directly after the line `	mux.HandleFunc("/api/code-studio/terminal", handlers.handleTerminal)`:

```go
	mux.HandleFunc("/api/code-studio/lsp", handlers.handleLSP)
```

- [ ] **Step 5: Replace the import block** of `internal/server/code_studio_lsp_test.go` with:

```go
import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)
```

- [ ] **Step 6: Append to the end of `internal/server/code_studio_lsp_test.go`:**

```go
// lspTestStream is an in-memory language-server exec: the bridge reads
// Docker-framed output from out and writes stdin into in.
type lspTestStream struct {
	outR *io.PipeReader
	outW *io.PipeWriter
	inR  *io.PipeReader
	inW  *io.PipeWriter

	mu        sync.Mutex
	stdinShut bool
	closed    bool
}

func newLSPTestStream() *lspTestStream {
	outR, outW := io.Pipe()
	inR, inW := io.Pipe()
	return &lspTestStream{outR: outR, outW: outW, inR: inR, inW: inW}
}

func (s *lspTestStream) Read(p []byte) (int, error)  { return s.outR.Read(p) }
func (s *lspTestStream) Write(p []byte) (int, error) { return s.inW.Write(p) }

// CloseWrite closes stdin. Like a real language server, the fake then exits:
// its output ends.
func (s *lspTestStream) CloseWrite() error {
	s.mu.Lock()
	s.stdinShut = true
	s.mu.Unlock()
	_ = s.inW.Close()
	return s.outW.Close()
}

func (s *lspTestStream) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	_ = s.inW.Close()
	return s.outR.Close()
}

// send writes one Docker attach frame (1 stdout, 2 stderr) to the bridge.
func (s *lspTestStream) send(t *testing.T, stream byte, payload string) {
	t.Helper()
	var frame bytes.Buffer
	writeAttachFrame(t, &frame, stream, []byte(payload))
	if _, err := s.outW.Write(frame.Bytes()); err != nil {
		t.Fatalf("write language server output: %v", err)
	}
}

type lspTestStart struct {
	cmd   []string
	nonce string
}

// lspTestDocker fakes the Docker side of the bridge: the `command -v` probe
// (Exec), the language-server exec (StartLSP) and the cleanup (EndLSP).
type lspTestDocker struct {
	fakeCodeStudioDockerAPI
	missing bool
	streams chan *lspTestStream

	mu     sync.Mutex
	probes [][]string
	starts []lspTestStart
	ends   []string
}

func newLSPTestDocker() *lspTestDocker {
	return &lspTestDocker{streams: make(chan *lspTestStream, 4)}
}

func (d *lspTestDocker) Exec(_ context.Context, _ string, cmd []string, _ time.Duration) (codeStudioExecResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.probes = append(d.probes, append([]string(nil), cmd...))
	if d.missing {
		return codeStudioExecResult{ExitCode: 127}, nil
	}
	return codeStudioExecResult{}, nil
}

func (d *lspTestDocker) StartLSP(_ context.Context, _ string, cmd []string, nonce string) (codeStudioLSPStream, error) {
	stream := newLSPTestStream()
	d.mu.Lock()
	d.starts = append(d.starts, lspTestStart{cmd: append([]string(nil), cmd...), nonce: nonce})
	d.mu.Unlock()
	d.streams <- stream
	return stream, nil
}

func (d *lspTestDocker) EndLSP(_ context.Context, _ string, nonce string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ends = append(d.ends, nonce)
	return nil
}

func (d *lspTestDocker) snapshot() ([][]string, []lspTestStart, []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([][]string(nil), d.probes...), append([]lspTestStart(nil), d.starts...), append([]string(nil), d.ends...)
}

// dialCodeStudioLSP opens the bridge through the full handler and gate chain.
func dialCodeStudioLSP(t *testing.T, docker *lspTestDocker, lang string) *websocket.Conn {
	t.Helper()
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	h := codeStudioHandlers{server: s, docker: docker}
	srv := httptest.NewServer(http.HandlerFunc(h.handleLSP))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/api/code-studio/lsp?lang="+lang, http.Header{"Origin": {srv.URL}})
	if err != nil {
		t.Fatalf("dial lsp %s: %v", lang, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func waitCodeStudioLSPStream(t *testing.T, docker *lspTestDocker) *lspTestStream {
	t.Helper()
	select {
	case stream := <-docker.streams:
		return stream
	case <-time.After(5 * time.Second):
		t.Fatal("the language server was not started")
		return nil
	}
}

func expectCodeStudioLSPClose(t *testing.T, conn *websocket.Conn, code int) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, data, err := conn.ReadMessage()
		if err == nil {
			t.Logf("frame before close: %q", data)
			continue
		}
		var closeErr *websocket.CloseError
		if !errors.As(err, &closeErr) || closeErr.Code != code {
			t.Fatalf("close = %v, want code %d", err, code)
		}
		return
	}
}

// codeStudioLSPReadStdin reads n bytes of the fake server's stdin, or all of
// it up to EOF when n < 0, and fails after five seconds.
func codeStudioLSPReadStdin(t *testing.T, stream *lspTestStream, n int) string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		if n < 0 {
			data, _ := io.ReadAll(stream.inR)
			done <- string(data)
			return
		}
		data := make([]byte, n)
		read, _ := io.ReadFull(stream.inR, data)
		done <- string(data[:read])
	}()
	select {
	case data := <-done:
		return data
	case <-time.After(5 * time.Second):
		t.Fatal("timed out reading the language server stdin")
		return ""
	}
}

func waitCodeStudioLSP(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out: %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func codeStudioLSPUpgradeRequest(lang string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/code-studio/lsp?lang="+lang, nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-Websocket-Version", "13")
	req.Header.Set("Sec-Websocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	return req
}

func TestCodeStudioLSPFramesMessagesBothWays(t *testing.T) {
	docker := newLSPTestDocker()
	conn := dialCodeStudioLSP(t, docker, "go")
	stream := waitCodeStudioLSPStream(t, docker)

	// Server to page: one header split over two stdout frames, a stderr frame in
	// between, and two messages in one frame. Each body becomes one text frame.
	first := `{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"hoverProvider":true}}}`
	second := `{"jsonrpc":"2.0","method":"window/logMessage","params":{"type":3,"message":"ready"}}`
	stream.send(t, 1, "Content-Le")
	stream.send(t, 2, "gopls: loading packages\n")
	stream.send(t, 1, fmt.Sprintf("ngth: %d\r\n\r\n%sContent-Length: %d\r\nContent-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n%s", len(first), first, len(second), second))
	for _, want := range []string{first, second} {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		kind, data, err := conn.ReadMessage()
		if err != nil || kind != websocket.TextMessage || string(data) != want {
			t.Fatalf("frame = %d %q, %v; want text %q", kind, data, err, want)
		}
	}

	// Page to server: one text frame becomes one Content-Length message on stdin.
	request := `{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":"file:///workspace/main.go"},"position":{"line":0,"character":0}}}`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(request)); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(request), request)
	if got := codeStudioLSPReadStdin(t, stream, len(want)); got != want {
		t.Fatalf("stdin = %q, want %q", got, want)
	}
}

func TestCodeStudioLSPRejectsInvalidJSONFrame(t *testing.T) {
	docker := newLSPTestDocker()
	conn := dialCodeStudioLSP(t, docker, "go")
	stream := waitCodeStudioLSPStream(t, docker)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"jsonrpc":"2.0","id":1,`)); err != nil {
		t.Fatal(err)
	}
	expectCodeStudioLSPClose(t, conn, websocket.CloseInvalidFramePayloadData)
	if got := codeStudioLSPReadStdin(t, stream, -1); got != "" {
		t.Fatalf("invalid frame reached stdin: %q", got)
	}
}

func TestCodeStudioLSPRejectsUnknownLanguageBeforeUpgrade(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := newLSPTestDocker()
	h := codeStudioHandlers{server: s, docker: docker}
	for _, lang := range []string{"", "rust", "GO", "gopls"} {
		rec := httptest.NewRecorder()
		h.handleLSP(rec, codeStudioLSPUpgradeRequest(lang))
		if rec.Code != http.StatusBadRequest || rec.Header().Get("Upgrade") != "" {
			t.Fatalf("lang %q: status %d upgrade %q; want 400 before the upgrade", lang, rec.Code, rec.Header().Get("Upgrade"))
		}
	}
	if probes, starts, _ := docker.snapshot(); len(probes)+len(starts) != 0 {
		t.Fatalf("an unknown language reached docker: probes=%v starts=%v", probes, starts)
	}
}

func TestCodeStudioLSPRejectsReadOnlyDesktop(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	s.CfgMu.Lock()
	s.Cfg.VirtualDesktop.ReadOnly = true
	s.CfgMu.Unlock()
	docker := newLSPTestDocker()
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleLSP(rec, codeStudioLSPUpgradeRequest("go"))
	if rec.Code != http.StatusForbidden || rec.Header().Get("Upgrade") != "" {
		t.Fatalf("read-only status %d upgrade %q; want 403 before the upgrade", rec.Code, rec.Header().Get("Upgrade"))
	}
	if probes, starts, _ := docker.snapshot(); len(probes)+len(starts) != 0 {
		t.Fatalf("a read-only desktop reached docker: probes=%v starts=%v", probes, starts)
	}
}

func TestCodeStudioLSPMissingBinaryCloses4404(t *testing.T) {
	docker := newLSPTestDocker()
	docker.missing = true
	conn := dialCodeStudioLSP(t, docker, "typescript")
	expectCodeStudioLSPClose(t, conn, codeStudioLSPCloseNotInstalled)
	probes, starts, _ := docker.snapshot()
	want := []string{"sh", "-c", `command -v "$1" >/dev/null`, "sh", "typescript-language-server"}
	if len(probes) != 1 || !slices.Equal(probes[0], want) {
		t.Fatalf("probes = %q, want %q", probes, want)
	}
	if len(starts) != 0 {
		t.Fatalf("a missing server was started: %v", starts)
	}
}

func TestCodeStudioLSPSessionCapCloses4429(t *testing.T) {
	if codeStudioLSPMaxSessions != 8 {
		t.Fatalf("session cap = %d, want 8", codeStudioLSPMaxSessions)
	}
	waitCodeStudioLSP(t, "no LSP session left from an earlier test", func() bool { return codeStudioLSPSessions.count() == 0 })
	for i := 0; i < codeStudioLSPMaxSessions; i++ {
		release, ok := codeStudioLSPSessions.acquire()
		if !ok {
			t.Fatalf("slot %d refused", i+1)
		}
		t.Cleanup(release)
	}
	docker := newLSPTestDocker()
	conn := dialCodeStudioLSP(t, docker, "python")
	expectCodeStudioLSPClose(t, conn, codeStudioLSPCloseTooMany)
	if probes, starts, _ := docker.snapshot(); len(probes)+len(starts) != 0 {
		t.Fatalf("a refused session reached docker: probes=%v starts=%v", probes, starts)
	}
}

func TestCodeStudioLSPCleansUpOnClose(t *testing.T) {
	docker := newLSPTestDocker()
	conn := dialCodeStudioLSP(t, docker, "go")
	stream := waitCodeStudioLSPStream(t, docker)
	_, starts, _ := docker.snapshot()
	if len(starts) != 1 || !slices.Equal(starts[0].cmd, []string{"gopls"}) || len(starts[0].nonce) != 32 {
		t.Fatalf("starts = %+v, want one gopls exec with a session tag", starts)
	}
	nonce := starts[0].nonce
	payload := codeStudioLSPExecPayload(starts[0].cmd, nonce)
	env, _ := payload["Env"].([]string)
	if payload["User"] != "developer" || payload["WorkingDir"] != codeStudioWorkspaceRoot || payload["Tty"] != false ||
		payload["AttachStdin"] != true || !slices.Contains(env, codeStudioLSPSessionEnv+"="+nonce) || !slices.Contains(env, "GOTOOLCHAIN=local") {
		t.Fatalf("exec payload = %v", payload)
	}

	// The tab goes away without a close frame.
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	waitCodeStudioLSP(t, "the cleanup ends the tagged processes", func() bool {
		_, _, ends := docker.snapshot()
		return len(ends) == 1
	})
	if _, _, ends := docker.snapshot(); ends[0] != nonce {
		t.Fatalf("cleanup tag = %q, want %q", ends[0], nonce)
	}
	stream.mu.Lock()
	stdinShut, closed := stream.stdinShut, stream.closed
	stream.mu.Unlock()
	if !stdinShut || !closed {
		t.Fatalf("stdin closed = %v, stream closed = %v; want both before the cleanup", stdinShut, closed)
	}
	waitCodeStudioLSP(t, "the session slot is released", func() bool { return codeStudioLSPSessions.count() == 0 })
}
```

- [ ] **Step 7: Format.**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && gofmt -l internal/server/code_studio_lsp.go internal/server/code_studio_lsp_test.go internal/server/code_studio_handlers.go`
Expected: no output.

- [ ] **Step 8: Test the bridge.**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && mkdir -p /c/tmp/go-build-lsp && GOTOOLCHAIN=go1.27.4 GOTMPDIR='C:\tmp\go-build-lsp' go test ./internal/server -run '(?i)(CodeStudioLSP)' -count=1 -v 2>&1 | grep -E -- '--- (PASS|FAIL|SKIP)|^(ok|FAIL|panic)'`
Expected: exactly these eight PASS lines (in any order), then `ok  	aurago/internal/server`:
```
--- PASS: TestCodeStudioLSPRejectsOversizeMessages
--- PASS: TestCodeStudioLSPFramesMessagesBothWays
--- PASS: TestCodeStudioLSPRejectsInvalidJSONFrame
--- PASS: TestCodeStudioLSPRejectsUnknownLanguageBeforeUpgrade
--- PASS: TestCodeStudioLSPRejectsReadOnlyDesktop
--- PASS: TestCodeStudioLSPMissingBinaryCloses4404
--- PASS: TestCodeStudioLSPSessionCapCloses4429
--- PASS: TestCodeStudioLSPCleansUpOnClose
```

- [ ] **Step 9: The terminal is unchanged.**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && GOTOOLCHAIN=go1.27.4 GOTMPDIR='C:\tmp\go-build-lsp' go test ./internal/server -run '(?i)(CodeStudioTerminal)' -count=1 -v 2>&1 | grep -E -- '--- (PASS|FAIL|SKIP)|^(ok|FAIL|panic)'`
Expected: every `TestCodeStudioTerminal...` test is `--- PASS`, there is no `FAIL`, and the output ends with `ok  	aurago/internal/server`.

- [ ] **Step 10: Graph check.** Run detect_changes. Expected:
- `registerCodeStudioRoutes` is modified.
- Everything else is new.

Report the risk, and call out HIGH or CRITICAL.

- [ ] **Step 11: Commit.** Write `C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/msg-2a-4.txt`:

```
feat(code-studio-api): WebSocket-to-stdio LSP bridge

GET /api/code-studio/lsp?lang=go|typescript|python|cpp with the terminal's
gate chain. One JSON-RPC message per text frame both ways; page frames are
checked with json.Valid and bounded by the max file size plus 64 KiB.
Close codes 4404 (not installed), 4429 (too many), 4500 (start failed),
1011 (server exited). On close: stdin closed, 2 s wait, tagged processes
ended.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

Run: `bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/msg-2a-4.txt internal/server/code_studio_lsp.go internal/server/code_studio_lsp_test.go internal/server/code_studio_handlers.go`
Expected: `committed <sha> (parent <sha>)` and no status lines.

The script commits the whole working-copy file `internal/server/code_studio_handlers.go`. Step 1 of Task 1 confirmed it was clean, but check again right before this commit:
- Run `git diff -- internal/server/code_studio_handlers.go`.
- Expected: only the one added route line.
- If another session has edited that file meanwhile, do not commit it. Stop and ask the user.

---

### Task 5: Opt-in check against the real image

**Files:**
- Create: `internal/server/code_studio_lsp_image_test.go`

- [ ] **Step 1: Impact check.** This is a new test file only, so there is no impact run.

- [ ] **Step 2: Create `internal/server/code_studio_lsp_image_test.go`** with this complete content:

```go
package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerutil"
	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

// codeStudioLSPSample is one small project and the hover its server must answer.
type codeStudioLSPSample struct {
	lang       string            // bridge lang parameter
	languageID string            // LSP languageId of the opened file
	path       string            // the file that is opened and hovered
	needle     string            // hover position: inside its last occurrence
	want       string            // text the hover result must contain
	files      map[string]string // container path -> content
}

var codeStudioLSPSamples = []codeStudioLSPSample{
	{
		lang: "go", languageID: "go", path: "/workspace/golsp/main.go", needle: "Greeting(", want: "Greeting",
		files: map[string]string{
			"/workspace/golsp/go.mod":  "module example.com/golsp\n\ngo 1.27\n",
			"/workspace/golsp/main.go": "package main\n\nimport \"fmt\"\n\n// Greeting returns a friendly greeting.\nfunc Greeting(name string) string {\n\treturn \"hello \" + name\n}\n\nfunc main() {\n\tfmt.Println(Greeting(\"lsp\"))\n}\n",
		},
	},
	{
		lang: "typescript", languageID: "typescript", path: "/workspace/tslsp/index.ts", needle: "greeting(", want: "greeting",
		files: map[string]string{
			"/workspace/tslsp/tsconfig.json": "{\"compilerOptions\": {\"strict\": true, \"noEmit\": true}}\n",
			"/workspace/tslsp/index.ts":      "export function greeting(name: string): string {\n  return \"hello \" + name;\n}\n\nexport const message = greeting(\"lsp\");\n",
		},
	},
	{
		lang: "python", languageID: "python", path: "/workspace/pylsp/main.py", needle: "greeting(", want: "greeting",
		files: map[string]string{
			"/workspace/pylsp/main.py": "def greeting(name: str) -> str:\n    \"\"\"Return a friendly greeting.\"\"\"\n    return \"hello \" + name\n\n\nmessage = greeting(\"lsp\")\n",
		},
	},
	{
		lang: "cpp", languageID: "c", path: "/workspace/clsp/main.c", needle: "add(", want: "add",
		files: map[string]string{
			"/workspace/clsp/main.c": "/* Adds two numbers. */\nint add(int a, int b) {\n    return a + b;\n}\n\nint main(void) {\n    return add(1, 2) == 3 ? 0 : 1;\n}\n",
		},
	},
}

// TestCodeStudioLSPRealImage drives the four language servers of a locally
// built Code Studio image through the real bridge and the real Docker
// adapter. It is opt-in: AURAGO_LSP_IMAGE names the image, for example
// aurago-code-studio:lsp-dev. AURAGO_LSP_DOCKER_HOST overrides the bridge's
// Docker endpoint (default: the OS default, the Docker Desktop pipe on
// Windows). The docker CLI creates and removes the container.
func TestCodeStudioLSPRealImage(t *testing.T) {
	image := strings.TrimSpace(os.Getenv("AURAGO_LSP_IMAGE"))
	if image == "" {
		t.Skip("set AURAGO_LSP_IMAGE=aurago-code-studio:lsp-dev to check the real language servers")
	}
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	tag, err := newContainerTerminalNonce()
	if err != nil {
		t.Fatal(err)
	}
	name := "aurago-lsp-it-" + tag[:12]
	// The same isolation as the managed Code Studio container.
	codeStudioLSPDockerCLI(t, nil, "run", "-d", "--name", name, "--network", "none", "--user", "developer",
		"--security-opt", "no-new-privileges:true", "--cap-drop", "ALL", "--cap-add", "CHOWN", "--cap-add", "FOWNER",
		"--memory", "4096m", "--pids-limit", "1024",
		"-e", "HOME=/home/developer", "-e", "USER=developer", "-e", "GOPATH=/home/developer/go",
		image, "sleep", "infinity")
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
	for _, sample := range codeStudioLSPSamples {
		for path, content := range sample.files {
			codeStudioLSPDockerCLI(t, strings.NewReader(content), "exec", "-i", name, "sh", "-c", `mkdir -p "$(dirname "$1")" && cat > "$1"`, "sh", path)
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h := codeStudioHandlers{
		server: &Server{Logger: logger},
		docker: codeStudioDockerAdapter{cfg: tools.DockerConfig{Host: dockerutil.NormalizeHost(os.Getenv("AURAGO_LSP_DOCKER_HOST"))}, logger: logger},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lsp, ok := codeStudioLSPServerFor(r.URL.Query().Get("lang"))
		if !ok {
			http.Error(w, "unknown language server", http.StatusBadRequest)
			return
		}
		conn, err := codeStudioWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		h.serveCodeStudioLSP(r.Context(), conn, name, lsp)
	}))
	defer srv.Close()

	for _, sample := range codeStudioLSPSamples {
		t.Run(sample.lang, func(t *testing.T) {
			runCodeStudioLSPSample(t, srv.URL, sample)
		})
	}

	// Every socket is closed now: the bridge cleanup must leave no server process.
	deadline := time.Now().Add(20 * time.Second)
	for {
		comms := codeStudioLSPDockerCLI(t, nil, "exec", name, "sh", "-c", `for d in /proc/[0-9]*; do cat "$d/comm" 2>/dev/null; done`)
		left := codeStudioLSPLeftovers(comms)
		if len(left) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("language server processes survived the bridge cleanup: %v", left)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// runCodeStudioLSPSample opens one bridge socket and checks initialize and hover.
func runCodeStudioLSPSample(t *testing.T, baseURL string, sample codeStudioLSPSample) {
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(baseURL, "http")+"/?lang="+sample.lang, http.Header{"Origin": {baseURL}})
	if err != nil {
		t.Fatalf("dial %s: %v", sample.lang, err)
	}
	defer conn.Close()
	client := &codeStudioLSPTestClient{t: t, conn: conn, deadline: time.Now().Add(3 * time.Minute)}
	result, rpcErr := client.call("initialize", map[string]interface{}{
		"processId":        nil,
		"rootUri":          "file:///workspace",
		"workspaceFolders": []map[string]string{{"uri": "file:///workspace", "name": "workspace"}},
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"hover": map[string]interface{}{"contentFormat": []string{"markdown", "plaintext"}},
			},
		},
	})
	if rpcErr != "" {
		t.Fatalf("initialize: %s", rpcErr)
	}
	var initialized struct {
		Capabilities map[string]json.RawMessage `json:"capabilities"`
	}
	if err := json.Unmarshal(result, &initialized); err != nil {
		t.Fatalf("decode initialize result %s: %v", result, err)
	}
	if hover := strings.TrimSpace(string(initialized.Capabilities["hoverProvider"])); hover == "" || hover == "false" || hover == "null" {
		t.Fatalf("initialize result %s has no hoverProvider", result)
	}
	client.notify("initialized", map[string]interface{}{})
	text := sample.files[sample.path]
	uri := "file://" + sample.path
	client.notify("textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri, "languageId": sample.languageID, "version": 1, "text": text},
	})
	line, character := codeStudioLSPPosition(text, sample.needle)
	// A server may answer null or an error until it has loaded the project, so ask again.
	for attempt := 1; ; attempt++ {
		hover, rpcErr := client.call("textDocument/hover", map[string]interface{}{
			"textDocument": map[string]string{"uri": uri},
			"position":     map[string]int{"line": line, "character": character},
		})
		if rpcErr == "" && strings.Contains(string(hover), sample.want) {
			return
		}
		if attempt == 30 {
			t.Fatalf("hover at %d:%d = %s (error %q), want text containing %q", line, character, hover, rpcErr, sample.want)
		}
		time.Sleep(time.Second)
	}
}

// codeStudioLSPTestClient is a minimal JSON-RPC client on a bridge socket.
type codeStudioLSPTestClient struct {
	t        *testing.T
	conn     *websocket.Conn
	nextID   int
	deadline time.Time
}

func (c *codeStudioLSPTestClient) send(msg map[string]interface{}) {
	c.t.Helper()
	msg["jsonrpc"] = "2.0"
	data, err := json.Marshal(msg)
	if err != nil {
		c.t.Fatalf("encode %v: %v", msg["method"], err)
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		c.t.Fatalf("send %v: %v", msg["method"], err)
	}
}

func (c *codeStudioLSPTestClient) notify(method string, params interface{}) {
	c.t.Helper()
	c.send(map[string]interface{}{"method": method, "params": params})
}

// call sends a request and returns its result, or the error message of an
// error response. Server requests that arrive meanwhile get an empty answer
// (workspace/configuration one null per item); notifications are skipped.
func (c *codeStudioLSPTestClient) call(method string, params interface{}) (json.RawMessage, string) {
	c.t.Helper()
	c.nextID++
	id := strconv.Itoa(c.nextID)
	c.send(map[string]interface{}{"id": c.nextID, "method": method, "params": params})
	for {
		_ = c.conn.SetReadDeadline(c.deadline)
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			c.t.Fatalf("%s: read: %v", method, err)
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			c.t.Fatalf("%s: decode %s: %v", method, data, err)
		}
		if msg.Method != "" {
			if len(msg.ID) > 0 {
				c.answer(msg.ID, msg.Method, msg.Params)
			}
			continue
		}
		if string(msg.ID) != id {
			continue
		}
		if msg.Error != nil {
			return nil, strconv.Itoa(msg.Error.Code) + " " + msg.Error.Message
		}
		if len(msg.Result) == 0 {
			msg.Result = json.RawMessage("null")
		}
		return msg.Result, ""
	}
}

func (c *codeStudioLSPTestClient) answer(id json.RawMessage, method string, params json.RawMessage) {
	c.t.Helper()
	var result interface{}
	if method == "workspace/configuration" {
		var request struct {
			Items []json.RawMessage `json:"items"`
		}
		_ = json.Unmarshal(params, &request)
		result = make([]interface{}, len(request.Items))
	}
	c.send(map[string]interface{}{"id": id, "result": result})
}

// codeStudioLSPPosition returns the LSP position (0-based line, UTF-16
// column) of the second character of the last occurrence of needle. The
// samples are ASCII.
func codeStudioLSPPosition(text, needle string) (int, int) {
	index := strings.LastIndex(text, needle)
	before := text[:index]
	return strings.Count(before, "\n"), index - (strings.LastIndex(before, "\n") + 1) + 1
}

// codeStudioLSPLeftovers lists process names that belong to a language
// server: the servers, their node runtimes and gopls's go children.
func codeStudioLSPLeftovers(comms string) []string {
	var left []string
	for _, line := range strings.Split(comms, "\n") {
		comm := strings.TrimSpace(line)
		switch {
		case comm == "gopls", comm == "go", comm == "node", strings.HasPrefix(comm, "clangd"),
			strings.HasPrefix(comm, "typescript"), strings.HasPrefix(comm, "pyright"), strings.HasPrefix(comm, "tsserver"):
			left = append(left, comm)
		}
	}
	return left
}

// codeStudioLSPDockerCLI runs the docker CLI and fails the test on an error.
func codeStudioLSPDockerCLI(t *testing.T, stdin io.Reader, args ...string) string {
	t.Helper()
	cmd := exec.Command("docker", args...)
	cmd.Stdin = stdin
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
```

- [ ] **Step 3: Format.**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && gofmt -l internal/server/code_studio_lsp_image_test.go`
Expected: no output.

- [ ] **Step 4: Run the default suite.** The real-image test skips here.

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && mkdir -p /c/tmp/go-build-lsp && GOTOOLCHAIN=go1.27.4 GOTMPDIR='C:\tmp\go-build-lsp' go test ./internal/server -run '(?i)(CodeStudioLSP)' -count=1 -v 2>&1 | grep -E -- '--- (PASS|FAIL|SKIP)|^(ok|FAIL|panic)'`
Expected: the eight PASS lines from Task 4, plus `--- SKIP: TestCodeStudioLSPRealImage`, then `ok  	aurago/internal/server`.

- [ ] **Step 5: Run the real-image check.** It needs the image from Task 1 on local Docker Desktop.

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && AURAGO_LSP_IMAGE=aurago-code-studio:lsp-dev GOTOOLCHAIN=go1.27.4 GOTMPDIR='C:\tmp\go-build-lsp' go test ./internal/server -run '(?i)(CodeStudioLSPRealImage)' -count=1 -v -timeout 20m 2>&1 | grep -E -- '--- (PASS|FAIL|SKIP)|^(ok|FAIL|panic)'`
Expected:
```
--- PASS: TestCodeStudioLSPRealImage (<time>)
    --- PASS: TestCodeStudioLSPRealImage/go (<time>)
    --- PASS: TestCodeStudioLSPRealImage/typescript (<time>)
    --- PASS: TestCodeStudioLSPRealImage/python (<time>)
    --- PASS: TestCodeStudioLSPRealImage/cpp (<time>)
ok  	aurago/internal/server	<time>
```

If something fails, run the same command again without the `| grep ...` part. The debug log then shows each server's stderr tail (`stderr_tail=`). Find the matching symptom:

| Symptom | Action |
| --- | --- |
| A subtest closes with 4500 and the log says `docker connection cannot close stdin` | The Docker Desktop pipe cannot half-close. Enable "Expose daemon on tcp://localhost:2375 without TLS" in Docker Desktop, or ask the user to. Then run again with `AURAGO_LSP_DOCKER_HOST=tcp://localhost:2375` added before `go test`. |
| `typescript` fails with "Could not find a valid TypeScript installation" | The global `typescript` is not found next to the server. Report it. The fix belongs in the image (or in 2b's `initializationOptions.tsserver.path`). Do not guess a fix in the bridge. |
| A server process survives the cleanup (the leftovers check fails) | Report which process survived. A child started with a fresh environment has no tag. The fix is to extend `codeStudioLSPEndScript` to also signal descendants of tagged processes (walk `PPid`). Do that as its own follow-up, with the user's agreement. |

- [ ] **Step 6: No container left behind.**

Run: `docker ps -a --filter name=aurago-lsp-it- --format '{{.Names}}'`
Expected: no output.

If this plan's run left one behind, remove it by its exact name: `docker rm -f <name>`. Keep the image `aurago-code-studio:lsp-dev`.

- [ ] **Step 7: Graph check.** Run detect_changes. Expected: only new test symbols.

- [ ] **Step 8: Commit.** Write `C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/msg-2a-5.txt`:

```
test(code-studio-api): opt-in LSP check against the real image

TestCodeStudioLSPRealImage starts a container from AURAGO_LSP_IMAGE with
network none and checks initialize and hover for gopls,
typescript-language-server, pyright and clangd through the real bridge,
then checks that the cleanup left no server process.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

Run: `bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/msg-2a-5.txt internal/server/code_studio_lsp_image_test.go`
Expected: `committed <sha> (parent <sha>)` and no status line.

- [ ] **Step 9: Final state.**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && git log --oneline -6 && git status --short -- deploy/docker/Dockerfile.code-studio internal/server/code_studio_handlers.go internal/server/code_studio_lsp.go internal/server/code_studio_lsp_test.go internal/server/code_studio_lsp_image_test.go`

Expected:
- The five commits of this plan appear (possibly interleaved with other sessions' commits).
- No status lines for these paths.
- Do not run `git push` and do not push the image.

Report to the controller:
- the commit SHAs
- the image size and the growth from Task 1, Step 7
- the PASS lines of Task 4 and Task 5
- every impact or detect_changes risk above LOW

---

## Risks and fallbacks

**Base image Node version.** `node:22-bookworm-slim` floats. Task 1, Step 5 refreshes it, and the `node -e` guard stops any later build with an old Node. To pin, use `FROM node:22.22.2-bookworm-slim` or a newer 22.x patch. If no 22.x tag of at least 22.22.2 exists, the user decides.

**gopls and the Go version.** The spec says gopls v0.24.0 builds with Go 1.27. If `go install` reports that it needs a newer patch, that is a decision for the user. It also touches `TestCodeStudioDockerfileIncludesRunButtonRuntimes`.

**Slower multi-arch publish.** The publish workflow builds linux/amd64 and linux/arm64. On the arm64 leg, `go install gopls` runs under QEMU, so the publish takes longer (several minutes more). This is acceptable for now. A later optimization is a cross-compiling build stage on `$BUILDPLATFORM`.

**Docker Desktop named pipe and half-close.** `openCodeStudioExecStdinStream` needs `CloseWrite`. Linux unix sockets and TCP support it. If the Windows pipe of Docker Desktop does not, use the TCP fallback in Task 5, Step 5. This only affects local testing. Production runs on Linux.

**Orphaned children.** The cleanup finds processes by the inherited `AURAGO_LSP_SESSION` tag. Task 5's leftovers check proves this on the real servers. If it fails, extend the script (see Task 5, Step 5) instead of weakening the test.

**Big files.** A page frame over max file size + 64 KiB closes with 1009. Code Studio cannot open larger files through its file API, so this happens only when a file grows past the limit while it is being edited. 2b's reconnect backoff caps the retries.

**Shared checkout.** Another session may edit `code_studio_handlers.go` or Go files that `internal/server` imports.
- Check your own paths before each commit.
- Use the overlay fallback for foreign build breaks.
- Never edit their files.

# Plan B: Code Studio HTML preview

**Goal.** A Code Studio user can preview an `.html`, `.htm` or `.svg` file of the active workspace:
- in a resizable side pane right of the editor group, or
- in a new browser tab.

The preview reloads live when the file is saved, swaps stylesheets in place when only CSS changed, and forwards the page's console errors to a badge. The preview is sandboxed so that it can never read AuraGo cookies, storage or API responses. It works for the local container workspace today. It works for SSH workspaces as soon as Plan A's backend lookup exists.

**Spec:** `docs/superpowers/specs/2026-11-25-code-studio-ssh-workspace-preview-design.md`, section 4.
**Worktree:** `C:\Users\Andi\Documents\repo\AuraGo\.claude\worktrees\cs-ssh-preview` (branch `feat/code-studio-ssh-preview`). Run every command from this directory.

---

## Depends on Plan A

Plan B is written against these Plan A names. Task 0 checks which of them exist and picks one of two modes.

| Plan A name | What Plan B uses it for |
| --- | --- |
| `codeStudioBackend` with `Probe(ctx, vpath) (codeStudioFileProbe, error)` and `Read(ctx, vpath, max int64) ([]byte, error)` | Plan B's read contract `codeStudioPreviewSource` has exactly these two methods, so any Plan A backend satisfies it without an adapter. |
| `h.backendFor(r)` | Resolves the request's workspace (header `X-Code-Studio-Workspace`, query `ws`, default local). Plan B reads `backend.Info().ID` to put the workspace id into the token. |
| Backend lookup by workspace id (written here as `h.backendByID(ctx, id)`; use Plan A's real name) | The preview route resolves the token's workspace without a request header. |
| Plan A's "workspace not found" sentinel error | Mapped to Plan B's `errCodeStudioPreviewWorkspaceNotFound`, which gives 404. |
| Calls to `h.notifyWorkspaceChange(wsID string, paths ...string)` after write, move, delete and upload | **Plan B owns the definition** (`code_studio_preview_versions.go`). If Plan A shipped a stub with that name, delete the stub (Task 2). |

**Mode L (local fallback, Plan A absent).**
- Task 4 adds `previewWorkspaceID` and `previewSource`. They accept only `local` and read through today's container path (`codeContainer`, `probeCodeStudioFile`, `codeStudioPathCommand("read", …)`).
- Task 7 adds the version-bump calls to the current handlers.

**Mode A (Plan A merged).**
- Task 4 uses the Mode A variant, which delegates to Plan A.
- Task 7 adds only the bumps Plan A does not already make.

Plan B never edits `server_routes.go`.

On the UI side:
- Plan C sends the workspace header from `api()`. Plan B's `POST /api/code-studio/preview` goes through `api()`, so it gets the header automatically.
- Plan C must call `CodeStudioApp.command('resetPreview')` when the workspace switches.

---

## Architecture

```
Browser (Code Studio, AuraGo origin)                  Go server
-------------------------------------                 ----------------------------------------------------
preview.js  --POST /api/code-studio/preview {path}--> handleIssue: session auth + desktopRead operation,
            <-- {url, base, token, expires_at}          workspace id -> HMAC token (ws, exp 12h, v1)
            --HEAD <url> (status for error card)----> handleServe (auth-exempt prefix, token = capability)
iframe sandbox="allow-scripts allow-forms             GET /code-studio-preview/<token>/<rel>
  allow-modals allow-popups allow-downloads"   ---->   verify token -> workspace source (Plan A backend or
  (opaque origin, no cookies)                           local container) -> Probe -> Read (25 MiB cap)
                                                       dir -> index.html, dir without "/" -> 307
                                                       headers: CSP sandbox, nosniff, no-referrer, no-store,
                                                       ACAO *, CORP cross-origin, X-Frame-Options removed
                                                       HTML: inject live.js right after <head> (else before
                                                       </body>, else appended); SVG: before </svg>
live.js  --GET __aurago/version?since=N (25 s)------> version registry per workspace (Bump from
         <-- {version, paths, full}                    notifyWorkspaceChange: write/move/delete/upload/
         css only -> swap <link> hrefs, else reload    git discard/branch switch)
         postMessage({auragoPreview:1, kind, text})
            -> parent checks event.source === iframe.contentWindow, origin "null", shape
```

### Token design
- **Format.** `base64url(json{v:1, ws, exp}) "." base64url(HMAC-SHA256)`.
- **Key.** `HMAC(sessionSecret, "aurago/code-studio-preview/v1")`. Because the key is derived with a purpose label, a preview token can never verify as a desktop embed token or a session cookie, and the reverse holds too.
- **Revocation.** Rotating the session secret (password change, "log out everywhere") revokes all preview links.
- **Login disabled.** Without a session secret, a random per-process key is used; links then end at a restart.
- **No session binding.** The sandboxed page sends no cookie, so there is nothing to bind to.
- **Scope.** Read-only access to one whole workspace, for 12 hours.

### Auth exemption
`codeStudioPreviewPrefix` is added to `authBypassSubtrees` in `auth.go`, the same pattern as `/api/game-maker/preview/`. The handler rejects everything without a valid token and every method except GET and HEAD.

### Route registration
`registerCodeStudioRoutes` (in `code_studio_handlers.go`) gets one added line that calls `registerCodeStudioPreviewRoutes`.

### Framing and headers
- `securityHeadersMiddleware` sends `X-Frame-Options: DENY` everywhere. The preview handler deletes it; its own CSP `frame-ancestors 'self'` replaces it.
- The handler also overrides the static-asset `Cache-Control` for `.css` and `.js` paths with `no-store`.

### Security notes
- Without `allow-same-origin`, the page has an opaque origin, even when it is opened top-level in its own tab.
- Requests from the page send `Origin: null` and carry no SameSite=Strict cookies.
- When login is disabled, the page's POSTs fail the existing CSRF origin check. Its GETs are unreadable to it under CORS. That is the same exposure as from any foreign website, so nothing new.
- Known limitation: pages that use `localStorage` or `document.cookie` get a SecurityError. That is the price of the isolation, and the console badge shows the error.

### UI
- **New files.** `preview-model.js` (pure helpers, node-tested) and `preview.js` (a pane fragment inside the shared bundle IIFE).
- **Hooks into existing code.** A few one-line hooks in `core.js`:
  - an aside in `shellMarkup`
  - `renderPreviewPane()` in `renderShell`
  - a `renderBreadcrumbs` wrapper (eye button, follow mode, dirty hint)
  - `previewAfterTabs()` at the end of `renderTabs`
  - a View-menu item
- **Elsewhere.** A shortcut, overlay row and command-table entries in `shortcuts.js`; two palette commands; two Explorer context-menu items.
- **CSS.** A separate file, `ui/css/code-studio-preview.css`, loaded through `module-loader.js`. This avoids conflicts with Plan C's edits to `code-studio.css`.

---

## File map

| File | Change |
| --- | --- |
| `internal/server/code_studio_preview_token.go` | new: key derivation, token issue/parse, URL path helpers |
| `internal/server/code_studio_preview_versions.go` | new: per-workspace version registry, long-poll `Wait`, the `notifyWorkspaceChange` hook |
| `internal/server/code_studio_preview_live.js` | new: live-reload and console-forwarding script (`go:embed`) |
| `internal/server/code_studio_preview_source.go` | new: `codeStudioPreviewSource` contract, workspace resolution; Mode L local source or Mode A delegation |
| `internal/server/code_studio_preview.go` | new: serve handler, issue handler, headers, injection, content types, error pages, route registration |
| `internal/server/code_studio_handlers.go` | edit: one line in `registerCodeStudioRoutes`; Mode L also adds bump calls in write/move/delete/upload/git branch |
| `internal/server/code_studio_git_workbench.go` | edit: bump after git discard |
| `internal/server/auth.go` | edit: one entry in `authBypassSubtrees` |
| `internal/server/code_studio_preview_token_test.go`, `…_versions_test.go`, `…_source_test.go`, `code_studio_preview_test.go`, `code_studio_preview_routes_test.go` | new Go tests |
| `ui/js/desktop/apps/code-studio/preview-model.js` + `preview-model.test.cjs` | new: pure helpers + node test |
| `ui/js/desktop/apps/code-studio/preview.js` | new: the pane |
| `ui/js/desktop/apps/code-studio/core.js` | edit: 5 small hooks |
| `ui/js/desktop/apps/code-studio/shortcuts.js` | edit: Ctrl+Shift+V, overlay row, command table |
| `ui/js/desktop/apps/code-studio/command-palette.js` | edit: 2 commands |
| `ui/js/desktop/apps/code-studio/sidebar.js` | edit: 2 Explorer context-menu items |
| `ui/css/code-studio-preview.css` | new: pane styles |
| `ui/js/desktop/core/module-loader.js` | edit: load the CSS file |
| `scripts/build-ui-bundles.js` + `ui/js/desktop/bundles/code-studio.bundle.js` | edit: 2 bundle parts, rebuilt bundle |
| `ui/lang/desktop/*.json` (16) | edit: 37 keys |
| `ui/desktop_code_studio_browser_test.go` | edit: fixture preview routes |
| `ui/desktop_code_studio_preview_browser_test.go` | new: one browser smoke |

---

## Environment notes for the executor

- **Go cache.** `GOCACHE=D:\dev-cache\go-build` (user env). If D: is full, trim it per memory `reference_go_cache_disk_d`. GOTMPDIR must be in backslash form. If `vet` is missing, prefix Go commands with `GOTOOLCHAIN=go1.27.4`.
- **Committing.** The worktree has its own index. `git add <own paths>` followed by `git commit` is safe here. Never use `git add -A`.
- **GitNexus.** The worktree is not indexed (known gap, memory `reference_gitnexus_index_gaps`). Run `impact` from the main checkout `C:\Users\Andi\Documents\repo\AuraGo`, whose symbols match the branch base:
  ```
  node ../../../.gitnexus/maintenance/AuraGo/graph/run.cjs impact "<symbol>" --direction upstream --repo .
  ```
  Treat `UNKNOWN` as unresolved and confirm with a text search. Report HIGH or CRITICAL before editing.
- **Write tool.** It decodes `\u` escapes. The code below contains none; keep it that way.
- **Browser smoke.** It needs `AURAGO_RUN_BROWSER_SMOKE=1`. If Chrome will not start from the test binary, follow memory `reference_chrome_launch_from_agent`.

---

## Task 0: Preflight and mode decision (5 min)

1. Check the worktree state:
   ```
   git status --short
   git log --oneline -3
   ```
   Expected: clean, HEAD at or after `0b366d5c9 docs(code-studio): design for SSH workspaces and HTML preview`.

2. Probe for Plan A with the Grep tool in `internal/server`:
   - `type codeStudioBackend interface`
   - `func \(h \*?codeStudioHandlers\) backendFor`
   - `func \(h \*?codeStudioHandlers\) notifyWorkspaceChange`
   - `notifyWorkspaceChange\(`
   - the by-id lookup (search `backendBy|workspaceBackend|lookupBackend`) and the not-found sentinel (search `errCodeStudioWorkspaceNotFound|workspace_not_found`)

   **Mode A** if both the backend interface and `backendFor` exist; otherwise **Mode L**. Write the decision and Plan A's exact names into the first commit message body of Task 4.

3. Run GitNexus impact (from the main checkout, see the environment notes) for each symbol you will edit, and record callers and risk:
   - `registerCodeStudioRoutes`, `handleWriteFile`, `handleMoveFile`, `handleDeleteFile`, `writeUploadedFile`, `handleGitDiscard`, `handleGitBranch`
   - `shellMarkup`, `renderShell`, `renderBreadcrumbs`, `renderTabs`, `renderWindowMenus`, `wireShortcuts`, `showShortcutOverlay`, `studioCommandTable`, `getCommands`, `showTreeContextMenu`
   - the variable `authBypassSubtrees` (likely `UNKNOWN`; confirm with Grep that it is used only by `isAuthBypassed` and tests)

   Stop and report if any result is HIGH or CRITICAL.

No commit.

---

## Task 1: Preview token: sign, verify, expiry, tamper (8 min)

**Test first.** Create `internal/server/code_studio_preview_token_test.go`:

```go
package server

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCodeStudioPreviewTokenSignVerifyExpiryTamper(t *testing.T) {
	key, err := codeStudioPreviewKey("secret-a")
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	now := time.Unix(1_800_000_000, 0)
	token, expires, err := issueCodeStudioPreviewToken(key, "local", now)
	if err != nil || !expires.Equal(now.Add(12*time.Hour)) {
		t.Fatalf("issue = %q, %v, %v; want a token expiring in 12 h", token, expires, err)
	}
	claims, ok := parseCodeStudioPreviewToken(key, token, now.Add(time.Hour))
	if !ok || claims.Workspace != "local" || claims.Version != 1 {
		t.Fatalf("parse = %+v, %v; want local v1", claims, ok)
	}
	if _, ok := parseCodeStudioPreviewToken(key, token, now.Add(12*time.Hour)); ok {
		t.Fatal("token is still valid at its expiry time")
	}
	other, _ := codeStudioPreviewKey("secret-b")
	if _, ok := parseCodeStudioPreviewToken(other, token, now); ok {
		t.Fatal("token verified with a different session secret")
	}
	payload, signature, _ := strings.Cut(token, ".")
	forged, _ := json.Marshal(codeStudioPreviewClaims{Version: 1, Workspace: "ssh-other", Expires: now.Add(24 * time.Hour).Unix()})
	if _, ok := parseCodeStudioPreviewToken(key, base64.RawURLEncoding.EncodeToString(forged)+"."+signature, now); ok {
		t.Fatal("a payload with a foreign workspace kept the old signature and verified")
	}
	for _, bad := range []string{"", ".", payload, payload + ".", token + ".x", strings.Repeat("a", 600)} {
		if _, ok := parseCodeStudioPreviewToken(key, bad, now); ok {
			t.Fatalf("malformed token %q verified", bad)
		}
	}
	if _, _, err := issueCodeStudioPreviewToken(key, "../evil", now); err == nil {
		t.Fatal("a workspace id outside [A-Za-z0-9_-] was accepted")
	}
	// The key is derived with a purpose label: a desktop embed token signed with the session secret never verifies.
	embed, err := issueDesktopEmbedTokenForPath("secret-a", "Apps/x/index.html", now)
	if err != nil {
		t.Fatalf("embed token: %v", err)
	}
	if _, ok := parseCodeStudioPreviewToken(key, embed, now); ok {
		t.Fatal("a desktop embed token was accepted as a preview token")
	}
}

func TestCodeStudioPreviewKeyWithoutSessionSecret(t *testing.T) {
	first, err := codeStudioPreviewKey("")
	if err != nil || len(first) == 0 {
		t.Fatalf("process key = %x, %v; want a random key", first, err)
	}
	second, _ := codeStudioPreviewKey("  ")
	if string(first) != string(second) {
		t.Fatal("the per-process key changed between calls")
	}
	derived, _ := codeStudioPreviewKey("x")
	if string(derived) == string(first) || string(derived) == "x" {
		t.Fatal("the session-secret key must be derived and differ from the process key")
	}
}
```

Run:
```
go test ./internal/server/ -run '(?i)(CodeStudioPreview)' -count=1
```
Expected: build failure, `undefined: codeStudioPreviewKey`.

**Implement.** Create `internal/server/code_studio_preview_token.go`:

```go
package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Code Studio HTML previews are served from /code-studio-preview/<token>/<path>. The sandboxed page has an opaque
// origin and the session cookie is SameSite=Strict, so preview requests carry no session: the token is the
// capability. It is stateless (HMAC), names one workspace, expires after 12 hours and only grants read access to
// that workspace's files. The key is derived from the session secret with a purpose label, so a preview token can
// never pass as a desktop embed token or session cookie, and rotating the secret (password change, "log out
// everywhere") revokes every preview link. Without a session secret (login disabled) a random per-process key is
// used; links then end with a restart. There is no session binding: the sandboxed page sends no cookie to bind to.
const (
	codeStudioPreviewPrefix      = "/code-studio-preview/"
	codeStudioPreviewTokenTTL    = 12 * time.Hour
	codeStudioPreviewKeyLabel    = "aurago/code-studio-preview/v1"
	codeStudioPreviewTokenMaxLen = 512
)

var codeStudioPreviewWorkspacePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type codeStudioPreviewClaims struct {
	Version   int    `json:"v"`
	Workspace string `json:"ws"`
	Expires   int64  `json:"exp"`
}

var codeStudioPreviewProcessSecret struct {
	once  sync.Once
	value string
}

func codeStudioPreviewKey(secret string) ([]byte, error) {
	if strings.TrimSpace(secret) == "" {
		codeStudioPreviewProcessSecret.once.Do(func() {
			raw := make([]byte, 32)
			if _, err := rand.Read(raw); err == nil {
				codeStudioPreviewProcessSecret.value = hex.EncodeToString(raw)
			}
		})
		secret = codeStudioPreviewProcessSecret.value
		if secret == "" {
			return nil, errors.New("preview signing key unavailable")
		}
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(codeStudioPreviewKeyLabel))
	return mac.Sum(nil), nil
}

func (s *Server) codeStudioPreviewSigningKey() ([]byte, error) {
	if s == nil || s.Cfg == nil {
		return nil, errors.New("server configuration unavailable")
	}
	s.CfgMu.RLock()
	secret := s.Cfg.Auth.SessionSecret
	s.CfgMu.RUnlock()
	return codeStudioPreviewKey(secret)
}

func codeStudioPreviewSignature(key []byte, payload string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func issueCodeStudioPreviewToken(key []byte, workspace string, now time.Time) (string, time.Time, error) {
	if len(key) == 0 {
		return "", time.Time{}, errors.New("preview signing key unavailable")
	}
	if !codeStudioPreviewWorkspacePattern.MatchString(workspace) {
		return "", time.Time{}, errors.New("invalid workspace id")
	}
	expires := now.Add(codeStudioPreviewTokenTTL).Truncate(time.Second)
	payload, err := json.Marshal(codeStudioPreviewClaims{Version: 1, Workspace: workspace, Expires: expires.Unix()})
	if err != nil {
		return "", time.Time{}, err
	}
	part := base64.RawURLEncoding.EncodeToString(payload)
	return part + "." + codeStudioPreviewSignature(key, part), expires, nil
}

func parseCodeStudioPreviewToken(key []byte, token string, now time.Time) (codeStudioPreviewClaims, bool) {
	var claims codeStudioPreviewClaims
	if len(key) == 0 || token == "" || len(token) > codeStudioPreviewTokenMaxLen {
		return claims, false
	}
	part, signature, found := strings.Cut(token, ".")
	if !found || part == "" || signature == "" || strings.Contains(signature, ".") {
		return claims, false
	}
	// The encoded signatures are compared, so no two spellings of one MAC can both pass.
	if !hmac.Equal([]byte(signature), []byte(codeStudioPreviewSignature(key, part))) {
		return claims, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil || json.Unmarshal(raw, &claims) != nil {
		return codeStudioPreviewClaims{}, false
	}
	if claims.Version != 1 || !codeStudioPreviewWorkspacePattern.MatchString(claims.Workspace) || claims.Expires <= now.Unix() {
		return codeStudioPreviewClaims{}, false
	}
	return claims, true
}

// splitCodeStudioPreviewPath returns the token and the workspace-relative rest of a preview URL path. slash reports
// whether a "/" followed the token; /code-studio-preview/<token> alone redirects to .../<token>/.
func splitCodeStudioPreviewPath(urlPath string) (token, rel string, slash bool) {
	if !strings.HasPrefix(urlPath, codeStudioPreviewPrefix) {
		return "", "", false
	}
	return strings.Cut(urlPath[len(codeStudioPreviewPrefix):], "/")
}

// codeStudioPreviewVirtualPath maps the relative part of a preview URL to a /workspace path. A ".." segment, a
// backslash or a NUL byte is rejected before cleaning, so a decoded traversal cannot climb out of the workspace.
func codeStudioPreviewVirtualPath(rel string) (string, error) {
	if strings.ContainsAny(rel, "\\\x00") {
		return "", errors.New("invalid preview path")
	}
	for _, segment := range strings.Split(rel, "/") {
		if segment == ".." {
			return "", errors.New("preview path escapes the workspace")
		}
	}
	return sanitizeCodeStudioPath(codeStudioWorkspaceRoot + "/" + rel)
}

func codeStudioPreviewEscapePath(rel string) string {
	if rel == "" {
		return ""
	}
	parts := strings.Split(rel, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
```

Run:
```
go test ./internal/server/ -run '(?i)(CodeStudioPreview)' -count=1 -v
```
Expected: `--- PASS: TestCodeStudioPreviewTokenSignVerifyExpiryTamper`, `--- PASS: TestCodeStudioPreviewKeyWithoutSessionSecret`, `ok  aurago/internal/server`.

Commit:
```
git add internal/server/code_studio_preview_token.go internal/server/code_studio_preview_token_test.go
git commit -m "feat(code-studio): stateless HMAC preview token scoped to one workspace"
```

---

## Task 2: Version registry and the `notifyWorkspaceChange` hook (8 min)

If Task 0 found a Plan A stub of `notifyWorkspaceChange`, delete that stub now. Plan B's definition below replaces it.

**Test first.** Create `internal/server/code_studio_preview_versions_test.go`:

```go
package server

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCodeStudioPreviewVersionRegistryLongPoll(t *testing.T) {
	ctx := context.Background()
	r := newCodeStudioPreviewVersionRegistry()
	if v := r.Current("local"); v != 0 {
		t.Fatalf("fresh version = %d, want 0", v)
	}
	r.Bump("local", "/workspace/a.css")
	if v, paths, full := r.Wait(ctx, "local", 0, time.Second); v != 1 || strings.Join(paths, ",") != "a.css" || full {
		t.Fatalf("behind client = %d %v %v, want 1 [a.css] false", v, paths, full)
	}
	type result struct {
		version uint64
		paths   []string
		full    bool
	}
	done := make(chan result, 1)
	go func() {
		v, p, f := r.Wait(ctx, "local", 1, 2*time.Second)
		done <- result{v, p, f}
	}()
	time.Sleep(30 * time.Millisecond)
	r.Bump("local", "/workspace/b.html", "/workspace/c.css")
	if got := <-done; got.version != 2 || strings.Join(got.paths, ",") != "b.html,c.css" || got.full {
		t.Fatalf("woken wait = %+v, want 2 [b.html c.css] false", got)
	}
	r.Bump("") // "" is the local workspace; no paths means anything may have changed
	if v, _, full := r.Wait(ctx, "local", 2, time.Second); v != 3 || !full {
		t.Fatalf("pathless bump = %d full=%v, want 3 full", v, full)
	}
	if r.Current("ssh-1") != 0 {
		t.Fatal("workspaces must count independently")
	}
	start := time.Now()
	if v, paths, _ := r.Wait(ctx, "local", 3, 50*time.Millisecond); v != 3 || paths != nil || time.Since(start) < 40*time.Millisecond {
		t.Fatalf("timeout = %d %v after %v, want the unchanged version after the timeout", v, paths, time.Since(start))
	}
	for i := 0; i < 70; i++ {
		r.Bump("local", "/workspace/x.css")
	}
	if _, _, full := r.Wait(ctx, "local", 3, time.Second); !full {
		t.Fatal("a client behind the kept history must get a full reload")
	}
	start = time.Now()
	if _, _, full := r.Wait(ctx, "local", 10_000, time.Second); !full || time.Since(start) > 200*time.Millisecond {
		t.Fatal("a client ahead of the server (restart) must get an immediate full answer")
	}
}
```

Run:
```
go test ./internal/server/ -run '(?i)(CodeStudioPreview)' -count=1
```
Expected: build failure, `undefined: newCodeStudioPreviewVersionRegistry`.

**Implement.** Create `internal/server/code_studio_preview_versions.go`:

```go
package server

import (
	"context"
	"strings"
	"sync"
	"time"
)

// Every Code Studio mutation bumps the version of its workspace through notifyWorkspaceChange; live.js in each
// preview long-polls that version and reloads (or swaps stylesheets when only .css files changed). The registry
// lives in memory per server: a restart sends every open preview one full reload.
const codeStudioPreviewHistory = 64

type codeStudioPreviewChange struct {
	version uint64
	paths   []string // workspace-relative; nil means anything may have changed
}

type codeStudioPreviewSpace struct {
	version uint64
	changes []codeStudioPreviewChange
	wake    chan struct{}
}

type codeStudioPreviewVersionRegistry struct {
	mu     sync.Mutex
	spaces map[string]*codeStudioPreviewSpace
}

func newCodeStudioPreviewVersionRegistry() *codeStudioPreviewVersionRegistry {
	return &codeStudioPreviewVersionRegistry{spaces: map[string]*codeStudioPreviewSpace{}}
}

// One registry per *Server without a field on Server (server.go is shared with other sessions).
var codeStudioPreviewRegistries sync.Map

func codeStudioPreviewVersionsFor(s *Server) *codeStudioPreviewVersionRegistry {
	if registry, ok := codeStudioPreviewRegistries.Load(s); ok {
		return registry.(*codeStudioPreviewVersionRegistry)
	}
	registry, _ := codeStudioPreviewRegistries.LoadOrStore(s, newCodeStudioPreviewVersionRegistry())
	return registry.(*codeStudioPreviewVersionRegistry)
}

// notifyWorkspaceChange is the hook every Code Studio write, move, delete, upload, LSP edit and agent write calls
// (Plan A wires the handlers, Plan D the agent tool). paths are /workspace virtual paths; none means "anything".
func (h codeStudioHandlers) notifyWorkspaceChange(wsID string, paths ...string) {
	codeStudioPreviewVersionsFor(h.server).Bump(wsID, paths...)
}

func codeStudioPreviewSpaceKey(workspace string) string {
	if workspace = strings.TrimSpace(workspace); workspace == "" {
		return "local"
	}
	return workspace
}

func (r *codeStudioPreviewVersionRegistry) spaceLocked(workspace string) *codeStudioPreviewSpace {
	key := codeStudioPreviewSpaceKey(workspace)
	space := r.spaces[key]
	if space == nil {
		space = &codeStudioPreviewSpace{wake: make(chan struct{})}
		r.spaces[key] = space
	}
	return space
}

func (r *codeStudioPreviewVersionRegistry) Current(workspace string) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.spaceLocked(workspace).version
}

func (r *codeStudioPreviewVersionRegistry) Bump(workspace string, paths ...string) uint64 {
	changed := make([]string, 0, len(paths))
	for _, p := range paths {
		rel := strings.Trim(strings.TrimPrefix(p, codeStudioWorkspaceRoot), "/")
		if rel == "" {
			changed = nil
			break
		}
		changed = append(changed, rel)
	}
	if len(changed) == 0 {
		changed = nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	space := r.spaceLocked(workspace)
	space.version++
	space.changes = append(space.changes, codeStudioPreviewChange{version: space.version, paths: changed})
	if extra := len(space.changes) - codeStudioPreviewHistory; extra > 0 {
		space.changes = append([]codeStudioPreviewChange(nil), space.changes[extra:]...)
	}
	close(space.wake)
	space.wake = make(chan struct{})
	return space.version
}

// changesLocked lists the paths changed after since. full is true when the kept history does not reach back that
// far, when one change named no paths, or when the client is ahead of the server (restart).
func (s *codeStudioPreviewSpace) changesLocked(since uint64) ([]string, bool) {
	if since > s.version {
		return nil, true
	}
	if since == s.version {
		return nil, false
	}
	full := len(s.changes) == 0 || s.changes[0].version > since+1
	seen := map[string]bool{}
	var paths []string
	for _, change := range s.changes {
		if change.version <= since {
			continue
		}
		if change.paths == nil {
			full = true
			continue
		}
		for _, p := range change.paths {
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	if full {
		return nil, true
	}
	return paths, false
}

// Wait returns as soon as the version differs from since, or the unchanged version after timeout or cancellation.
func (r *codeStudioPreviewVersionRegistry) Wait(ctx context.Context, workspace string, since uint64, timeout time.Duration) (uint64, []string, bool) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		r.mu.Lock()
		space := r.spaceLocked(workspace)
		version := space.version
		if version != since {
			paths, full := space.changesLocked(since)
			r.mu.Unlock()
			return version, paths, full
		}
		wake := space.wake
		r.mu.Unlock()
		select {
		case <-wake:
		case <-ctx.Done():
			return version, nil, false
		case <-timer.C:
			return version, nil, false
		}
	}
}
```

Run:
```
go test ./internal/server/ -run '(?i)(CodeStudioPreview)' -count=1 -v
```
Expected: 3 `--- PASS: TestCodeStudioPreview…` lines and `ok`.

Commit:
```
git add internal/server/code_studio_preview_versions.go internal/server/code_studio_preview_versions_test.go
git commit -m "feat(code-studio): per-workspace preview version registry with long-poll wait"
```

---

## Task 3: `live.js` (6 min)

Create `internal/server/code_studio_preview_live.js`:

```js
/* AuraGo Code Studio preview helper, injected into previewed HTML and SVG documents.
   - Long-polls the workspace version and reloads the page; when only stylesheets changed it swaps the
     <link rel="stylesheet"> hrefs instead, so the page keeps its state.
   - Forwards console.error, uncaught errors, failed resources and unhandled rejections to Code Studio via
     postMessage ({auragoPreview: 1, kind, text}); the parent accepts them only from its own iframe.
   The page runs sandboxed with an opaque origin: it has no access to AuraGo cookies, storage or APIs. */
(function () {
    'use strict';
    if (window.__auragoPreviewLive) return;
    window.__auragoPreviewLive = true;
    var script = document.currentScript;
    if (!script) return;
    var source = script.getAttribute('src') || script.getAttribute('href') || '';
    var marker = '__aurago/live.js';
    var cut = source.lastIndexOf(marker);
    if (cut < 0) return;
    var base = new URL(source.slice(0, cut), location.href).href;
    var targetOrigin = new URL(base).origin;
    var version = Number(script.getAttribute('data-aurago-version')) || 0;
    var embedded = window.parent && window.parent !== window;
    var MAX_TEXT = 2000;

    function post(kind, text) {
        if (!embedded) return;
        try {
            window.parent.postMessage({ auragoPreview: 1, kind: kind, text: String(text == null ? '' : text).slice(0, MAX_TEXT) }, targetOrigin);
        } catch (_) {}
    }

    function describe(value) {
        if (value && value.stack) return String(value.stack);
        if (value && value.message) return String(value.message);
        if (typeof value === 'string') return value;
        try { return JSON.stringify(value); } catch (_) { return String(value); }
    }

    function relative(url) {
        var text = String(url || '');
        return text.indexOf(base) === 0 ? text.slice(base.length) : text;
    }

    var originalError = console.error;
    console.error = function () {
        var parts = [];
        for (var i = 0; i < arguments.length; i++) parts.push(describe(arguments[i]));
        post('console', parts.join(' '));
        return originalError.apply(console, arguments);
    };
    window.addEventListener('error', function (event) {
        var element = event.target;
        if (element && element !== window && element.nodeType === 1) {
            post('resource', 'Failed to load ' + relative(element.currentSrc || element.src || element.href || element.tagName));
            return;
        }
        var where = event.filename ? ' (' + relative(event.filename) + ':' + (event.lineno || 0) + ')' : '';
        post('error', (event.error ? describe(event.error) : (event.message || 'Error')) + where);
    }, true);
    window.addEventListener('unhandledrejection', function (event) {
        post('rejection', 'Unhandled rejection: ' + describe(event.reason));
    });
    post('location', relative(location.href.split('#')[0]));

    function swapStylesheets() {
        var links = document.querySelectorAll('link[rel~="stylesheet"][href]');
        for (var i = 0; i < links.length; i++) {
            (function (link) {
                var url = new URL(link.getAttribute('href'), location.href);
                url.searchParams.set('__aurago', String(version));
                var next = link.cloneNode(false);
                next.href = url.href;
                next.addEventListener('load', function () { if (link.parentNode) link.parentNode.removeChild(link); });
                next.addEventListener('error', function () { if (next.parentNode) next.parentNode.removeChild(next); });
                link.parentNode.insertBefore(next, link.nextSibling);
            })(links[i]);
        }
    }

    function poll(delay) {
        setTimeout(function () {
            fetch(base + '__aurago/version?since=' + version, { cache: 'no-store', credentials: 'omit' })
                .then(function (response) {
                    if (response.status === 401) { post('expired', ''); return null; }
                    if (!response.ok) throw new Error('HTTP ' + response.status);
                    return response.json();
                })
                .then(function (data) {
                    if (!data) return;
                    var next = Number(data.version) || 0;
                    if (next === version) { poll(0); return; }
                    var paths = Array.isArray(data.paths) ? data.paths : [];
                    var cssOnly = next > version && !data.full && paths.length > 0 && paths.every(function (p) { return /\.css$/i.test(String(p)); });
                    version = next;
                    if (cssOnly && document.querySelector('link[rel~="stylesheet"][href]')) {
                        swapStylesheets();
                        post('reloaded', 'css');
                        poll(0);
                        return;
                    }
                    location.reload();
                })
                .catch(function () { poll(Math.min(30000, Math.max(1000, delay * 2))); });
        }, delay);
    }
    poll(0);
})();
```

Run:
```
node --check internal/server/code_studio_preview_live.js
```
Expected: no output, exit 0.

Commit:
```
git add internal/server/code_studio_preview_live.js
git commit -m "feat(code-studio): preview live script with long-poll reload, CSS hot swap and console forwarding"
```

---

## Task 4: Workspace source contract (Mode L fallback or Mode A) (8 min)

**Test first (Mode L).** Create `internal/server/code_studio_preview_source_test.go`. In Mode A, replace the cases with Plan A's ids: unknown id → `errCodeStudioPreviewWorkspaceNotFound`, missing or `local` → Plan A's local id.

```go
package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCodeStudioPreviewWorkspaceIDFromRequest(t *testing.T) {
	h := codeStudioHandlers{}
	for _, tc := range []struct {
		header, query, want string
		notFound            bool
	}{
		{"", "", "local", false},
		{"local", "", "local", false},
		{"", "local", "local", false},
		{"ssh-1", "", "", true},
		{"", "ssh-1", "", true},
	} {
		target := "/api/code-studio/preview"
		if tc.query != "" {
			target += "?ws=" + tc.query
		}
		req := httptest.NewRequest(http.MethodPost, target, nil)
		if tc.header != "" {
			req.Header.Set("X-Code-Studio-Workspace", tc.header)
		}
		got, err := h.previewWorkspaceID(req)
		if tc.notFound {
			if !errors.Is(err, errCodeStudioPreviewWorkspaceNotFound) {
				t.Fatalf("%+v: err = %v, want workspace not found", tc, err)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("%+v: got %q, %v; want %q", tc, got, err, tc.want)
		}
	}
}
```

Run:
```
go test ./internal/server/ -run '(?i)(CodeStudioPreview)' -count=1
```
Expected: build failure, `h.previewWorkspaceID undefined`.

**Implement, Mode L (fallback, local only).** Create `internal/server/code_studio_preview_source.go`:

```go
package server

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// codeStudioPreviewSource is the read side of a workspace backend. Plan A's codeStudioBackend has the same two
// methods, so every backend can serve previews without an adapter.
type codeStudioPreviewSource interface {
	Probe(ctx context.Context, vpath string) (codeStudioFileProbe, error)
	Read(ctx context.Context, vpath string, max int64) ([]byte, error)
}

var errCodeStudioPreviewWorkspaceNotFound = errors.New("workspace_not_found")

// --- Mode L: Plan A is not merged yet; only the local container workspace exists. ---------------------------
// Replace this block with the Mode A variant (plan Task 4) when Plan A lands.

func (h codeStudioHandlers) previewWorkspaceID(r *http.Request) (string, error) {
	id := strings.TrimSpace(r.Header.Get("X-Code-Studio-Workspace"))
	if id == "" {
		id = strings.TrimSpace(r.URL.Query().Get("ws"))
	}
	if id == "" || id == "local" {
		return "local", nil
	}
	return "", errCodeStudioPreviewWorkspaceNotFound
}

func (h codeStudioHandlers) previewSource(ctx context.Context, wsID string) (codeStudioPreviewSource, error) {
	if wsID != "local" {
		return nil, errCodeStudioPreviewWorkspaceNotFound
	}
	studio := h
	_, containerID, err := studio.codeContainer(ctx, true)
	if err != nil {
		return nil, err
	}
	return codeStudioLocalPreviewSource{studio: studio, containerID: containerID}, nil
}

// codeStudioLocalPreviewSource reads through today's container path. Read takes its own size limit: the preview
// cap (25 MiB) is independent of the editor's configured maximum file size.
type codeStudioLocalPreviewSource struct {
	studio      codeStudioHandlers
	containerID string
}

func (s codeStudioLocalPreviewSource) Probe(ctx context.Context, vpath string) (codeStudioFileProbe, error) {
	return s.studio.probeCodeStudioFile(ctx, s.containerID, vpath)
}

func (s codeStudioLocalPreviewSource) Read(ctx context.Context, vpath string, max int64) ([]byte, error) {
	result, err := s.studio.docker.Exec(ctx, s.containerID, codeStudioPathCommand("read", vpath, strconv.FormatInt(max, 10)), 60*time.Second)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		message := strings.TrimSpace(result.Output)
		if message == "" {
			message = "read code studio file failed"
		}
		return nil, errors.New(message)
	}
	content, err := base64.StdEncoding.DecodeString(strings.TrimSpace(result.Output))
	if err != nil {
		return nil, errors.New("invalid file content encoding")
	}
	return content, nil
}
```

**Mode A variant.** Use this instead of the Mode L block. Keep the interface and error variable; drop the local source type and the imports that become unused.

```go
// --- Mode A: Plan A's workspace backends. ------------------------------------------------------------------
func (h codeStudioHandlers) previewWorkspaceID(r *http.Request) (string, error) {
	backend, err := h.backendFor(r) // adapt to Plan A's signature
	if err != nil {
		if errors.Is(err, errCodeStudioWorkspaceNotFound) { // Plan A's sentinel
			return "", fmt.Errorf("%w: %v", errCodeStudioPreviewWorkspaceNotFound, err)
		}
		return "", err
	}
	return backend.Info().ID, nil
}

func (h codeStudioHandlers) previewSource(ctx context.Context, wsID string) (codeStudioPreviewSource, error) {
	backend, err := h.backendByID(ctx, wsID) // Plan A's lookup by workspace id; adapt the name
	if err != nil {
		if errors.Is(err, errCodeStudioWorkspaceNotFound) {
			return nil, fmt.Errorf("%w: %v", errCodeStudioPreviewWorkspaceNotFound, err)
		}
		return nil, err
	}
	return backend, nil
}
```

Check that Plan A's workspace ids match `^[A-Za-z0-9_-]{1,64}$`. If they do not, widen `codeStudioPreviewWorkspacePattern` in Task 1's file to Plan A's id alphabet; never allow `/` or `.`.

Run:
```
go test ./internal/server/ -run '(?i)(CodeStudioPreview)' -count=1 -v
```
Expected: 4 PASS lines (Mode L) and `ok`.

Commit (body names the mode and Plan A's names from Task 0):
```
git add internal/server/code_studio_preview_source.go internal/server/code_studio_preview_source_test.go
git commit -m "feat(code-studio): preview workspace source with local container fallback" -m "Mode L: Plan A backend not present; reads via codeContainer/probe/read."
```

---

## Task 5: Serve handler: headers, confinement, injection, types, errors, version endpoint (10 min)

**Test first.** Create `internal/server/code_studio_preview_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

type fakeCodeStudioPreviewSource struct {
	files map[string]string
	sizes map[string]int64
	calls []string
}

func (f *fakeCodeStudioPreviewSource) Probe(_ context.Context, vpath string) (codeStudioFileProbe, error) {
	f.calls = append(f.calls, "probe "+vpath)
	content, ok := f.files[vpath]
	if !ok {
		return codeStudioFileProbe{}, codeStudioProbeRejection{message: "not a regular file"}
	}
	size := int64(len(content))
	if override, ok := f.sizes[vpath]; ok {
		size = override
	}
	return codeStudioFileProbe{Path: vpath, Size: size, Modified: time.Unix(1710000000, 0)}, nil
}

func (f *fakeCodeStudioPreviewSource) Read(_ context.Context, vpath string, max int64) ([]byte, error) {
	f.calls = append(f.calls, "read "+vpath)
	content, ok := f.files[vpath]
	if !ok {
		return nil, errors.New("missing")
	}
	if int64(len(content)) > max {
		return nil, errors.New("file exceeds size limit")
	}
	return []byte(content), nil
}

func newTestCodeStudioPreview(t *testing.T, files map[string]string) (*codeStudioPreviewHandler, *fakeCodeStudioPreviewSource, string) {
	t.Helper()
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.SessionSecret = "preview-test-secret"
	if files == nil {
		files = map[string]string{}
	}
	source := &fakeCodeStudioPreviewSource{files: files, sizes: map[string]int64{}}
	p := &codeStudioPreviewHandler{
		studio:   codeStudioHandlers{server: s},
		versions: codeStudioPreviewVersionsFor(s),
		source: func(_ context.Context, workspace string) (codeStudioPreviewSource, error) {
			if workspace != "local" {
				return nil, errCodeStudioPreviewWorkspaceNotFound
			}
			return source, nil
		},
		now:         time.Now,
		pollTimeout: 2 * time.Second,
	}
	return p, source, testCodeStudioPreviewToken(t, s, "local")
}

func testCodeStudioPreviewToken(t *testing.T, s *Server, workspace string) string {
	t.Helper()
	key, err := s.codeStudioPreviewSigningKey()
	if err != nil {
		t.Fatalf("signing key: %v", err)
	}
	token, _, err := issueCodeStudioPreviewToken(key, workspace, time.Now())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return token
}

func serveCodeStudioPreview(p *codeStudioPreviewHandler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	p.handleServe(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestCodeStudioPreviewServesHTMLWithSandboxHeaders(t *testing.T) {
	p, _, token := newTestCodeStudioPreview(t, map[string]string{
		"/workspace/index.html": "<!doctype html><html><head><title>x</title></head><body><h1>Hi</h1></BODY></html>",
	})
	p.studio.notifyWorkspaceChange("local", "/workspace/other.css")
	handler := securityHeadersMiddleware(http.HandlerFunc(p.handleServe), false, false)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, codeStudioPreviewPrefix+token+"/index.html", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	for name, want := range map[string]string{
		"Content-Security-Policy":     codeStudioPreviewCSP,
		"X-Content-Type-Options":      "nosniff",
		"Referrer-Policy":             "no-referrer",
		"Cache-Control":               "no-store",
		"Access-Control-Allow-Origin": "*",
		"Content-Type":                "text/html; charset=utf-8",
		"X-Frame-Options":             "",
	} {
		if got := rec.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if strings.Contains(rec.Header().Get("Content-Security-Policy"), "allow-same-origin") {
		t.Fatal("the preview sandbox must not allow the same origin")
	}
	tag := `<script src="` + codeStudioPreviewPrefix + token + `/__aurago/live.js" data-aurago-version="1"></script>`
	if !strings.Contains(rec.Body.String(), "<head>"+tag+"<title>") {
		t.Fatalf("live script not injected right after <head>: %s", rec.Body.String())
	}
}

func TestCodeStudioPreviewInjectsLiveScript(t *testing.T) {
	const base = "/code-studio-preview/t/"
	htmlTag := `<script src="/code-studio-preview/t/__aurago/live.js" data-aurago-version="3"></script>`
	svgTag := `<script xmlns="http://www.w3.org/2000/svg" href="/code-studio-preview/t/__aurago/live.js" data-aurago-version="3"></script>`
	for _, tc := range []struct{ name, contentType, in, want string }{
		{"head with attributes", "text/html; charset=utf-8", `<HTML><HEAD lang="en"><title>x</title></HEAD></HTML>`, `<HTML><HEAD lang="en">` + htmlTag + `<title>x</title></HEAD></HTML>`},
		{"header is not head", "text/html; charset=utf-8", `<body><header>x</header></Body>`, `<body><header>x</header>` + htmlTag + `</Body>`},
		{"fragment", "text/html; charset=utf-8", `<p>plain</p>`, `<p>plain</p>` + htmlTag},
		{"svg", "image/svg+xml", `<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`, `<svg xmlns="http://www.w3.org/2000/svg"><rect/>` + svgTag + `</svg>`},
		{"css untouched", "text/css; charset=utf-8", `body{}`, `body{}`},
	} {
		if got := string(injectCodeStudioPreviewLiveScript([]byte(tc.in), tc.contentType, base, 3)); got != tc.want {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, got, tc.want)
		}
	}
}

func TestCodeStudioPreviewDirectoryIndexAndRedirects(t *testing.T) {
	p, _, token := newTestCodeStudioPreview(t, map[string]string{
		"/workspace/index.html":      "<body>root</body>",
		"/workspace/docs/index.html": "<body>docs</body>",
	})
	base := codeStudioPreviewPrefix + token
	for _, tc := range []struct {
		target   string
		status   int
		location string
		body     string
	}{
		{base, http.StatusTemporaryRedirect, base + "/", ""},
		{base + "/", http.StatusOK, "", "root"},
		{base + "/docs?x=1", http.StatusTemporaryRedirect, base + "/docs/?x=1", ""},
		{base + "/docs/", http.StatusOK, "", "docs"},
	} {
		rec := serveCodeStudioPreview(p, http.MethodGet, tc.target)
		if rec.Code != tc.status || rec.Header().Get("Location") != tc.location || !strings.Contains(rec.Body.String(), tc.body) {
			t.Errorf("%s: status %d location %q body %q", tc.target, rec.Code, rec.Header().Get("Location"), rec.Body.String())
		}
	}
}

func TestCodeStudioPreviewConfinesPaths(t *testing.T) {
	p, source, token := newTestCodeStudioPreview(t, map[string]string{"/workspace/index.html": "<body></body>"})
	base := codeStudioPreviewPrefix + token
	for _, target := range []string{base + "/../secret.txt", base + "/a/%2e%2e/%2e%2e/etc/passwd", base + "/a%5Cb.html", base + "/a%00.html"} {
		if rec := serveCodeStudioPreview(p, http.MethodGet, target); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", target, rec.Code)
		}
	}
	if len(source.calls) != 0 {
		t.Fatalf("source touched for rejected paths: %v", source.calls)
	}
	if rec := serveCodeStudioPreview(p, http.MethodGet, base+"/__aurago/other"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown internal path status = %d, want 404", rec.Code)
	}
	serveCodeStudioPreview(p, http.MethodGet, base+"/index.html")
	for _, call := range source.calls {
		if !strings.Contains(call, " /workspace/") {
			t.Fatalf("source call outside the workspace: %q", call)
		}
	}
}

func TestCodeStudioPreviewRejectsBadTokensAndMethods(t *testing.T) {
	p, source, token := newTestCodeStudioPreview(t, map[string]string{"/workspace/index.html": "<body></body>"})
	last := "A"
	if strings.HasSuffix(token, "A") {
		last = "B"
	}
	if rec := serveCodeStudioPreview(p, http.MethodGet, codeStudioPreviewPrefix+token[:len(token)-1]+last+"/index.html"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("tampered token status = %d, want 401", rec.Code)
	}
	p.now = func() time.Time { return time.Now().Add(13 * time.Hour) }
	rec := serveCodeStudioPreview(p, http.MethodGet, codeStudioPreviewPrefix+token+"/index.html")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "expired") {
		t.Fatalf("expired token = %d %s, want 401 expired page", rec.Code, rec.Body.String())
	}
	p.now = time.Now
	rec = serveCodeStudioPreview(p, http.MethodPost, codeStudioPreviewPrefix+token+"/index.html")
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST = %d Allow %q, want 405 GET, HEAD", rec.Code, rec.Header().Get("Allow"))
	}
	gone := testCodeStudioPreviewToken(t, p.studio.server, "ssh-gone")
	if rec := serveCodeStudioPreview(p, http.MethodGet, codeStudioPreviewPrefix+gone+"/index.html"); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "Workspace not found") {
		t.Fatalf("unknown workspace = %d %s, want 404", rec.Code, rec.Body.String())
	}
	if len(source.calls) != 0 {
		t.Fatalf("source touched without a valid local token: %v", source.calls)
	}
}

func TestCodeStudioPreviewMissingTooLargeAndContentTypes(t *testing.T) {
	p, source, token := newTestCodeStudioPreview(t, map[string]string{
		"/workspace/big.html":  "<body></body>",
		"/workspace/style.css": "body{color:red}",
		"/workspace/app.mjs":   "export const x = 1;",
		"/workspace/logo.svg":  `<svg xmlns="http://www.w3.org/2000/svg"></svg>`,
		"/workspace/data.bin":  "\x89PNG\r\n\x1a\n0000",
	})
	source.sizes["/workspace/big.html"] = codeStudioPreviewMaxBytes + 1
	base := codeStudioPreviewPrefix + token
	rec := serveCodeStudioPreview(p, http.MethodGet, base+"/%3Cimg%20src=x%3E.html")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "&lt;img src=x&gt;.html") || strings.Contains(rec.Body.String(), "<img src=x>") {
		t.Fatalf("missing file page = %d %s, want an escaped 404", rec.Code, rec.Body.String())
	}
	if rec := serveCodeStudioPreview(p, http.MethodHead, base+"/missing.html"); rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
		t.Fatalf("HEAD missing = %d with %d body bytes, want 404 without body", rec.Code, rec.Body.Len())
	}
	if rec := serveCodeStudioPreview(p, http.MethodGet, base+"/big.html"); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("big file status = %d, want 413", rec.Code)
	}
	for _, call := range source.calls {
		if call == "read /workspace/big.html" {
			t.Fatal("an oversized file was read")
		}
	}
	handler := securityHeadersMiddleware(http.HandlerFunc(p.handleServe), false, false)
	for path, want := range map[string]string{
		"/style.css": "text/css; charset=utf-8",
		"/app.mjs":   "text/javascript; charset=utf-8",
		"/logo.svg":  "image/svg+xml",
		"/data.bin":  "image/png",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, base+path, nil))
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != want || rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: %d %q cache %q, want 200 %q no-store", path, rec.Code, rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"), want)
		}
		if path == "/logo.svg" && !strings.Contains(rec.Body.String(), `href="`+base+`/__aurago/live.js"`) {
			t.Errorf("svg without live script: %s", rec.Body.String())
		}
	}
}

func TestCodeStudioPreviewVersionAndLiveScriptEndpoints(t *testing.T) {
	p, _, token := newTestCodeStudioPreview(t, nil)
	base := codeStudioPreviewPrefix + token
	rec := serveCodeStudioPreview(p, http.MethodGet, base+"/__aurago/live.js")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "text/javascript; charset=utf-8" || !strings.Contains(rec.Body.String(), "auragoPreview") {
		t.Fatalf("live.js = %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		p.studio.notifyWorkspaceChange("local", "/workspace/styles.css")
	}()
	start := time.Now()
	rec = serveCodeStudioPreview(p, http.MethodGet, base+"/__aurago/version?since=0")
	var body struct {
		Version uint64   `json:"version"`
		Paths   []string `json:"paths"`
		Full    bool     `json:"full"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Version != 1 || len(body.Paths) != 1 || body.Paths[0] != "styles.css" || body.Full {
		t.Fatalf("version = %s (%v), want 1 [styles.css] false", rec.Body.String(), err)
	}
	if time.Since(start) > time.Second || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("long poll took %v, ACAO %q", time.Since(start), rec.Header().Get("Access-Control-Allow-Origin"))
	}
	p.pollTimeout = 50 * time.Millisecond
	rec = serveCodeStudioPreview(p, http.MethodGet, base+"/__aurago/version?since=1")
	body.Paths = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Version != 1 || body.Paths == nil || len(body.Paths) != 0 {
		t.Fatalf("idle poll = %s, want version 1 with an empty path list", rec.Body.String())
	}
}
```

Run:
```
go test ./internal/server/ -run '(?i)(CodeStudioPreview)' -count=1
```
Expected: build failure, `undefined: codeStudioPreviewHandler`.

**Implement.** Create `internal/server/code_studio_preview.go`:

```go
package server

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"net/http"
	pathpkg "path"
	"strconv"
	"strings"
	"time"
)

//go:embed code_studio_preview_live.js
var codeStudioPreviewLiveScript []byte

const (
	codeStudioPreviewMaxBytes    = int64(25 << 20)
	codeStudioPreviewPollTimeout = 25 * time.Second
	codeStudioPreviewInternal    = "__aurago/"
	// The iframe carries the same flags. No allow-same-origin: the page gets an opaque origin and can never read
	// AuraGo cookies, storage or API responses, even when it is opened top-level in its own tab.
	codeStudioPreviewCSP = "sandbox allow-scripts allow-forms allow-modals allow-popups allow-downloads; frame-ancestors 'self'"
)

type codeStudioPreviewHandler struct {
	studio   codeStudioHandlers
	versions *codeStudioPreviewVersionRegistry
	// source resolves the workspace a token names; tests replace it with an in-memory fake.
	source      func(ctx context.Context, workspace string) (codeStudioPreviewSource, error)
	now         func() time.Time
	pollTimeout time.Duration
}

func newCodeStudioPreviewHandler(studio codeStudioHandlers) *codeStudioPreviewHandler {
	return &codeStudioPreviewHandler{
		studio:      studio,
		versions:    codeStudioPreviewVersionsFor(studio.server),
		source:      studio.previewSource,
		now:         time.Now,
		pollTimeout: codeStudioPreviewPollTimeout,
	}
}

func setCodeStudioPreviewHeaders(header http.Header) {
	header.Set("Content-Security-Policy", codeStudioPreviewCSP)
	// securityHeadersMiddleware sends X-Frame-Options: DENY for every path; frame-ancestors 'self' replaces it.
	header.Del("X-Frame-Options")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("Cache-Control", "no-store")
	header.Del("Pragma")
	// The sandboxed page fetches with Origin: null (live.js, module scripts, fonts). The token in the path is the
	// capability and no credentials are sent, so a wildcard is safe.
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Cross-Origin-Resource-Policy", "cross-origin")
}

func (p *codeStudioPreviewHandler) handleServe(w http.ResponseWriter, r *http.Request) {
	setCodeStudioPreviewHeaders(w.Header())
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeCodeStudioPreviewPage(w, r, http.StatusMethodNotAllowed, "Method not allowed", "The preview is read-only.")
		return
	}
	token, rel, slash := splitCodeStudioPreviewPath(r.URL.Path)
	key, err := p.studio.server.codeStudioPreviewSigningKey()
	if err != nil {
		writeCodeStudioPreviewPage(w, r, http.StatusServiceUnavailable, "Preview unavailable", "The preview key is not available.")
		return
	}
	claims, ok := parseCodeStudioPreviewToken(key, token, p.now())
	if !ok {
		writeCodeStudioPreviewPage(w, r, http.StatusUnauthorized, "Preview link expired", "This preview link has expired. Open the preview again from Code Studio.")
		return
	}
	if !slash {
		codeStudioPreviewRedirect(w, r, r.URL.EscapedPath()+"/")
		return
	}
	base := codeStudioPreviewPrefix + token + "/"
	switch rel {
	case codeStudioPreviewInternal + "live.js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(codeStudioPreviewLiveScript)
		}
		return
	case codeStudioPreviewInternal + "version":
		p.serveVersion(w, r, claims.Workspace)
		return
	}
	if strings.HasPrefix(rel, codeStudioPreviewInternal) {
		writeCodeStudioPreviewPage(w, r, http.StatusNotFound, "Not found", rel)
		return
	}
	directory := rel == "" || strings.HasSuffix(rel, "/")
	vpath, err := codeStudioPreviewVirtualPath(rel)
	if err != nil {
		writeCodeStudioPreviewPage(w, r, http.StatusBadRequest, "Invalid path", err.Error())
		return
	}
	if directory {
		vpath = pathpkg.Join(vpath, "index.html")
	}
	source, err := p.source(r.Context(), claims.Workspace)
	if err != nil {
		if errors.Is(err, errCodeStudioPreviewWorkspaceNotFound) {
			writeCodeStudioPreviewPage(w, r, http.StatusNotFound, "Workspace not found", "This workspace is no longer open in Code Studio.")
		} else {
			writeCodeStudioPreviewPage(w, r, http.StatusServiceUnavailable, "Preview unavailable", err.Error())
		}
		return
	}
	probe, err := source.Probe(r.Context(), vpath)
	if err != nil && !directory {
		// A folder without its trailing slash: redirect so relative links inside index.html resolve in the folder.
		if _, indexErr := source.Probe(r.Context(), pathpkg.Join(vpath, "index.html")); indexErr == nil {
			codeStudioPreviewRedirect(w, r, r.URL.EscapedPath()+"/")
			return
		}
	}
	display := strings.TrimPrefix(vpath, codeStudioWorkspaceRoot+"/")
	if err != nil {
		if codeStudioPreviewMissing(err) {
			writeCodeStudioPreviewPage(w, r, http.StatusNotFound, "File not found", display+" does not exist in this workspace.")
		} else {
			writeCodeStudioPreviewPage(w, r, http.StatusBadGateway, "Preview unavailable", err.Error())
		}
		return
	}
	if probe.Size > codeStudioPreviewMaxBytes {
		writeCodeStudioPreviewPage(w, r, http.StatusRequestEntityTooLarge, "File too large", display+" is larger than 25 MB.")
		return
	}
	contentType := codeStudioPreviewContentType(probe.Path)
	if r.Method == http.MethodHead {
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		return
	}
	// The version is taken before the read: a write in between makes live.js reload once more, never miss it.
	version := p.versions.Current(claims.Workspace)
	content, err := source.Read(r.Context(), probe.Path, codeStudioPreviewMaxBytes)
	if err != nil {
		writeCodeStudioPreviewPage(w, r, http.StatusBadGateway, "Preview unavailable", err.Error())
		return
	}
	if contentType == "" {
		contentType = http.DetectContentType(content)
	}
	content = injectCodeStudioPreviewLiveScript(content, contentType, base, version)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (p *codeStudioPreviewHandler) serveVersion(w http.ResponseWriter, r *http.Request, workspace string) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	since, _ := strconv.ParseUint(r.URL.Query().Get("since"), 10, 64)
	version, paths, full := p.versions.Wait(r.Context(), workspace, since, p.pollTimeout)
	if paths == nil {
		paths = []string{}
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"version": version, "paths": paths, "full": full})
}

func codeStudioPreviewMissing(err error) bool {
	var rejected codeStudioProbeRejection
	return errors.As(err, &rejected) || errors.Is(err, fs.ErrNotExist)
}

func codeStudioPreviewRedirect(w http.ResponseWriter, r *http.Request, target string) {
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	w.Header().Set("Location", target)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

// The page lives under the preview CSP sandbox too; inline styles are allowed there, scripts are not needed.
const codeStudioPreviewPageTemplate = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title><style>:root{color-scheme:light dark;font-family:system-ui,sans-serif}body{margin:0;min-height:100vh;display:grid;place-items:center;background:Canvas;color:CanvasText}main{max-width:28rem;padding:2rem;text-align:center}h1{font-size:1.15rem;margin:0 0 .5rem}p{margin:0;opacity:.75;line-height:1.5;overflow-wrap:anywhere}</style></head><body><main><h1>%s</h1><p>%s</p></main></body></html>`

func writeCodeStudioPreviewPage(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	escapedTitle := html.EscapeString(title)
	_, _ = fmt.Fprintf(w, codeStudioPreviewPageTemplate, escapedTitle, escapedTitle, html.EscapeString(detail))
}

// A fixed table instead of mime.TypeByExtension: the Windows registry maps .js to text/plain on some machines,
// which nosniff would then refuse to execute.
var codeStudioPreviewTypes = map[string]string{
	".html": "text/html; charset=utf-8", ".htm": "text/html; charset=utf-8",
	".css": "text/css; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".mjs": "text/javascript; charset=utf-8",
	".json": "application/json", ".map": "application/json", ".webmanifest": "application/manifest+json",
	".svg": "image/svg+xml", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
	".webp": "image/webp", ".avif": "image/avif", ".ico": "image/x-icon", ".bmp": "image/bmp",
	".woff": "font/woff", ".woff2": "font/woff2", ".ttf": "font/ttf", ".otf": "font/otf",
	".wasm": "application/wasm", ".mp3": "audio/mpeg", ".wav": "audio/wav", ".ogg": "audio/ogg",
	".mp4": "video/mp4", ".webm": "video/webm", ".txt": "text/plain; charset=utf-8", ".md": "text/plain; charset=utf-8",
	".csv": "text/csv; charset=utf-8", ".xml": "application/xml", ".pdf": "application/pdf",
}

func codeStudioPreviewContentType(vpath string) string {
	return codeStudioPreviewTypes[strings.ToLower(pathpkg.Ext(vpath))]
}

// injectCodeStudioPreviewLiveScript adds live.js to HTML (right after <head>, so it also sees errors of the page's
// own scripts; else before </body>; else at the end) and to SVG documents (before </svg>).
func injectCodeStudioPreviewLiveScript(content []byte, contentType, base string, version uint64) []byte {
	src := base + codeStudioPreviewInternal + "live.js"
	versionText := strconv.FormatUint(version, 10)
	switch {
	case strings.HasPrefix(contentType, "text/html"):
		tag := []byte(`<script src="` + src + `" data-aurago-version="` + versionText + `"></script>`)
		if at := codeStudioPreviewHeadEnd(content); at >= 0 {
			return codeStudioPreviewSplice(content, at, tag)
		}
		if at := codeStudioPreviewLastIndexFold(content, "</body>"); at >= 0 {
			return codeStudioPreviewSplice(content, at, tag)
		}
		return append(append([]byte{}, content...), tag...)
	case strings.HasPrefix(contentType, "image/svg+xml"):
		tag := []byte(`<script xmlns="http://www.w3.org/2000/svg" href="` + src + `" data-aurago-version="` + versionText + `"></script>`)
		if at := codeStudioPreviewLastIndexFold(content, "</svg>"); at >= 0 {
			return codeStudioPreviewSplice(content, at, tag)
		}
	}
	return content
}

// codeStudioPreviewHeadEnd returns the offset just after the first <head> start tag, or -1. "<header" does not count.
func codeStudioPreviewHeadEnd(content []byte) int {
	for from := 0; from < len(content); {
		at := codeStudioPreviewIndexFold(content[from:], "<head")
		if at < 0 {
			return -1
		}
		next := from + at + len("<head")
		if next < len(content) {
			switch content[next] {
			case '>', ' ', '\t', '\n', '\r', '/':
				if end := bytes.IndexByte(content[next:], '>'); end >= 0 {
					return next + end + 1
				}
				return -1
			}
		}
		from = next
	}
	return -1
}

func codeStudioPreviewEqualFold(window []byte, needle string) bool {
	for i := 0; i < len(needle); i++ {
		c := window[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != needle[i] {
			return false
		}
	}
	return true
}

// ASCII-only folding keeps byte offsets stable (bytes.ToLower may change the length of non-ASCII text).
func codeStudioPreviewIndexFold(haystack []byte, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if codeStudioPreviewEqualFold(haystack[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

func codeStudioPreviewLastIndexFold(haystack []byte, needle string) int {
	for i := len(haystack) - len(needle); i >= 0; i-- {
		if codeStudioPreviewEqualFold(haystack[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

func codeStudioPreviewSplice(content []byte, at int, insert []byte) []byte {
	out := make([]byte, 0, len(content)+len(insert))
	out = append(out, content[:at]...)
	out = append(out, insert...)
	return append(out, content[at:]...)
}
```

Run:
```
go test ./internal/server/ -run '(?i)(CodeStudioPreview)' -count=1 -v
```
Expected: 11 PASS lines (Mode L) and `ok`.

Commit:
```
git add internal/server/code_studio_preview.go internal/server/code_studio_preview_test.go
git commit -m "feat(code-studio): sandboxed preview route with live script injection and version long-poll"
```

---

## Task 6: Issue endpoint, route registration, auth exemption (8 min)

**Test first.** Create `internal/server/code_studio_preview_routes_test.go`:

```go
package server

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestCodeStudioPreviewIssueReturnsWorkspaceScopedURL(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	s.Cfg.VirtualDesktop.ReadOnly = true // a preview only reads: it must work on a read-only desktop
	p := newCodeStudioPreviewHandler(codeStudioHandlers{server: s})
	issue := func(method, body, workspace string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/code-studio/preview", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if workspace != "" {
			req.Header.Set("X-Code-Studio-Workspace", workspace)
		}
		rec := httptest.NewRecorder()
		p.handleIssue(rec, req)
		return rec
	}
	rec := issue(http.MethodPost, `{"path":"/workspace/site/my page.html"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("issue = %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		URL       string `json:"url"`
		Base      string `json:"base"`
		Token     string `json:"token"`
		Workspace string `json:"workspace"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Base != codeStudioPreviewPrefix+got.Token+"/" || got.URL != got.Base+"site/my%20page.html" || got.Workspace != "local" || got.ExpiresAt == "" {
		t.Fatalf("issue body = %+v", got)
	}
	key, _ := s.codeStudioPreviewSigningKey()
	claims, ok := parseCodeStudioPreviewToken(key, got.Token, time.Now())
	if !ok || claims.Workspace != "local" || time.Until(time.Unix(claims.Expires, 0)) < 11*time.Hour+59*time.Minute {
		t.Fatalf("claims = %+v %v, want local for 12 h", claims, ok)
	}
	if rec := issue(http.MethodPost, `{"path":"/workspace/index.html"}`, "ssh-missing"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown workspace = %d, want 404", rec.Code)
	}
	if rec := issue(http.MethodPost, `{"path":"/etc/passwd"}`, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("outside path = %d, want 400", rec.Code)
	}
	if rec := issue(http.MethodGet, "", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d, want 405", rec.Code)
	}
}

// Mode L: the real local source reads through the container with the 25 MiB preview cap, not the 1 MiB editor cap.
// Mode A: keep it when Plan A's local backend still runs through h.docker; otherwise drop it.
func TestCodeStudioPreviewLocalSourceReadsThroughContainer(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{
		{ExitCode: 0, Output: "13|1710000000\n/workspace/index.html\x00"},
		{ExitCode: 0, Output: base64.StdEncoding.EncodeToString([]byte("<body></body>"))},
	}}
	p := newCodeStudioPreviewHandler(codeStudioHandlers{server: s, docker: docker})
	rec := serveCodeStudioPreview(p, http.MethodGet, codeStudioPreviewPrefix+testCodeStudioPreviewToken(t, s, "local")+"/index.html")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "__aurago/live.js") {
		t.Fatalf("local preview = %d %s", rec.Code, rec.Body.String())
	}
	if len(docker.commands) != 2 || docker.commands[0][4] != "probe" || docker.commands[1][4] != "read" || docker.commands[1][6] != "26214400" {
		t.Fatalf("commands = %q, want probe then read with the 25 MiB cap", docker.commands)
	}
}

func TestCodeStudioPreviewRouteIsRegisteredAndExemptFromSessionAuth(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	s.Cfg.Auth.PasswordHash = "configured"
	mux := http.NewServeMux()
	registerCodeStudioRoutes(mux, s)
	for path, want := range map[string]string{
		"/code-studio-preview/tok/index.html": codeStudioPreviewPrefix,
		"/api/code-studio/preview":            "/api/code-studio/preview",
	} {
		if _, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, path, nil)); pattern != want {
			t.Fatalf("%s routed to %q, want %q", path, pattern, want)
		}
	}
	handler := authMiddleware(s, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for path, exempt := range map[string]bool{
		"/code-studio-preview/tok/index.html":        true,
		"/code-studio-preview/tok/__aurago/version":  true,
		"/code-studio-previewx/tok/index.html":       false,
		"/api/code-studio/preview":                   false,
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if exempt != (rec.Code == http.StatusNoContent) {
			t.Errorf("%s: status %d, exempt=%v", path, rec.Code, exempt)
		}
	}
}
```

Run:
```
go test ./internal/server/ -run '(?i)(CodeStudioPreview)' -count=1
```
Expected: build failure, `p.handleIssue undefined`.

**Implement.**

1. Append to `internal/server/code_studio_preview.go`:

```go
func registerCodeStudioPreviewRoutes(mux *http.ServeMux, studio codeStudioHandlers) {
	preview := newCodeStudioPreviewHandler(studio)
	mux.HandleFunc("/api/code-studio/preview", preview.handleIssue)
	mux.HandleFunc(codeStudioPreviewPrefix, preview.handleServe)
}

// handleIssue hands out the workspace-scoped preview token. It is a read: a read-only desktop still previews, so
// the operation is desktopRead even though the method is POST.
func (p *codeStudioPreviewHandler) handleIssue(w http.ResponseWriter, r *http.Request) {
	if !requireDesktopOperation(p.studio.server, w, r, desktopScopeRead, desktopRead) {
		return
	}
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := decodeJSONRequest(w, r, &body, p.studio.server); err != nil {
		jsonError(w, "Invalid JSON", jsonDecodeStatus(err))
		return
	}
	vpath, err := sanitizeCodeStudioPath(body.Path)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	workspace, err := p.studio.previewWorkspaceID(r)
	if err != nil {
		if errors.Is(err, errCodeStudioPreviewWorkspaceNotFound) {
			jsonError(w, "workspace_not_found", http.StatusNotFound)
		} else {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
		}
		return
	}
	key, err := p.studio.server.codeStudioPreviewSigningKey()
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	token, expires, err := issueCodeStudioPreviewToken(key, workspace, p.now())
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	base := codeStudioPreviewPrefix + token + "/"
	rel := strings.TrimPrefix(strings.TrimPrefix(vpath, codeStudioWorkspaceRoot), "/")
	writeJSON(w, map[string]interface{}{
		"status":     "ok",
		"url":        base + codeStudioPreviewEscapePath(rel),
		"base":       base,
		"token":      token,
		"path":       vpath,
		"workspace":  workspace,
		"expires_at": expires.UTC().Format(time.RFC3339),
		"version":    p.versions.Current(workspace),
	})
}
```

2. Edit `registerCodeStudioRoutes` in `internal/server/code_studio_handlers.go`. After `mux.HandleFunc("/api/code-studio/git/blame", handlers.handleGitBlame)`, add:
```go
	registerCodeStudioPreviewRoutes(mux, handlers) // Plan B: HTML preview (token route is auth-exempt, see authBypassSubtrees)
```

3. Edit `authBypassSubtrees` in `internal/server/auth.go`. After the `"/api/game-maker/preview/",` line, add:
```go
	codeStudioPreviewPrefix,          // Code Studio HTML preview — the stateless workspace token in the path is validated by the handler
```

Run:
```
go test ./internal/server/ -run '(?i)(CodeStudioPreview|AuthBypass)' -count=1 -v
```
Expected: 14 `--- PASS: TestCodeStudioPreview…` lines (Mode L), plus the `TestAuthBypass…` tests passing (the "subtrees end with slash" test included). Ends with `ok`.

Commit:
```
git add internal/server/code_studio_preview.go internal/server/code_studio_preview_routes_test.go internal/server/code_studio_handlers.go internal/server/auth.go
git commit -m "feat(code-studio): preview token endpoint, route registration and auth exemption"
```

---

## Task 7: Version bumps at the mutation points (6 min)

**Mode A.** Grep for `notifyWorkspaceChange(` in `internal/server`.
- If Plan A calls it after write, move, delete and upload, add only the git discard and branch switch bumps below. Use Plan A's way of getting the workspace id.
- If any of those four calls is missing, add it the same way as Mode L.

**Mode L.**

1. Append to `internal/server/code_studio_preview_versions.go` (add `"net/http"` to its imports):
```go
// notifyRequestWorkspaceChange bumps the preview version of the workspace this request names.
func (h codeStudioHandlers) notifyRequestWorkspaceChange(r *http.Request, paths ...string) {
	if wsID, err := h.previewWorkspaceID(r); err == nil {
		h.notifyWorkspaceChange(wsID, paths...)
	}
}
```

2. In `internal/server/code_studio_handlers.go`, insert one line directly before each success `writeJSON`:
   - `handleWriteFile`, before `writeJSON(w, map[string]interface{}{"status": "ok", "path": path})`:
     `h.notifyRequestWorkspaceChange(r, path) // Plan B: live preview`
   - `handleMoveFile`, before `writeJSON(... "old_path": oldPath, "new_path": newPath})`:
     `h.notifyRequestWorkspaceChange(r, oldPath, newPath)`
   - `handleDeleteFile`, before its success `writeJSON`:
     `h.notifyRequestWorkspaceChange(r, path)`
   - `writeUploadedFile`, before its success `writeJSON`:
     `h.notifyRequestWorkspaceChange(r, path)`
   - `handleGitBranch`, `case http.MethodPost:` after `h.mutateGitBranch(w, r)`:
     `h.notifyRequestWorkspaceChange(r) // a branch switch can change any file`

3. In `internal/server/code_studio_git_workbench.go`, `handleGitDiscard`, after `h.runGitSteps(w, r, containerID, steps, "git discard failed")`, add:
```go
	h.notifyRequestWorkspaceChange(r) // restored or cleaned files change the preview
```

Not bumped, on purpose:
- `exec` (Run / diagnostic checks): bumping there would reload the preview on every background check.
- Terminal edits.

Both are documented limitations; the Reload button covers them. Plan D adds the bumps for agent writes.

Run:
```
go test ./internal/server/ -run '(?i)(CodeStudio)' -count=1
```
Expected: `ok  aurago/internal/server`. All existing Code Studio handler tests still pass; the bump is in memory only.

Commit:
```
git add internal/server/code_studio_preview_versions.go internal/server/code_studio_handlers.go internal/server/code_studio_git_workbench.go
git commit -m "feat(code-studio): bump the preview version on writes, moves, deletes, uploads and git discard/switch"
```

---

## Task 8: `preview-model.js` pure helpers + node test (8 min)

**Test first.** Create `ui/js/desktop/apps/code-studio/preview-model.test.cjs`:

```js
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

// package.json has "type": "module": run the UMD source in a vm with a fake module, like lsp-protocol.test.cjs.
const mod = { exports: {} };
vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'preview-model.js'), 'utf8'), { module: mod, window: {} });
const M = mod.exports;
const json = value => JSON.parse(JSON.stringify(value));

test('only HTML, HTM and SVG files are previewable', () => {
    for (const p of ['/workspace/index.html', '/workspace/a/B.HTM', '/workspace/logo.svg']) assert.equal(M.isPreviewablePath(p), true, p);
    for (const p of ['/workspace/app.js', '/workspace/html', '/workspace/.html', '', null]) assert.equal(M.isPreviewablePath(p), false, String(p));
});

test('relative paths strip the workspace root', () => {
    assert.equal(M.relativePath('/workspace'), '');
    assert.equal(M.relativePath('/workspace/site/index.html'), 'site/index.html');
    assert.equal(M.relativePath('/etc/passwd'), null);
});

test('addresses stay inside the workspace', () => {
    assert.deepEqual(json(M.parseAddress('site/index.html?x=1#top')), { path: '/workspace/site/index.html', directory: false, suffix: '?x=1#top' });
    assert.deepEqual(json(M.parseAddress('/workspace/site/')), { path: '/workspace/site', directory: true, suffix: '' });
    assert.deepEqual(json(M.parseAddress('/')), { path: '/workspace', directory: true, suffix: '' });
    assert.deepEqual(json(M.parseAddress('./docs/my%20page.html')), { path: '/workspace/docs/my page.html', directory: false, suffix: '' });
    for (const bad of ['', '   ', '../secret.html', 'a/../../b', 'https://example.com/', '//evil/x', 'javascript:alert(1)', 'a\\b.html', 'a%2Fb.html', 'a/%2e%2e/b']) {
        assert.equal(M.parseAddress(bad), null, bad);
    }
});

test('preview URLs encode every segment and the address shows the relative path', () => {
    assert.equal(M.buildUrl('/code-studio-preview/t/', '/workspace/docs/my page#1.html', false, '?q=1'), '/code-studio-preview/t/docs/my%20page%231.html?q=1');
    assert.equal(M.buildUrl('/code-studio-preview/t/', '/workspace/docs', true, ''), '/code-studio-preview/t/docs/');
    assert.equal(M.buildUrl('/code-studio-preview/t/', '/workspace', true, ''), '/code-studio-preview/t/');
    assert.equal(M.addressText('/workspace/docs', true, '?a=1'), 'docs/?a=1');
    assert.equal(M.addressText('/workspace/index.html', false, ''), 'index.html');
});

test('the pane width leaves room for the editor', () => {
    assert.equal(M.clampWidth(100, 1200), 280);
    assert.equal(M.clampWidth(5000, 1200), 880);
    assert.equal(M.clampWidth('x', 1200), 480);
    assert.equal(M.clampWidth(600, 0), 600);
});

test('device frames scale down to fit the stage', () => {
    assert.equal(M.deviceScale('responsive', 300, 300), 1);
    assert.equal(M.deviceScale('mobile', 1000, 1000), 1);
    assert.equal(M.deviceScale('mobile', 348, 1000, 24), 0.8);
    assert.equal(M.deviceScale('tablet', 2000, 560, 24), 0.5);
    assert.equal(M.deviceScale('mobile', 50, 50), 0.25);
});

test('messages from the preview need the exact shape', () => {
    assert.deepEqual(json(M.normalizeMessage({ auragoPreview: 1, kind: 'console', text: 'boom' })), { kind: 'console', text: 'boom' });
    assert.deepEqual(json(M.normalizeMessage({ auragoPreview: 1, kind: 'location' })), { kind: 'location', text: '' });
    assert.equal(M.normalizeMessage({ auragoPreview: 1, kind: 'console', text: 'x'.repeat(5000) }).text.length, 4000);
    for (const bad of [null, 'text', { kind: 'console', text: 'x' }, { auragoPreview: '1', kind: 'console', text: 'x' },
        { auragoPreview: 1, kind: 'eval', text: 'x' }, { auragoPreview: 1, kind: 'toString', text: 'x' },
        { auragoPreview: 1, kind: 'console', text: { toString() { return 'x'; } } }]) {
        assert.equal(M.normalizeMessage(bad), null);
    }
});

test('the console keeps the newest entries', () => {
    let list = [];
    for (let i = 0; i < 205; i++) list = M.pushEntry(list, { kind: 'console', text: String(i) });
    assert.equal(list.length, 200);
    assert.equal(list[0].text, '5');
    assert.equal(M.isErrorKind('resource'), true);
    assert.equal(M.isErrorKind('location'), false);
});

test('tokens are refreshed ten minutes before they expire', () => {
    const now = Date.parse('2026-11-25T10:00:00Z');
    assert.equal(M.tokenNeedsRefresh('2026-11-25T22:00:00Z', now), false);
    assert.equal(M.tokenNeedsRefresh('2026-11-25T10:05:00Z', now), true);
    assert.equal(M.tokenNeedsRefresh('', now), true);
});
```

Run:
```
node --test ui/js/desktop/apps/code-studio/preview-model.test.cjs
```
Expected: fails with `ENOENT … preview-model.js`.

**Implement.** Create `ui/js/desktop/apps/code-studio/preview-model.js`:

```js
/* Code Studio HTML preview helpers (Plan B): previewable files, workspace-relative addresses, preview URLs, the
   pane width, device scaling and the message shape of the sandboxed preview. Pure functions without DOM access.
   The same file runs inside the code-studio bundle (window.CodeStudioPreviewModel) and under node
   (preview-model.test.cjs). */
(function (root, factory) {
    'use strict';
    var api = factory();
    if (typeof module === 'object' && module && module.exports) module.exports = api;
    if (root) root.CodeStudioPreviewModel = api;
})(typeof window !== 'undefined' ? window : null, function () {
    'use strict';

    var WORKSPACE_ROOT = '/workspace';
    var PREVIEWABLE = { html: true, htm: true, svg: true };
    var VIEWPORTS = { tablet: { width: 768, height: 1024 }, mobile: { width: 375, height: 812 } };
    var MIN_WIDTH = 280;
    var MIN_EDITOR_WIDTH = 320;
    var DEFAULT_WIDTH = 480;
    var NO_CONTAINER_MAX = 1600;
    var MESSAGE_KINDS = { console: true, error: true, rejection: true, resource: true, location: true, reloaded: true, expired: true };
    var ERROR_KINDS = { console: true, error: true, rejection: true, resource: true };
    var MAX_TEXT = 4000;
    var MAX_ENTRIES = 200;
    var REFRESH_MARGIN_MS = 10 * 60 * 1000;

    function own(map, key) {
        return Object.prototype.hasOwnProperty.call(map, key);
    }

    function extensionOf(path) {
        var name = String(path == null ? '' : path).split('?')[0].split('#')[0].split('/').pop();
        var dot = name.lastIndexOf('.');
        return dot > 0 ? name.slice(dot + 1).toLowerCase() : '';
    }

    function isPreviewablePath(path) {
        return own(PREVIEWABLE, extensionOf(path));
    }

    function relativePath(vpath) {
        var value = String(vpath == null ? '' : vpath);
        if (value === WORKSPACE_ROOT) return '';
        if (value.indexOf(WORKSPACE_ROOT + '/') === 0) return value.slice(WORKSPACE_ROOT.length + 1);
        return null;
    }

    function safeDecode(segment) {
        try { return decodeURIComponent(segment); } catch (_) { return null; }
    }

    // An address typed into the preview toolbar or reported by live.js: a workspace-relative path with an optional
    // ?query and #hash. Returns { path, directory, suffix } or null when it is empty, names another origin or scheme,
    // or contains a ".." segment (also percent-encoded).
    function parseAddress(input) {
        var raw = String(input == null ? '' : input).trim();
        if (!raw || raw.indexOf('\\') >= 0 || raw.indexOf('//') === 0 || /^[a-z][a-z0-9+.-]*:/i.test(raw)) return null;
        var cut = raw.search(/[?#]/);
        var pathPart = cut >= 0 ? raw.slice(0, cut) : raw;
        var suffix = cut >= 0 ? raw.slice(cut) : '';
        if (pathPart === WORKSPACE_ROOT || pathPart.indexOf(WORKSPACE_ROOT + '/') === 0) pathPart = pathPart.slice(WORKSPACE_ROOT.length);
        var directory = pathPart === '' || pathPart.charAt(pathPart.length - 1) === '/';
        var parts = [];
        var segments = pathPart.split('/');
        for (var i = 0; i < segments.length; i++) {
            var segment = safeDecode(segments[i]);
            if (segment === null || segment.indexOf('/') >= 0 || segment.indexOf('\\') >= 0 || segment.indexOf('\0') >= 0) return null;
            if (segment === '' || segment === '.') continue;
            if (segment === '..') return null;
            parts.push(segment);
        }
        return { path: WORKSPACE_ROOT + (parts.length ? '/' + parts.join('/') : ''), directory: directory || parts.length === 0, suffix: suffix };
    }

    function addressText(vpath, directory, suffix) {
        var rel = relativePath(vpath) || '';
        return rel + (directory && rel ? '/' : '') + String(suffix || '');
    }

    function encodePath(rel) {
        return rel ? rel.split('/').map(encodeURIComponent).join('/') : '';
    }

    function buildUrl(base, vpath, directory, suffix) {
        var rel = relativePath(vpath) || '';
        return String(base || '') + encodePath(rel) + (directory && rel ? '/' : '') + String(suffix || '');
    }

    // The pane may grow until the editor keeps MIN_EDITOR_WIDTH of the space both share (available).
    function clampWidth(width, available) {
        var numeric = Number(width);
        if (!Number.isFinite(numeric)) numeric = DEFAULT_WIDTH;
        var max = available > 0 ? Math.max(MIN_WIDTH, available - MIN_EDITOR_WIDTH) : NO_CONTAINER_MAX;
        return Math.round(Math.min(max, Math.max(MIN_WIDTH, numeric)));
    }

    function deviceScale(viewport, stageWidth, stageHeight, inset) {
        var size = own(VIEWPORTS, viewport) ? VIEWPORTS[viewport] : null;
        if (!size) return 1;
        var pad = inset == null ? 24 : inset;
        var scale = Math.min(1, (stageWidth - 2 * pad) / size.width, (stageHeight - 2 * pad) / size.height);
        if (!Number.isFinite(scale)) return 1;
        return Math.max(0.25, Math.round(scale * 100) / 100);
    }

    function normalizeMessage(data) {
        if (!data || typeof data !== 'object' || data.auragoPreview !== 1) return null;
        if (typeof data.kind !== 'string' || !own(MESSAGE_KINDS, data.kind)) return null;
        if (data.text !== undefined && typeof data.text !== 'string') return null;
        return { kind: data.kind, text: (data.text || '').slice(0, MAX_TEXT) };
    }

    function isErrorKind(kind) {
        return own(ERROR_KINDS, String(kind));
    }

    function pushEntry(list, entry, max) {
        var next = Array.isArray(list) ? list.slice() : [];
        next.push(entry);
        var limit = max || MAX_ENTRIES;
        return next.length > limit ? next.slice(next.length - limit) : next;
    }

    function tokenNeedsRefresh(expiresAt, now) {
        var expires = Date.parse(String(expiresAt || ''));
        return !Number.isFinite(expires) || expires - now < REFRESH_MARGIN_MS;
    }

    return {
        WORKSPACE_ROOT: WORKSPACE_ROOT,
        VIEWPORTS: VIEWPORTS,
        isPreviewablePath: isPreviewablePath,
        relativePath: relativePath,
        parseAddress: parseAddress,
        addressText: addressText,
        buildUrl: buildUrl,
        clampWidth: clampWidth,
        deviceScale: deviceScale,
        normalizeMessage: normalizeMessage,
        isErrorKind: isErrorKind,
        pushEntry: pushEntry,
        tokenNeedsRefresh: tokenNeedsRefresh
    };
});
```

Run:
```
node --test ui/js/desktop/apps/code-studio/preview-model.test.cjs
```
Expected: `# tests 9`, `# pass 9`, `# fail 0`.

Commit:
```
git add ui/js/desktop/apps/code-studio/preview-model.js ui/js/desktop/apps/code-studio/preview-model.test.cjs
git commit -m "feat(code-studio): pure preview helpers for addresses, URLs, sizing and message validation"
```

---

## Task 9: `preview.js` part 1: state, pane markup, wiring (10 min)

Create `ui/js/desktop/apps/code-studio/preview.js`. It is a fragment inside the bundle IIFE (like `settings.js`): declarations only, nothing runs at load time.

```js
    // === Plan B: HTML preview pane ===
    // The pane sits right of the editor group and shows the saved file through /code-studio-preview/<token>/. The page
    // runs sandboxed without allow-same-origin (the server sends the same flags as a CSP sandbox), so it has an opaque
    // origin and can never read AuraGo cookies, storage or API responses. It reaches Code Studio only via postMessage;
    // a message counts only when its source is this pane's iframe, its origin is opaque and its shape matches.

    const PREVIEW_PREFS_KEY = 'aurago.codeStudio.preview.v1';
    const PREVIEW_SANDBOX = 'allow-scripts allow-forms allow-modals allow-popups allow-downloads';
    const PREVIEW_VIEWPORTS = ['responsive', 'tablet', 'mobile'];
    const PREVIEW_RESIZE_STEP = 16;
    const PREVIEW_DEFAULT_WIDTH = 480;
    const PREVIEW_DEVICE_INSET = 34; // stage padding 24 px + device bezel 10 px
    const PREVIEW_GLYPHS = {
        eye: '<path d="M1.5 8S4 3.5 8 3.5 14.5 8 14.5 8 12 12.5 8 12.5 1.5 8 1.5 8z"/><circle cx="8" cy="8" r="2"/>',
        reload: '<path d="M13.25 8A5.25 5.25 0 1 1 11.7 4.3"/><path d="M13.25 2.25v3.5h-3.5"/>',
        globe: '<circle cx="8" cy="8" r="6"/><path d="M2 8h12M8 2c1.8 1.7 2.7 3.7 2.7 6S9.8 12.3 8 14C6.2 12.3 5.3 10.3 5.3 8S6.2 3.7 8 2z"/>',
        responsive: '<rect x="1.5" y="2.75" width="13" height="8.5" rx="1.25"/><path d="M5.5 13.75h5M8 11.25v2.5"/>',
        tablet: '<rect x="3" y="1.5" width="10" height="13" rx="1.5"/><path d="M7.25 12.25h1.5"/>',
        mobile: '<rect x="4.5" y="1.5" width="7" height="13" rx="1.5"/><path d="M7.25 12.25h1.5"/>',
        alert: '<circle cx="8" cy="8" r="6"/><path d="M8 4.75v3.75M8 11.1v.15"/>',
        pin: '<path d="M9.75 1.75l4.5 4.5-1.9.6-2.6 2.6.3 3-1.3 1.3-6.5-6.5 1.3-1.3 3 .3 2.6-2.6z"/><path d="M5.2 10.8 1.75 14.25"/>',
        external: '<path d="M9.25 2.25h4.5v4.5M13.75 2.25 7.5 8.5"/><path d="M12 9.5v3.25a1 1 0 0 1-1 1H3.25a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1H6.5"/>',
        close: '<path d="M4 4l8 8M12 4l-8 8"/>'
    };

    function previewModel() {
        return window.CodeStudioPreviewModel || null;
    }

    function previewGlyph(name, size) {
        const pixels = Number(size) || 14;
        return `<svg class="cs-hp-glyph" viewBox="0 0 16 16" width="${pixels}" height="${pixels}" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">${PREVIEW_GLYPHS[name] || ''}</svg>`;
    }

    function loadPreviewPrefs() {
        try {
            const saved = JSON.parse(localStorage.getItem(PREVIEW_PREFS_KEY) || '{}') || {};
            const width = Number(saved.width);
            return {
                viewport: PREVIEW_VIEWPORTS.includes(saved.viewport) ? saved.viewport : 'responsive',
                width: Number.isFinite(width) && width > 0 ? width : PREVIEW_DEFAULT_WIDTH
            };
        } catch (_) {
            return { viewport: 'responsive', width: PREVIEW_DEFAULT_WIDTH };
        }
    }

    function savePreviewPrefs() {
        const preview = previewState();
        try {
            localStorage.setItem(PREVIEW_PREFS_KEY, JSON.stringify({ viewport: preview.viewport, width: Math.round(preview.width) }));
        } catch (_) {}
    }

    function previewState() {
        if (!state.htmlPreview) {
            const prefs = loadPreviewPrefs();
            state.htmlPreview = {
                open: false, pinned: false, followedPath: '',
                path: '', directory: false, suffix: '',
                base: '', expiresAt: '', tokenRequest: null,
                status: 'empty', errorTitle: '', errorDetail: '',
                viewport: prefs.viewport, width: prefs.width,
                console: [], consoleOpen: false,
                navigation: 0, pane: null, observer: null, messageWired: false
            };
        }
        return state.htmlPreview;
    }

    function previewPane() {
        const root = studioRoot();
        return root ? root.querySelector('[data-html-preview]') : null;
    }

    function isPreviewablePath(path) {
        const model = previewModel();
        return !!model && model.isPreviewablePath(path);
    }

    function isCodeEditorTarget(element) {
        return !!(element && typeof element.closest === 'function' && element.closest('.cm-editor'));
    }

    // Called by renderShell and whenever the pane opens or closes. Returns true when it (re)built the pane markup.
    function renderPreviewPane(options) {
        const root = studioRoot();
        const pane = previewPane();
        if (!root || !pane) return false;
        const preview = previewState();
        root.dataset.preview = preview.open ? 'visible' : 'hidden';
        pane.hidden = !preview.open;
        applyPreviewWidth();
        if (!preview.open) return false;
        let built = false;
        if (preview.pane !== pane) {
            buildPreviewPane(pane);
            built = true;
        }
        updatePreviewChrome();
        if (built) {
            if (preview.path && !(options && options.navigate === false)) navigatePreview();
            else if (!preview.path) setPreviewStatus('empty');
        }
        return built;
    }

    function buildPreviewPane(pane) {
        const preview = previewState();
        const consoleId = 'cs-hp-console-' + String(state.windowId).replace(/[^A-Za-z0-9_-]/g, '');
        const t = (key, fallback) => esc(tr(key, fallback));
        const viewportLabels = {
            responsive: tr('codeStudio.previewResponsive', 'Responsive'),
            tablet: tr('codeStudio.previewTablet', 'Tablet (768 px)'),
            mobile: tr('codeStudio.previewMobile', 'Mobile (375 px)')
        };
        pane.setAttribute('aria-label', tr('codeStudio.previewTitle', 'Preview'));
        pane.innerHTML = `
            <div class="cs-hp-resize" data-hp-resize role="separator" aria-orientation="vertical" tabindex="0" aria-label="${t('codeStudio.previewResize', 'Resize preview')}" title="${t('codeStudio.previewResize', 'Resize preview')}"></div>
            <div class="cs-hp-toolbar" role="toolbar" aria-label="${t('codeStudio.previewTitle', 'Preview')}">
                <button type="button" class="cs-hp-button" data-hp-action="reload" aria-label="${t('codeStudio.previewReload', 'Reload preview')}" title="${t('codeStudio.previewReload', 'Reload preview')}">${previewGlyph('reload')}</button>
                <form class="cs-hp-address" data-hp-address-form>
                    <span class="cs-hp-address-icon" aria-hidden="true">${previewGlyph('globe', 13)}</span>
                    <input type="text" class="cs-hp-address-input" data-hp-address spellcheck="false" autocomplete="off" placeholder="index.html" aria-label="${t('codeStudio.previewAddress', 'Preview address (path inside the workspace)')}">
                </form>
                <div class="cs-hp-segmented" role="radiogroup" data-hp-viewports aria-label="${t('codeStudio.previewViewport', 'Viewport')}">
                    ${PREVIEW_VIEWPORTS.map(id => `<button type="button" role="radio" class="cs-hp-segment" data-hp-viewport="${id}" aria-checked="false" tabindex="-1" aria-label="${esc(viewportLabels[id])}" title="${esc(viewportLabels[id])}">${previewGlyph(id)}</button>`).join('')}
                </div>
                <button type="button" class="cs-hp-button cs-hp-console-button" data-hp-action="console" aria-expanded="false" aria-controls="${consoleId}" title="${t('codeStudio.previewConsole', 'Console')}">${previewGlyph('alert')}<span class="cs-hp-badge" data-hp-console-count hidden>0</span></button>
                <button type="button" class="cs-hp-button" data-hp-action="pin" aria-pressed="false" aria-label="${t('codeStudio.previewPin', 'Keep showing this file')}">${previewGlyph('pin')}</button>
                <button type="button" class="cs-hp-button" data-hp-action="external" aria-label="${t('codeStudio.previewOpenExternal', 'Open in new browser tab')}" title="${t('codeStudio.previewOpenExternal', 'Open in new browser tab')}">${previewGlyph('external')}</button>
                <button type="button" class="cs-hp-button" data-hp-action="close" aria-label="${t('codeStudio.previewClose', 'Close preview')}" title="${t('codeStudio.previewClose', 'Close preview')}">${previewGlyph('close')}</button>
            </div>
            <div class="cs-hp-hint" data-hp-dirty role="status" hidden><span class="cs-hp-hint-dot" aria-hidden="true"></span><span>${t('codeStudio.previewDirty', 'Showing saved version. Save to update the preview.')}</span></div>
            <section class="cs-hp-console" id="${consoleId}" data-hp-console hidden aria-label="${t('codeStudio.previewConsole', 'Console')}">
                <header class="cs-hp-console-head"><span>${t('codeStudio.previewConsole', 'Console')}</span><button type="button" class="cs-hp-text-button" data-hp-action="clear-console">${t('codeStudio.previewConsoleClear', 'Clear console')}</button></header>
                <ol class="cs-hp-console-list" data-hp-console-list></ol>
            </section>
            <div class="cs-hp-stage" data-hp-stage data-viewport="${esc(preview.viewport)}" aria-busy="false">
                <div class="cs-hp-device" data-hp-device>
                    <iframe class="cs-hp-frame" data-hp-frame sandbox="${PREVIEW_SANDBOX}" referrerpolicy="no-referrer" title="${t('codeStudio.previewTitle', 'Preview')}"></iframe>
                </div>
                <div class="cs-hp-device-label" data-hp-device-label hidden></div>
                <div class="cs-hp-loading" data-hp-loading hidden>
                    <span class="cs-hp-shimmer"></span><span class="cs-hp-shimmer short"></span><span class="cs-hp-shimmer"></span><span class="cs-hp-shimmer short"></span>
                    <span class="cs-hp-sr">${t('codeStudio.previewLoading', 'Loading preview...')}</span>
                </div>
                <div class="cs-hp-card" data-hp-empty hidden>
                    <span class="cs-hp-card-icon" aria-hidden="true">${previewGlyph('eye', 26)}</span>
                    <strong>${t('codeStudio.previewEmptyTitle', 'Open an HTML file to preview it')}</strong>
                    <p>${t('codeStudio.previewEmptyHint', 'HTML, HTM and SVG files open here and reload when you save.')}</p>
                    <div class="cs-hp-card-actions"><button type="button" class="cs-button primary" data-hp-action="preview-active">${t('codeStudio.previewEmptyAction', 'Preview active file')}</button></div>
                </div>
                <div class="cs-hp-card cs-hp-error" data-hp-error role="alert" hidden>
                    <span class="cs-hp-card-icon" aria-hidden="true">${previewGlyph('alert', 24)}</span>
                    <strong data-hp-error-title></strong>
                    <p data-hp-error-detail></p>
                    <div class="cs-hp-card-actions">
                        <button type="button" class="cs-button primary" data-hp-action="retry">${t('codeStudio.previewRetry', 'Try again')}</button>
                        <button type="button" class="cs-button" data-hp-action="close">${t('codeStudio.previewClose', 'Close preview')}</button>
                    </div>
                </div>
            </div>`;
        preview.pane = pane;
        wirePreviewPane(pane);
    }

    function wirePreviewPane(pane) {
        const preview = previewState();
        pane.addEventListener('click', bind(event => {
            const control = event.target.closest('[data-hp-action],[data-hp-viewport]');
            if (!control || !pane.contains(control) || control.disabled) return;
            if (control.dataset.hpViewport) setPreviewViewport(control.dataset.hpViewport);
            else runPreviewAction(control.dataset.hpAction);
        }));
        const viewports = pane.querySelector('[data-hp-viewports]');
        viewports.addEventListener('keydown', bind(event => {
            const index = PREVIEW_VIEWPORTS.indexOf(previewState().viewport);
            let next = -1;
            if (event.key === 'ArrowRight' || event.key === 'ArrowDown') next = (index + 1) % PREVIEW_VIEWPORTS.length;
            else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') next = (index + PREVIEW_VIEWPORTS.length - 1) % PREVIEW_VIEWPORTS.length;
            else if (event.key === 'Home') next = 0;
            else if (event.key === 'End') next = PREVIEW_VIEWPORTS.length - 1;
            if (next < 0) return;
            event.preventDefault();
            setPreviewViewport(PREVIEW_VIEWPORTS[next]);
            const button = viewports.querySelector(`[data-hp-viewport="${PREVIEW_VIEWPORTS[next]}"]`);
            if (button) button.focus();
        }));
        const input = pane.querySelector('[data-hp-address]');
        pane.querySelector('[data-hp-address-form]').addEventListener('submit', bind(event => {
            event.preventDefault();
            navigatePreviewAddress(input.value);
        }));
        input.addEventListener('input', () => input.removeAttribute('aria-invalid'));
        input.addEventListener('keydown', bind(event => {
            if (event.key !== 'Escape') return;
            event.preventDefault();
            event.stopPropagation();
            input.value = previewAddressText();
            input.removeAttribute('aria-invalid');
            input.blur();
        }));
        pane.querySelector('[data-hp-console]').addEventListener('keydown', bind(event => {
            if (event.key !== 'Escape') return;
            event.preventDefault();
            event.stopPropagation();
            togglePreviewConsole(false, true);
        }));
        const frame = pane.querySelector('[data-hp-frame]');
        frame.addEventListener('load', bind(() => onPreviewFrameLoad(frame)));
        wirePreviewResize(pane);
        if (preview.observer) preview.observer.disconnect();
        if (typeof ResizeObserver === 'function') {
            const observer = new ResizeObserver(bind(() => applyPreviewViewport()));
            observer.observe(pane.querySelector('[data-hp-stage]'));
            preview.observer = observer;
            registerDisposer(() => observer.disconnect());
        }
        if (!preview.messageWired) {
            preview.messageWired = true;
            const onMessage = bind(event => handlePreviewMessage(event));
            window.addEventListener('message', onMessage);
            registerDisposer(() => window.removeEventListener('message', onMessage));
        }
    }
```

Run:
```
node --check ui/js/desktop/apps/code-studio/preview.js
```
Expected: exit 0. The file references part-2 functions that do not exist yet; that is fine for a syntax check.

Commit:
```
git add ui/js/desktop/apps/code-studio/preview.js
git commit -m "feat(code-studio): preview pane state, markup and wiring"
```

---

## Task 10: `preview.js` part 2: navigation, console, viewport, resize, follow mode (10 min)

Append to `ui/js/desktop/apps/code-studio/preview.js`:

```js
    function openPreview(path, options) {
        const preview = previewState();
        const next = normalizeCodeStudioPath(path);
        if (next === WORKSPACE_ROOT) return;
        const same = preview.open && preview.path === next && !preview.directory && !preview.suffix && preview.status === 'ready';
        preview.open = true;
        preview.path = next;
        preview.directory = false;
        preview.suffix = '';
        preview.followedPath = next;
        if (options && options.pin === true) preview.pinned = true;
        const built = renderPreviewPane({ navigate: false });
        if (built || !same) navigatePreview();
        syncPreviewEyeButton();
        renderWindowMenus();
    }

    function openPreviewForActive() {
        const tab = activeTab();
        if (isSourceTab(tab) && isPreviewablePath(tab.path)) {
            openPreview(tab.path);
            return;
        }
        const preview = previewState();
        const wasOpen = preview.open;
        preview.open = true;
        const built = renderPreviewPane({ navigate: false });
        if (preview.path && (built || !wasOpen)) navigatePreview();
        else if (!preview.path) setPreviewStatus('empty');
        renderWindowMenus();
    }

    function togglePreviewPane() {
        if (previewState().open) closePreviewPane();
        else openPreviewForActive();
    }

    function closePreviewPane() {
        const preview = previewState();
        if (!preview.open) return;
        const pane = preview.pane;
        const hadFocus = !!(pane && pane.contains(document.activeElement));
        preview.open = false;
        preview.consoleOpen = false;
        preview.navigation += 1;
        const frame = pane && pane.querySelector('[data-hp-frame]');
        if (frame) frame.src = 'about:blank';
        renderPreviewPane();
        syncPreviewEyeButton();
        renderWindowMenus();
        measureVisibleEditors();
        scheduleTerminalFit();
        if (hadFocus) {
            const view = focusedEditorView(activeTab());
            if (view && typeof view.focus === 'function') view.focus();
        }
    }

    // Plan C calls this when the workspace changes: a token names exactly one workspace.
    function resetPreview() {
        const preview = previewState();
        closePreviewPane();
        Object.assign(preview, { path: '', directory: false, suffix: '', base: '', expiresAt: '', tokenRequest: null, console: [], followedPath: '', pinned: false, status: 'empty' });
    }

    function ensurePreviewToken(target, force, path) {
        const preview = target.htmlPreview;
        const model = previewModel();
        if (!force && preview.base && !model.tokenNeedsRefresh(preview.expiresAt, Date.now())) return Promise.resolve(preview.base);
        if (!force && preview.tokenRequest) return preview.tokenRequest;
        const request = api('/api/code-studio/preview', {
            method: 'POST',
            body: JSON.stringify({ path: path || preview.path || WORKSPACE_ROOT })
        }).then(result => {
            const base = String(result && result.base || '');
            if (!base.startsWith('/code-studio-preview/') || !base.endsWith('/')) throw new Error('invalid preview response');
            preview.base = base;
            preview.expiresAt = String(result.expires_at || '');
            return base;
        }).finally(() => {
            if (preview.tokenRequest === request) preview.tokenRequest = null;
        });
        preview.tokenRequest = request;
        return request;
    }

    // HEAD first (the sandboxed page cannot tell the parent its status), then the iframe. A 401 means the token
    // expired or the session secret rotated: one retry with a fresh token.
    async function navigatePreview() {
        const target = state;
        const preview = previewState();
        const model = previewModel();
        if (!model || !preview.open || !preview.path) {
            setPreviewStatus('empty');
            return;
        }
        const navigation = ++preview.navigation;
        setPreviewStatus('loading');
        updatePreviewChrome();
        let status = 0;
        let failure = '';
        let url = '';
        for (let attempt = 0; attempt < 2; attempt += 1) {
            failure = '';
            try {
                const base = await ensurePreviewToken(target, attempt > 0, preview.path);
                if (!isLiveInstance(target) || preview.navigation !== navigation) return;
                url = model.buildUrl(base, preview.path, preview.directory, preview.suffix);
                const response = await fetch(url, { method: 'HEAD', credentials: 'omit', cache: 'no-store' });
                status = response.status;
            } catch (err) {
                status = 0;
                failure = err && err.message ? err.message : String(err);
            }
            if (!isLiveInstance(target) || preview.navigation !== navigation) return;
            if (status !== 401) break;
            preview.base = '';
        }
        runWithInstance(target, () => {
            const frame = preview.pane && preview.pane.querySelector('[data-hp-frame]');
            if (!frame || !preview.open) return;
            if (status >= 200 && status < 300) {
                frame.title = tr('codeStudio.previewFrameTitle', 'Preview of {{path}}', { path: previewAddressText() || '/' });
                frame.src = url; // assigning the same URL navigates again, so this also reloads
                return;
            }
            showPreviewError(status, failure);
        });
    }

    function onPreviewFrameLoad(frame) {
        const preview = previewState();
        if (preview.status !== 'loading') return;
        const src = frame.getAttribute('src') || '';
        if (!src.startsWith('/code-studio-preview/')) return;
        setPreviewStatus('ready');
    }

    function showPreviewError(status, failure) {
        const preview = previewState();
        const path = previewAddressText() || '/';
        if (status === 404) {
            preview.errorTitle = tr('codeStudio.previewNotFoundTitle', 'File not found');
            preview.errorDetail = tr('codeStudio.previewNotFound', '{{path}} does not exist in this workspace.', { path });
        } else if (status === 413) {
            preview.errorTitle = tr('codeStudio.previewTooLargeTitle', 'File too large');
            preview.errorDetail = tr('codeStudio.previewTooLarge', '{{path}} is larger than 25 MB and cannot be previewed.', { path });
        } else {
            preview.errorTitle = tr('codeStudio.previewFailedTitle', 'Preview unavailable');
            preview.errorDetail = tr('codeStudio.previewFailed', 'The preview could not be loaded: {{error}}', { error: failure || ('HTTP ' + status) });
        }
        const frame = preview.pane && preview.pane.querySelector('[data-hp-frame]');
        if (frame) frame.src = 'about:blank';
        setPreviewStatus('error');
    }

    function setPreviewStatus(status) {
        const preview = previewState();
        preview.status = status;
        const pane = preview.pane;
        if (!pane || !pane.isConnected) return;
        pane.querySelector('[data-hp-loading]').hidden = status !== 'loading';
        pane.querySelector('[data-hp-empty]').hidden = status !== 'empty';
        const error = pane.querySelector('[data-hp-error]');
        error.hidden = status !== 'error';
        if (status === 'error') {
            error.querySelector('[data-hp-error-title]').textContent = preview.errorTitle;
            error.querySelector('[data-hp-error-detail]').textContent = preview.errorDetail;
        }
        pane.querySelector('[data-hp-device]').hidden = status === 'empty' || status === 'error';
        pane.querySelector('[data-hp-stage]').setAttribute('aria-busy', String(status === 'loading'));
        const emptyAction = pane.querySelector('[data-hp-action="preview-active"]');
        if (emptyAction) {
            const tab = activeTab();
            emptyAction.disabled = !(isSourceTab(tab) && isPreviewablePath(tab.path));
        }
        applyPreviewViewport();
    }

    function previewAddressText() {
        const preview = previewState();
        const model = previewModel();
        return model ? model.addressText(preview.path, preview.directory, preview.suffix) : '';
    }

    function navigatePreviewAddress(text) {
        const preview = previewState();
        const model = previewModel();
        const input = preview.pane && preview.pane.querySelector('[data-hp-address]');
        const parsed = model && model.parseAddress(text);
        if (!parsed) {
            if (input) input.setAttribute('aria-invalid', 'true');
            flashStatus(tr('codeStudio.previewAddressInvalid', 'Enter a path inside the workspace.'));
            return;
        }
        if (input) input.removeAttribute('aria-invalid');
        preview.path = parsed.path;
        preview.directory = parsed.directory;
        preview.suffix = parsed.suffix;
        navigatePreview();
    }

    // live.js reports every page load (also link clicks inside the preview): the address follows, the console clears.
    function applyPreviewLocation(text) {
        const preview = previewState();
        const parsed = previewModel().parseAddress(text || '/');
        if (parsed) {
            preview.path = parsed.path;
            preview.directory = parsed.directory;
            preview.suffix = parsed.suffix;
        }
        preview.console = [];
        updatePreviewChrome();
    }

    function updatePreviewChrome() {
        const preview = previewState();
        const pane = preview.pane;
        if (!pane || !pane.isConnected) return;
        const hasPath = !!preview.path;
        const input = pane.querySelector('[data-hp-address]');
        if (input && document.activeElement !== input) input.value = previewAddressText();
        const pin = pane.querySelector('[data-hp-action="pin"]');
        if (pin) {
            pin.setAttribute('aria-pressed', String(preview.pinned));
            pin.title = preview.pinned ? tr('codeStudio.previewUnpin', 'Follow the active file') : tr('codeStudio.previewPin', 'Keep showing this file');
            pin.disabled = !hasPath;
        }
        ['reload', 'external'].forEach(action => {
            const button = pane.querySelector(`[data-hp-action="${action}"]`);
            if (button) button.disabled = !hasPath;
        });
        pane.querySelectorAll('[data-hp-viewport]').forEach(button => {
            const active = button.dataset.hpViewport === preview.viewport;
            button.setAttribute('aria-checked', String(active));
            button.tabIndex = active ? 0 : -1;
            button.classList.toggle('active', active);
        });
        renderPreviewConsole();
        syncPreviewDirtyHint();
        applyPreviewViewport();
        syncPreviewEyeButton();
    }

    function runPreviewAction(action) {
        const preview = previewState();
        switch (action) {
            case 'reload':
            case 'retry':
                navigatePreview();
                break;
            case 'console':
                togglePreviewConsole();
                break;
            case 'clear-console':
                preview.console = [];
                renderPreviewConsole();
                break;
            case 'pin':
                preview.pinned = !preview.pinned;
                if (!preview.pinned) {
                    preview.followedPath = '';
                    previewFollowActiveTab();
                }
                updatePreviewChrome();
                break;
            case 'external':
                openPreviewInBrowserTab();
                break;
            case 'close':
                closePreviewPane();
                break;
            case 'preview-active': {
                const tab = activeTab();
                if (isSourceTab(tab) && isPreviewablePath(tab.path)) openPreview(tab.path);
                break;
            }
            default:
                break;
        }
    }

    function handlePreviewMessage(event) {
        const preview = state.htmlPreview;
        const frame = preview && preview.pane && preview.pane.querySelector('[data-hp-frame]');
        // Only our own sandboxed frame counts: same WindowProxy, opaque origin, exact shape.
        if (!frame || !frame.contentWindow || event.source !== frame.contentWindow || event.origin !== 'null') return;
        const message = previewModel().normalizeMessage(event.data);
        if (!message) return;
        if (message.kind === 'location') {
            applyPreviewLocation(message.text);
            return;
        }
        if (message.kind === 'expired') {
            preview.base = '';
            navigatePreview();
            return;
        }
        if (message.kind === 'reloaded') return;
        preview.console = previewModel().pushEntry(preview.console, { kind: message.kind, text: message.text, at: Date.now() });
        renderPreviewConsole();
    }

    function renderPreviewConsole() {
        const preview = previewState();
        const pane = preview.pane;
        const model = previewModel();
        if (!pane || !model) return;
        const count = preview.console.filter(entry => model.isErrorKind(entry.kind)).length;
        const badge = pane.querySelector('[data-hp-console-count]');
        const button = pane.querySelector('[data-hp-action="console"]');
        const box = pane.querySelector('[data-hp-console]');
        const list = pane.querySelector('[data-hp-console-list]');
        badge.hidden = count === 0;
        badge.textContent = count > 99 ? '99+' : String(count);
        button.classList.toggle('has-errors', count > 0);
        button.setAttribute('aria-label', tr('codeStudio.previewConsoleCount', 'Console: {{count}} errors', { count }));
        button.setAttribute('aria-expanded', String(preview.consoleOpen));
        box.hidden = !preview.consoleOpen;
        if (!preview.consoleOpen) return;
        list.innerHTML = preview.console.length
            ? preview.console.map(entry => `<li class="cs-hp-console-entry" data-kind="${esc(entry.kind)}"><time>${esc(new Date(entry.at).toLocaleTimeString())}</time><pre>${esc(entry.text)}</pre></li>`).join('')
            : `<li class="cs-hp-console-empty">${esc(tr('codeStudio.previewConsoleEmpty', 'No errors reported by this page.'))}</li>`;
    }

    function togglePreviewConsole(force, returnFocus) {
        const preview = previewState();
        preview.consoleOpen = typeof force === 'boolean' ? force : !preview.consoleOpen;
        renderPreviewConsole();
        const pane = preview.pane;
        if (!pane) return;
        if (preview.consoleOpen) {
            const clear = pane.querySelector('[data-hp-action="clear-console"]');
            if (clear) clear.focus();
        } else if (returnFocus) {
            const button = pane.querySelector('[data-hp-action="console"]');
            if (button) button.focus();
        }
    }

    function setPreviewViewport(viewport) {
        const preview = previewState();
        if (!PREVIEW_VIEWPORTS.includes(viewport) || preview.viewport === viewport) return;
        preview.viewport = viewport;
        savePreviewPrefs();
        updatePreviewChrome();
    }

    function applyPreviewViewport() {
        const preview = state && state.htmlPreview;
        const pane = preview && preview.pane;
        const model = previewModel();
        if (!pane || !pane.isConnected || !model) return;
        const stage = pane.querySelector('[data-hp-stage]');
        const device = pane.querySelector('[data-hp-device]');
        const label = pane.querySelector('[data-hp-device-label]');
        stage.dataset.viewport = preview.viewport;
        const size = model.VIEWPORTS[preview.viewport];
        if (!size || preview.status === 'empty' || preview.status === 'error') {
            label.hidden = true;
            if (!size) ['--cs-hp-device-w', '--cs-hp-device-h', '--cs-hp-scale'].forEach(name => device.style.removeProperty(name));
            return;
        }
        const rect = stage.getBoundingClientRect();
        const scale = model.deviceScale(preview.viewport, rect.width, rect.height, PREVIEW_DEVICE_INSET);
        device.style.setProperty('--cs-hp-device-w', size.width + 'px');
        device.style.setProperty('--cs-hp-device-h', size.height + 'px');
        device.style.setProperty('--cs-hp-scale', String(scale));
        label.hidden = false;
        label.textContent = tr('codeStudio.previewDeviceLabel', '{{width}} × {{height}} at {{scale}}%', { width: size.width, height: size.height, scale: Math.round(scale * 100) });
    }

    function applyPreviewWidth() {
        const root = studioRoot();
        const model = previewModel();
        if (!root || !model) return;
        const preview = previewState();
        const main = root.querySelector('.code-studio-main');
        const pane = previewPane();
        const available = (main ? main.getBoundingClientRect().width : 0) + (pane && !pane.hidden ? pane.getBoundingClientRect().width : 0);
        preview.width = model.clampWidth(preview.width, preview.open ? available : 0);
        root.style.setProperty('--cs-preview-width', preview.width + 'px');
        const handle = pane && pane.querySelector('[data-hp-resize]');
        if (handle) {
            handle.setAttribute('aria-valuemin', '280');
            handle.setAttribute('aria-valuemax', String(model.clampWidth(Number.MAX_SAFE_INTEGER, available)));
            handle.setAttribute('aria-valuenow', String(preview.width));
        }
    }

    function finishPreviewResize() {
        applyPreviewWidth();
        savePreviewPrefs();
        measureVisibleEditors();
        scheduleTerminalFit();
    }

    // The handle sits on the pane's left edge: dragging or ArrowLeft widens the pane, ArrowRight narrows it.
    function wirePreviewResize(pane) {
        const handle = pane.querySelector('[data-hp-resize]');
        let startX = 0;
        let startWidth = 0;
        const onMove = bind(event => {
            previewState().width = startWidth + (startX - event.clientX);
            applyPreviewWidth();
        });
        const onUp = bind(event => {
            handle.classList.remove('dragging');
            pane.classList.remove('resizing');
            try { handle.releasePointerCapture(event.pointerId); } catch (_) {}
            handle.removeEventListener('pointermove', onMove);
            handle.removeEventListener('pointerup', onUp);
            handle.removeEventListener('pointercancel', onUp);
            finishPreviewResize();
        });
        handle.addEventListener('pointerdown', bind(event => {
            if (event.button !== 0) return;
            event.preventDefault();
            startX = event.clientX;
            startWidth = previewState().width;
            handle.classList.add('dragging');
            pane.classList.add('resizing'); // the iframe would swallow pointer events mid-drag
            handle.setPointerCapture(event.pointerId);
            handle.addEventListener('pointermove', onMove);
            handle.addEventListener('pointerup', onUp);
            handle.addEventListener('pointercancel', onUp);
        }));
        handle.addEventListener('keydown', bind(event => {
            const preview = previewState();
            let next = null;
            if (event.key === 'ArrowLeft') next = preview.width + PREVIEW_RESIZE_STEP;
            else if (event.key === 'ArrowRight') next = preview.width - PREVIEW_RESIZE_STEP;
            else if (event.key === 'Home') next = Number.MAX_SAFE_INTEGER;
            else if (event.key === 'End') next = 0;
            if (next === null) return;
            event.preventDefault();
            preview.width = next;
            finishPreviewResize();
        }));
        handle.addEventListener('dblclick', bind(() => {
            previewState().width = PREVIEW_DEFAULT_WIDTH;
            finishPreviewResize();
        }));
    }

    function openPreviewInBrowserTab(path) {
        const target = state;
        const preview = previewState();
        const model = previewModel();
        if (!model) return;
        const vpath = path ? normalizeCodeStudioPath(path) : preview.path;
        if (!vpath || vpath === WORKSPACE_ROOT && !preview.directory) return;
        const current = vpath === preview.path;
        const urlFor = base => model.buildUrl(base, vpath, current && preview.directory, current ? preview.suffix : '');
        if (preview.base && !model.tokenNeedsRefresh(preview.expiresAt, Date.now())) {
            window.open(urlFor(preview.base), '_blank', 'noopener');
            return;
        }
        // No usable token yet: open the tab inside the click (popup blockers), cut its opener, then send it on.
        const win = window.open('about:blank', '_blank');
        if (win) {
            try { win.opener = null; } catch (_) {}
        }
        ensurePreviewToken(target, false, vpath).then(base => {
            if (win && !win.closed) win.location.replace(urlFor(base));
        }).catch(err => {
            if (win && !win.closed) win.close();
            if (isLiveInstance(target)) runWithInstance(target, () => showOperationError(err));
        });
    }

    function previewFollowActiveTab() {
        const preview = state && state.htmlPreview;
        if (!preview || !preview.open || preview.pinned) return;
        const tab = activeTab();
        if (!isSourceTab(tab) || !isPreviewablePath(tab.path) || tab.path === preview.followedPath) return;
        openPreview(tab.path);
    }

    // Hook at the end of renderBreadcrumbs: the eye button in the editor title area, follow mode, dirty hint.
    function previewAfterBreadcrumbs() {
        const root = studioRoot();
        const bar = root && root.querySelector('[data-breadcrumbs]');
        if (!bar) return;
        bar.querySelectorAll('[data-hp-eye]').forEach(element => element.remove());
        const tab = activeTab();
        if (isSourceTab(tab) && isPreviewablePath(tab.path)) {
            const label = tr('codeStudio.previewEye', 'Open preview to the side') + ' (Ctrl+Shift+V)';
            const button = document.createElement('button');
            button.type = 'button';
            button.className = 'cs-hp-eye';
            button.dataset.hpEye = '';
            button.setAttribute('aria-label', label);
            button.title = label;
            button.innerHTML = previewGlyph('eye', 15);
            button.addEventListener('click', bind(() => {
                const current = activeTab();
                if (!isSourceTab(current)) return;
                const preview = previewState();
                if (preview.open && preview.path === current.path) closePreviewPane();
                else openPreview(current.path);
            }));
            bar.appendChild(button);
        }
        syncPreviewEyeButton();
        previewFollowActiveTab();
        syncPreviewDirtyHint();
    }

    // Hook at the end of renderTabs: a tab turned dirty or clean.
    function previewAfterTabs() {
        syncPreviewDirtyHint();
        syncPreviewEyeButton();
    }

    function syncPreviewDirtyHint() {
        const preview = state && state.htmlPreview;
        const hint = preview && preview.pane && preview.pane.querySelector('[data-hp-dirty]');
        if (!hint) return;
        const tab = state.openTabs.find(item => !isVirtualTab(item) && item.path === preview.path);
        hint.hidden = !(preview.open && tab && tab.modified);
    }

    function syncPreviewEyeButton() {
        const root = studioRoot();
        const eye = root && root.querySelector('[data-hp-eye]');
        if (!eye) return;
        const preview = state.htmlPreview;
        const tab = activeTab();
        eye.setAttribute('aria-pressed', String(!!(preview && preview.open && tab && preview.path === tab.path)));
    }

    function previewOpenToSideCommand() {
        const tab = activeTab();
        if (isSourceTab(tab) && isPreviewablePath(tab.path)) openPreview(tab.path);
        else openPreviewForActive();
    }

    function previewOpenInBrowserTabCommand() {
        const tab = activeTab();
        if (isSourceTab(tab) && isPreviewablePath(tab.path)) openPreviewInBrowserTab(tab.path);
    }
```

Run:
```
node --check ui/js/desktop/apps/code-studio/preview.js
```
Expected: exit 0.

Commit:
```
git add ui/js/desktop/apps/code-studio/preview.js
git commit -m "feat(code-studio): preview navigation, console badge, viewports, splitter and follow mode"
```

---

## Task 11: `core.js` hooks (6 min)

Impact was checked in Task 0. Apply exactly these edits to `ui/js/desktop/apps/code-studio/core.js`.

1. **`shellMarkup`.** Replace
   ```
                </main>
                <aside class="code-studio-chat" data-agent-panel></aside>
   ```
   with
   ```
                </main>
                <aside class="code-studio-preview" data-html-preview hidden></aside>
                <aside class="code-studio-chat" data-agent-panel></aside>
   ```

2. **`renderShell`.** Replace
   ```
        renderAgentPanel();
        renderGitPanel();
        renderStatus();
   ```
   with
   ```
        renderAgentPanel();
        renderGitPanel();
        renderPreviewPane(); // Plan B: HTML preview pane
        renderStatus();
   ```

3. **`renderBreadcrumbs`.** Replace
   ```
    function renderBreadcrumbs() {
        const bar = shellPart('[data-breadcrumbs]');
   ```
   with
   ```
    function renderBreadcrumbs() {
        renderBreadcrumbsBase();
        previewAfterBreadcrumbs(); // Plan B: eye button, follow mode and dirty hint
    }

    function renderBreadcrumbsBase() {
        const bar = shellPart('[data-breadcrumbs]');
   ```

4. **End of `renderTabs`.** Replace
   ```
        updatePolicyControls();
        updateSuggestionStatus();
    }

    function updateToolbarTitle() {
   ```
   with
   ```
        updatePolicyControls();
        updateSuggestionStatus();
        previewAfterTabs(); // Plan B: dirty hint and eye state
    }

    function updateToolbarTitle() {
   ```

5. **`renderWindowMenus`, View menu.** After the `{ id: 'git-panel', … },` line, insert:
   ```
                    { id: 'html-preview', label: tr('codeStudio.previewToggle', 'Toggle HTML Preview'), icon: 'columns', shortcut: 'Ctrl+Shift+V', checked: !!(state.htmlPreview && state.htmlPreview.open), action: bind(togglePreviewPane) }, // Plan B
   ```

Run:
```
node --check ui/js/desktop/apps/code-studio/core.js
```
`core.js` opens the IIFE and does not close it. If `--check` reports the expected `Unexpected end of input`, check the syntax of the bundle in Task 14 instead.

Commit:
```
git add ui/js/desktop/apps/code-studio/core.js
git commit -m "feat(code-studio): mount the preview pane and hook breadcrumbs, tabs and the View menu"
```

---

## Task 12: Shortcut, overlay, command table, palette, Explorer menu (6 min)

1. **`shortcuts.js`, in `wireShortcuts`.** Replace
   ```
            } else if (mod && event.shiftKey && key === 'a') {
   ```
   with
   ```
            } else if (mod && event.shiftKey && !event.altKey && (key === 'v' || event.code === 'KeyV') && !inTerminal && (!editing || isCodeEditorTarget(event.target))) {
                // Plan B: Ctrl+Shift+V toggles the HTML preview. Inputs and the terminal keep paste-as-plain-text.
                event.preventDefault();
                togglePreviewPane();
            } else if (mod && event.shiftKey && key === 'a') {
   ```

2. **`shortcuts.js`, overlay "View" section.** After `{ label: tr('codeStudio.zenMode', 'Zen Mode'), keys: 'Ctrl+K' },`, add:
   ```
                { label: tr('codeStudio.previewToggle', 'Toggle HTML Preview'), keys: 'Ctrl+Shift+V' },
   ```

3. **`shortcuts.js`, `studioCommandTable`.** Replace
   ```
            lspRenameSymbolCommand,
            lspQuickFixCommand
        };
   ```
   with
   ```
            lspRenameSymbolCommand,
            lspQuickFixCommand,
            // Plan B: HTML preview
            togglePreviewPane,
            openPreview,
            closePreviewPane,
            resetPreview,
            previewOpenToSideCommand,
            previewOpenInBrowserTabCommand
        };
   ```

4. **`command-palette.js`, `getCommands`.**
   - After `const modified = !!(state.openTabs || []).some(tab => tab.modified);`, add:
     ```js
        const activeFile = hasTab ? state.openTabs[state.activeTabIndex] : null;
        const previewable = sourceTab && !!window.CodeStudioPreviewModel && window.CodeStudioPreviewModel.isPreviewablePath(activeFile && activeFile.path);
     ```
   - After the `toggle-split` entry, add:
     ```js
            // Plan B: HTML preview
            { id: 'preview-side', label: tr('codeStudio.previewOpenSide', 'Preview: Open to the Side'), shortcut: 'Ctrl+Shift+V', icon: 'columns', run: 'previewOpenToSideCommand', disabled: !previewable },
            { id: 'preview-browser-tab', label: tr('codeStudio.previewOpenTab', 'Preview: Open in Browser Tab'), icon: 'layout', run: 'previewOpenInBrowserTabCommand', disabled: !previewable },
     ```

5. **`sidebar.js`, `showTreeContextMenu`.** After the `open-to-side` line, add:
   ```js
        if (file && !isDir && isPreviewablePath(file.path)) { // Plan B
            items.push({ id: 'preview-open', label: tr('codeStudio.previewOpenMenu', 'Open Preview'), icon: 'columns', action: bind(() => openPreview(file.path)) });
            items.push({ id: 'preview-browser-tab', label: tr('codeStudio.previewOpenTabMenu', 'Open in Browser Tab'), icon: 'layout', action: bind(() => openPreviewInBrowserTab(file.path)) });
        }
   ```
   The preview ids are not in `policyDisables`, so both entries stay enabled on a read-only desktop. That is correct: a preview only reads.

Run:
```
node --check ui/js/desktop/apps/code-studio/command-palette.js
```
Expected: exit 0. (`shortcuts.js` and `sidebar.js` are IIFE fragments; they are checked via the bundle in Task 14.)

Commit:
```
git add ui/js/desktop/apps/code-studio/shortcuts.js ui/js/desktop/apps/code-studio/command-palette.js ui/js/desktop/apps/code-studio/sidebar.js
git commit -m "feat(code-studio): preview shortcut, palette commands and Explorer menu entries"
```

---

## Task 13: Preview CSS + module loader (8 min)

1. Create `ui/css/code-studio-preview.css`:

```css
/* Code Studio HTML preview (Plan B). The pane lives inside .code-studio, so every --cs-* token and app material
   applies; the file is separate from code-studio.css to keep it apart from parallel Code Studio work. */
.code-studio-preview {
    position: relative;
    z-index: 1;
    display: none;
    flex: 0 0 var(--cs-preview-width, 480px);
    flex-direction: column;
    width: var(--cs-preview-width, 480px);
    min-width: 280px;
    min-height: 0;
    border-left: 1px solid var(--cs-line);
    background: var(--vd-mat-side, var(--cs-panel));
}

.code-studio[data-preview="visible"] .code-studio-preview { display: flex; }
.code-studio-preview[hidden],
.code-studio[data-zen="true"] .code-studio-preview { display: none !important; }

.cs-hp-resize {
    position: absolute;
    top: 0;
    bottom: 0;
    left: -3px;
    z-index: 6;
    width: 6px;
    cursor: col-resize;
    background: transparent;
    transition: background var(--cs-transition-fast);
}

.cs-hp-resize:hover,
.cs-hp-resize.dragging,
.cs-hp-resize:focus-visible {
    outline: none;
    background: var(--cs-accent-glow);
}

.code-studio-preview.resizing { user-select: none; }
.code-studio-preview.resizing .cs-hp-frame { pointer-events: none; }

.cs-hp-toolbar {
    display: flex;
    align-items: center;
    gap: 4px;
    min-height: 38px;
    padding: 4px 6px 4px 8px;
    border-bottom: 1px solid var(--cs-line);
    background: var(--vd-mat-bar, var(--vd-theme-chrome-bg));
    box-shadow: var(--vd-mat-bar-edge, none);
}

.cs-hp-button {
    position: relative;
    display: inline-flex;
    flex: 0 0 auto;
    align-items: center;
    justify-content: center;
    gap: 4px;
    min-width: 28px;
    height: 28px;
    padding: 0 6px;
    border: 1px solid transparent;
    border-radius: var(--vd-mat-btn-radius, var(--cs-radius-sm));
    color: var(--cs-muted);
    background: transparent;
    font: inherit;
    cursor: pointer;
    transition: color var(--cs-transition-fast), background var(--cs-transition-fast), border-color var(--cs-transition-fast);
}

.cs-hp-button:hover:not(:disabled) { color: var(--cs-text); background: var(--cs-hover); }
.cs-hp-button[aria-pressed="true"],
.cs-hp-button[aria-expanded="true"] { color: var(--cs-accent); border-color: var(--cs-accent-border); background: var(--cs-accent-soft); }
.cs-hp-button:disabled { opacity: 0.45; cursor: default; }
.cs-hp-glyph { display: block; flex: 0 0 auto; }

.cs-hp-address {
    display: flex;
    flex: 1 1 auto;
    align-items: center;
    gap: 6px;
    min-width: 80px;
    height: 28px;
    padding: 0 8px;
    border: 1px solid var(--vd-mat-field-border, var(--cs-border));
    border-radius: var(--vd-mat-field-radius, var(--cs-radius-sm));
    color: var(--cs-muted);
    background: var(--vd-mat-field, var(--cs-panel-soft));
    box-shadow: var(--vd-mat-field-shadow, none);
}

.cs-hp-address:focus-within { border-color: var(--cs-accent-border); box-shadow: 0 0 0 2px var(--cs-accent-soft); }
.cs-hp-address:has(.cs-hp-address-input[aria-invalid="true"]) { border-color: var(--cs-danger); }

.cs-hp-address-input {
    flex: 1 1 auto;
    min-width: 0;
    height: 100%;
    padding: 0;
    border: 0;
    outline: none;
    color: var(--cs-text);
    background: transparent;
    font: 12px/1 var(--cs-mono-font);
}

.cs-hp-address-input[aria-invalid="true"] { color: var(--cs-danger); }

.cs-hp-segmented {
    display: inline-flex;
    flex: 0 0 auto;
    gap: 2px;
    padding: 2px;
    border: 1px solid var(--cs-line);
    border-radius: var(--cs-radius-sm);
    background: var(--vd-mat-well, var(--cs-panel-soft));
}

.cs-hp-segment {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 22px;
    border: 0;
    border-radius: calc(var(--cs-radius-sm) - 2px);
    color: var(--cs-muted);
    background: transparent;
    cursor: pointer;
}

.cs-hp-segment:hover { color: var(--cs-text); }
.cs-hp-segment.active { color: var(--cs-accent-contrast); background: var(--cs-accent); }

.cs-hp-badge {
    min-width: 16px;
    height: 16px;
    padding: 0 4px;
    border-radius: 8px;
    color: #fff;
    background: var(--cs-danger);
    font-size: 10px;
    font-weight: 700;
    line-height: 16px;
    text-align: center;
}

.cs-hp-badge[hidden] { display: none; }
.cs-hp-console-button.has-errors { color: var(--cs-danger); }

.code-studio-preview :focus-visible { outline: 2px solid var(--cs-accent); outline-offset: 1px; }

.cs-hp-hint {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 5px 10px;
    border-bottom: 1px solid var(--cs-line);
    color: var(--cs-text);
    background: color-mix(in srgb, var(--cs-warning) 14%, transparent);
    font-size: 12px;
}

.cs-hp-hint[hidden] { display: none; }
.cs-hp-hint-dot { flex: 0 0 auto; width: 7px; height: 7px; border-radius: 50%; background: var(--cs-warning); }

.cs-hp-console {
    position: absolute;
    top: 42px;
    right: 8px;
    z-index: 8;
    display: flex;
    flex-direction: column;
    width: min(420px, calc(100% - 16px));
    max-height: min(320px, 60%);
    overflow: hidden;
    border: 1px solid var(--cs-border);
    border-radius: var(--cs-radius-md);
    background: var(--cs-panel-elevated);
    box-shadow: var(--cs-shadow-lg);
}

.cs-hp-console[hidden] { display: none; }

.cs-hp-console-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 10px;
    border-bottom: 1px solid var(--cs-line);
    color: var(--vd-mat-label-color, var(--cs-muted));
    font-size: 11px;
    font-weight: 600;
    letter-spacing: var(--vd-mat-label-tracking, 0.4px);
    text-transform: var(--vd-mat-label-transform, uppercase);
}

.cs-hp-text-button {
    padding: 2px 4px;
    border: 0;
    border-radius: var(--cs-radius-sm);
    color: var(--cs-accent);
    background: transparent;
    font: inherit;
    font-size: 12px;
    letter-spacing: 0;
    text-transform: none;
    cursor: pointer;
}

.cs-hp-text-button:hover { background: var(--cs-hover); }
.cs-hp-console-list { margin: 0; padding: 4px 0; overflow: auto; list-style: none; }

.cs-hp-console-entry {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 8px;
    padding: 6px 10px;
    border-left: 3px solid var(--cs-danger);
}

.cs-hp-console-entry + .cs-hp-console-entry { border-top: 1px solid var(--cs-line); }
.cs-hp-console-entry[data-kind="resource"] { border-left-color: var(--cs-warning); }
.cs-hp-console-entry time { color: var(--cs-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
.cs-hp-console-entry pre { margin: 0; color: var(--cs-text); font: 12px/1.45 var(--cs-mono-font); white-space: pre-wrap; word-break: break-word; }
.cs-hp-console-empty { padding: 14px 12px; color: var(--cs-muted); font-size: 12px; text-align: center; }

.cs-hp-stage {
    position: relative;
    flex: 1 1 auto;
    min-height: 0;
    overflow: hidden;
    background: var(--vd-mat-well, var(--cs-panel-soft));
}

.cs-hp-device { position: absolute; inset: 0; }
.cs-hp-device[hidden] { display: none; }

/* Pages expect the browser's white canvas, whatever the studio theme. */
.cs-hp-frame { display: block; width: 100%; height: 100%; border: 0; background: #fff; color-scheme: light; }

.cs-hp-stage:not([data-viewport="responsive"]) .cs-hp-device {
    inset: auto;
    top: 50%;
    left: 50%;
    box-sizing: content-box;
    width: var(--cs-hp-device-w, 375px);
    height: var(--cs-hp-device-h, 812px);
    overflow: hidden;
    border: 10px solid var(--cs-panel-elevated);
    border-radius: 26px;
    background: var(--cs-panel-elevated);
    box-shadow: var(--cs-shadow-lg), 0 0 0 1px var(--cs-line);
    transform: translate(-50%, -50%) scale(var(--cs-hp-scale, 1));
    transform-origin: center;
    transition: transform var(--cs-transition-normal);
}

.cs-hp-stage[data-viewport="tablet"] .cs-hp-device { border-radius: 20px; }
.cs-hp-stage:not([data-viewport="responsive"]) .cs-hp-frame { border-radius: 16px; }

.cs-hp-device-label {
    position: absolute;
    bottom: 8px;
    left: 50%;
    padding: 2px 8px;
    border: 1px solid var(--cs-line);
    border-radius: 999px;
    color: var(--cs-muted);
    background: var(--cs-panel);
    font-size: 11px;
    font-variant-numeric: tabular-nums;
    transform: translateX(-50%);
    pointer-events: none;
}

.cs-hp-device-label[hidden] { display: none; }

.cs-hp-loading {
    position: absolute;
    inset: 0;
    z-index: 3;
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 24px;
    background: var(--cs-panel);
}

.cs-hp-loading[hidden] { display: none; }

.cs-hp-shimmer {
    display: block;
    height: 14px;
    border-radius: 7px;
    background: linear-gradient(90deg, var(--cs-panel-soft) 0%, var(--cs-hover) 50%, var(--cs-panel-soft) 100%);
    background-size: 200% 100%;
    animation: cs-hp-shimmer 1.2s ease-in-out infinite;
}

.cs-hp-shimmer:first-child { width: 60%; height: 28px; }
.cs-hp-shimmer.short { width: 40%; }

@keyframes cs-hp-shimmer {
    from { background-position: 200% 0; }
    to { background-position: -200% 0; }
}

@media (prefers-reduced-motion: reduce) {
    .cs-hp-shimmer { animation: none; }
    .cs-hp-stage .cs-hp-device { transition: none; }
}

.cs-hp-card {
    position: absolute;
    inset: 0;
    z-index: 4;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    padding: 24px;
    color: var(--cs-muted);
    background: var(--vd-mat-well, var(--cs-panel-soft));
    text-align: center;
}

.cs-hp-card[hidden] { display: none; }
.cs-hp-card strong { color: var(--vd-mat-heading-color, var(--cs-text)); font-family: var(--vd-mat-heading-font, inherit); font-size: 14px; font-weight: 600; }
.cs-hp-card p { max-width: 320px; margin: 0; font-size: 12px; line-height: 1.5; overflow-wrap: anywhere; }

.cs-hp-card-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 52px;
    height: 52px;
    border-radius: 50%;
    color: var(--cs-accent);
    background: var(--cs-accent-soft);
}

.cs-hp-error .cs-hp-card-icon { color: var(--cs-danger); background: var(--cs-danger-soft); }
.cs-hp-card-actions { display: flex; gap: 8px; margin-top: 4px; }
.cs-hp-card-actions .cs-button { padding: 0 12px; }

.cs-hp-sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }

.cs-hp-eye {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 22px;
    margin-left: auto;
    border: 1px solid transparent;
    border-radius: var(--cs-radius-sm);
    color: var(--cs-muted);
    background: transparent;
    cursor: pointer;
}

.cs-hp-eye:hover { color: var(--cs-text); background: var(--cs-hover); }
.cs-hp-eye[aria-pressed="true"] { color: var(--cs-accent); border-color: var(--cs-accent-border); background: var(--cs-accent-soft); }
.cs-hp-eye:focus-visible { outline: 2px solid var(--cs-accent); outline-offset: 1px; }

/* Narrow windows: the preview takes the editor's place instead of squeezing both. */
@container code-studio (max-width: 720px) {
    .code-studio[data-preview="visible"] .code-studio-main { display: none; }
    .code-studio[data-preview="visible"] .code-studio-preview { flex: 1 1 auto; width: auto; }
    .cs-hp-resize { display: none; }
}
```

2. In `ui/js/desktop/core/module-loader.js`, entry `'code-studio'`, replace
   ```
            styles: appStyles('/css/code-studio.css', '/css/xterm.css', '/css/hljs-github-dark.min.css', '/css/learnings.css'),
   ```
   with
   ```
            styles: appStyles('/css/code-studio.css', '/css/code-studio-preview.css', '/css/xterm.css', '/css/hljs-github-dark.min.css', '/css/learnings.css'),
   ```

Run:
```
node --check ui/js/desktop/core/module-loader.js
```
Expected: exit 0.

Commit:
```
git add ui/css/code-studio-preview.css ui/js/desktop/core/module-loader.js
git commit -m "style(code-studio): preview pane, toolbar, device frames, console popover and cards"
```

---

## Task 14: Bundle parts + rebuild (4 min)

In `scripts/build-ui-bundles.js`, in the `code-studio.bundle.js` parts, after `'ui/js/desktop/apps/code-studio/lsp-actions.js',`, insert:
```js
      // Plan B: HTML preview. The pure helpers (window.CodeStudioPreviewModel) first, then the pane fragment; both sit
      // before shortcuts.js, which closes the shared IIFE and lists the pane commands.
      'ui/js/desktop/apps/code-studio/preview-model.js',
      'ui/js/desktop/apps/code-studio/preview.js',
```

Run:
```
npm run build:ui
node --check ui/js/desktop/bundles/code-studio.bundle.js
npm run build:ui -- --check
```
Expected: the build rewrites `ui/js/desktop/bundles/code-studio.bundle.js`, `node --check` exits 0, and `--check` exits 0 without reporting a stale bundle. If the build also rewrote other bundles, someone else's sources moved: commit only the code-studio bundle and report it.

Commit:
```
git add scripts/build-ui-bundles.js ui/js/desktop/bundles/code-studio.bundle.js
git commit -m "build(code-studio): bundle the preview helpers and pane"
```

---

## Task 15: Locale strings, all 16 languages (8 min)

### Keys and English text (37)

| Key | English |
| --- | --- |
| `codeStudio.previewTitle` | Preview |
| `codeStudio.previewOpenSide` | Preview: Open to the Side |
| `codeStudio.previewOpenTab` | Preview: Open in Browser Tab |
| `codeStudio.previewOpenMenu` | Open Preview |
| `codeStudio.previewOpenTabMenu` | Open in Browser Tab |
| `codeStudio.previewToggle` | Toggle HTML Preview |
| `codeStudio.previewEye` | Open preview to the side |
| `codeStudio.previewReload` | Reload preview |
| `codeStudio.previewAddress` | Preview address (path inside the workspace) |
| `codeStudio.previewAddressInvalid` | Enter a path inside the workspace. |
| `codeStudio.previewViewport` | Viewport |
| `codeStudio.previewResponsive` | Responsive |
| `codeStudio.previewTablet` | Tablet (768 px) |
| `codeStudio.previewMobile` | Mobile (375 px) |
| `codeStudio.previewConsole` | Console |
| `codeStudio.previewConsoleCount` | Console: {{count}} errors |
| `codeStudio.previewConsoleEmpty` | No errors reported by this page. |
| `codeStudio.previewConsoleClear` | Clear console |
| `codeStudio.previewPin` | Keep showing this file |
| `codeStudio.previewUnpin` | Follow the active file |
| `codeStudio.previewOpenExternal` | Open in new browser tab |
| `codeStudio.previewClose` | Close preview |
| `codeStudio.previewResize` | Resize preview |
| `codeStudio.previewDirty` | Showing saved version. Save to update the preview. |
| `codeStudio.previewEmptyTitle` | Open an HTML file to preview it |
| `codeStudio.previewEmptyHint` | HTML, HTM and SVG files open here and reload when you save. |
| `codeStudio.previewEmptyAction` | Preview active file |
| `codeStudio.previewLoading` | Loading preview... |
| `codeStudio.previewNotFoundTitle` | File not found |
| `codeStudio.previewNotFound` | {{path}} does not exist in this workspace. |
| `codeStudio.previewTooLargeTitle` | File too large |
| `codeStudio.previewTooLarge` | {{path}} is larger than 25 MB and cannot be previewed. |
| `codeStudio.previewFailedTitle` | Preview unavailable |
| `codeStudio.previewFailed` | The preview could not be loaded: {{error}} |
| `codeStudio.previewRetry` | Try again |
| `codeStudio.previewFrameTitle` | Preview of {{path}} |
| `codeStudio.previewDeviceLabel` | {{width}} × {{height}} at {{scale}}% |

### Script

Write `C:\disposable\cs-preview\add-preview-i18n.cjs` with the Write tool. It is a disposable script; never place it in the repo.
- The script inserts the keys after `"codeStudio.lspFailHiddenUnsaved"`, keeps each file's line endings and is idempotent.
- It checks that placeholders match English and that each file still parses as JSON.

```js
const fs = require('node:fs');
const path = require('node:path');

const repo = process.argv[2];
if (!repo) throw new Error('usage: node add-preview-i18n.cjs <repo root>');

const KEYS = [
    'previewTitle', 'previewOpenSide', 'previewOpenTab', 'previewOpenMenu', 'previewOpenTabMenu', 'previewToggle',
    'previewEye', 'previewReload', 'previewAddress', 'previewAddressInvalid', 'previewViewport', 'previewResponsive',
    'previewTablet', 'previewMobile', 'previewConsole', 'previewConsoleCount', 'previewConsoleEmpty',
    'previewConsoleClear', 'previewPin', 'previewUnpin', 'previewOpenExternal', 'previewClose', 'previewResize',
    'previewDirty', 'previewEmptyTitle', 'previewEmptyHint', 'previewEmptyAction', 'previewLoading',
    'previewNotFoundTitle', 'previewNotFound', 'previewTooLargeTitle', 'previewTooLarge', 'previewFailedTitle',
    'previewFailed', 'previewRetry', 'previewFrameTitle', 'previewDeviceLabel'
];

const T = {
    en: ["Preview", "Preview: Open to the Side", "Preview: Open in Browser Tab", "Open Preview", "Open in Browser Tab", "Toggle HTML Preview",
        "Open preview to the side", "Reload preview", "Preview address (path inside the workspace)", "Enter a path inside the workspace.", "Viewport", "Responsive",
        "Tablet (768 px)", "Mobile (375 px)", "Console", "Console: {{count}} errors", "No errors reported by this page.",
        "Clear console", "Keep showing this file", "Follow the active file", "Open in new browser tab", "Close preview", "Resize preview",
        "Showing saved version. Save to update the preview.", "Open an HTML file to preview it", "HTML, HTM and SVG files open here and reload when you save.", "Preview active file", "Loading preview...",
        "File not found", "{{path}} does not exist in this workspace.", "File too large", "{{path}} is larger than 25 MB and cannot be previewed.", "Preview unavailable",
        "The preview could not be loaded: {{error}}", "Try again", "Preview of {{path}}", "{{width}} × {{height}} at {{scale}}%"],
    de: ["Vorschau", "Vorschau: Seitlich öffnen", "Vorschau: In Browser-Tab öffnen", "Vorschau öffnen", "In Browser-Tab öffnen", "HTML-Vorschau umschalten",
        "Vorschau seitlich öffnen", "Vorschau neu laden", "Vorschauadresse (Pfad im Arbeitsbereich)", "Gib einen Pfad innerhalb des Arbeitsbereichs ein.", "Ansicht", "Responsiv",
        "Tablet (768 px)", "Smartphone (375 px)", "Konsole", "Konsole: {{count}} Fehler", "Diese Seite hat keine Fehler gemeldet.",
        "Konsole leeren", "Diese Datei weiter anzeigen", "Der aktiven Datei folgen", "In neuem Browser-Tab öffnen", "Vorschau schließen", "Vorschaugröße ändern",
        "Gespeicherte Version wird angezeigt. Speichern aktualisiert die Vorschau.", "Öffne eine HTML-Datei, um sie in der Vorschau zu sehen", "HTML-, HTM- und SVG-Dateien öffnen sich hier und laden beim Speichern neu.", "Aktive Datei anzeigen", "Vorschau wird geladen...",
        "Datei nicht gefunden", "{{path}} existiert in diesem Arbeitsbereich nicht.", "Datei zu groß", "{{path}} ist größer als 25 MB und kann nicht angezeigt werden.", "Vorschau nicht verfügbar",
        "Die Vorschau konnte nicht geladen werden: {{error}}", "Erneut versuchen", "Vorschau von {{path}}", "{{width}} × {{height}} bei {{scale}} %"],
    cs: ["Náhled", "Náhled: otevřít vedle", "Náhled: otevřít na kartě prohlížeče", "Otevřít náhled", "Otevřít na kartě prohlížeče", "Přepnout náhled HTML",
        "Otevřít náhled vedle", "Znovu načíst náhled", "Adresa náhledu (cesta v pracovním prostoru)", "Zadejte cestu uvnitř pracovního prostoru.", "Zobrazení", "Responzivní",
        "Tablet (768 px)", "Mobil (375 px)", "Konzole", "Konzole: chyby: {{count}}", "Tato stránka nehlásí žádné chyby.",
        "Vymazat konzoli", "Dál zobrazovat tento soubor", "Sledovat aktivní soubor", "Otevřít na nové kartě prohlížeče", "Zavřít náhled", "Změnit velikost náhledu",
        "Zobrazuje se uložená verze. Uložením náhled aktualizujete.", "Otevřete soubor HTML a zobrazte jeho náhled", "Soubory HTML, HTM a SVG se otevírají zde a po uložení se znovu načtou.", "Zobrazit náhled aktivního souboru", "Načítání náhledu...",
        "Soubor nenalezen", "{{path}} v tomto pracovním prostoru neexistuje.", "Soubor je příliš velký", "{{path}} je větší než 25 MB a nelze zobrazit jeho náhled.", "Náhled není k dispozici",
        "Náhled se nepodařilo načíst: {{error}}", "Zkusit znovu", "Náhled souboru {{path}}", "{{width}} × {{height}} při {{scale}} %"],
    da: ["Forhåndsvisning", "Forhåndsvisning: Åbn ved siden af", "Forhåndsvisning: Åbn i browserfane", "Åbn forhåndsvisning", "Åbn i browserfane", "Slå HTML-forhåndsvisning til/fra",
        "Åbn forhåndsvisning ved siden af", "Genindlæs forhåndsvisning", "Forhåndsvisningens adresse (sti i arbejdsområdet)", "Angiv en sti i arbejdsområdet.", "Visningsområde", "Responsiv",
        "Tablet (768 px)", "Mobil (375 px)", "Konsol", "Konsol: {{count}} fejl", "Siden har ikke rapporteret nogen fejl.",
        "Ryd konsol", "Bliv ved med at vise denne fil", "Følg den aktive fil", "Åbn i ny browserfane", "Luk forhåndsvisning", "Tilpas størrelsen på forhåndsvisningen",
        "Den gemte version vises. Gem for at opdatere forhåndsvisningen.", "Åbn en HTML-fil for at forhåndsvise den", "HTML-, HTM- og SVG-filer åbnes her og genindlæses, når du gemmer.", "Forhåndsvis aktiv fil", "Indlæser forhåndsvisning...",
        "Filen blev ikke fundet", "{{path}} findes ikke i dette arbejdsområde.", "Filen er for stor", "{{path}} er større end 25 MB og kan ikke forhåndsvises.", "Forhåndsvisning ikke tilgængelig",
        "Forhåndsvisningen kunne ikke indlæses: {{error}}", "Prøv igen", "Forhåndsvisning af {{path}}", "{{width}} × {{height}} ved {{scale}} %"],
    el: ["Προεπισκόπηση", "Προεπισκόπηση: Άνοιγμα στο πλάι", "Προεπισκόπηση: Άνοιγμα σε καρτέλα προγράμματος περιήγησης", "Άνοιγμα προεπισκόπησης", "Άνοιγμα σε καρτέλα προγράμματος περιήγησης", "Εναλλαγή προεπισκόπησης HTML",
        "Άνοιγμα προεπισκόπησης στο πλάι", "Επαναφόρτωση προεπισκόπησης", "Διεύθυνση προεπισκόπησης (διαδρομή στον χώρο εργασίας)", "Εισαγάγετε μια διαδρομή μέσα στον χώρο εργασίας.", "Περιοχή προβολής", "Προσαρμοστική",
        "Tablet (768 px)", "Κινητό (375 px)", "Κονσόλα", "Κονσόλα: {{count}} σφάλματα", "Αυτή η σελίδα δεν ανέφερε σφάλματα.",
        "Εκκαθάριση κονσόλας", "Συνέχιση εμφάνισης αυτού του αρχείου", "Παρακολούθηση του ενεργού αρχείου", "Άνοιγμα σε νέα καρτέλα προγράμματος περιήγησης", "Κλείσιμο προεπισκόπησης", "Αλλαγή μεγέθους προεπισκόπησης",
        "Εμφανίζεται η αποθηκευμένη έκδοση. Αποθηκεύστε για να ενημερωθεί η προεπισκόπηση.", "Ανοίξτε ένα αρχείο HTML για προεπισκόπηση", "Τα αρχεία HTML, HTM και SVG ανοίγουν εδώ και επαναφορτώνονται κατά την αποθήκευση.", "Προεπισκόπηση ενεργού αρχείου", "Φόρτωση προεπισκόπησης...",
        "Το αρχείο δεν βρέθηκε", "Το {{path}} δεν υπάρχει σε αυτόν τον χώρο εργασίας.", "Πολύ μεγάλο αρχείο", "Το {{path}} ξεπερνά τα 25 MB και δεν μπορεί να προβληθεί.", "Η προεπισκόπηση δεν είναι διαθέσιμη",
        "Δεν ήταν δυνατή η φόρτωση της προεπισκόπησης: {{error}}", "Δοκιμάστε ξανά", "Προεπισκόπηση του {{path}}", "{{width}} × {{height}} στο {{scale}}%"],
    es: ["Vista previa", "Vista previa: abrir en el lateral", "Vista previa: abrir en una pestaña del navegador", "Abrir vista previa", "Abrir en una pestaña del navegador", "Alternar vista previa HTML",
        "Abrir vista previa en el lateral", "Recargar vista previa", "Dirección de la vista previa (ruta dentro del espacio de trabajo)", "Introduce una ruta dentro del espacio de trabajo.", "Ventana gráfica", "Adaptable",
        "Tableta (768 px)", "Móvil (375 px)", "Consola", "Consola: {{count}} errores", "Esta página no ha informado de errores.",
        "Limpiar consola", "Seguir mostrando este archivo", "Seguir el archivo activo", "Abrir en una nueva pestaña del navegador", "Cerrar vista previa", "Cambiar tamaño de la vista previa",
        "Se muestra la versión guardada. Guarda para actualizar la vista previa.", "Abre un archivo HTML para verlo en vista previa", "Los archivos HTML, HTM y SVG se abren aquí y se recargan al guardar.", "Vista previa del archivo activo", "Cargando vista previa...",
        "Archivo no encontrado", "{{path}} no existe en este espacio de trabajo.", "Archivo demasiado grande", "{{path}} supera los 25 MB y no se puede previsualizar.", "Vista previa no disponible",
        "No se pudo cargar la vista previa: {{error}}", "Reintentar", "Vista previa de {{path}}", "{{width}} × {{height}} al {{scale}} %"],
    fr: ["Aperçu", "Aperçu : ouvrir sur le côté", "Aperçu : ouvrir dans un onglet du navigateur", "Ouvrir l’aperçu", "Ouvrir dans un onglet du navigateur", "Afficher/masquer l’aperçu HTML",
        "Ouvrir l’aperçu sur le côté", "Recharger l’aperçu", "Adresse de l’aperçu (chemin dans l’espace de travail)", "Saisissez un chemin dans l’espace de travail.", "Fenêtre d’affichage", "Adaptatif",
        "Tablette (768 px)", "Mobile (375 px)", "Console", "Console : {{count}} erreurs", "Cette page n’a signalé aucune erreur.",
        "Vider la console", "Continuer à afficher ce fichier", "Suivre le fichier actif", "Ouvrir dans un nouvel onglet du navigateur", "Fermer l’aperçu", "Redimensionner l’aperçu",
        "Version enregistrée affichée. Enregistrez pour mettre à jour l’aperçu.", "Ouvrez un fichier HTML pour l’afficher en aperçu", "Les fichiers HTML, HTM et SVG s’ouvrent ici et se rechargent à l’enregistrement.", "Aperçu du fichier actif", "Chargement de l’aperçu...",
        "Fichier introuvable", "{{path}} n’existe pas dans cet espace de travail.", "Fichier trop volumineux", "{{path}} dépasse 25 Mo et ne peut pas être prévisualisé.", "Aperçu indisponible",
        "Impossible de charger l’aperçu : {{error}}", "Réessayer", "Aperçu de {{path}}", "{{width}} × {{height}} à {{scale}} %"],
    hi: ["पूर्वावलोकन", "पूर्वावलोकन: बगल में खोलें", "पूर्वावलोकन: ब्राउज़र टैब में खोलें", "पूर्वावलोकन खोलें", "ब्राउज़र टैब में खोलें", "HTML पूर्वावलोकन टॉगल करें",
        "पूर्वावलोकन बगल में खोलें", "पूर्वावलोकन फिर से लोड करें", "पूर्वावलोकन पता (वर्कस्पेस के अंदर पथ)", "वर्कस्पेस के अंदर का पथ दर्ज करें।", "व्यूपोर्ट", "रिस्पॉन्सिव",
        "टैबलेट (768 px)", "मोबाइल (375 px)", "कंसोल", "कंसोल: {{count}} त्रुटियाँ", "इस पेज ने कोई त्रुटि नहीं बताई।",
        "कंसोल साफ़ करें", "यही फ़ाइल दिखाते रहें", "सक्रिय फ़ाइल का अनुसरण करें", "नए ब्राउज़र टैब में खोलें", "पूर्वावलोकन बंद करें", "पूर्वावलोकन का आकार बदलें",
        "सहेजा गया संस्करण दिख रहा है। पूर्वावलोकन अपडेट करने के लिए सहेजें।", "पूर्वावलोकन के लिए कोई HTML फ़ाइल खोलें", "HTML, HTM और SVG फ़ाइलें यहाँ खुलती हैं और सहेजने पर फिर से लोड होती हैं।", "सक्रिय फ़ाइल का पूर्वावलोकन", "पूर्वावलोकन लोड हो रहा है...",
        "फ़ाइल नहीं मिली", "{{path}} इस वर्कस्पेस में मौजूद नहीं है।", "फ़ाइल बहुत बड़ी है", "{{path}} 25 MB से बड़ी है और उसका पूर्वावलोकन नहीं हो सकता।", "पूर्वावलोकन उपलब्ध नहीं",
        "पूर्वावलोकन लोड नहीं हो सका: {{error}}", "फिर से कोशिश करें", "{{path}} का पूर्वावलोकन", "{{width}} × {{height}}, {{scale}}% पर"],
    it: ["Anteprima", "Anteprima: apri lateralmente", "Anteprima: apri in una scheda del browser", "Apri anteprima", "Apri in una scheda del browser", "Attiva/disattiva anteprima HTML",
        "Apri l'anteprima lateralmente", "Ricarica anteprima", "Indirizzo dell'anteprima (percorso nell'area di lavoro)", "Inserisci un percorso all'interno dell'area di lavoro.", "Viewport", "Adattivo",
        "Tablet (768 px)", "Smartphone (375 px)", "Console", "Console: {{count}} errori", "Questa pagina non ha segnalato errori.",
        "Svuota console", "Continua a mostrare questo file", "Segui il file attivo", "Apri in una nuova scheda del browser", "Chiudi anteprima", "Ridimensiona anteprima",
        "Viene mostrata la versione salvata. Salva per aggiornare l'anteprima.", "Apri un file HTML per vederne l'anteprima", "I file HTML, HTM e SVG si aprono qui e si ricaricano al salvataggio.", "Anteprima del file attivo", "Caricamento anteprima...",
        "File non trovato", "{{path}} non esiste in quest'area di lavoro.", "File troppo grande", "{{path}} supera i 25 MB e non può essere visualizzato in anteprima.", "Anteprima non disponibile",
        "Impossibile caricare l'anteprima: {{error}}", "Riprova", "Anteprima di {{path}}", "{{width}} × {{height}} al {{scale}}%"],
    ja: ["プレビュー", "プレビュー: 横に開く", "プレビュー: ブラウザーのタブで開く", "プレビューを開く", "ブラウザーのタブで開く", "HTML プレビューの切り替え",
        "プレビューを横に開く", "プレビューを再読み込み", "プレビューのアドレス (ワークスペース内のパス)", "ワークスペース内のパスを入力してください。", "ビューポート", "レスポンシブ",
        "タブレット (768 px)", "モバイル (375 px)", "コンソール", "コンソール: エラー {{count}} 件", "このページからエラーは報告されていません。",
        "コンソールをクリア", "このファイルを表示し続ける", "アクティブなファイルに追従", "新しいブラウザータブで開く", "プレビューを閉じる", "プレビューのサイズを変更",
        "保存済みのバージョンを表示しています。保存するとプレビューが更新されます。", "HTML ファイルを開くとプレビューできます", "HTML、HTM、SVG ファイルはここに表示され、保存すると再読み込みされます。", "アクティブなファイルをプレビュー", "プレビューを読み込んでいます...",
        "ファイルが見つかりません", "{{path}} はこのワークスペースに存在しません。", "ファイルが大きすぎます", "{{path}} は 25 MB を超えているためプレビューできません。", "プレビューを利用できません",
        "プレビューを読み込めませんでした: {{error}}", "再試行", "{{path}} のプレビュー", "{{width}} × {{height}} ({{scale}}%)"],
    nl: ["Voorbeeld", "Voorbeeld: aan de zijkant openen", "Voorbeeld: in browsertabblad openen", "Voorbeeld openen", "In browsertabblad openen", "HTML-voorbeeld in-/uitschakelen",
        "Voorbeeld aan de zijkant openen", "Voorbeeld opnieuw laden", "Voorbeeldadres (pad binnen de werkruimte)", "Voer een pad binnen de werkruimte in.", "Weergavegebied", "Responsief",
        "Tablet (768 px)", "Mobiel (375 px)", "Console", "Console: {{count}} fouten", "Deze pagina heeft geen fouten gemeld.",
        "Console wissen", "Dit bestand blijven tonen", "Het actieve bestand volgen", "In nieuw browsertabblad openen", "Voorbeeld sluiten", "Voorbeeldgrootte wijzigen",
        "Opgeslagen versie wordt getoond. Sla op om het voorbeeld bij te werken.", "Open een HTML-bestand om er een voorbeeld van te zien", "HTML-, HTM- en SVG-bestanden openen hier en laden opnieuw bij opslaan.", "Voorbeeld van actief bestand", "Voorbeeld laden...",
        "Bestand niet gevonden", "{{path}} bestaat niet in deze werkruimte.", "Bestand te groot", "{{path}} is groter dan 25 MB en kan niet worden getoond.", "Voorbeeld niet beschikbaar",
        "Het voorbeeld kon niet worden geladen: {{error}}", "Opnieuw proberen", "Voorbeeld van {{path}}", "{{width}} × {{height}} op {{scale}}%"],
    no: ["Forhåndsvisning", "Forhåndsvisning: Åpne ved siden av", "Forhåndsvisning: Åpne i nettleserfane", "Åpne forhåndsvisning", "Åpne i nettleserfane", "Slå HTML-forhåndsvisning av/på",
        "Åpne forhåndsvisning ved siden av", "Last inn forhåndsvisningen på nytt", "Forhåndsvisningens adresse (sti i arbeidsområdet)", "Skriv inn en sti i arbeidsområdet.", "Visningsområde", "Responsiv",
        "Nettbrett (768 px)", "Mobil (375 px)", "Konsoll", "Konsoll: {{count}} feil", "Siden har ikke rapportert noen feil.",
        "Tøm konsollen", "Fortsett å vise denne filen", "Følg den aktive filen", "Åpne i ny nettleserfane", "Lukk forhåndsvisning", "Endre størrelse på forhåndsvisningen",
        "Den lagrede versjonen vises. Lagre for å oppdatere forhåndsvisningen.", "Åpne en HTML-fil for å forhåndsvise den", "HTML-, HTM- og SVG-filer åpnes her og lastes inn på nytt når du lagrer.", "Forhåndsvis aktiv fil", "Laster inn forhåndsvisning...",
        "Finner ikke filen", "{{path}} finnes ikke i dette arbeidsområdet.", "Filen er for stor", "{{path}} er større enn 25 MB og kan ikke forhåndsvises.", "Forhåndsvisning utilgjengelig",
        "Forhåndsvisningen kunne ikke lastes inn: {{error}}", "Prøv igjen", "Forhåndsvisning av {{path}}", "{{width}} × {{height}} ved {{scale}} %"],
    pl: ["Podgląd", "Podgląd: otwórz z boku", "Podgląd: otwórz w karcie przeglądarki", "Otwórz podgląd", "Otwórz w karcie przeglądarki", "Przełącz podgląd HTML",
        "Otwórz podgląd z boku", "Odśwież podgląd", "Adres podglądu (ścieżka w obszarze roboczym)", "Podaj ścieżkę w obszarze roboczym.", "Widok", "Responsywny",
        "Tablet (768 px)", "Telefon (375 px)", "Konsola", "Konsola: błędy: {{count}}", "Ta strona nie zgłosiła błędów.",
        "Wyczyść konsolę", "Nadal pokazuj ten plik", "Podążaj za aktywnym plikiem", "Otwórz w nowej karcie przeglądarki", "Zamknij podgląd", "Zmień rozmiar podglądu",
        "Wyświetlana jest zapisana wersja. Zapisz, aby zaktualizować podgląd.", "Otwórz plik HTML, aby zobaczyć podgląd", "Pliki HTML, HTM i SVG otwierają się tutaj i odświeżają po zapisaniu.", "Podgląd aktywnego pliku", "Wczytywanie podglądu...",
        "Nie znaleziono pliku", "{{path}} nie istnieje w tym obszarze roboczym.", "Plik jest za duży", "{{path}} ma ponad 25 MB i nie można go wyświetlić w podglądzie.", "Podgląd niedostępny",
        "Nie udało się wczytać podglądu: {{error}}", "Spróbuj ponownie", "Podgląd: {{path}}", "{{width}} × {{height}} przy {{scale}}%"],
    pt: ["Pré-visualização", "Pré-visualização: abrir ao lado", "Pré-visualização: abrir num separador do navegador", "Abrir pré-visualização", "Abrir num separador do navegador", "Alternar pré-visualização HTML",
        "Abrir pré-visualização ao lado", "Recarregar pré-visualização", "Endereço da pré-visualização (caminho no espaço de trabalho)", "Introduza um caminho dentro do espaço de trabalho.", "Área de visualização", "Adaptável",
        "Tablet (768 px)", "Telemóvel (375 px)", "Consola", "Consola: {{count}} erros", "Esta página não comunicou erros.",
        "Limpar consola", "Continuar a mostrar este ficheiro", "Seguir o ficheiro ativo", "Abrir num novo separador do navegador", "Fechar pré-visualização", "Redimensionar pré-visualização",
        "A mostrar a versão guardada. Guarde para atualizar a pré-visualização.", "Abra um ficheiro HTML para o pré-visualizar", "Os ficheiros HTML, HTM e SVG abrem aqui e recarregam ao guardar.", "Pré-visualizar ficheiro ativo", "A carregar pré-visualização...",
        "Ficheiro não encontrado", "{{path}} não existe neste espaço de trabalho.", "Ficheiro demasiado grande", "{{path}} tem mais de 25 MB e não pode ser pré-visualizado.", "Pré-visualização indisponível",
        "Não foi possível carregar a pré-visualização: {{error}}", "Tentar novamente", "Pré-visualização de {{path}}", "{{width}} × {{height}} a {{scale}}%"],
    sv: ["Förhandsgranskning", "Förhandsgranskning: öppna vid sidan", "Förhandsgranskning: öppna i webbläsarflik", "Öppna förhandsgranskning", "Öppna i webbläsarflik", "Växla HTML-förhandsgranskning",
        "Öppna förhandsgranskning vid sidan", "Läs in förhandsgranskningen igen", "Förhandsgranskningens adress (sökväg i arbetsytan)", "Ange en sökväg inom arbetsytan.", "Visningsområde", "Responsiv",
        "Surfplatta (768 px)", "Mobil (375 px)", "Konsol", "Konsol: {{count}} fel", "Sidan har inte rapporterat några fel.",
        "Rensa konsolen", "Fortsätt visa den här filen", "Följ den aktiva filen", "Öppna i ny webbläsarflik", "Stäng förhandsgranskning", "Ändra storlek på förhandsgranskningen",
        "Den sparade versionen visas. Spara för att uppdatera förhandsgranskningen.", "Öppna en HTML-fil för att förhandsgranska den", "HTML-, HTM- och SVG-filer öppnas här och läses in igen när du sparar.", "Förhandsgranska aktiv fil", "Läser in förhandsgranskning...",
        "Filen hittades inte", "{{path}} finns inte i den här arbetsytan.", "Filen är för stor", "{{path}} är större än 25 MB och kan inte förhandsgranskas.", "Förhandsgranskning inte tillgänglig",
        "Förhandsgranskningen kunde inte läsas in: {{error}}", "Försök igen", "Förhandsgranskning av {{path}}", "{{width}} × {{height}} vid {{scale}} %"],
    zh: ["预览", "预览：在侧边打开", "预览：在浏览器标签页中打开", "打开预览", "在浏览器标签页中打开", "切换 HTML 预览",
        "在侧边打开预览", "重新加载预览", "预览地址（工作区内的路径）", "请输入工作区内的路径。", "视口", "自适应",
        "平板 (768 px)", "手机 (375 px)", "控制台", "控制台：{{count}} 个错误", "此页面未报告任何错误。",
        "清空控制台", "继续显示此文件", "跟随活动文件", "在新浏览器标签页中打开", "关闭预览", "调整预览大小",
        "正在显示已保存的版本。保存后预览会更新。", "打开 HTML 文件即可预览", "HTML、HTM 和 SVG 文件会在此打开，并在保存时重新加载。", "预览活动文件", "正在加载预览...",
        "未找到文件", "此工作区中不存在 {{path}}。", "文件过大", "{{path}} 超过 25 MB，无法预览。", "预览不可用",
        "无法加载预览：{{error}}", "重试", "{{path}} 的预览", "{{width}} × {{height}}，{{scale}}%"]
};

const placeholders = text => (text.match(/\{\{\w+\}\}/g) || []).sort().join(',');
const langs = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
for (const lang of langs) {
    const values = T[lang];
    if (!values || values.length !== KEYS.length) throw new Error(lang + ': ' + (values ? values.length : 0) + ' values, want ' + KEYS.length);
    values.forEach((value, i) => {
        if (placeholders(value) !== placeholders(T.en[i])) throw new Error(lang + ' ' + KEYS[i] + ': placeholders differ from English');
    });
    const file = path.join(repo, 'ui', 'lang', 'desktop', lang + '.json');
    const text = fs.readFileSync(file, 'utf8');
    const eol = text.includes('\r\n') ? '\r\n' : '\n';
    const data = JSON.parse(text);
    const lines = text.split(eol);
    const anchor = lines.findIndex(line => line.trimStart().startsWith('"codeStudio.lspFailHiddenUnsaved"'));
    if (anchor < 0) throw new Error(lang + ': anchor codeStudio.lspFailHiddenUnsaved missing');
    if (!lines[anchor].trimEnd().endsWith(',')) throw new Error(lang + ': anchor is the last entry');
    const add = [];
    KEYS.forEach((key, i) => {
        const full = 'codeStudio.' + key;
        if (Object.prototype.hasOwnProperty.call(data, full)) return;
        add.push('  ' + JSON.stringify(full) + ': ' + JSON.stringify(values[i]) + ',');
    });
    lines.splice(anchor + 1, 0, ...add);
    const out = lines.join(eol);
    JSON.parse(out);
    fs.writeFileSync(file, out);
    console.log(lang + ': +' + add.length);
}
```

Run:
```
node C:/disposable/cs-preview/add-preview-i18n.cjs C:/Users/Andi/Documents/repo/AuraGo/.claude/worktrees/cs-ssh-preview
node -e "const fs=require('fs');for(const l of ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh']){const d=JSON.parse(fs.readFileSync('ui/lang/desktop/'+l+'.json','utf8'));const n=Object.keys(d).filter(k=>k.startsWith('codeStudio.preview')).length;if(n!==37)throw new Error(l+' '+n);}console.log('ok 16x37')"
go test ./ui/ -run '(?i)CodeStudio' -count=1
```
Expected:
- 16 lines `xx: +37`
- `ok 16x37`
- `ok  aurago/ui` (browser tests skip without `AURAGO_RUN_BROWSER_SMOKE`; the asset and i18n tests pass)

Commit:
```
git add ui/lang/desktop/*.json
git commit -m "i18n(code-studio): preview strings in all 16 desktop languages"
```

---

## Task 16: One browser smoke for the pane (10 min)

1. **Fixture.** In `ui/desktop_code_studio_browser_test.go`, `register()`, insert before `upgrader := websocket.Upgrader{…}`:

```go
	// Plan B: the HTML preview. The token route answers like the server (sandbox CSP, 404 for missing files) and
	// injects a script that reports its location and one console error the way live.js does.
	mux.HandleFunc("/api/code-studio/preview", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Path string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		base := "/code-studio-preview/fixture-token/"
		writeJSON(w, map[string]interface{}{"status": "ok", "base": base, "url": base + strings.TrimPrefix(body.Path, "/workspace/"), "token": "fixture-token", "workspace": "local", "version": 0, "expires_at": time.Now().Add(12 * time.Hour).UTC().Format(time.RFC3339)})
	})
	mux.HandleFunc("/code-studio-preview/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "sandbox allow-scripts allow-forms allow-modals allow-popups allow-downloads; frame-ancestors 'self'")
		w.Header().Set("Cache-Control", "no-store")
		rel := strings.TrimPrefix(r.URL.Path, "/code-studio-preview/fixture-token/")
		if rel == "" || strings.HasSuffix(rel, "/") {
			rel += "index.html"
		}
		b.mu.Lock()
		content, ok := b.files["/workspace/"+rel]
		b.mu.Unlock()
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		probe := `<script>parent.postMessage({auragoPreview:1,kind:'location',text:'` + rel + `'},'*');parent.postMessage({auragoPreview:1,kind:'console',text:'fixture boom'},'*');</script>`
		fmt.Fprint(w, strings.Replace(content, "</body>", probe+"</body>", 1))
	})
```

2. **Test.** Create `ui/desktop_code_studio_preview_browser_test.go`:

```go
package ui

import "testing"

// Plan B: the HTML preview pane against the fixture preview route. Checks the sandbox flags, the console badge
// (messages from the preview frame count, forged ones from the top window do not), the device viewport, the
// keyboard splitter with persistence, the eye button and dirty hint, the error card and Ctrl+Shift+V.
// The fixture UI runs in German, so every check uses data attributes and state, not labels.
func TestDesktopCodeStudioPreviewBrowser(t *testing.T) {
	page, backend := newCodeStudioBrowser(t)
	backend.mu.Lock()
	backend.files["/workspace/index.html"] = "<!doctype html><html><head><title>Fixture</title></head><body><h1>Hallo</h1></body></html>"
	backend.mu.Unlock()
	browserErrors := func() string { return page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str() }
	scenario := func(name, js string, args ...interface{}) {
		t.Helper()
		result, err := page.Eval(js, args...)
		if err != nil {
			t.Fatalf("%s: eval failed: %v; browser errors: %s", name, err, browserErrors())
		}
		if !result.Value.Get("ok").Bool() {
			t.Fatalf("%s: %s; browser errors: %s", name, result.Value.JSON("", ""), browserErrors())
		}
	}
	if _, err := page.Eval(`async()=>{await csWait(()=>csRoot()&&csRoot().querySelectorAll('.cs-tree-item').length>0,20000);}`); err != nil {
		t.Fatalf("Code Studio did not open: %v; errors=%s", err, browserErrors())
	}

	scenario("pane opens sandboxed", `async()=>{
		CodeStudioApp.command('openPreview',['/workspace/index.html']);
		const pane=await csWait(()=>{const el=csRoot().querySelector('[data-html-preview]');return el&&!el.hidden&&el.getBoundingClientRect().width>200&&el;},8000);
		await csWait(()=>csState().htmlPreview.status==='ready',8000);
		const frame=pane.querySelector('[data-hp-frame]');
		const sandbox=frame.getAttribute('sandbox');
		return {ok:csRoot().dataset.preview==='visible'&&sandbox==='allow-scripts allow-forms allow-modals allow-popups allow-downloads'&&!/allow-same-origin/.test(sandbox)&&frame.src.endsWith('/code-studio-preview/fixture-token/index.html'),sandbox,src:frame.src};
	}`)

	scenario("console badge counts only messages from the preview frame", `async()=>{
		const pane=csRoot().querySelector('[data-html-preview]');
		const badge=pane.querySelector('[data-hp-console-count]');
		await csWait(()=>!badge.hidden&&badge.textContent==='1',8000);
		window.postMessage({auragoPreview:1,kind:'console',text:'forged'},'*');
		await new Promise(resolve=>setTimeout(resolve,300));
		const count=badge.textContent;
		pane.querySelector('[data-hp-action="console"]').click();
		const entries=[...pane.querySelectorAll('.cs-hp-console-entry pre')].map(el=>el.textContent);
		const address=pane.querySelector('[data-hp-address]').value;
		pane.querySelector('[data-hp-action="clear-console"]').click();
		return {ok:count==='1'&&entries.length===1&&entries[0]==='fixture boom'&&address==='index.html'&&badge.hidden,count,entries,address};
	}`)

	scenario("mobile viewport shows a 375 px device", `async()=>{
		const pane=csRoot().querySelector('[data-html-preview]');
		const button=pane.querySelector('[data-hp-viewport="mobile"]');
		button.click();
		const frame=pane.querySelector('[data-hp-frame]');
		await csWait(()=>frame.offsetWidth===375,4000);
		return {ok:pane.querySelector('[data-hp-stage]').dataset.viewport==='mobile'&&button.getAttribute('aria-checked')==='true'&&!pane.querySelector('[data-hp-device-label]').hidden,width:frame.offsetWidth};
	}`)

	scenario("splitter resizes with the keyboard and persists", `async()=>{
		const handle=csRoot().querySelector('[data-hp-resize]');
		const before=csState().htmlPreview.width;
		handle.focus();
		csKey(handle,{key:'ArrowLeft'});
		await csWait(()=>csState().htmlPreview.width===before+16,2000);
		const saved=JSON.parse(localStorage.getItem('aurago.codeStudio.preview.v1')||'{}');
		return {ok:saved.width===before+16&&handle.getAttribute('aria-valuenow')===String(before+16)&&saved.viewport==='mobile',before,saved};
	}`)

	scenario("eye button and dirty hint follow the editor tab", `async()=>{
		await CodeStudioApp.openFile('/workspace/index.html');
		const tab=await csWait(()=>csState().openTabs.find(item=>item.path==='/workspace/index.html'&&item.view&&item.view.state),10000);
		const eye=await csWait(()=>csRoot().querySelector('[data-breadcrumbs] [data-hp-eye]'),4000);
		const hint=csRoot().querySelector('[data-hp-dirty]');
		const hiddenBefore=hint.hidden;
		tab.view.dispatch({changes:{from:0,insert:'<!-- edit -->'}});
		await csWait(()=>tab.modified&&!hint.hidden,4000);
		return {ok:eye.getAttribute('aria-pressed')==='true'&&hiddenBefore&&!hint.hidden,pressed:eye.getAttribute('aria-pressed'),hiddenBefore};
	}`)

	scenario("missing file shows the error card", `async()=>{
		CodeStudioApp.command('openPreview',['/workspace/gone.html']);
		const card=await csWait(()=>{const el=csRoot().querySelector('[data-hp-error]');return el&&!el.hidden&&el;},8000);
		return {ok:csState().htmlPreview.status==='error'&&card.getAttribute('role')==='alert'&&!!card.querySelector('[data-hp-action="retry"]')&&card.querySelector('[data-hp-error-detail]').textContent.includes('gone.html'),status:csState().htmlPreview.status};
	}`)

	scenario("Ctrl+Shift+V toggles the pane for the active HTML tab", `async()=>{
		const content=csRoot().querySelector('.cm-content');
		content.focus();
		csKey(content,{key:'V',code:'KeyV',ctrlKey:true,shiftKey:true});
		await csWait(()=>csRoot().dataset.preview==='hidden'&&csRoot().querySelector('[data-html-preview]').hidden,2000);
		csKey(content,{key:'V',code:'KeyV',ctrlKey:true,shiftKey:true});
		await csWait(()=>csRoot().dataset.preview==='visible'&&csState().htmlPreview.path==='/workspace/index.html',4000);
		return {ok:true};
	}`)

	scenario("no browser errors", `()=>({ok:fixtureErrors.length===0,errors:fixtureErrors})`)
}
```

Run (PowerShell):
```
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./ui/ -run '(?i)DesktopCodeStudioPreviewBrowser' -count=1 -v -timeout 10m; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
```
Expected: `--- PASS: TestDesktopCodeStudioPreviewBrowser` and `ok  aurago/ui`. If Chrome cannot start, use the memory `reference_chrome_launch_from_agent` recipe (test binary built into `D:\tmp\go-build`, sandbox off).

Commit:
```
git add ui/desktop_code_studio_browser_test.go ui/desktop_code_studio_preview_browser_test.go
git commit -m "test(code-studio): browser smoke for the HTML preview pane"
```

---

## Task 17: Final verification and graph check (5 min)

Run:
```
go test ./internal/server/ -run '(?i)(CodeStudio|AuthBypass|SecurityHeaders)' -count=1
node --test ui/js/desktop/apps/code-studio/preview-model.test.cjs ui/js/desktop/apps/code-studio/lsp-protocol.test.cjs ui/js/desktop/apps/code-studio/lsp-edits.test.cjs
npm run build:ui -- --check
go test ./ui/ -run '(?i)CodeStudio' -count=1
go vet ./internal/server/
```
Expected: every Go command ends in `ok`; node reports `# fail 0`; the bundle check exits 0; vet reports nothing.

Graph change analysis (CLAUDE.md):
```
node ../../../.gitnexus/maintenance/AuraGo/graph/run.cjs detect-changes --scope compare --base-ref "main" --repo .
```
Run it from the main checkout, pointed at this branch if the CLI supports a ref; otherwise run it in the worktree. If the worktree is not indexed (known gap) or the output is `partial`/`truncated`, record that in the hand-off together with the Task 0 impact results. Do not report it as clean.

No extra commit unless something was fixed.

---

## Risks and notes

- **Parallel plans.**
  - Plan A also edits `code_studio_handlers.go`. Plan B's edits are one line in `registerCodeStudioRoutes` plus, in Mode L only, bump lines.
  - Plan C edits `core.js`, `sidebar.js` and `code-studio.css`. Plan B uses five one-line hooks and its own CSS file.
  - Plans C and D also add locale keys. Plan B's insertion anchor (`codeStudio.lspFailHiddenUnsaved`) may produce a textual merge conflict; resolve it by keeping both blocks.
- **Isolation cost.** No `allow-same-origin`, so `localStorage`, `sessionStorage`, IndexedDB and `document.cookie` throw a SecurityError inside previews. The console badge makes this visible. A future option is the existing isolated `preview_domain` gateway; that is out of scope here.
- **Bumps.** Only Code Studio API writes bump the version (plus agent writes once Plan D lands). Terminal edits, build output from Run, and writes by other tools do not reload the preview; the Reload button covers them.
- **Connections.** Each open preview keeps one long-poll request open. Under HTTP/1.1 with many preview tabs, the 6-connections-per-host limit can be reached; HTTPS/HTTP2 removes the issue.
- **Logs.** The token is in the URL path. If access logs record paths, administrators can see it. It grants read-only access for 12 hours, and `Referrer-Policy: no-referrer` keeps it away from third-party resources.
- **Charset.** HTML is always served as UTF-8. Pages in another encoding that rely on `<meta charset>` show mojibake.
- **Injection point.** `live.js` goes right after `<head>`, not before `</body>` as the spec says. This lets it capture errors from the page's own scripts. When there is no `<head>`, it falls back to the spec's placement.

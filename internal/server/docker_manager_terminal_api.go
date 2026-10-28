package server

import (
	"encoding/json"
	"net/http"
	"time"

	"aurago/internal/dockerhost"

	"github.com/gorilla/websocket"
)

// ── Docker Manager terminal route ───────────────────────────────────────────
// GET /hosts/{id}/containers/{ref}/terminal is a WebSocket with the protocol of
// /api/containers/{id}/terminal (binary stdin and stdout, text frames
// {"type":"resize","cols","rows"} and {"type":"end"}) on any registered host.
// Everything that can refuse the request is checked, and answered as ordinary
// JSON, before the upgrade; a browser hides the body of a refused handshake, so
// the desktop app repeats the request without the upgrade headers to read why. For
// that reason a request without them goes through every check up to the stream
// slot too (all of them only read) and is answered 400 websocket_required only
// then; it never creates an exec.

// dmTerminalSetupTimeout bounds the checks before the upgrade (inspect, the
// protection lookup).
const dmTerminalSetupTimeout = 30 * time.Second

// dmTerminalInspect is the part of a projected inspect document the terminal
// route reads (tools.ProjectDockerInspect keeps the Engine's own state object).
type dmTerminalInspect struct {
	State struct {
		Running bool `json:"Running"`
	} `json:"state"`
	Config struct {
		Labels map[string]string `json:"labels"`
	} `json:"config"`
}

// containerTerminal serves the terminal of a container. The order of the
// answers: 405 (not GET), 403 read_only, 403 forbidden_origin, 404 for an unknown
// host or container, 409 container_not_running, 409 helper_container, 409
// container_protected_confirmation_required (the protection rule of
// /api/containers: needs ?confirm=protected; the same rule for a remote host as for
// removing a container), 429 too_many_streams, 400 websocket_required, then the
// exec is created and the connection upgraded.
func (dm *dmRequest) containerTerminal(hostID, ref string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	if dm.readOnly {
		dm.readOnlyRefusal()
		return
	}
	if !sameOriginOrNoOrigin(dm.r) {
		dmWriteError(dm.w, http.StatusForbidden, dmCodeForbiddenOrigin, "forbidden websocket origin", nil)
		return
	}
	live, ok := dm.liveService()
	if !ok {
		return
	}
	ctx, cancel := dm.context(dmTerminalSetupTimeout)
	defer cancel()
	host, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	raw, err := dockerhost.InspectContainer(ctx, client, ref)
	if err != nil {
		dm.engineFailure(err, dmCodeContainerNotFound)
		return
	}
	var info dmTerminalInspect
	if err := json.Unmarshal(raw, &info); err != nil {
		dm.internalFailure("Docker container inspect could not be read", err)
		return
	}
	if !info.State.Running {
		dmWriteError(dm.w, http.StatusConflict, dmCodeNotRunning, "The container is not running", nil)
		return
	}
	if role := dmHelperRole(info.Config.Labels); role != "" {
		dmWriteError(dm.w, http.StatusConflict, dmCodeHelperContainer, "This is one of the Docker Manager's own helper containers; it does not take a terminal", dmFields{"role": role})
		return
	}
	if !dm.protectionAllowed(ctx, host, client, ref, "terminal") {
		return
	}
	release, err := live.Acquire(hostID)
	if err != nil {
		dm.liveFailure(err, "")
		return
	}
	defer release()
	if !websocket.IsWebSocketUpgrade(dm.r) {
		release() // a diagnosing GET holds no slot; release is idempotent
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeWebSocketRequired, "The terminal needs a WebSocket upgrade", nil)
		return
	}

	session, err := newDockerManagerTerminalSession(dm.r.Context(), client, ref, 120, 30)
	if err != nil {
		if dockerhost.StatusOf(err) == http.StatusConflict {
			dmWriteError(dm.w, http.StatusConflict, dmCodeNotRunning, "The container is not running", nil)
			return
		}
		dm.engineFailure(err, dmCodeContainerNotFound)
		return
	}
	if dm.r.Context().Err() != nil {
		_ = session.Close()
		http.Error(dm.w, "terminal request was canceled", http.StatusRequestTimeout)
		return
	}
	conn, err := containerTerminalUpgrader.Upgrade(dm.w, dm.r, nil)
	if err != nil {
		_ = session.Close() // Upgrade has answered the handshake itself
		return
	}
	serveDockerManagerTerminal(dm.r.Context(), conn, session)
}

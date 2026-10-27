package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

// ── Docker Manager API ──────────────────────────────────────────────────────
// Everything below /api/docker-manager/. Authentication is the generic
// /api/ middleware (cookie session, or a bearer token with the admin scope),
// the same path as /api/containers. Envelope: {"status":"ok", ...} or
// {"status":"error","code":"...","message":"..."}.

const (
	dockerManagerPrefix = "/api/docker-manager/"
	dmMaxBody           = 1 << 20
	dmRegistryTimeout   = 15 * time.Second
)

// Error codes. The desktop app switches on them, so they are stable.
const (
	dmCodeDisabled          = "docker_disabled"
	dmCodeReadOnly          = "read_only"
	dmCodeHostNotFound      = "host_not_found"
	dmCodeInvalidHost       = "invalid_host"
	dmCodeLocalHostFixed    = "local_host_fixed"
	dmCodeHostUnreachable   = "host_unreachable"
	dmCodeNestNotFound      = "nest_not_found"
	dmCodeNestNotDocker     = "nest_not_docker"
	dmCodeContainerNotFound = "container_not_found"
	dmCodeInvalidAction     = "invalid_action"
	dmCodeDockerError       = "docker_error"
	// Not in the contract table; they cover requests that reach no feature route.
	dmCodeUnavailable = "docker_hosts_unavailable"
	dmCodeNotFound    = "not_found"
	dmCodeMethod      = "method_not_allowed"
	dmCodeBadRequest  = "bad_request"
	dmCodeInternal    = "internal_error"
)

// dmFields are the payload members next to "status".
type dmFields map[string]any

func dmWriteOK(w http.ResponseWriter, status int, fields dmFields) {
	body := map[string]any{"status": "ok"}
	for key, value := range fields {
		body[key] = value
	}
	containerJSON(w, status, body)
}

func dmWriteError(w http.ResponseWriter, status int, code, message string, extra dmFields) {
	body := map[string]any{"status": "error", "code": code, "message": message}
	for key, value := range extra {
		body[key] = value
	}
	containerJSON(w, status, body)
}

// dmRequest is one routed request with the Docker switches read once.
type dmRequest struct {
	s        *Server
	w        http.ResponseWriter
	r        *http.Request
	readOnly bool
}

func (dm *dmRequest) registry() *dockerhost.Registry { return dm.s.DockerHosts }

func (dm *dmRequest) context(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(dm.r.Context(), timeout)
}

// allow answers 405 unless the request method is one of methods.
func (dm *dmRequest) allow(methods ...string) bool {
	for _, method := range methods {
		if dm.r.Method == method {
			return true
		}
	}
	dm.w.Header().Set("Allow", strings.Join(methods, ", "))
	dmWriteError(dm.w, http.StatusMethodNotAllowed, dmCodeMethod, "method not allowed", nil)
	return false
}

// decode reads a JSON body of at most dmMaxBody bytes.
func (dm *dmRequest) decode(dst any) bool {
	body := http.MaxBytesReader(dm.w, dm.r.Body, dmMaxBody)
	if err := json.NewDecoder(body).Decode(dst); err != nil {
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeBadRequest, "the request body is not valid JSON", nil)
		return false
	}
	return true
}

func (dm *dmRequest) notFound() {
	dmWriteError(dm.w, http.StatusNotFound, dmCodeNotFound, "unknown Docker Manager route", nil)
}

func (dm *dmRequest) readOnlyRefusal() {
	dmWriteError(dm.w, http.StatusForbidden, dmCodeReadOnly, "Docker is in read-only mode", nil)
}

// internalFailure logs the cause and answers a generic 500.
func (dm *dmRequest) internalFailure(what string, err error) {
	dm.s.logDockerManager(what, err)
	dmWriteError(dm.w, http.StatusInternalServerError, dmCodeInternal, "The Docker Manager could not complete the request", nil)
}

// host resolves a host id; an unknown id answers 404.
func (dm *dmRequest) host(ctx context.Context, id string) (dockerhost.Host, bool) {
	host, err := dm.registry().Host(ctx, id)
	if err != nil {
		dm.hostFailure(err)
		return dockerhost.Host{}, false
	}
	return host, true
}

// engine resolves a host id to the host and its Engine client.
func (dm *dmRequest) engine(ctx context.Context, id string) (dockerhost.Host, *dockerutil.Client, bool) {
	host, ok := dm.host(ctx, id)
	if !ok {
		return dockerhost.Host{}, nil, false
	}
	client, err := dm.registry().Client(ctx, id)
	if err != nil {
		dm.engineFailure(err, "")
		return host, nil, false
	}
	return host, client, true
}

// hostFailure maps registry and validation errors onto the contract codes.
func (dm *dmRequest) hostFailure(err error) {
	var invalid *dockerhost.ValidationError
	switch {
	case errors.Is(err, dockerhost.ErrHostNotFound):
		dmWriteError(dm.w, http.StatusNotFound, dmCodeHostNotFound, "Docker host not found", nil)
	case errors.Is(err, dockerhost.ErrLocalHostFixed):
		dmWriteError(dm.w, http.StatusConflict, dmCodeLocalHostFixed, dockerhost.ErrLocalHostFixed.Error(), nil)
	case errors.As(err, &invalid):
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeInvalidHost, invalid.Message, dmFields{"field": invalid.Field})
	default:
		dm.internalFailure("Docker host operation failed", err)
	}
}

// engineFailure answers an error from an Engine call or from the client of a
// host. dockerhost.StatusOf separates the cases: a rejected action or container
// reference is a 400; 404 with notFoundCode (when given) is "no such object";
// any other Engine status is docker_error; status 0 means nothing answered
// (dial, TLS, SSH, timeout or a client that could not be built) and becomes
// host_unreachable with the classified reason.
func (dm *dmRequest) engineFailure(err error, notFoundCode string) {
	message := dockerutil.SanitizeOneLine(err.Error(), 512)
	status := dockerhost.StatusOf(err)
	switch {
	case errors.Is(err, dockerhost.ErrInvalidAction):
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeInvalidAction, message, nil)
	case errors.Is(err, dockerhost.ErrInvalidContainerRef):
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeBadRequest, message, nil)
	case status == http.StatusNotFound && notFoundCode != "":
		dmWriteError(dm.w, http.StatusNotFound, notFoundCode, message, nil)
	case status > 0:
		dmWriteError(dm.w, http.StatusBadGateway, dmCodeDockerError, message, nil)
	default:
		dmWriteError(dm.w, http.StatusBadGateway, dmCodeHostUnreachable, message, dmFields{"reason": dockerhost.ClassifyError(err)})
	}
}

// handleDockerManager serves /api/docker-manager/. Docker disabled answers 503
// for every route; docker.readonly blocks only the container mutations.
func handleDockerManager(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, enabled, readOnly := containerDockerConfig(s)
		if !enabled {
			dmWriteError(w, http.StatusServiceUnavailable, dmCodeDisabled, "Docker is not enabled", nil)
			return
		}
		if s.DockerHosts == nil {
			dmWriteError(w, http.StatusServiceUnavailable, dmCodeUnavailable, "The Docker host registry is not available", nil)
			return
		}
		dm := &dmRequest{s: s, w: w, r: r, readOnly: readOnly}
		dm.route(dockerManagerSegments(r.URL.Path))
	}
}

// dockerManagerSegments splits the path below the prefix; an empty segment
// makes the whole path unroutable.
func dockerManagerSegments(path string) []string {
	rest := strings.Trim(strings.TrimPrefix(path, dockerManagerPrefix), "/")
	if rest == "" {
		return nil
	}
	segs := strings.Split(rest, "/")
	for _, seg := range segs {
		if seg == "" {
			return nil
		}
	}
	return segs
}

// route dispatches by path shape. The fixed words "test" and "import-nest"
// come before the host-id patterns; generated host ids never equal them.
func (dm *dmRequest) route(segs []string) {
	switch {
	default:
		dm.notFound()
	}
}

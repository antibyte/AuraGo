package server

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/cydcreator"
	"aurago/internal/dockerutil"
	"aurago/internal/tools"
)

// The CYD Creator builder runs one long-lived toolchain container
// (sleep infinity) and executes cyd-build / cyd-addr2line in it. Files move
// through the Docker archive API (PUT /archive upload, GET /archive
// download), never bind mounts, so it works with a remote docker.host and
// when AuraGo itself runs in Docker. The root filesystem is read-only; /work
// and /cache are named volumes (the archive API cannot write into tmpfs
// mounts or a read-only root) and /tmp is a tmpfs.
//
// The container compiles user and agent code, so everything that comes back
// from it (exec output frames, the out/ archive) is untrusted and bounded.
const (
	cydBuilderContainerName = "aurago-cyd-builder"
	cydBuilderCacheVolume   = "aurago-cyd-cache"
	cydBuilderWorkVolume    = "aurago-cyd-work"
	cydBuilderOwner         = "cyd-creator"
	cydBuilderUser          = "1000:1000"
	cydBuilderPidsLimit     = 512
	cydBuilderTmpfsOptions  = "rw,exec,nosuid,nodev,size=1g"
	cydBuilderWorkDir       = "/work"
	cydBuilderMaxArtifacts  = 256 << 20 // GET /archive of out/, also the sum of the extracted files
	cydBuilderMaxA2LOutput  = 1 << 20
	cydBuilderMaxLogLine    = 4096
	cydBuilderPullTimeout   = 2 * time.Hour

	// cyd_creator defaults (contract: --memory 2g --memory-swap 2g --cpus 2),
	// used when the settings carry none.
	cydBuilderDefaultMemoryMB = 2048
	cydBuilderDefaultCPUCores = 2

	cydBuilderMaxFrame     = 1 << 20 // one multiplexed exec output frame
	cydBuilderMaxRawOutput = 8 << 20 // exec output without frame headers (TTY)
	// cydBuilderMaxImageBytes caps merged.bin and firmware.bin: a merged image
	// never exceeds the flash chip, and the largest supported one is 16 MB
	// (cydcreator.PartitionTable: 4, 8 or 16).
	cydBuilderMaxImageBytes      = 16 << 20
	cydBuilderMaxResultJSON      = 1 << 20
	cydBuilderMaxArtifactEntries = 1024
	// cydBuilderArtifactRoot is the first path segment of every GET /archive
	// entry: the Engine names them below the base name of /work/<id>/out.
	cydBuilderArtifactRoot = "out"

	// cydDockerUnreachable is the user-visible text for a failed Engine
	// connection; the cause (socket path, address) goes to the server log.
	cydDockerUnreachable = "Docker is not reachable; the server log has the details"
)

// Arguments the adapter passes to the container are checked here too
// (defence in depth; cyd-build and cyd-addr2line check them again): the
// PlatformIO env is a board id, the cache key follows cyd-build's ID_RE and
// addresses cyd-addr2line's ADDR_RE (32-bit, at most 64).
var (
	cydBuildIDPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{0,63}$`)
	cydPIOEnvPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,47}$`)
	cydCacheKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
	cydAddressPattern  = regexp.MustCompile(`^0x[0-9a-fA-F]{1,8}$`)
)

const cydBuilderMaxAddresses = 64

// cydBuilderSettings are read from cyd_creator at startup.
type cydBuilderSettings struct {
	Image    string
	MemoryMB int
	CPUCores float64
	IdleStop time.Duration
}

// cydDockerBuilder implements cydcreator.Builder on the Docker Engine API
// through the internal/tools helpers (runtime Docker gates apply).
type cydDockerBuilder struct {
	docker   tools.DockerConfig
	settings cydBuilderSettings
	logger   *slog.Logger

	containerName, cacheVolume, workVolume string // fixed names; tests override them
	stopGrace                              time.Duration

	mu        sync.Mutex // container lifecycle and idle state
	busy      int
	idleTimer *time.Timer
	closed    bool
	claimed   bool // this process created, restarted or checked the container

	pullMu   sync.Mutex
	pulling  bool
	progress float64
	pullErr  string
}

func newCYDDockerBuilder(docker tools.DockerConfig, settings cydBuilderSettings, logger *slog.Logger) *cydDockerBuilder {
	if logger == nil {
		logger = slog.Default()
	}
	if settings.MemoryMB <= 0 {
		settings.MemoryMB = cydBuilderDefaultMemoryMB
	}
	if settings.CPUCores <= 0 || math.IsNaN(settings.CPUCores) || math.IsInf(settings.CPUCores, 0) {
		settings.CPUCores = cydBuilderDefaultCPUCores
	}
	return &cydDockerBuilder{docker: docker, settings: settings, logger: logger,
		containerName: cydBuilderContainerName, cacheVolume: cydBuilderCacheVolume, workVolume: cydBuilderWorkVolume,
		stopGrace: 2 * time.Second}
}

var _ cydcreator.Builder = (*cydDockerBuilder)(nil)

// cydEngineError is a non-2xx Engine answer. Error() keeps the sanitized
// Engine text for the server log; cydPublicDockerError shows only the
// operation and the HTTP status, because capabilities (ToolchainStatus.Error)
// are visible to read-only sessions.
type cydEngineError struct {
	op   string
	code int
	text string
}

func (e *cydEngineError) Error() string {
	return fmt.Sprintf("%s: HTTP %d: %s", e.op, e.code, e.text)
}

func newCYDEngineError(op string, code int, body []byte) error {
	return &cydEngineError{op: op, code: code, text: engineMessage(body)}
}

// cydPublicDockerError is the user-visible form of a failed Engine request:
// the HTTP status of an Engine answer, otherwise a generic "not reachable"
// (transport errors carry socket paths and addresses).
func cydPublicDockerError(err error) string {
	var engine *cydEngineError
	if errors.As(err, &engine) {
		return fmt.Sprintf("Docker refused the %s request (HTTP %d)", engine.op, engine.code)
	}
	return cydDockerUnreachable
}

// dockerUnavailable logs a failed Engine request and returns the generic
// CYD_DOCKER_UNAVAILABLE error users see.
func (b *cydDockerBuilder) dockerUnavailable(op string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		b.logger.Debug("CYD Creator builder: Docker request ended with its context", "op", op, "error", err)
	} else {
		b.logger.Warn("CYD Creator builder: Docker request failed", "op", op, "error", err)
	}
	return cydcreator.NewError(cydcreator.CodeDockerUnavailable, cydPublicDockerError(err), nil)
}

// cydContextError returns ctx.Err() once ctx ended (the Builder contract for
// cancel and timeout), otherwise err.
func cydContextError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// dockerGate reports why the runtime Docker permissions refuse builds.
func (b *cydDockerBuilder) dockerGate(ctx context.Context) string {
	perms, configured := tools.EffectiveRuntimePermissions(ctx)
	switch {
	case !configured || !perms.DockerEnabled:
		return "Docker is switched off (docker.enabled)"
	case perms.DockerReadOnly:
		return "Docker is read-only (docker.read_only)"
	}
	return ""
}

func (b *cydDockerBuilder) request(ctx context.Context, method, endpoint string, body any) ([]byte, int, error) {
	payload := ""
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		payload = string(encoded)
	}
	return tools.DockerRequestContext(ctx, b.docker, method, endpoint, payload)
}

func engineMessage(data []byte) string {
	return dockerutil.SanitizeOneLine(dockerutil.EngineErrorMessage(data), 300)
}

func (b *cydDockerBuilder) imagePresent(ctx context.Context) (bool, error) {
	data, code, err := b.request(ctx, http.MethodGet, "/images/"+url.PathEscape(b.settings.Image)+"/json", nil)
	if err != nil {
		return false, err
	}
	switch code {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	}
	return false, newCYDEngineError("image inspect", code, data)
}

// Status reports Docker reachability, image presence and pull progress. Its
// Error is generic (gate, HTTP status or "not reachable", the pull reason).
func (b *cydDockerBuilder) Status(ctx context.Context) cydcreator.ToolchainStatus {
	b.pullMu.Lock()
	st := cydcreator.ToolchainStatus{Image: b.settings.Image, Pulling: b.pulling, Error: b.pullErr}
	if b.pulling {
		st.Progress = b.progress
	}
	b.pullMu.Unlock()
	if gate := b.dockerGate(ctx); gate != "" {
		st.Error = gate
		return st
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	present, err := b.imagePresent(ctx)
	if err != nil {
		// Capabilities polls Status: keep the cause out of the warning log.
		b.logger.Debug("CYD Creator builder status check failed", "error", err)
		st.Error = cydPublicDockerError(err)
		return st
	}
	st.DockerAvailable, st.Present = true, present
	return st
}

// Pull pulls the image with progress; it never builds it (the default socket
// proxy keeps BUILD=0).
func (b *cydDockerBuilder) Pull(ctx context.Context) error {
	b.pullMu.Lock()
	if b.pulling {
		b.pullMu.Unlock()
		return nil
	}
	b.pulling, b.progress, b.pullErr = true, 0, ""
	b.pullMu.Unlock()
	err := b.pull(ctx)
	b.pullMu.Lock()
	b.pulling = false
	if err != nil {
		var typed *cydcreator.Error
		if errors.As(err, &typed) {
			b.pullErr = typed.Message
		} else {
			b.pullErr = "the image pull failed; the server log has the details"
		}
	}
	b.pullMu.Unlock()
	return err
}

func (b *cydDockerBuilder) pull(ctx context.Context) error {
	if gate := b.dockerGate(ctx); gate != "" {
		return cydcreator.NewError(cydcreator.CodeDockerUnavailable, gate, nil)
	}
	ctx, cancel := context.WithTimeout(ctx, cydBuilderPullTimeout)
	defer cancel()
	resp, err := tools.DockerStreamContext(ctx, b.docker, http.MethodPost, "/images/create?fromImage="+url.QueryEscape(b.settings.Image), nil, "")
	if err != nil {
		if ctx.Err() != nil {
			return b.pullFailed(0, "cancelled: "+ctx.Err().Error())
		}
		return b.dockerUnavailable("image pull", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return b.pullFailed(resp.StatusCode, engineMessage(dockerutil.ReadErrorBody(resp.Body)))
	}
	tracker := &cydPullProgress{report: func(p float64) {
		b.pullMu.Lock()
		b.progress = p
		b.pullMu.Unlock()
	}}
	if err := dockerutil.DrainJSONMessages(io.TeeReader(resp.Body, tracker)); err != nil {
		if ctx.Err() != nil {
			return b.pullFailed(0, "cancelled: "+ctx.Err().Error())
		}
		return b.pullFailed(0, dockerutil.SanitizeOneLine(err.Error(), 300))
	}
	b.pullMu.Lock()
	b.progress = 1
	b.pullMu.Unlock()
	return nil
}

// pullFailed logs the Engine's text and returns CYD_IMAGE_PULL_FAILED with
// a short generic reason.
func (b *cydDockerBuilder) pullFailed(status int, text string) error {
	b.logger.Warn("CYD Creator toolchain image pull failed", "image", b.settings.Image, "status", status, "error", text)
	return cydcreator.NewError(cydcreator.CodeImagePullFailed,
		fmt.Sprintf("pulling %s failed: %s", b.settings.Image, cydPullReason(status, text)), nil)
}

// cydPullReason maps a failed pull to a short user-visible reason. The
// Engine's own text can carry registry URLs, resolver addresses and host
// paths, so it is only logged.
func cydPullReason(status int, text string) string {
	lower := strings.ToLower(text)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(lower, w) {
				return true
			}
		}
		return false
	}
	switch {
	case strings.HasPrefix(lower, "cancelled: "):
		return "the pull was cancelled or timed out"
	case has("permission denied"):
		// A local file or socket permission of the Docker host, not the registry.
		return "the Docker host refused a local operation (permission denied); the server log has the details"
	case has("unauthorized", "authentication required", "access denied", "access to the resource is denied"):
		return "the registry refused access (unauthorized)"
	case has("manifest unknown", "not found", "does not exist"):
		return "the image or tag does not exist"
	case has("no space left"):
		return "the Docker host has no disk space left"
	case has("no such host", "dial tcp", "timeout", "connection refused", "network is unreachable", "tls:", "x509"):
		return "the registry is not reachable from the Docker host"
	case status == http.StatusForbidden:
		return "the Docker socket proxy refused the pull (HTTP 403)"
	case status != 0:
		return fmt.Sprintf("the Docker Engine refused the pull (HTTP %d)", status)
	}
	return "the Docker Engine reported an error; the server log has the details"
}

// cydPullProgress turns pull progress lines into a 0..0.99 fraction over all
// layers with a known size.
type cydPullProgress struct {
	partial []byte
	current map[string]int64
	total   map[string]int64
	report  func(float64)
}

func (p *cydPullProgress) Write(data []byte) (int, error) {
	n := len(data)
	for len(data) > 0 {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			if len(p.partial)+len(data) <= dockerutil.MaxJSONMessageLine {
				p.partial = append(p.partial, data...)
			}
			break
		}
		p.observe(append(p.partial, data[:i]...))
		p.partial = p.partial[:0]
		data = data[i+1:]
	}
	return n, nil
}

func (p *cydPullProgress) observe(line []byte) {
	var ev struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Detail struct {
			Current int64 `json:"current"`
			Total   int64 `json:"total"`
		} `json:"progressDetail"`
	}
	if json.Unmarshal(bytes.TrimSpace(line), &ev) != nil || ev.ID == "" {
		return
	}
	if p.current == nil {
		p.current, p.total = map[string]int64{}, map[string]int64{}
	}
	switch {
	case ev.Status == "Downloading" && ev.Detail.Total > 0:
		p.current[ev.ID], p.total[ev.ID] = ev.Detail.Current, ev.Detail.Total
	case ev.Status == "Download complete" || ev.Status == "Pull complete" || ev.Status == "Already exists":
		if t, ok := p.total[ev.ID]; ok {
			p.current[ev.ID] = t
		}
	default:
		return
	}
	// The server package shadows the min/max builtins, so clamp explicitly.
	var cur, tot int64
	for id, t := range p.total {
		c := p.current[id]
		if c > t {
			c = t
		}
		cur, tot = cur+c, tot+t
	}
	if tot > 0 && p.report != nil {
		fraction := float64(cur) / float64(tot)
		if fraction > 0.99 {
			fraction = 0.99
		}
		p.report(fraction)
	}
}

// fingerprint changes whenever a setting that is baked into the container
// changes; ensureContainer then recreates it. Bump "v1" when containerSpec
// changes.
func (b *cydDockerBuilder) fingerprint() string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("v1|%s|%d|%g|%s|%s|%s|%d", b.settings.Image, b.settings.MemoryMB, b.settings.CPUCores,
		cydBuilderUser, cydBuilderTmpfsOptions, b.workVolume+","+b.cacheVolume, cydBuilderPidsLimit)))
	return hex.EncodeToString(sum[:8])
}

type cydContainerInspect struct {
	ID    string `json:"Id"`
	State struct {
		Running bool `json:"Running"`
	} `json:"State"`
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

func (b *cydDockerBuilder) inspectContainer(ctx context.Context) (*cydContainerInspect, error) {
	data, code, err := b.request(ctx, http.MethodGet, "/containers/"+url.PathEscape(b.containerName)+"/json", nil)
	if err != nil {
		return nil, b.dockerUnavailable("container inspect", err)
	}
	if code == http.StatusNotFound {
		return nil, nil
	}
	if code != http.StatusOK {
		return nil, newCYDEngineError("container inspect", code, data)
	}
	var info cydContainerInspect
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("decode container inspect: %w", err)
	}
	return &info, nil
}

// containerSpec is the hardened builder container (overview contract:
// --init, --memory/--memory-swap equal, --pids-limit 512, read-only root,
// tmpfs /tmp rw,exec,nosuid,nodev,size=1g, --cap-drop ALL,
// no-new-privileges, --network none, uid 1000:1000, HOME=/tmp, managed labels).
func (b *cydDockerBuilder) containerSpec() map[string]any {
	memory := int64(b.settings.MemoryMB) << 20
	host := map[string]any{
		"NetworkMode":    "none",
		"CapDrop":        []string{"ALL"},
		"SecurityOpt":    []string{"no-new-privileges:true"},
		"ReadonlyRootfs": true,
		"Tmpfs":          map[string]string{"/tmp": cydBuilderTmpfsOptions},
		"Mounts": []map[string]any{
			{"Type": "volume", "Source": b.cacheVolume, "Target": "/cache"},
			{"Type": "volume", "Source": b.workVolume, "Target": cydBuilderWorkDir},
		},
		"PidsLimit":     cydBuilderPidsLimit,
		"RestartPolicy": map[string]string{"Name": "no"},
		"Init":          true,
		"Memory":        memory,
		"MemorySwap":    memory,
		"NanoCpus":      int64(b.settings.CPUCores * 1e9),
	}
	return map[string]any{
		"Image":      b.settings.Image,
		"User":       cydBuilderUser,
		"Cmd":        []string{"sleep", "infinity"},
		"WorkingDir": cydBuilderWorkDir,
		"Env":        []string{"HOME=/tmp"},
		"Labels":     dockerutil.ManagedLabels(cydBuilderOwner, "cyd-creator", "builder", b.fingerprint()),
		"HostConfig": host,
	}
}

func (b *cydDockerBuilder) createVolume(ctx context.Context, name, role string) error {
	data, code, err := b.request(ctx, http.MethodPost, "/volumes/create", map[string]any{
		"Name": name, "Labels": dockerutil.ManagedLabels(cydBuilderOwner, "cyd-creator", role, ""),
	})
	if err != nil {
		return b.dockerUnavailable("volume create", err)
	}
	if code != http.StatusCreated && code != http.StatusOK {
		return fmt.Errorf("%s: %w", name, newCYDEngineError("volume create", code, data))
	}
	return nil
}

func (b *cydDockerBuilder) startContainer(ctx context.Context, id string) error {
	data, code, err := b.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/start", nil)
	if err != nil {
		return b.dockerUnavailable("container start", err)
	}
	if code != http.StatusNoContent && code != http.StatusNotModified && code != http.StatusOK {
		return newCYDEngineError("container start", code, data)
	}
	return nil
}

func (b *cydDockerBuilder) stopContainer(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), b.stopGrace+15*time.Second)
	defer cancel()
	secs := strconv.Itoa(int(b.stopGrace / time.Second))
	if _, _, err := b.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/stop?t="+secs, nil); err != nil {
		b.logger.Warn("CYD Creator builder container could not be stopped", "error", err)
	}
}

// removeContainer force-removes the container; force also kills whatever
// still runs in it. A container that is already gone counts as removed.
func (b *cydDockerBuilder) removeContainer(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	data, code, err := b.request(ctx, http.MethodDelete, "/containers/"+url.PathEscape(id)+"?force=true", nil)
	if err != nil {
		return err
	}
	if code != http.StatusNoContent && code != http.StatusOK && code != http.StatusNotFound {
		return newCYDEngineError("container remove", code, data)
	}
	return nil
}

// discardContainer ends everything that runs in the container: the Engine
// cannot kill a single exec, so a cancelled, timed-out or broken build stops
// the container (SIGTERM, SIGKILL after stopGrace) and removes it. The next
// build creates a fresh container on the same volumes; a container that
// survives a failed removal carries the managed labels for a sweep.
func (b *cydDockerBuilder) discardContainer(id string) {
	b.stopContainer(id)
	if err := b.removeContainer(id); err != nil {
		b.logger.Warn("CYD Creator builder container could not be removed", "error", err)
	}
}

// ensureContainer returns the id of the running builder container, creating
// or recreating it when it is missing or its settings changed. Callers hold mu.
func (b *cydDockerBuilder) ensureContainer(ctx context.Context) (string, error) {
	if gate := b.dockerGate(ctx); gate != "" {
		return "", cydcreator.NewError(cydcreator.CodeDockerUnavailable, gate, nil)
	}
	info, err := b.inspectContainer(ctx)
	if err != nil {
		return "", err
	}
	if info != nil && !info.managed() {
		return "", b.foreignContainer(info)
	}
	// A running container found by the first use in this process belongs to
	// an earlier (maybe crashed) AuraGo: an orphan cyd-build could still hold
	// the container lock and its /work and /tmp state must not leak into new
	// builds, so it is discarded rather than reused.
	stale := info != nil && info.State.Running && !b.claimed
	if info != nil && (stale || info.Config.Labels["aurago.fingerprint"] != b.fingerprint()) {
		if stale {
			b.stopContainer(info.ID)
		}
		if err := b.removeContainer(info.ID); err != nil {
			return "", b.dockerUnavailable("container remove", err)
		}
		info = nil
	}
	if info != nil {
		b.claimed = true
		if !info.State.Running {
			if err := b.startOrRemove(ctx, info.ID); err != nil {
				return "", err
			}
			b.cleanupWork(ctx, info.ID)
		}
		return info.ID, nil
	}
	present, err := b.imagePresent(ctx)
	if err != nil {
		return "", b.dockerUnavailable("image inspect", err)
	}
	if !present {
		return "", cydcreator.NewError(cydcreator.CodeImageMissing, "the toolchain image "+b.settings.Image+" is not installed", map[string]any{"image": b.settings.Image})
	}
	for _, v := range [][2]string{{b.cacheVolume, "cache"}, {b.workVolume, "work"}} {
		if err := b.createVolume(ctx, v[0], v[1]); err != nil {
			return "", err
		}
	}
	data, code, err := b.request(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(b.containerName), b.containerSpec())
	if err != nil {
		return "", b.dockerUnavailable("container create", err)
	}
	if code != http.StatusCreated {
		return "", newCYDEngineError("container create", code, data)
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &created); err != nil || created.ID == "" {
		return "", fmt.Errorf("decode container create answer: %v", err)
	}
	if err := b.startOrRemove(ctx, created.ID); err != nil {
		return "", err
	}
	b.claimed = true
	b.cleanupWork(ctx, created.ID)
	return created.ID, nil
}

// managed reports whether the container carries the CYD Creator labels.
func (i *cydContainerInspect) managed() bool {
	return dockerutil.ManagedBy(i.Config.Labels, cydBuilderOwner)
}

// foreignContainer refuses to touch a container that uses the builder's
// name without AuraGo's labels (it is never stopped or removed).
func (b *cydDockerBuilder) foreignContainer(info *cydContainerInspect) error {
	b.logger.Warn("CYD Creator builder: a container with the builder's name is not managed by AuraGo; it is left alone",
		"container", b.containerName, "id", info.ID)
	return cydcreator.NewError(cydcreator.CodeDockerUnavailable,
		"another container already uses the builder container name; the server log has the details", nil)
}

// startOrRemove starts the container; one that cannot start is removed so
// it neither lingers nor blocks the next attempt, which recreates it.
func (b *cydDockerBuilder) startOrRemove(ctx context.Context, id string) error {
	err := b.startContainer(ctx, id)
	if err != nil {
		if rerr := b.removeContainer(id); rerr != nil {
			b.logger.Warn("CYD Creator builder container could not be removed", "error", rerr)
		}
	}
	return err
}

// cleanupWork removes work directories left by an earlier process or a
// stopped build.
func (b *cydDockerBuilder) cleanupWork(ctx context.Context, id string) {
	if _, _, err := b.execCapture(ctx, id, []string{"sh", "-c", "rm -rf /work/ccb_* /work/a2l_*"}, 0); err != nil {
		b.logger.Warn("CYD Creator work cleanup failed", "error", err)
	}
}

// acquire marks the builder busy (no idle stop) and returns the running
// container id.
func (b *cydDockerBuilder) acquire(ctx context.Context) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return "", cydcreator.NewError(cydcreator.CodeDockerUnavailable, "the builder is shutting down", nil)
	}
	if b.idleTimer != nil {
		b.idleTimer.Stop()
		b.idleTimer = nil
	}
	id, err := b.ensureContainer(ctx)
	if err != nil {
		// The idle timer was stopped above: a container that is still up
		// (started by this attempt or before) must not run forever.
		b.armIdleLocked()
		return "", err
	}
	b.busy++
	return id, nil
}

// release arms the idle stop once nothing runs in the container.
func (b *cydDockerBuilder) release() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.busy--
	b.armIdleLocked()
}

// armIdleLocked starts the idle stop timer when nothing uses the container.
// Callers hold mu.
func (b *cydDockerBuilder) armIdleLocked() {
	if b.busy > 0 || b.closed || b.settings.IdleStop <= 0 || b.idleTimer != nil {
		return
	}
	b.idleTimer = time.AfterFunc(b.settings.IdleStop, b.idleStop)
}

func (b *cydDockerBuilder) idleStop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.busy > 0 || b.closed {
		return
	}
	b.idleTimer = nil
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	info, err := b.inspectContainer(ctx)
	if err != nil || info == nil || !info.managed() || !info.State.Running {
		return
	}
	b.stopContainer(info.ID)
	b.logger.Info("CYD Creator builder container stopped after idle time")
}

// Close stops the idle timer and stops and removes the container (best
// effort; it carries the managed labels, so a sweep finds one that survives).
func (b *cydDockerBuilder) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	if b.idleTimer != nil {
		b.idleTimer.Stop()
		b.idleTimer = nil
	}
	if b.dockerGate(context.Background()) != "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if info, err := b.inspectContainer(ctx); err == nil && info != nil && info.managed() {
		b.discardContainer(info.ID)
	}
}

// buildTar packs files below prefix with explicit parent directories, owned
// by the container user, with a fixed modification time.
func buildTar(prefix string, files map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	mtime := time.Unix(0, 0)
	dirs := map[string]bool{}
	names := make([]string, 0, len(files))
	for name := range files {
		clean := path.Clean(name)
		if clean != name || name == "." || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || name == ".." {
			return nil, fmt.Errorf("invalid tree path %q", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	addDir := func(dir string) error {
		var chain []string
		for d := dir; d != "." && d != "/" && !dirs[d]; d = path.Dir(d) {
			chain = append(chain, d)
		}
		for i := len(chain) - 1; i >= 0; i-- {
			dirs[chain[i]] = true
			if err := tw.WriteHeader(&tar.Header{Name: chain[i] + "/", Typeflag: tar.TypeDir, Mode: 0o755, Uid: 1000, Gid: 1000, ModTime: mtime}); err != nil {
				return err
			}
		}
		return nil
	}
	if err := addDir(prefix); err != nil {
		return nil, err
	}
	for _, name := range names {
		full := path.Join(prefix, name)
		if err := addDir(path.Dir(full)); err != nil {
			return nil, err
		}
		data := files[name]
		if err := tw.WriteHeader(&tar.Header{Name: full, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(data)), Uid: 1000, Gid: 1000, ModTime: mtime}); err != nil {
			return nil, err
		}
		if _, err := tw.Write(data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (b *cydDockerBuilder) putArchive(ctx context.Context, id string, archive []byte) error {
	endpoint := "/containers/" + url.PathEscape(id) + "/archive?path=" + url.QueryEscape(cydBuilderWorkDir) + "&noOverwriteDirNonDir=true&copyUIDGID=true"
	data, code, err := tools.DockerRequestBytesContext(ctx, b.docker, http.MethodPut, endpoint, archive, "application/x-tar")
	if err != nil {
		return b.dockerUnavailable("archive upload", err)
	}
	if code != http.StatusOK {
		return newCYDEngineError("archive upload", code, data)
	}
	return nil
}

// cydLineWriter splits a byte stream into lines (CR stripped, overlong lines
// cut at cydBuilderMaxLogLine on a UTF-8 rune boundary, so the pending line
// never grows beyond it).
type cydLineWriter struct {
	partial []byte
	emit    func(string)
}

func (w *cydLineWriter) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		end := len(p)
		if i >= 0 {
			end = i
		}
		w.partial = append(w.partial, p[:end]...)
		if len(w.partial) > cydBuilderMaxLogLine {
			k := 0
			for len(w.partial)-k > cydBuilderMaxLogLine {
				cut := cydBuilderMaxLogLine
				// Back off to the start of a rune (at most 3 bytes); bytes
				// that are no UTF-8 at all are cut at the limit.
				for back := 0; back < utf8.UTFMax-1 && cut > 1 && !utf8.RuneStart(w.partial[k+cut]); back++ {
					cut--
				}
				if !utf8.RuneStart(w.partial[k+cut]) {
					cut = cydBuilderMaxLogLine
				}
				w.emit(string(w.partial[k : k+cut]))
				k += cut
			}
			w.partial = append(w.partial[:0], w.partial[k:]...)
		}
		if i < 0 {
			break
		}
		w.emit(strings.TrimRight(string(w.partial), "\r"))
		w.partial = w.partial[:0]
		p = p[i+1:]
	}
	return n, nil
}

func (w *cydLineWriter) flush() {
	if len(w.partial) > 0 {
		w.emit(strings.TrimRight(string(w.partial), "\r"))
		w.partial = w.partial[:0]
	}
}

// demuxDockerStream copies a multiplexed exec stream (8-byte frame headers:
// stream byte 1 stdout or 2 stderr, three zero bytes, big-endian length) to
// stdout and stderr. Every frame is checked: another stream byte (0 = stdin),
// non-zero padding or a frame above cydBuilderMaxFrame is an error. A stream
// whose first header has non-zero padding is raw (TTY) output and is copied
// to stdout up to cydBuilderMaxRawOutput bytes.
func demuxDockerStream(r io.Reader, stdout, stderr io.Writer) error {
	header := make([]byte, 8)
	for first := true; ; first = false {
		n, err := io.ReadFull(r, header)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if errors.Is(err, io.ErrUnexpectedEOF) && first {
				_, werr := stdout.Write(header[:n]) // shorter than one header: raw output
				return werr
			}
			return fmt.Errorf("exec output ended inside a frame header: %w", err)
		}
		padded := header[1] != 0 || header[2] != 0 || header[3] != 0
		if first && padded {
			return copyRawExecOutput(header, r, stdout)
		}
		if padded {
			return errors.New("exec output frame header is malformed")
		}
		var dst io.Writer
		switch header[0] {
		case 1:
			dst = stdout
		case 2:
			dst = stderr
		default:
			return fmt.Errorf("exec output frame on stream %d (want 1 stdout or 2 stderr)", header[0])
		}
		size := binary.BigEndian.Uint32(header[4:])
		if size > cydBuilderMaxFrame {
			return fmt.Errorf("exec output frame of %d bytes exceeds the %d byte limit", size, cydBuilderMaxFrame)
		}
		if _, err := io.CopyN(dst, r, int64(size)); err != nil {
			return fmt.Errorf("exec output ended inside a frame: %w", err)
		}
	}
}

// copyRawExecOutput copies raw (TTY) exec output, starting with the bytes
// already read as a header, and refuses more than cydBuilderMaxRawOutput.
func copyRawExecOutput(head []byte, r io.Reader, stdout io.Writer) error {
	if _, err := stdout.Write(head); err != nil {
		return err
	}
	_, err := io.CopyN(stdout, r, cydBuilderMaxRawOutput-int64(len(head)))
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	var probe [1]byte
	if n, _ := io.ReadFull(r, probe[:]); n > 0 {
		return fmt.Errorf("raw exec output exceeds the %d byte limit", cydBuilderMaxRawOutput)
	}
	return nil
}

// execStream runs cmd in the container, streams stdout/stderr and returns the
// exit code. A cancelled ctx returns ctx.Err(); the process keeps running
// until the caller stops the container.
func (b *cydDockerBuilder) execStream(ctx context.Context, id string, cmd []string, stdout, stderr io.Writer) (int, error) {
	data, code, err := b.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/exec", map[string]any{
		"AttachStdout": true, "AttachStderr": true, "Tty": false, "Cmd": cmd, "WorkingDir": cydBuilderWorkDir,
	})
	if err != nil {
		return -1, cydContextError(ctx, b.dockerUnavailable("exec create", err))
	}
	if code != http.StatusCreated {
		return -1, newCYDEngineError("exec create", code, data)
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &created); err != nil || created.ID == "" {
		return -1, fmt.Errorf("decode exec create answer: %v", err)
	}
	resp, err := tools.DockerStreamContext(ctx, b.docker, http.MethodPost, "/exec/"+url.PathEscape(created.ID)+"/start",
		[]byte(`{"Detach":false,"Tty":false}`), "application/json")
	if err != nil {
		if ctx.Err() != nil {
			return -1, ctx.Err()
		}
		return -1, b.dockerUnavailable("exec start", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusSwitchingProtocols {
		body := dockerutil.ReadErrorBody(resp.Body)
		_ = resp.Body.Close()
		return -1, newCYDEngineError("exec start", resp.StatusCode, body)
	}
	streamErr := demuxDockerStream(resp.Body, stdout, stderr)
	_ = resp.Body.Close()
	if ctx.Err() != nil {
		return -1, ctx.Err()
	}
	if streamErr != nil {
		return -1, fmt.Errorf("read exec output: %w", streamErr)
	}
	for attempt := 0; attempt < 100; attempt++ {
		data, code, err := b.request(ctx, http.MethodGet, "/exec/"+url.PathEscape(created.ID)+"/json", nil)
		if err != nil {
			return -1, cydContextError(ctx, b.dockerUnavailable("exec inspect", err))
		}
		if code != http.StatusOK {
			return -1, newCYDEngineError("exec inspect", code, data)
		}
		var inspect struct {
			Running  bool `json:"Running"`
			ExitCode int  `json:"ExitCode"`
		}
		if err := json.Unmarshal(data, &inspect); err != nil {
			return -1, fmt.Errorf("decode exec inspect: %w", err)
		}
		if !inspect.Running {
			return inspect.ExitCode, nil
		}
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return -1, errors.New("exec did not report an exit code")
}

// cappedBuffer keeps at most max bytes.
type cappedBuffer struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if room := c.max - c.buf.Len(); room > 0 {
		if len(p) > room {
			c.buf.Write(p[:room])
			c.truncated = true
		} else {
			c.buf.Write(p)
		}
	} else if len(p) > 0 {
		c.truncated = true
	}
	return len(p), nil
}

// cydBuildLogTruncated ends a build log that reached MaxBuildLogBytes (the
// service's marker text); its length is reserved, so the log including it
// never exceeds the cap.
const cydBuildLogTruncated = "[cyd-creator] log truncated\n"

// cydBuildLog keeps whole lines of the build log up to MaxBuildLogBytes and
// then the truncation marker. Lines come from cydLineWriter, so the cut is
// always on a line (and rune) boundary.
type cydBuildLog struct {
	buf       strings.Builder
	truncated bool
}

func (l *cydBuildLog) add(line string) {
	if l.truncated {
		return
	}
	if l.buf.Len()+len(line)+1 > cydcreator.MaxBuildLogBytes-len(cydBuildLogTruncated) {
		l.buf.WriteString(cydBuildLogTruncated)
		l.truncated = true
		return
	}
	l.buf.WriteString(line)
	l.buf.WriteByte('\n')
}

// execCapture runs a short command and returns its stdout and stderr.
func (b *cydDockerBuilder) execCapture(ctx context.Context, id string, cmd []string, timeout time.Duration) ([]byte, int, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	stdout := &cappedBuffer{max: cydBuilderMaxA2LOutput}
	stderr := &cappedBuffer{max: 64 << 10}
	code, err := b.execStream(ctx, id, cmd, stdout, stderr)
	if err != nil {
		return nil, code, err
	}
	if code != 0 {
		return stdout.buf.Bytes(), code, fmt.Errorf("%s exited with %d: %s", cmd[0], code, dockerutil.SanitizeOneLine(stderr.buf.String(), 300))
	}
	return stdout.buf.Bytes(), code, nil
}

func (b *cydDockerBuilder) removeWorkDir(id, dir string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, _, err := b.execCapture(ctx, id, []string{"rm", "-rf", "--", path.Join(cydBuilderWorkDir, dir)}, 0); err != nil {
		b.logger.Warn("CYD Creator work directory could not be removed", "dir", dir, "error", err)
	}
}

// cydArtifactLimits bound what extractArtifacts accepts from the untrusted
// GET /archive download of out/: per-file caps by artefact, the sum of all
// files and the number of tar entries.
type cydArtifactLimits struct {
	image, elf, font, result, log int64
	total                         int64
	entries                       int
}

var cydDefaultArtifactLimits = cydArtifactLimits{
	image:   cydBuilderMaxImageBytes,
	elf:     cydcreator.MaxELFArtifactBytes,
	font:    cydcreator.MaxFontArtifactBytes,
	result:  cydBuilderMaxResultJSON,
	log:     cydcreator.MaxBuildLogBytes,
	total:   cydBuilderMaxArtifacts,
	entries: cydBuilderMaxArtifactEntries,
}

// artifact maps an exact archive path to its BuildOutput key and size cap.
// cyd-build writes out/{merged.bin,firmware.bin,firmware.elf,result.json}
// and out/fonts/{ui_font_*.c,fonts.json}; out/build.log is accepted for a
// future builder but not kept (the streamed log is authoritative).
func (l cydArtifactLimits) artifact(name string) (string, int64, bool) {
	root := cydBuilderArtifactRoot + "/"
	switch name {
	case root + cydcreator.ArtifactMergedBin:
		return cydcreator.ArtifactMergedBin, l.image, true
	case root + cydcreator.ArtifactFirmwareBin:
		return cydcreator.ArtifactFirmwareBin, l.image, true
	case root + cydcreator.ArtifactFirmwareELF:
		return cydcreator.ArtifactFirmwareELF, l.elf, true
	case root + cydcreator.ArtifactResultJSON:
		return cydcreator.ArtifactResultJSON, l.result, true
	case root + cydcreator.ArtifactBuildLog:
		return cydcreator.ArtifactBuildLog, l.log, true
	}
	font, ok := strings.CutPrefix(name, root+cydcreator.ArtifactFontsDir+"/")
	if ok && cydcreator.FontArtifactName(font) {
		return cydcreator.ArtifactFontsDir + "/" + font, l.font, true
	}
	return "", 0, false
}

// artifactEntryName validates the shape of a tar entry name (relative,
// clean, below out/) and returns it without a directory's trailing slash.
func artifactEntryName(hdr *tar.Header) (string, error) {
	name := hdr.Name
	if hdr.Typeflag == tar.TypeDir {
		name = strings.TrimSuffix(name, "/")
	}
	quoted := strconv.Quote(dockerutil.SanitizeOneLine(hdr.Name, 128))
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, `\`) || path.Clean(name) != name ||
		name == ".." || strings.HasPrefix(name, "../") {
		return "", fmt.Errorf("artefact archive entry %s has an unsafe name", quoted)
	}
	if name != cydBuilderArtifactRoot && !strings.HasPrefix(name, cydBuilderArtifactRoot+"/") {
		return "", fmt.Errorf("artefact archive entry %s is outside %s/", quoted, cydBuilderArtifactRoot)
	}
	return name, nil
}

// extractArtifacts reads out/{merged.bin,firmware.bin,firmware.elf,result.json}
// and the converted fonts in out/fonts/ (keyed "fonts/<name>") from a
// GET /archive tar.
func extractArtifacts(archive []byte) (map[string][]byte, []byte, error) {
	return extractArtifactsWithin(archive, cydDefaultArtifactLimits)
}

// extractArtifactsWithin is extractArtifacts with explicit limits. The tar
// comes from a container that compiled user and agent code: any entry that
// is not an expected path is an error (no skipping), and so are links,
// devices, FIFOs, unsafe names, duplicates and sizes above the caps; a size
// is checked before the content is read.
func extractArtifactsWithin(archive []byte, limits cydArtifactLimits) (map[string][]byte, []byte, error) {
	files := map[string][]byte{}
	var result []byte
	seen := map[string]bool{}
	var total int64
	tr := tar.NewReader(bytes.NewReader(archive))
	for entries := 0; ; entries++ {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files, result, nil
		}
		if err != nil {
			return nil, nil, fmt.Errorf("read artefact archive: %w", err)
		}
		if entries >= limits.entries {
			return nil, nil, fmt.Errorf("artefact archive has more than %d entries (limit)", limits.entries)
		}
		name, err := artifactEntryName(hdr)
		if err != nil {
			return nil, nil, err
		}
		if seen[name] {
			return nil, nil, fmt.Errorf("artefact archive lists %s twice", name)
		}
		seen[name] = true
		switch hdr.Typeflag {
		case tar.TypeDir:
			if name != cydBuilderArtifactRoot && name != cydBuilderArtifactRoot+"/"+cydcreator.ArtifactFontsDir {
				return nil, nil, fmt.Errorf("artefact archive has an unexpected directory %s", name)
			}
			continue
		case tar.TypeReg:
		default:
			return nil, nil, fmt.Errorf("artefact %s is not a regular file (type %q)", name, hdr.Typeflag)
		}
		key, limit, ok := limits.artifact(name)
		if !ok {
			return nil, nil, fmt.Errorf("artefact archive has an unexpected file %s", name)
		}
		if hdr.Size < 0 || hdr.Size > limit {
			return nil, nil, fmt.Errorf("artefact %s has %d bytes (limit %d)", name, hdr.Size, limit)
		}
		if total += hdr.Size; total > limits.total {
			return nil, nil, fmt.Errorf("artefacts exceed %d bytes in total (limit)", limits.total)
		}
		data, err := io.ReadAll(io.LimitReader(tr, limit+1))
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", name, err)
		}
		if int64(len(data)) != hdr.Size {
			return nil, nil, fmt.Errorf("artefact %s has %d bytes, its header says %d", name, len(data), hdr.Size)
		}
		switch key {
		case cydcreator.ArtifactResultJSON:
			result = data
		case cydcreator.ArtifactBuildLog:
		default:
			files[key] = data
		}
	}
}

// Build uploads the tree to /work/<build_id>, runs cyd-build with streamed
// output, downloads the artefacts and removes the work directory. A
// cancelled or expired ctx (or req.Timeout) and a broken output stream stop
// and remove the container, which kills cyd-build.
func (b *cydDockerBuilder) Build(ctx context.Context, req cydcreator.BuildRequest, onLog func(line string)) (cydcreator.BuildOutput, error) {
	if !cydBuildIDPattern.MatchString(req.BuildID) || !cydPIOEnvPattern.MatchString(req.Env) || !cydCacheKeyPattern.MatchString(req.CacheKey) {
		return cydcreator.BuildOutput{}, fmt.Errorf("invalid build request")
	}
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}
	id, err := b.acquire(ctx)
	if err != nil {
		return cydcreator.BuildOutput{}, cydContextError(ctx, err)
	}
	defer b.release()
	archive, err := buildTar(req.BuildID, req.Tree)
	if err != nil {
		return cydcreator.BuildOutput{}, err
	}
	if err := b.putArchive(ctx, id, archive); err != nil {
		return cydcreator.BuildOutput{}, cydContextError(ctx, err)
	}
	discarded := false
	defer func() {
		if !discarded {
			b.removeWorkDir(id, req.BuildID)
		}
	}()

	logBuf := &cydBuildLog{}
	var logMu sync.Mutex
	emit := func(line string) {
		logMu.Lock()
		defer logMu.Unlock()
		logBuf.add(line)
		if onLog != nil {
			onLog(line)
		}
	}
	stdout, stderr := &cydLineWriter{emit: emit}, &cydLineWriter{emit: emit}
	work := path.Join(cydBuilderWorkDir, req.BuildID)
	exit, err := b.execStream(ctx, id, []string{"cyd-build", "--work", work, "--env", req.Env, "--build-id", req.BuildID,
		"--cache-key", req.CacheKey}, stdout, stderr)
	stdout.flush()
	stderr.flush()
	out := cydcreator.BuildOutput{ExitCode: exit, Log: logBuf.buf.String()}
	if err != nil || ctx.Err() != nil {
		// cyd-build may still run (cancel, timeout, a broken or hostile
		// output stream) and the Engine cannot kill a single exec.
		discarded = true
		b.discardContainer(id)
		return out, cydContextError(ctx, err)
	}
	archiveOut, err := tools.DockerGetArchiveContext(ctx, b.docker, id, path.Join(work, cydBuilderArtifactRoot), cydBuilderMaxArtifacts)
	switch {
	case errors.Is(err, tools.ErrDockerArchiveNotFound):
		// The container exists but out/ does not (cyd-build failed before it
		// wrote result.json); a vanished container is a plain error below.
		return out, nil
	case err != nil:
		return out, cydContextError(ctx, fmt.Errorf("download artefacts: %w", err))
	}
	files, result, err := extractArtifacts(archiveOut)
	if err != nil {
		return out, fmt.Errorf("artefacts of %s: %w", req.BuildID, err)
	}
	out.Files = files
	if len(result) > 0 {
		if err := json.Unmarshal(result, &out.Result); err != nil {
			b.logger.Warn("CYD Creator result.json is not valid JSON", "build_id", req.BuildID, "error", err)
		}
	}
	return out, nil
}

// Addr2Line uploads the ELF and runs cyd-addr2line.
func (b *cydDockerBuilder) Addr2Line(ctx context.Context, req cydcreator.Addr2LineRequest) ([]cydcreator.Frame, error) {
	if req.Chip != cydcreator.ChipESP32 && req.Chip != cydcreator.ChipESP32S3 {
		return nil, cydcreator.NewError(cydcreator.CodeBadRequest, "unknown chip "+req.Chip, nil)
	}
	if len(req.Addresses) == 0 || len(req.Addresses) > cydBuilderMaxAddresses {
		return nil, cydcreator.NewError(cydcreator.CodeBadRequest, fmt.Sprintf("decode 1 to %d addresses", cydBuilderMaxAddresses), nil)
	}
	for _, addr := range req.Addresses {
		if !cydAddressPattern.MatchString(addr) {
			return nil, cydcreator.NewError(cydcreator.CodeBadRequest, "backtrace addresses look like 0x400d1234", nil)
		}
	}
	id, err := b.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer b.release()
	var suffix [6]byte
	_, _ = rand.Read(suffix[:])
	dir := "a2l_" + hex.EncodeToString(suffix[:])
	archive, err := buildTar(dir, map[string][]byte{"firmware.elf": req.ELF})
	if err != nil {
		return nil, err
	}
	if err := b.putArchive(ctx, id, archive); err != nil {
		return nil, err
	}
	defer b.removeWorkDir(id, dir)
	cmd := append([]string{"cyd-addr2line", "--elf", path.Join(cydBuilderWorkDir, dir, "firmware.elf"), "--chip", req.Chip}, req.Addresses...)
	stdout, _, err := b.execCapture(ctx, id, cmd, 90*time.Second)
	if err != nil {
		return nil, err
	}
	var frames []cydcreator.Frame
	if err := json.Unmarshal(bytes.TrimSpace(stdout), &frames); err != nil {
		return nil, fmt.Errorf("decode cyd-addr2line output: %w", err)
	}
	return frames, nil
}

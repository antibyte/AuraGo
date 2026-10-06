package server

// Self proofs that survive network sharing. Behind a network sidecar
// (network_mode container:/service:) Docker gives AuraGo the provider's
// hostname and /etc files, so the signals of containerSelfSignals cannot tell
// AuraGo from its provider and containerSelfInList marks the whole group
// shared-network. The proofs here decide that group:
//
//   - the overlay upper directory of "/" (overlay2 graph driver only: inspect
//     reports it as GraphDriver.Data.UpperDir);
//   - a marker file with a random name that AuraGo writes into its own
//     writable layer at startup: exactly one group member holds it (HEAD
//     /containers/{id}/archive), which also works on the containerd image store.
//
// They only ever add a self proof; without one (no marker, a Docker answer
// that is neither 200 nor 404, several matches), today's shared-network
// confirmation stays. This file is kept apart from the hostname and /proc
// helpers in containers_protection.go on purpose.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/tools"
)

// containerSelfProofGroupLimit bounds the inspects one shared-network group
// may cost; a larger group proves nothing.
const containerSelfProofGroupLimit = 16

// ownOverlayUpperDir returns the upperdir option of the kernel overlay mount at
// "/" in mountinfo, or "" when "/" is no overlay mount, names no upper
// directory, or several "/" overlay mounts disagree. Unlike the hostname and
// the /etc files, the upper directory is the container's own even when it
// joins another container's network namespace.
func ownOverlayUpperDir(text string) string {
	found := ""
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[4] != "/" {
			continue
		}
		sep := slices.Index(fields, "-")
		if sep < 0 || len(fields) < sep+4 || fields[sep+1] != "overlay" {
			continue
		}
		upper := ""
		for _, option := range strings.Split(fields[sep+3], ",") {
			if value, ok := strings.CutPrefix(option, "upperdir="); ok {
				upper = unescapeMountinfo(value)
			}
		}
		if upper == "" {
			continue
		}
		if found != "" && found != upper {
			return ""
		}
		found = upper
	}
	if found == "" {
		return ""
	}
	return path.Clean(found)
}

// unescapeMountinfo decodes the octal escapes (\040 for a space) mountinfo
// uses in paths and options.
func unescapeMountinfo(value string) string {
	if !strings.Contains(value, `\`) {
		return value
	}
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' && i+4 <= len(value) {
			if n, err := strconv.ParseUint(value[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(value[i])
	}
	return b.String()
}

// sameUpperDir compares an inspect GraphDriver.Data.UpperDir with AuraGo's own.
func sameUpperDir(inspected, own string) bool {
	inspected, own = strings.TrimSpace(inspected), strings.TrimSpace(own)
	return inspected != "" && own != "" && path.Clean(inspected) == path.Clean(own)
}

// containerOwnUpperDir is the overlay upper directory of AuraGo's own "/", or
// "" outside the Docker runtime or when /proc names none.
func containerOwnUpperDir(isDocker bool) string {
	if !isDocker {
		return ""
	}
	data, err := containerSelfProcFile("/proc/self/mountinfo")
	if err != nil {
		return ""
	}
	return ownOverlayUpperDir(string(data))
}

// containerUpperDirProvesSelf reports whether an inspected
// GraphDriver.Data.UpperDir is AuraGo's own. It reads /proc only when Docker
// reported an upper directory (the overlay2 graph driver).
func containerUpperDirProvesSelf(isDocker bool, inspectedUpperDir string) bool {
	if strings.TrimSpace(inspectedUpperDir) == "" {
		return false
	}
	return sameUpperDir(inspectedUpperDir, containerOwnUpperDir(isDocker))
}

// containerUpperDirProof inspects fullID and compares its
// GraphDriver.Data.UpperDir with AuraGo's own. known is false when Docker gave
// no usable answer or reports no upper directory (the containerd image store,
// btrfs, zfs and vfs).
func containerUpperDirProof(ctx context.Context, cfg tools.DockerConfig, fullID, ownUpperDir string) (self, known bool) {
	data, code, err := tools.DockerRequestContext(ctx, cfg, http.MethodGet, "/containers/"+url.PathEscape(fullID)+"/json", "")
	if err != nil || code != http.StatusOK {
		return false, false
	}
	var info struct {
		GraphDriver struct {
			Data map[string]string `json:"Data"`
		} `json:"GraphDriver"`
	}
	if json.Unmarshal(data, &info) != nil {
		return false, false
	}
	upper := info.GraphDriver.Data["UpperDir"]
	if strings.TrimSpace(upper) == "" {
		return false, false
	}
	return sameUpperDir(upper, ownUpperDir), true
}

// containerSelfMarkerPrefix starts the name of the marker file.
const containerSelfMarkerPrefix = ".aurago-self-"

// containerSelfMarkerState holds this process's marker path, "" when none was
// written (native runtime, no directory on the writable layer, write failed).
var containerSelfMarkerState struct {
	mu   sync.Mutex
	path string
}

func currentContainerSelfMarker() string {
	containerSelfMarkerState.mu.Lock()
	defer containerSelfMarkerState.mu.Unlock()
	return containerSelfMarkerState.path
}

func setContainerSelfMarker(marker string) {
	containerSelfMarkerState.mu.Lock()
	containerSelfMarkerState.path = marker
	containerSelfMarkerState.mu.Unlock()
}

// containerSelfMarkerDirs lists the directories the marker may go to, in
// order; tests replace it.
var containerSelfMarkerDirs = func() []string {
	dirs := []string{os.TempDir(), "/tmp", "/var/tmp"}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, home)
	}
	var out []string
	for _, dir := range dirs {
		if dir != "" && !slices.Contains(out, dir) {
			out = append(out, dir)
		}
	}
	return out
}

// containerSelfMarkerDirCheck is containerSelfMarkerOnRootLayer; tests on a
// non-Linux host replace it.
var containerSelfMarkerDirCheck = containerSelfMarkerOnRootLayer

// containerSelfMarkerOnRootLayer reports whether the mount that covers dir in
// mountinfo is "/", the container's own writable layer. A tmpfs may be
// invisible to the archive API, and a volume or bind mount may be shared with
// another container, which could then hold the marker too.
func containerSelfMarkerOnRootLayer(mountinfo, dir string) bool {
	if !strings.HasPrefix(dir, "/") {
		return false
	}
	dir = path.Clean(dir)
	hasRoot, covering := false, ""
	for _, line := range strings.Split(mountinfo, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		mountPoint := path.Clean(unescapeMountinfo(fields[4]))
		if mountPoint == "/" {
			hasRoot = true
		}
		if mountPoint == "/" || dir == mountPoint || strings.HasPrefix(dir, mountPoint+"/") {
			if len(mountPoint) > len(covering) {
				covering = mountPoint
			}
		}
	}
	return hasRoot && covering == "/"
}

// writeContainerSelfMarker removes markers of earlier runs from dir and writes
// a new one with a random name (mode 0600). It returns the marker's path.
func writeContainerSelfMarker(dir string) (string, error) {
	if stale, err := filepath.Glob(filepath.Join(dir, containerSelfMarkerPrefix+"*")); err == nil {
		for _, old := range stale {
			if info, err := os.Lstat(old); err == nil && info.Mode().IsRegular() {
				_ = os.Remove(old)
			}
		}
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	marker := filepath.Join(dir, containerSelfMarkerPrefix+hex.EncodeToString(nonce[:]))
	f, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	_, writeErr := f.WriteString("AuraGo self marker: the container holding this file runs the AuraGo process that wrote it.\n")
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(marker)
		if writeErr != nil {
			return "", writeErr
		}
		return "", closeErr
	}
	return marker, nil
}

// initContainerSelfMarker writes this process's marker into the first
// directory on its own writable layer. It runs once at startup and only in the
// Docker runtime; without a marker, nothing is proven and the confirmation
// stays.
func initContainerSelfMarker(isDocker bool, logger *slog.Logger) {
	if !isDocker {
		return
	}
	mountinfo, err := containerSelfProcFile("/proc/self/mountinfo")
	if err != nil {
		if logger != nil {
			logger.Debug("[Containers] No self marker: mountinfo unreadable", "error", err)
		}
		return
	}
	for _, dir := range containerSelfMarkerDirs() {
		resolved := dir
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			resolved = real
		}
		if !containerSelfMarkerDirCheck(string(mountinfo), filepath.ToSlash(resolved)) {
			continue
		}
		marker, err := writeContainerSelfMarker(resolved)
		if err != nil {
			continue
		}
		setContainerSelfMarker(filepath.ToSlash(marker))
		resetContainerSelfProofCache()
		if logger != nil {
			logger.Debug("[Containers] Self marker written", "path", filepath.ToSlash(marker))
		}
		return
	}
	if logger != nil {
		logger.Debug("[Containers] No self marker: no writable directory on the container's own layer")
	}
}

// containerHoldsSelfMarker asks Docker whether fullID's filesystem holds the
// marker. known is false unless Docker answers 200 (present) or 404 (absent):
// a refusing proxy or an engine error proves nothing.
func containerHoldsSelfMarker(ctx context.Context, cfg tools.DockerConfig, fullID, marker string) (holds, known bool) {
	endpoint := "/containers/" + url.PathEscape(fullID) + "/archive?path=" + url.QueryEscape(marker)
	_, code, err := tools.DockerRequestContext(ctx, cfg, http.MethodHead, endpoint, "")
	if err != nil {
		return false, false
	}
	switch code {
	case http.StatusOK:
		return true, true
	case http.StatusNotFound:
		return false, true
	default:
		return false, false
	}
}

// containerSelfProofCacheLimit bounds the cached answers; the cache starts
// over when it is full.
const containerSelfProofCacheLimit = 256

// containerSelfProofCache keeps known answers per engine, proof inputs and
// container; unknown answers are never cached.
var containerSelfProofCache struct {
	mu      sync.Mutex
	results map[string]bool
}

func resetContainerSelfProofCache() {
	containerSelfProofCache.mu.Lock()
	containerSelfProofCache.results = nil
	containerSelfProofCache.mu.Unlock()
}

func cachedContainerSelfProof(key string) (self, ok bool) {
	containerSelfProofCache.mu.Lock()
	defer containerSelfProofCache.mu.Unlock()
	self, ok = containerSelfProofCache.results[key]
	return self, ok
}

func storeContainerSelfProof(key string, self bool) {
	containerSelfProofCache.mu.Lock()
	defer containerSelfProofCache.mu.Unlock()
	if containerSelfProofCache.results == nil || len(containerSelfProofCache.results) >= containerSelfProofCacheLimit {
		containerSelfProofCache.results = map[string]bool{}
	}
	containerSelfProofCache.results[key] = self
}

// containerProvenSelf reports whether fullID is the container AuraGo runs in by
// a proof that survives network sharing: the overlay2 upper directory first,
// then the marker file. known is false when neither gave a usable answer,
// which proves nothing.
func containerProvenSelf(ctx context.Context, cfg tools.DockerConfig, fullID, ownUpperDir, marker string) (self, known bool) {
	if fullID == "" || (ownUpperDir == "" && marker == "") {
		return false, false
	}
	key := strings.Join([]string{cfg.Host, ownUpperDir, marker, strings.ToLower(fullID)}, "\x00")
	if self, ok := cachedContainerSelfProof(key); ok {
		return self, true
	}
	if ownUpperDir != "" {
		if self, known := containerUpperDirProof(ctx, cfg, fullID, ownUpperDir); known {
			storeContainerSelfProof(key, self)
			return self, true
		}
	}
	if marker != "" {
		if holds, known := containerHoldsSelfMarker(ctx, cfg, fullID, marker); known {
			storeContainerSelfProof(key, holds)
			return holds, true
		}
	}
	return false, false
}

// proveSelfInSharedGroup returns the lower-case full ID of the one container of
// a shared-network group (keys as containerSelfInList returns them) that is
// proven to be the container AuraGo runs in, or "" when none is. Every member
// must give a usable answer and exactly one must match: a committed copy of
// AuraGo's container would hold the marker too.
func proveSelfInSharedGroup(ctx context.Context, cfg tools.DockerConfig, isDocker bool, group map[string]bool) string {
	if !isDocker || len(group) == 0 || len(group) > containerSelfProofGroupLimit {
		return ""
	}
	ownUpper := containerOwnUpperDir(isDocker)
	marker := currentContainerSelfMarker()
	if ownUpper == "" && marker == "" {
		return ""
	}
	ids := make([]string, 0, len(group))
	for id := range group {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	proven := ""
	for _, id := range ids {
		self, known := containerProvenSelf(ctx, cfg, id, ownUpper, marker)
		if !known {
			return ""
		}
		if self {
			if proven != "" {
				return ""
			}
			proven = id
		}
	}
	return proven
}

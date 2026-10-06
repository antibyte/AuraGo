package server

// Self proofs that survive network sharing. Behind a network sidecar
// (network_mode container:/service:) Docker gives AuraGo the provider's
// hostname and /etc files, so the signals of containerSelfSignals cannot tell
// AuraGo from its provider and containerSelfInList marks the whole group
// shared-network. The proofs here decide that group:
//
//   - the overlay upper directory of "/" (overlay2 graph driver only: inspect
//     reports it as GraphDriver.Data.UpperDir).
//
// They only ever add a self proof; without one, today's shared-network
// confirmation stays. This file is kept apart from the hostname and /proc
// helpers in containers_protection.go on purpose.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
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

// containerProvenSelf reports whether fullID is the container AuraGo runs in by
// a proof that survives network sharing. known is false when Docker gave no
// usable answer, which proves nothing.
func containerProvenSelf(ctx context.Context, cfg tools.DockerConfig, fullID, ownUpperDir string) (self, known bool) {
	if fullID == "" || ownUpperDir == "" {
		return false, false
	}
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
		// The containerd image store, btrfs, zfs and vfs report none.
		return false, false
	}
	return sameUpperDir(upper, ownUpperDir), true
}

// proveSelfInSharedGroup returns the lower-case full ID of the one container of
// a shared-network group (keys as containerSelfInList returns them) that is
// proven to be the container AuraGo runs in, or "" when none is. Every member
// must give a usable answer and exactly one must match.
func proveSelfInSharedGroup(ctx context.Context, cfg tools.DockerConfig, isDocker bool, group map[string]bool) string {
	if len(group) == 0 || len(group) > containerSelfProofGroupLimit {
		return ""
	}
	ownUpper := containerOwnUpperDir(isDocker)
	if ownUpper == "" {
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
		self, known := containerProvenSelf(ctx, cfg, id, ownUpper)
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

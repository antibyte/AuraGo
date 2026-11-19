package dockerhost

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"aurago/internal/dockerutil"
)

var cleanupObjectID = regexp.MustCompile(`^[0-9a-f]{64}$`)

// CleanupManagedLabels recognizes both generations of AuraGo ownership labels.
func CleanupManagedLabels(labels map[string]string) bool {
	return labels["aurago.managed"] != "" || strings.EqualFold(labels["com.aurago.managed"], "true") ||
		labels["aurago.docker-manager.role"] != "" || strings.EqualFold(labels["com.docker.compose.project"], "aurago") ||
		CleanupReservedName(labels["com.docker.compose.volume"]) || CleanupReservedName(labels["com.docker.compose.network"])
}

// CleanupReservedName protects AuraGo's resources even before their owning container exists.
func CleanupReservedName(name string) bool {
	name = strings.ToLower(name)
	return name == "aurago" || strings.HasPrefix(name, "aurago-") || strings.HasPrefix(name, "aurago_")
}

// CleanupSizeHints reads optional writable-layer or volume sizes. Socket proxies
// may allow resource management while denying /system/df.
func CleanupSizeHints(ctx context.Context, c *dockerutil.Client, kind string) map[string]int64 {
	var raw struct {
		Containers []struct {
			ID     string
			SizeRw *int64
		}
		Volumes []struct {
			Name      string
			UsageData *struct{ Size *int64 }
		}
	}
	var resource string
	switch kind {
	case "containers":
		resource = "container"
	case "volumes":
		resource = "volume"
	default:
		return nil
	}
	if engineJSON(ctx, c, http.MethodGet, "system/df?type="+resource, &raw) != nil {
		return nil
	}
	sizes := make(map[string]int64)
	for _, ctr := range raw.Containers {
		if ctr.SizeRw != nil {
			sizes[ctr.ID] = *ctr.SizeRw
		}
	}
	for _, vol := range raw.Volumes {
		if vol.UsageData != nil && vol.UsageData.Size != nil {
			sizes[vol.Name] = *vol.UsageData.Size
		}
	}
	return sizes
}

// ValidCleanupID accepts only the immutable full ID, or the exact name for volumes.
func ValidCleanupID(kind, id string) bool {
	switch kind {
	case "images":
		digest, ok := strings.CutPrefix(id, "sha256:")
		return ok && cleanupObjectID.MatchString(digest)
	case "containers", "networks":
		return cleanupObjectID.MatchString(id)
	case "volumes":
		return len(id) <= 255 && reContainerRef.MatchString(id)
	}
	return false
}

// RemoveCleanupItem never forces removal, deletes attached volumes or prunes parent images.
// Volumes have no immutable ID, so check their creation stamp again immediately before deletion.
func RemoveCleanupItem(ctx context.Context, c *dockerutil.Client, kind, id, volumeCreatedAt string) error {
	if !ValidCleanupID(kind, id) {
		return errors.New("invalid cleanup resource")
	}
	path := kind + "/" + url.PathEscape(id)
	switch kind {
	case "images":
		path += "?noprune=true"
	case "volumes":
		var current struct {
			Name, CreatedAt string
			Labels          map[string]string
		}
		if err := engineJSON(ctx, c, http.MethodGet, path, &current); err != nil {
			return err
		}
		if current.Name != id || volumeCreatedAt == "" || current.CreatedAt != volumeCreatedAt || CleanupManagedLabels(current.Labels) {
			return &StatusError{Status: http.StatusConflict, Err: errors.New("volume changed after preview")}
		}
	}
	return engineJSON(ctx, c, http.MethodDelete, path, nil)
}

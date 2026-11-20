package virtualcomputers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// mergeUpstreamVolume keeps the locally owned fields (name, format,
// availability, storage epoch) and takes everything boringd reports.
func mergeUpstreamVolume(local, fresh Volume) Volume {
	merged := fresh
	merged.Name = local.Name
	merged.Format = local.Format
	merged.Availability = local.Availability
	merged.StorageEpochID = local.StorageEpochID
	if merged.Name == "" {
		merged.Name = fresh.Name
	}
	return merged
}

// TrackUpstreamVolume stores a volume freshly read from boringd while keeping
// the local name, format and availability. A volume that was never tracked
// becomes a legacy_root / available row.
func TrackUpstreamVolume(ctx context.Context, ledger *Ledger, fresh Volume) (Volume, error) {
	if ledger == nil {
		return fresh, fmt.Errorf("volume ledger is required")
	}
	local, found, err := ledger.GetVolume(ctx, fresh.ID)
	if err != nil {
		return fresh, err
	}
	if !found {
		local = Volume{ID: fresh.ID, Format: "legacy_root", Availability: "available"}
	}
	merged := mergeUpstreamVolume(local, fresh)
	now := time.Now().UTC()
	merged.LastVerifiedAt = &now
	merged.VerificationStatus = "verified"
	if err := ledger.UpsertVolume(ctx, merged); err != nil {
		return merged, err
	}
	return merged, nil
}

const volumeRetentionCacheTTL = 5 * time.Minute

type volumeRetentionEntry struct {
	value   bool
	expires time.Time
}

var (
	volumeRetentionMu    sync.Mutex
	volumeRetentionCache = map[string]volumeRetentionEntry{}
)

// CachedVolumeRetention reports whether boringd supports volume retention.
// Successful probes are cached per control plane for five minutes; errors are
// never cached.
func CachedVolumeRetention(ctx context.Context, client *Client) bool {
	if client == nil || client.baseURL == nil {
		return false
	}
	key := client.baseURL.String()
	volumeRetentionMu.Lock()
	entry, ok := volumeRetentionCache[key]
	volumeRetentionMu.Unlock()
	if ok && time.Now().Before(entry.expires) {
		return entry.value
	}
	supported, err := client.ProbeVolumeRetention(ctx)
	if err != nil {
		return false
	}
	volumeRetentionMu.Lock()
	volumeRetentionCache[key] = volumeRetentionEntry{value: supported, expires: time.Now().Add(volumeRetentionCacheTTL)}
	volumeRetentionMu.Unlock()
	return supported
}

// ResetVolumeRetentionCache forgets probe results, e.g. after Install / Repair.
func ResetVolumeRetentionCache() {
	volumeRetentionMu.Lock()
	volumeRetentionCache = map[string]volumeRetentionEntry{}
	volumeRetentionMu.Unlock()
}

func ListTrackedVolumes(ctx context.Context, ledger *Ledger, client *Client) ([]Volume, error) {
	if ledger == nil || client == nil {
		return nil, fmt.Errorf("volume ledger and boringd client are required")
	}
	tracked, err := ledger.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex
	for _, volume := range tracked {
		volume := volume
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			fresh, getErr := client.GetVolume(ctx, volume.ID)
			if getErr == nil {
				// boringd's volume API predates AuraGo's workspace format marker and
				// knows nothing about local names. Keep the local view of those and
				// take the retention state from upstream.
				fresh = mergeUpstreamVolume(volume, fresh)
				now := time.Now().UTC()
				fresh.LastVerifiedAt = &now
				fresh.VerificationStatus = "verified"
				if err := ledger.UpsertVolume(ctx, fresh); err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
				}
				return
			}
			var restErr RESTError
			if errors.As(getErr, &restErr) && restErr.StatusCode == http.StatusNotFound && looksLikeJSON(restErr.Body) {
				// Volumes bound to a previous storage epoch must remain visible after a
				// confirmed switch-without-migration; never auto-delete them on 404.
				if volume.Availability == "previous_store" || volume.Availability == "unavailable" {
					if err := ledger.MarkVolumeStale(ctx, volume.ID); err != nil {
						errMu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						errMu.Unlock()
					}
					return
				}
				if err := ledger.DeleteVolume(ctx, volume.ID); err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
				}
				return
			}
			if err := ledger.MarkVolumeStale(ctx, volume.ID); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return ledger.ListVolumes(ctx)
}

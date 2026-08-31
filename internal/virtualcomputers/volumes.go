package virtualcomputers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

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
				// boringd's volume API predates AuraGo's workspace format marker.
				// Preserve the locally tracked format instead of silently turning a
				// workspace_v2 checkpoint back into a legacy /root volume on refresh.
				if fresh.Format == "" {
					fresh.Format = volume.Format
				}
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

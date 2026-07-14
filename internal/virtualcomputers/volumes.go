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

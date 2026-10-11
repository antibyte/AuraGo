package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"aurago/internal/tools"
)

type storageTransferItem struct {
	from, to      tools.StorageLocation
	entry         tools.StorageEntry
	targetVersion string
	verifiedHash  []byte
}

func sameStorage(a, b tools.StorageLocation) bool {
	return a.Provider == b.Provider && a.MountID == b.MountID
}

func (d *desktopStorageSession) transferPlan(req desktopStorageRequest) ([]storageTransferItem, error) {
	if req.Mode != "copy" && req.Mode != "move" {
		return nil, fmt.Errorf("invalid transfer mode")
	}
	if err := d.authorize(req.Destination, true); err != nil {
		return nil, err
	}
	if req.Location.Path == "" || req.Destination.Path == "" {
		return nil, fmt.Errorf("cannot transfer storage root")
	}
	if sameStorage(req.Location, req.Destination) && (req.Location.Path == req.Destination.Path || strings.HasPrefix(req.Destination.Path, req.Location.Path+"/")) {
		return nil, fmt.Errorf("destination is inside source")
	}
	var items []storageTransferItem
	var total int64
	var walk func(tools.StorageLocation, tools.StorageLocation, int) error
	walk = func(from, to tools.StorageLocation, depth int) error {
		if depth > 64 || len(items) >= 10000 {
			return fmt.Errorf("storage transfer exceeds entry or depth limit")
		}
		e, err := d.stat(from)
		if err != nil {
			return err
		}
		if e.Type != "file" && e.Type != "directory" {
			return fmt.Errorf("storage transfers require regular files or directories")
		}
		if e.Size < 0 || e.Size > d.maxBytes {
			return fmt.Errorf("storage file exceeds size limit")
		}
		if e.Type == "file" {
			total += e.Size
			if total > 512<<20 {
				return fmt.Errorf("storage transfer exceeds 512 MiB")
			}
		}
		item := storageTransferItem{from: from, to: to, entry: e}
		if depth == 0 {
			item.targetVersion = req.Version
		}
		if target, err := d.stat(to); err == nil {
			if e.Type == "directory" || target.Type == "directory" || item.targetVersion == "" || item.targetVersion != target.Version {
				return tools.ErrStorageConflict
			}
		} else if !errors.Is(err, tools.ErrStorageNotFound) && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		items = append(items, item)
		if e.Type == "directory" {
			children, err := d.list(from)
			if err != nil {
				return err
			}
			for _, child := range children {
				f, t := from, to
				f.Path = child.Path
				t.Path = path.Join(to.Path, path.Base(child.Path))
				if err = walk(f, t, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	err := walk(req.Location, req.Destination, 0)
	return items, err
}

func (d *desktopStorageSession) serveTransfer(w http.ResponseWriter, r *http.Request, req desktopStorageRequest) {
	ctx, cancel := context.WithCancel(d.ctx)
	defer cancel()
	worker := *d
	worker.ctx = ctx
	d = &worker
	items, err := d.transferPlan(req)
	if err != nil {
		d.writeConflict(w, err, req.Destination)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	controller := http.NewResponseController(w)
	send := func(value interface{}) error {
		body, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(w, "data: %s\n\n", body); err != nil {
			return err
		}
		return controller.Flush()
	}
	if err = send(map[string]interface{}{"status": "started", "total": len(items)}); err != nil {
		return
	}
	// Flush heartbeats while a provider is working without racing the response writer.
	type update struct {
		value interface{}
		ack   chan error
	}
	updates := make(chan update)
	finished := make(chan error, 1)
	joined := make(chan struct{})
	defer func() { cancel(); <-joined }()
	go func() {
		defer close(joined)
		finished <- d.transfer(req, items, func(done int) error {
			u := update{map[string]interface{}{"status": "progress", "done": done, "total": len(items)}, make(chan error, 1)}
			select {
			case updates <- u:
			case <-d.ctx.Done():
				return d.ctx.Err()
			}
			select {
			case err := <-u.ack:
				return err
			case <-d.ctx.Done():
				return d.ctx.Err()
			}
		})
	}()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case u := <-updates:
			err = send(u.value)
			u.ack <- err
			if err != nil {
				return
			}
		case err = <-finished:
			if err != nil {
				_ = send(map[string]interface{}{"status": "error", "error": err.Error(), "partial": true})
			} else {
				_ = send(map[string]interface{}{"status": "done", "location": req.Destination})
			}
			return
		case <-ticker.C:
			if err = send(map[string]string{"status": "pending"}); err != nil {
				return
			}
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *desktopStorageSession) transfer(req desktopStorageRequest, items []storageTransferItem, progress func(int) error) error {
	if req.Mode == "move" && sameStorage(req.Location, req.Destination) && req.Version == "" {
		if err := d.authorize(req.Location, true); err != nil {
			return err
		}
		if req.Location.Provider != "local" {
			return d.cloud.Move(d.ctx, req.Location, req.Destination, items[0].entry.Version)
		}
		return d.svc.MovePathTo(d.ctx, req.Location.Path, req.Destination.Path, "desktop", nil)
	}
	for i, item := range items {
		if item.entry.Type == "directory" {
			if err := d.mkdir(item.to); err != nil {
				return err
			}
		} else if err := d.copyStorageFile(&items[i]); err != nil {
			return err
		}
		if err := progress(i + 1); err != nil {
			return err
		}
	}
	if req.Mode != "move" {
		return nil
	}
	// A previously verified destination may have changed while later files were
	// copying. Recheck the complete result before deleting any source.
	for _, item := range items {
		if item.entry.Type != "file" {
			continue
		}
		if err := d.verifyStorageCopy(item.to, item.entry.Size, item.verifiedHash); err != nil {
			return err
		}
	}
	// Delete only after the entire selection was copied and verified. Every file
	// deletion remains conditional; directory cleanup must be atomically empty-only.
	var retained bool
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if err := d.authorize(item.from, true); err != nil {
			return err
		}
		if item.entry.Type == "file" && item.entry.Version == "" {
			retained = true
			continue
		}
		var err error
		if item.from.Provider == "local" {
			err = d.svc.RemoveStorageSource(d.ctx, item.from.Path, item.entry.Version, item.entry.Type == "directory")
		} else {
			err = d.cloud.Remove(d.ctx, item.from, item.entry.Version, item.entry.Type == "directory")
		}
		if err != nil {
			retained = true
		}
	}
	if retained {
		return tools.ErrStorageSourceRetained
	}
	return nil
}

func (d *desktopStorageSession) copyStorageFile(item *storageTransferItem) error {
	in, err := d.open(item.from, item.entry.Version)
	if err != nil {
		return err
	}
	defer in.Close()
	sourceHash := sha256.New()
	counted := &storageCountingReader{Reader: io.TeeReader(in, sourceHash)}
	written, err := d.put(item.to, counted, item.targetVersion)
	if err != nil {
		return err
	}
	if counted.n != item.entry.Size || written.Size != item.entry.Size {
		return fmt.Errorf("storage copy size verification failed; source retained")
	}
	if latest, err := d.stat(item.from); err != nil || latest.Version != item.entry.Version || latest.Size != item.entry.Size || latest.Modified != item.entry.Modified {
		return fmt.Errorf("storage source changed during copy; source retained")
	}
	item.verifiedHash = sourceHash.Sum(nil)
	return d.verifyStorageCopy(item.to, counted.n, item.verifiedHash)
}

func (d *desktopStorageSession) verifyStorageCopy(loc tools.StorageLocation, size int64, hash []byte) error {
	verify, err := d.open(loc, "")
	if err != nil {
		return err
	}
	defer verify.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(storageContextReader{d.ctx, verify}, d.maxBytes+1))
	if err != nil {
		return err
	}
	if n != size || string(h.Sum(nil)) != string(hash) {
		return fmt.Errorf("storage copy content verification failed; source retained")
	}
	return nil
}

type storageCountingReader struct {
	io.Reader
	n int64
}

func (r *storageCountingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.n += int64(n)
	return n, err
}

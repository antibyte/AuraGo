package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/videostudio"
)

// Media upkeep: removals that stopped part-way, leftovers of interrupted work and missing posters. The chores run in
// one background goroutine fed by a small bounded queue; a request only queues them and never waits.

// videoStudioMaintenanceQueueSize bounds the queued chores. A full queue drops new ones; the next listing asks again.
const videoStudioMaintenanceQueueSize = 64

// videoStudioLeftoverMinAge keeps the sweep away from files and records a writer may still be working on (a variable
// for tests).
var videoStudioLeftoverMinAge = 10 * time.Minute

// videoStudioSweepInterval bounds how often a listing sweeps one project (a variable for tests).
var videoStudioSweepInterval = time.Minute

type videoStudioMaintenanceTask struct {
	kind      string // "sweep" or "poster"
	projectID string
	assetID   string
}

func (t videoStudioMaintenanceTask) key() string { return t.kind + ":" + t.projectID + ":" + t.assetID }

// queueMaintenanceLocked queues a chore without waiting. It reports false when the manager is closed, the chore is
// already queued or the queue is full. The caller holds m.mu.
func (m *videoStudioManager) queueMaintenanceLocked(task videoStudioMaintenanceTask) bool {
	if m.closed || m.maintenanceQueued[task.key()] {
		return false
	}
	select {
	case m.maintenance <- task:
		m.maintenanceQueued[task.key()] = true
		return true
	default:
		return false
	}
}

// requestMediaSweep queues a sweep of the project's leftovers, at most once per videoStudioSweepInterval.
func (m *videoStudioManager) requestMediaSweep(projectID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if last, ok := m.sweptAt[projectID]; ok && time.Since(last) < videoStudioSweepInterval {
		return
	}
	if m.queueMaintenanceLocked(videoStudioMaintenanceTask{kind: "sweep", projectID: projectID}) {
		m.sweptAt[projectID] = time.Now()
	}
}

func (m *videoStudioManager) maintenanceWorker() {
	defer m.wg.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case task := <-m.maintenance:
			m.runMaintenance(task)
			m.mu.Lock()
			delete(m.maintenanceQueued, task.key())
			m.mu.Unlock()
		}
	}
}

// runMaintenance runs one chore while Video Studio and the Desktop accept writes; otherwise the chore is dropped.
func (m *videoStudioManager) runMaintenance(task videoStudioMaintenanceTask) {
	cfg, err := normalizedVideoStudioConfig(m.server)
	if err != nil || !videoStudioUpkeepAllowed(cfg) {
		return
	}
	svc, _, err := m.server.getDesktopService(m.ctx)
	if err != nil {
		return
	}
	switch task.kind {
	case "sweep":
		m.sweepProjectMedia(m.ctx, svc, task.projectID)
	}
}

// videoStudioUpkeepAllowed reports whether upkeep may write: Video Studio and the Desktop are enabled and writable.
func videoStudioUpkeepAllowed(cfg *config.Config) bool {
	return cfg != nil && cfg.VideoStudio.Enabled && cfg.VirtualDesktop.Enabled && !cfg.VideoStudio.ReadOnly && !cfg.VirtualDesktop.ReadOnly
}

// mediaRecordForRemoval returns an asset's media record, also when its removal has started.
func (m *videoStudioManager) mediaRecordForRemoval(projectID, assetID string) (videoStudioMediaRecord, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.media[projectID][assetID]
	return record, ok
}

// markMediaRemoving persists that the asset's removal started, before any of its files is deleted. From then on
// readers treat the asset as gone (mediaRecord, mediaForProject).
func (m *videoStudioManager) markMediaRemoving(projectID, assetID string) (videoStudioMediaRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.media[projectID][assetID]
	if !ok {
		return videoStudioMediaRecord{}, os.ErrNotExist
	}
	if record.Removing {
		return record, nil
	}
	marked := record
	marked.Removing = true
	m.media[projectID][assetID] = marked
	if err := m.persistLocked(); err != nil {
		m.media[projectID][assetID] = record
		return videoStudioMediaRecord{}, err
	}
	return marked, nil
}

// finishMediaRemoval deletes a marked asset's media file, poster and cached previews, then its record, in that order.
// A file that is already gone counts as removed, so a removal that stopped part-way finishes on the next try. The
// caller holds the project lock.
func (m *videoStudioManager) finishMediaRemoval(ctx context.Context, svc *desktop.Service, projectID string, record videoStudioMediaRecord) error {
	if !record.Removing {
		return fmt.Errorf("media removal was not recorded")
	}
	// The manifest only holds validated paths; the check keeps a damaged record from naming another file.
	if videoStudioMediaPathValid(record.Path, record.Asset.ID) {
		if err := svc.DeletePath(ctx, videoStudioProjectPath(projectID)+"/"+record.Path, desktop.SourceUser); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove media file: %w", err)
		}
	}
	if err := svc.DeletePath(ctx, m.posterPath(projectID, record.Asset.ID), desktop.SourceUser); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove media poster: %w", err)
	}
	m.discardAssetPreviews(projectID, record.Asset.ID)
	if err := m.removeMedia(projectID, record.Asset.ID); err != nil {
		return fmt.Errorf("remove media record: %w", err)
	}
	return nil
}

// markedMediaRecords lists the records whose removal started, of one project or (with "") of all projects.
func (m *videoStudioManager) markedMediaRecords(projectID string) []videoStudioMediaRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []videoStudioMediaRecord
	for id, records := range m.media {
		if projectID != "" && id != projectID {
			continue
		}
		for _, record := range records {
			if record.Removing {
				out = append(out, record)
			}
		}
	}
	return out
}

// finishInterruptedMediaRemovals finishes, when the worker starts, the removals that stopped part-way before the last
// shutdown. One that fails again stays marked and is retried at the next listing of its project or the next start.
func (m *videoStudioManager) finishInterruptedMediaRemovals() {
	marked := m.markedMediaRecords("")
	if len(marked) == 0 {
		return
	}
	if cfg, err := normalizedVideoStudioConfig(m.server); err != nil || !videoStudioUpkeepAllowed(cfg) {
		return
	}
	svc, _, err := m.server.getDesktopService(m.ctx)
	if err != nil {
		videoStudioLog(m.server.Logger, "Video Studio could not finish interrupted media removals", "error", err)
		return
	}
	for _, record := range marked {
		if m.ctx.Err() != nil {
			return
		}
		m.finishMarkedRemoval(m.ctx, svc, record.ProjectID, record.Asset.ID)
	}
}

// finishMarkedRemoval finishes one marked removal under the project lock, if the record is still marked.
func (m *videoStudioManager) finishMarkedRemoval(ctx context.Context, svc *desktop.Service, projectID, assetID string) {
	lock := m.projectLock(projectID)
	lock.Lock()
	defer lock.Unlock()
	m.finishMarkedRemovalLocked(ctx, svc, projectID, assetID)
}

func (m *videoStudioManager) finishMarkedRemovalLocked(ctx context.Context, svc *desktop.Service, projectID, assetID string) {
	record, ok := m.mediaRecordForRemoval(projectID, assetID)
	if !ok || !record.Removing {
		return
	}
	if err := m.finishMediaRemoval(ctx, svc, projectID, record); err != nil {
		videoStudioLog(m.server.Logger, "Video Studio could not finish a media removal", "project_id", projectID, "asset_id", assetID, "error", err)
		return
	}
	if m.server.Logger != nil {
		m.server.Logger.Info("Video Studio finished an interrupted media removal", "project_id", projectID, "asset_id", assetID)
	}
}

// sweepProjectMedia cleans up one project after interrupted work, under the project lock (a busy project is skipped
// and swept at a later listing):
//   - removals that stopped part-way finish;
//   - a record whose media file is gone and that the saved project does not use is removed with its poster;
//   - a media file ("<asset id>_<name>") or poster ("<asset id>.jpg") whose asset has no record and is not used by the
//     saved project is deleted.
//
// Records and files younger than videoStudioLeftoverMinAge are left alone. While a generate, reverse or still job of the
// project runs, only the marked removals finish: those jobs write their media file before they record it.
func (m *videoStudioManager) sweepProjectMedia(ctx context.Context, svc *desktop.Service, projectID string) {
	if !validVideoStudioProjectID(projectID) {
		return
	}
	lock := m.projectLock(projectID)
	if !lock.TryLock() {
		m.mu.Lock()
		delete(m.sweptAt, projectID)
		m.mu.Unlock()
		return
	}
	defer lock.Unlock()
	for _, record := range m.markedMediaRecords(projectID) {
		m.finishMarkedRemovalLocked(ctx, svc, projectID, record.Asset.ID)
	}
	if m.projectHasActiveJobs(projectID, "generate", "reverse", "still") {
		return
	}
	project, _, _, err := readVideoStudioProject(ctx, svc, videoStudioProjectPath(projectID))
	if err != nil {
		return
	}
	used := videoStudioUsedAssets(project)
	mediaFiles, err := videoStudioDirEntries(ctx, svc, videoStudioProjectPath(projectID)+"/media")
	if err != nil {
		return
	}
	posterFiles, err := videoStudioDirEntries(ctx, svc, videoStudioProjectPath(projectID)+"/posters")
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-videoStudioLeftoverMinAge)
	recorded := make(map[string]bool)
	m.mu.Lock()
	records := make([]videoStudioMediaRecord, 0, len(m.media[projectID]))
	for id, record := range m.media[projectID] {
		recorded[id] = true
		records = append(records, record)
	}
	m.mu.Unlock()
	for _, record := range records {
		id := record.Asset.ID
		if record.Removing || record.Asset.Kind == "" || used[id] || record.CreatedAt.After(cutoff) {
			continue
		}
		if _, present := mediaFiles[strings.TrimPrefix(record.Path, "media/")]; present {
			continue
		}
		// The media file is gone: a removal that stopped before it was recorded, or a file deleted outside the studio.
		if _, err := m.markMediaRemoving(projectID, id); err != nil {
			videoStudioLog(m.server.Logger, "Video Studio could not mark media without a file", "project_id", projectID, "asset_id", id, "error", err)
			continue
		}
		m.finishMarkedRemovalLocked(ctx, svc, projectID, id)
	}
	leftover := func(id string, entry desktop.FileEntry) bool {
		return validVideoStudioAssetID(id) && !recorded[id] && !used[id] && entry.ModTime.Before(cutoff)
	}
	for name, entry := range mediaFiles {
		if id, _, ok := strings.Cut(name, "_"); ok && leftover(id, entry) {
			m.deleteLeftover(ctx, svc, projectID, "media/"+name)
		}
	}
	for name, entry := range posterFiles {
		if id, ok := strings.CutSuffix(name, ".jpg"); ok && leftover(id, entry) {
			m.deleteLeftover(ctx, svc, projectID, "posters/"+name)
		}
	}
}

// deleteLeftover deletes one project file the sweep found without a record.
func (m *videoStudioManager) deleteLeftover(ctx context.Context, svc *desktop.Service, projectID, path string) {
	if err := svc.DeletePath(ctx, videoStudioProjectPath(projectID)+"/"+path, desktop.SourceUser); err != nil && !errors.Is(err, os.ErrNotExist) {
		videoStudioLog(m.server.Logger, "Video Studio could not delete a leftover media file", "project_id", projectID, "path", path, "error", err)
		return
	}
	if m.server.Logger != nil {
		m.server.Logger.Info("Video Studio deleted a leftover media file", "project_id", projectID, "path", path)
	}
}

// videoStudioDirEntries lists the regular files of a desktop folder by name; a missing folder has none.
func videoStudioDirEntries(ctx context.Context, svc *desktop.Service, dir string) (map[string]desktop.FileEntry, error) {
	entries, err := svc.ListFiles(ctx, dir)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]desktop.FileEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := make(map[string]desktop.FileEntry, len(entries))
	for _, entry := range entries {
		if entry.Type == "file" {
			files[entry.Name] = entry
		}
	}
	return files, nil
}

// videoStudioUsedAssets lists the assets the project document lists or places a clip of.
func videoStudioUsedAssets(project videostudio.Project) map[string]bool {
	used := make(map[string]bool, len(project.Assets))
	for _, asset := range project.Assets {
		used[asset.ID] = true
	}
	for _, track := range project.Tracks {
		for _, clip := range track.Clips {
			used[clip.AssetID] = true
		}
	}
	return used
}

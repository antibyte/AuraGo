package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/videostudio"
)

// posterPath is the project-relative location of an asset's poster.
func (m *videoStudioManager) posterPath(projectID, assetID string) string {
	return videoStudioProjectPath(projectID) + "/posters/" + assetID + ".jpg"
}

// attachPoster writes the poster of a visual asset when the project has room for it and marks the asset. A poster is
// best effort: when it cannot be made or stored the asset simply has none and the import still succeeds.
func (m *videoStudioManager) attachPoster(ctx context.Context, svc *desktop.Service, cfg *config.Config, projectID, stageDir, staged string, asset *videostudio.Asset) {
	output, ok := renderVideoStudioPoster(ctx, cfg, stageDir, staged, *asset)
	if !ok {
		return
	}
	lock := m.projectLock(projectID)
	lock.Lock()
	defer lock.Unlock()
	asset.HasPoster = m.storePosterLocked(ctx, svc, cfg, projectID, asset.ID, output)
}

// renderVideoStudioPoster writes the poster JPEG of a staged video or image asset into stageDir; audio has none.
func renderVideoStudioPoster(ctx context.Context, cfg *config.Config, stageDir, staged string, asset videostudio.Asset) (string, bool) {
	if asset.Kind != videostudio.AssetVideo && asset.Kind != videostudio.AssetImage {
		return "", false
	}
	// The staged source is "<asset id><ext>", so a JPEG source would collide with "<asset id>.jpg".
	output := filepath.Join(stageDir, "poster-"+asset.ID+".jpg")
	if err := videostudio.WritePoster(ctx, cfg.VideoStudio.FFmpegPath, staged, output, asset); err != nil {
		return "", false
	}
	return output, true
}

// storePosterLocked stores a rendered poster as the asset's poster when the project has room for it. The caller holds
// the project lock.
func (m *videoStudioManager) storePosterLocked(ctx context.Context, svc *desktop.Service, cfg *config.Config, projectID, assetID, output string) bool {
	info, err := os.Stat(output)
	if err != nil {
		return false
	}
	file, err := os.Open(output)
	if err != nil {
		return false
	}
	defer file.Close()
	used, err := m.projectStorageSize(ctx, svc, projectID)
	if err != nil || used+info.Size() > int64(cfg.VideoStudio.MaxProjectSizeMB)<<20 {
		return false
	}
	_ = svc.CreateDirectory(ctx, videoStudioProjectPath(projectID)+"/posters", desktop.SourceUser)
	_, err = svc.WriteFileStreamConditional(ctx, m.posterPath(projectID, assetID), file, info.Size(), desktop.SourceUser, nil)
	return err == nil
}

// discardPoster removes the poster of an import that did not finish. It runs without the request's cancellation so
// that a cancelled job still cleans up.
func (m *videoStudioManager) discardPoster(ctx context.Context, svc *desktop.Service, projectID string, asset videostudio.Asset) {
	if asset.HasPoster {
		_ = svc.DeletePath(context.WithoutCancel(ctx), m.posterPath(projectID, asset.ID), desktop.SourceUser)
	}
}

// handleVideoStudioPoster serves an asset's poster to anyone who can read the project.
func handleVideoStudioPoster(s *Server, w http.ResponseWriter, r *http.Request, projectID, assetID string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeVideoStudioMethodError(w)
		return
	}
	if !validVideoStudioAssetID(assetID) {
		writeVideoStudioError(w, http.StatusNotFound, "media_not_found", "Media not found.")
		return
	}
	svc, manager, _, ok := videoStudioReadAccess(s, w, r)
	if !ok {
		return
	}
	record, ok := manager.mediaRecord(projectID, assetID)
	if !ok || !record.Asset.HasPoster {
		if ok {
			// Media imported before posters existed gets one in the background (video_studio_media_upkeep.go).
			manager.requestPosterBackfill(record)
		}
		writeVideoStudioError(w, http.StatusNotFound, "media_not_found", "Media not found.")
		return
	}
	file, entry, _, err := svc.OpenPreviewFile(r.Context(), manager.posterPath(projectID, assetID))
	if err != nil {
		writeVideoStudioError(w, http.StatusNotFound, "media_not_found", "Media not found.")
		return
	}
	defer file.Close()
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeContent(w, r, "poster.jpg", entry.ModTime, file)
}

// handleVideoStudioMediaDelete removes an imported asset with its file, poster, cached previews and record. The saved
// project must no longer reference the asset, and no render, preview, probe, reverse, still, duplicate or range_render
// job of the project may be queued or running, because those read the file. A generation only adds new media, so it does not block.
// The removal is recorded first (markMediaRemoving), so the asset is gone for every reader at once, and then runs in
// order (finishMediaRemoval). When a step fails, the answer is an error and the record stays marked: a retry skips the
// checks and finishes the removal, and so do the next start and the next listing of the project.
func handleVideoStudioMediaDelete(s *Server, w http.ResponseWriter, r *http.Request, projectID, assetID string) {
	if !validVideoStudioAssetID(assetID) {
		writeVideoStudioError(w, http.StatusNotFound, "media_not_found", "Media not found.")
		return
	}
	svc, manager, _, ok := videoStudioMutationAccess(s, w, r, desktopScopeWrite)
	if !ok {
		return
	}
	lock := manager.projectLock(projectID)
	lock.Lock()
	defer lock.Unlock()
	record, ok := manager.mediaRecordForRemoval(projectID, assetID)
	if !ok {
		writeVideoStudioError(w, http.StatusNotFound, "media_not_found", "Media not found.")
		return
	}
	if !record.Removing {
		project, _, _, err := readVideoStudioProject(r.Context(), svc, videoStudioProjectPath(projectID))
		if err != nil {
			writeVideoStudioError(w, http.StatusNotFound, "project_not_found", "Project not found.")
			return
		}
		if videoStudioProjectUsesAsset(project, assetID) || manager.projectHasActiveJobs(projectID, "render", "preview", "probe", "reverse", "still", "duplicate", "range_render") {
			writeVideoStudioError(w, http.StatusConflict, "media_in_use", "This media is still in use. Save the project and try again.")
			return
		}
		if record, err = manager.markMediaRemoving(projectID, assetID); err != nil {
			writeVideoStudioError(w, http.StatusInternalServerError, "media_delete_failed", "The media could not be removed.")
			return
		}
	}
	if err := manager.finishMediaRemoval(r.Context(), svc, projectID, record); err != nil {
		writeVideoStudioMediaDeleteError(w, r, err, "The media could not be removed completely. Try again.")
		return
	}
	writeVideoStudioJSON(w, http.StatusOK, map[string]interface{}{"deleted": true, "id": assetID})
}

// writeVideoStudioMediaDeleteError answers a failed media removal: 409 when a publication gate (configuration change,
// read-only, shutdown) refused the write, 500 otherwise. The removal stays recorded, so a retry finishes it.
func writeVideoStudioMediaDeleteError(w http.ResponseWriter, r *http.Request, err error, message string) {
	if videoStudioPublicationRevoked(r, err) {
		writeVideoStudioError(w, http.StatusConflict, "configuration_changed", "Video Studio configuration changed before the media was removed.")
		return
	}
	writeVideoStudioError(w, http.StatusInternalServerError, "media_delete_failed", message)
}

// videoStudioProjectUsesAsset reports whether the project document lists the asset or places a clip of it.
func videoStudioProjectUsesAsset(project videostudio.Project, assetID string) bool {
	for _, asset := range project.Assets {
		if asset.ID == assetID {
			return true
		}
	}
	for _, track := range project.Tracks {
		for _, clip := range track.Clips {
			if clip.AssetID == assetID {
				return true
			}
		}
	}
	return false
}

// projectHasActiveJobs reports whether a queued or running job of one of the kinds belongs to the project.
func (m *videoStudioManager) projectHasActiveJobs(projectID string, kinds ...string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, job := range m.jobs {
		if job.ProjectID != projectID || (job.Status != "queued" && job.Status != "running") {
			continue
		}
		for _, kind := range kinds {
			if job.Kind == kind {
				return true
			}
		}
	}
	return false
}

// videoStudioProjectSizeWalkEntries caps the entries one project-list size walk visits (a variable for tests).
var videoStudioProjectSizeWalkEntries = 20000

// listedProjectStorage is the storage of a project for the project manager: its desktop tree plus its cached
// previews, the measure the quota uses. ok is false when the walk reached videoStudioProjectSizeWalkEntries or a
// folder could not be listed; the project is then listed without a size. The duplicate size check is not capped.
func (m *videoStudioManager) listedProjectStorage(ctx context.Context, svc *desktop.Service, projectID string) (int64, bool) {
	budget := videoStudioProjectSizeWalkEntries
	desktopBytes, ok := videoStudioTreeSizeWithin(ctx, svc, videoStudioProjectPath(projectID), &budget)
	if !ok {
		return 0, false
	}
	previewBytes, err := videoStudioPrivateTreeSize(filepath.Join(m.previewDir, projectID))
	if err != nil {
		return 0, false
	}
	return desktopBytes + previewBytes, true
}

// videoStudioTreeSizeWithin sums the files below a desktop folder. Every listed entry spends one unit of budget.
func videoStudioTreeSizeWithin(ctx context.Context, svc *desktop.Service, dir string, budget *int) (int64, bool) {
	if ctx.Err() != nil {
		return 0, false
	}
	entries, err := svc.ListFiles(ctx, dir)
	if err != nil {
		return 0, false
	}
	var total int64
	for _, entry := range entries {
		*budget--
		if *budget < 0 {
			return 0, false
		}
		if entry.Type == "directory" {
			size, ok := videoStudioTreeSizeWithin(ctx, svc, dir+"/"+entry.Name, budget)
			if !ok {
				return 0, false
			}
			total += size
			continue
		}
		total += entry.Size
	}
	return total, true
}

// discardAssetPreviews deletes the cached previews of a removed asset, so that they stop counting toward the project
// quota, and withdraws their download from the finished preview jobs.
func (m *videoStudioManager) discardAssetPreviews(projectID, assetID string) {
	ids := m.previewJobIDs(projectID, assetID, "")
	if len(ids) == 0 {
		return
	}
	for _, id := range ids {
		_ = os.Remove(filepath.Join(m.previewDir, projectID, id+".mp4"))
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range ids {
		if job := m.jobs[id]; job != nil && job.ProjectID == projectID && job.Kind == "preview" {
			job.Artifact = nil
			job.Message = "The media of this cached preview was removed."
		}
	}
	_ = m.persistLocked()
}

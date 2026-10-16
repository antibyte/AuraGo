package server

import (
	"context"
	"errors"
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
	if asset.Kind != videostudio.AssetVideo && asset.Kind != videostudio.AssetImage {
		return
	}
	// The staged source is "<asset id><ext>", so a JPEG source would collide with "<asset id>.jpg".
	output := filepath.Join(stageDir, "poster-"+asset.ID+".jpg")
	if err := videostudio.WritePoster(ctx, cfg.VideoStudio.FFmpegPath, staged, output, *asset); err != nil {
		return
	}
	info, err := os.Stat(output)
	if err != nil {
		return
	}
	file, err := os.Open(output)
	if err != nil {
		return
	}
	defer file.Close()
	lock := m.projectLock(projectID)
	lock.Lock()
	defer lock.Unlock()
	used, err := m.projectStorageSize(ctx, svc, projectID)
	if err != nil || used+info.Size() > int64(cfg.VideoStudio.MaxProjectSizeMB)<<20 {
		return
	}
	_ = svc.CreateDirectory(ctx, videoStudioProjectPath(projectID)+"/posters", desktop.SourceUser)
	if _, err := svc.WriteFileStreamConditional(ctx, m.posterPath(projectID, asset.ID), file, info.Size(), desktop.SourceUser, nil); err == nil {
		asset.HasPoster = true
	}
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
// project must no longer reference the asset, and no render, preview, probe, reverse, still or duplicate job of the
// project may be queued or running, because those read the file. A generation only adds new media, so it does not block.
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
	record, ok := manager.mediaRecord(projectID, assetID)
	if !ok {
		writeVideoStudioError(w, http.StatusNotFound, "media_not_found", "Media not found.")
		return
	}
	project, _, _, err := readVideoStudioProject(r.Context(), svc, videoStudioProjectPath(projectID))
	if err != nil {
		writeVideoStudioError(w, http.StatusNotFound, "project_not_found", "Project not found.")
		return
	}
	if videoStudioProjectUsesAsset(project, assetID) || manager.projectHasActiveJobs(projectID, "render", "preview", "probe", "reverse", "still", "duplicate") {
		writeVideoStudioError(w, http.StatusConflict, "media_in_use", "This media is still in use. Save the project and try again.")
		return
	}
	// The manifest only holds validated paths; the check keeps a damaged record from naming another file.
	if videoStudioMediaPathValid(record.Path, assetID) {
		if err := svc.DeletePath(r.Context(), videoStudioProjectPath(projectID)+"/"+record.Path, desktop.SourceUser); err != nil && !errors.Is(err, os.ErrNotExist) {
			writeVideoStudioMediaDeleteError(w, r, err, "The media file could not be removed.")
			return
		}
	}
	if err := svc.DeletePath(r.Context(), manager.posterPath(projectID, assetID), desktop.SourceUser); err != nil && !errors.Is(err, os.ErrNotExist) {
		writeVideoStudioMediaDeleteError(w, r, err, "The media poster could not be removed.")
		return
	}
	manager.discardAssetPreviews(projectID, assetID)
	if err := manager.removeMedia(projectID, assetID); err != nil {
		writeVideoStudioError(w, http.StatusInternalServerError, "media_delete_failed", "The media record could not be removed.")
		return
	}
	writeVideoStudioJSON(w, http.StatusOK, map[string]interface{}{"deleted": true, "id": assetID})
}

// writeVideoStudioMediaDeleteError answers a failed media removal: 409 when a publication gate (configuration change,
// read-only, shutdown) refused the write, 500 otherwise. Nothing was recorded as removed, so the user can retry.
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

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

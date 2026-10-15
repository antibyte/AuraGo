package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/videostudio"
)

// processReverse renders the requested source range backwards and imports it as "<name> (reversed).mp4".
func (m *videoStudioManager) processReverse(ctx context.Context, svc *desktop.Service, job *videoStudioJob, work *videoStudioWork, cfg *config.Config) (*videoStudioArtifact, map[string]interface{}, error) {
	record, ok := m.mediaRecord(work.projectID, work.request.AssetID)
	if !ok || record.Asset.Kind != videostudio.AssetVideo {
		return nil, nil, fmt.Errorf("reverse requires a video asset")
	}
	stageDir, cleanup, err := m.newStageDir(job.ID)
	if err != nil {
		return nil, nil, err
	}
	defer cleanup()
	staged, err := m.stageAsset(ctx, svc, cfg, record, stageDir, 0)
	if err != nil {
		return nil, nil, err
	}
	output := filepath.Join(stageDir, "reversed.mp4")
	err = videostudio.ReverseClip(ctx, videostudio.ReverseOptions{
		FFmpegPath: cfg.VideoStudio.FFmpegPath, Input: staged, Output: output,
		Offset: work.request.Offset, Frames: work.request.Duration, Rate: videostudio.ProjectRate(work.project),
		Width: record.Asset.Width, Height: record.Asset.Height,
		Encoder: videoStudioEncoder(ctx, cfg).Encoder, MaxOutputBytes: int64(cfg.VideoStudio.MaxAssetSizeMB) << 20,
		Progress: func(progress float64) { m.updateProgress(job.ID, progress) },
	})
	if err != nil {
		_ = os.Remove(output)
		if errors.Is(err, videostudio.ErrOutputLimit) {
			// The reversed copy becomes a project asset, so the limit it hit is the asset size limit.
			return nil, nil, errVideoStudioAssetSizeLimit
		}
		return nil, nil, err
	}
	asset, err := m.importDerivedMedia(ctx, svc, cfg, job, reversedAssetName(record.Asset.Name), output)
	if err != nil {
		return nil, nil, err
	}
	return nil, map[string]interface{}{"asset": asset, "purpose": "reverse"}, nil
}

// processStill captures one source frame as PNG: imported as a still asset for a freeze frame, or published as a
// download for an export.
func (m *videoStudioManager) processStill(ctx context.Context, svc *desktop.Service, job *videoStudioJob, work *videoStudioWork, cfg *config.Config) (*videoStudioArtifact, map[string]interface{}, error) {
	record, ok := m.mediaRecord(work.projectID, work.request.AssetID)
	if !ok || record.Asset.Kind != videostudio.AssetVideo {
		return nil, nil, fmt.Errorf("frame capture requires a video asset")
	}
	stageDir, cleanup, err := m.newStageDir(job.ID)
	if err != nil {
		return nil, nil, err
	}
	defer cleanup()
	staged, err := m.stageAsset(ctx, svc, cfg, record, stageDir, 0)
	if err != nil {
		return nil, nil, err
	}
	output := filepath.Join(stageDir, "frame.png")
	if err := videostudio.ExtractStill(ctx, cfg.VideoStudio.FFmpegPath, staged, output, work.request.Frame, videostudio.ProjectRate(work.project)); err != nil {
		return nil, nil, err
	}
	if work.request.Purpose == "export" {
		return m.publishStillExport(ctx, svc, job, cfg, output)
	}
	asset, err := m.importDerivedMedia(ctx, svc, cfg, job, stillAssetName(record.Asset.Name, work.request.Frame), output)
	if err != nil {
		return nil, nil, err
	}
	return nil, map[string]interface{}{"asset": asset, "purpose": "freeze"}, nil
}

// publishStillExport stores a captured frame in the project's exports folder, under the export and quota limits.
func (m *videoStudioManager) publishStillExport(ctx context.Context, svc *desktop.Service, job *videoStudioJob, cfg *config.Config, output string) (*videoStudioArtifact, map[string]interface{}, error) {
	info, err := os.Stat(output)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return nil, nil, fmt.Errorf("frame output is unavailable")
	}
	exportLimit := int64(cfg.VideoStudio.MaxExportSizeMB) << 20
	if info.Size() > exportLimit {
		return nil, nil, errVideoStudioExportSizeLimit
	}
	maxProject := int64(cfg.VideoStudio.MaxProjectSizeMB) << 20
	projectLock := m.projectLock(job.ProjectID)
	projectLock.Lock()
	defer projectLock.Unlock()
	used, err := m.projectStorageSize(ctx, svc, job.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	remaining := maxProject - used
	if remaining <= 0 || info.Size() > remaining {
		return nil, nil, errVideoStudioProjectSizeLimit
	}
	file, err := os.Open(output)
	if err != nil {
		return nil, nil, err
	}
	exportPath := videoStudioProjectPath(job.ProjectID) + "/exports/" + job.ID + ".png"
	_, err = svc.WriteFileStreamConditional(ctx, exportPath, file, min64(exportLimit, remaining), desktop.SourceUser, nil)
	_ = file.Close()
	if err != nil {
		return nil, nil, err
	}
	artifact := &videoStudioArtifact{Name: job.ID + ".png", Path: "exports/" + job.ID + ".png",
		DownloadURL: fmt.Sprintf("/api/desktop/video-studio/projects/%s/exports/%s.png", job.ProjectID, job.ID), Size: info.Size()}
	return artifact, map[string]interface{}{"purpose": "export"}, nil
}

// importDerivedMedia stores a file a job produced as a new project asset, the way generated videos are stored: media
// file, manifest entry and project merge. The media file and manifest entry are removed again when a later step fails.
func (m *videoStudioManager) importDerivedMedia(ctx context.Context, svc *desktop.Service, cfg *config.Config, job *videoStudioJob, name, localPath string) (videostudio.Asset, error) {
	info, err := os.Lstat(localPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return videostudio.Asset{}, fmt.Errorf("derived media output is unavailable")
	}
	maxAsset := int64(cfg.VideoStudio.MaxAssetSizeMB) << 20
	if info.Size() > maxAsset {
		return videostudio.Asset{}, errVideoStudioAssetSizeLimit
	}
	assetID := uidNew()
	// name stays the readable display name (asset.Name and record.Asset.Name); only the media file name is sanitized.
	mediaPath := "media/" + assetID + "_" + safeVideoStudioFilename(name)
	desktopPath := videoStudioProjectPath(job.ProjectID) + "/" + mediaPath
	input, err := os.Open(localPath)
	if err != nil {
		return videostudio.Asset{}, err
	}
	projectLock := m.projectLock(job.ProjectID)
	projectLock.Lock()
	if len(m.mediaForProject(job.ProjectID)) >= videostudio.MaxAssetsPerProject {
		projectLock.Unlock()
		_ = input.Close()
		return videostudio.Asset{}, fmt.Errorf("asset_count_limit")
	}
	used, err := m.projectStorageSize(ctx, svc, job.ProjectID)
	if err != nil {
		projectLock.Unlock()
		_ = input.Close()
		return videostudio.Asset{}, err
	}
	remaining := (int64(cfg.VideoStudio.MaxProjectSizeMB) << 20) - used
	if remaining <= 0 || info.Size() > remaining {
		projectLock.Unlock()
		_ = input.Close()
		return videostudio.Asset{}, errVideoStudioProjectSizeLimit
	}
	_, err = svc.WriteFileStreamConditional(ctx, desktopPath, input, min64(maxAsset, remaining), desktop.SourceUser, nil)
	_ = input.Close()
	projectLock.Unlock()
	if err != nil {
		return videostudio.Asset{}, err
	}
	published := true
	defer func() {
		if !published {
			return
		}
		if removeErr := m.removeMedia(job.ProjectID, assetID); removeErr != nil {
			videoStudioLog(m.server.Logger, "Video Studio derived media cleanup could not update manifest", "job_id", job.ID, "error", removeErr)
			return
		}
		if deleteErr := svc.DeletePath(context.Background(), desktopPath, desktop.SourceUser); deleteErr != nil {
			videoStudioLog(m.server.Logger, "Video Studio derived media file cleanup failed", "job_id", job.ID, "error", deleteErr)
		}
	}()
	stageDir, cleanup, err := m.newStageDir(job.ID + "-derived")
	if err != nil {
		return videostudio.Asset{}, err
	}
	defer cleanup()
	record := videoStudioMediaRecord{ProjectID: job.ProjectID, Asset: videostudio.Asset{ID: assetID, Name: name, Path: mediaPath},
		Path: mediaPath, SHA256: fileSHA256(localPath), CreatedAt: time.Now().UTC()}
	staged, err := m.stageAsset(ctx, svc, cfg, record, stageDir, 0)
	if err != nil {
		return videostudio.Asset{}, err
	}
	asset, err := videostudio.Probe(ctx, cfg.VideoStudio.FFmpegPath, staged)
	if err != nil {
		return videostudio.Asset{}, err
	}
	asset.ID, asset.Name, asset.Path = assetID, name, mediaPath
	record.Asset = asset
	if err := m.recordMedia(record); err != nil {
		return videostudio.Asset{}, err
	}
	if err := m.mergeProjectAsset(ctx, svc, cfg, job.ProjectID, asset); err != nil {
		return videostudio.Asset{}, err
	}
	published = false
	return asset, nil
}

// reversedAssetName names a reversed copy after its source: "<name> (reversed).mp4".
func reversedAssetName(source string) string {
	return strings.TrimSuffix(source, filepath.Ext(source)) + " (reversed).mp4"
}

// stillAssetName names a captured frame after its source: "<name> (frame N).png".
func stillAssetName(source string, frame int) string {
	return fmt.Sprintf("%s (frame %d).png", strings.TrimSuffix(source, filepath.Ext(source)), frame)
}

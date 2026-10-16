package server

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
	asset, err := m.importDerivedMedia(ctx, svc, cfg, job, reversedAssetName(record.Asset.Name, work.request.Name), output)
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
	options := videoStudioStillOptions(work.request)
	options.FFmpegPath, options.Input, options.Output, options.Rate = cfg.VideoStudio.FFmpegPath, staged, output, videostudio.ProjectRate(work.project)
	if err := videostudio.ExtractStill(ctx, options); err != nil {
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

// videoStudioStillOptions is the clip position a still job captures. A request with a speed names clip frame ClipFrame
// of a clip that starts at source frame Offset, so the capture samples the source the way the renderer does at that
// speed; a request without one names source frame Frame at 1x.
func videoStudioStillOptions(request videoStudioJobRequest) videostudio.StillOptions {
	if request.Speed == 0 {
		return videostudio.StillOptions{Offset: request.Frame, Speed: 1}
	}
	return videostudio.StillOptions{Offset: request.Offset, Frame: request.ClipFrame, Speed: request.Speed}
}

// validVideoStudioStillPosition checks a still request's clip position: a clip speed, a non-negative offset and clip
// frame, and Frame equal to the source frame floor(Offset + ClipFrame·Speed) that position shows (the client computes
// it the same way, with the same 1e-9 tolerance), so the frame range check covers the frame the job captures.
func validVideoStudioStillPosition(request videoStudioJobRequest) bool {
	if request.Speed == 0 {
		return request.Offset == 0 && request.ClipFrame == 0
	}
	if math.IsNaN(request.Speed) || request.Speed < videostudio.MinClipSpeed || request.Speed > videostudio.MaxClipSpeed {
		return false
	}
	if request.Offset < 0 || request.ClipFrame < 0 || request.Offset > request.Frame {
		return false
	}
	return math.Floor(float64(request.Offset)+float64(request.ClipFrame)*request.Speed+1e-9) == float64(request.Frame)
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
	m.attachPoster(ctx, svc, cfg, job.ProjectID, stageDir, staged, &asset)
	record.Asset = asset
	if err := m.recordMedia(record); err != nil {
		m.discardPoster(ctx, svc, job.ProjectID, asset)
		return videostudio.Asset{}, err
	}
	saved, err := m.mergeProjectAsset(ctx, svc, cfg, job.ProjectID, asset)
	if err != nil {
		m.discardPoster(ctx, svc, job.ProjectID, asset)
		return videostudio.Asset{}, err
	}
	published = false
	return saved, nil
}

// videoStudioReversedSuffix is the English suffix of a reversed copy; the editor sends its own name with the
// localized suffix (videoStudio.reversedSuffix) toggled instead.
const videoStudioReversedSuffix = " (reversed)"

// reversedAssetName names a reversed copy. requested is the editor's name for it without an extension (the display
// name, extension dropped, with the localized suffix toggled); without one, the source name gets " (reversed)"
// toggled: added to a plain name, removed from a reversed one, so reversing back and forth never grows the name. The
// name ends in ".mp4", the reversed file's container, and is cut to fit the asset name limit.
func reversedAssetName(source, requested string) string {
	base := strings.TrimSpace(requested)
	if base == "" {
		base = trimMediaExtension(source)
		if trimmed, ok := strings.CutSuffix(base, videoStudioReversedSuffix); ok && strings.TrimSpace(trimmed) != "" {
			base = trimmed
		} else {
			base += videoStudioReversedSuffix
		}
		base = strings.TrimSpace(base)
	}
	if runes := []rune(base); len(runes) > videostudio.MaxAssetNameRunes-len(".mp4") {
		base = strings.TrimSpace(string(runes[:videostudio.MaxAssetNameRunes-len(".mp4")]))
	}
	return base + ".mp4"
}

// validVideoStudioDerivedName accepts an optional editor-supplied name for derived media: at most
// MaxAssetNameRunes runes and no control characters.
func validVideoStudioDerivedName(name string) bool {
	if utf8.RuneCountInString(name) > videostudio.MaxAssetNameRunes {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// trimMediaExtension drops a file extension (a dot and one to five letters or digits), so "Mr. Smith" keeps its name.
func trimMediaExtension(name string) string {
	ext := filepath.Ext(name)
	if len(ext) < 2 || len(ext) > 6 || len(ext) == len(name) {
		return name
	}
	for _, r := range ext[1:] {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return name
		}
	}
	return strings.TrimSuffix(name, ext)
}

// stillAssetName names a captured frame after its source: "<name> (frame N).png".
func stillAssetName(source string, frame int) string {
	return fmt.Sprintf("%s (frame %d).png", trimMediaExtension(source), frame)
}

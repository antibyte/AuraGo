package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	stdimage "image"
	_ "image/gif"
	_ "image/jpeg"
	png "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/fileutil"
	"aurago/internal/tools"
	"aurago/internal/videostudio"
	_ "golang.org/x/image/webp"
)

func (m *videoStudioManager) process(ctx context.Context, job *videoStudioJob, work *videoStudioWork, cfg *config.Config) (*videoStudioArtifact, map[string]interface{}, error) {
	svc, _, err := m.server.getDesktopService(ctx)
	if err != nil {
		return nil, nil, err
	}
	if cfg == nil {
		return nil, nil, fmt.Errorf("configuration unavailable")
	}
	if work.kind == "probe" {
		return m.processProbe(ctx, svc, job, work, cfg)
	}
	if work.kind == "preview" {
		return m.processPreview(ctx, svc, job, work, cfg)
	}
	if work.kind == "render" {
		return m.processRender(ctx, svc, job, work, cfg)
	}
	if work.kind == "generate" {
		return m.processGenerate(ctx, svc, job, work, cfg)
	}
	return nil, nil, fmt.Errorf("unsupported video studio job")
}

func (m *videoStudioManager) processProbe(ctx context.Context, svc *desktop.Service, job *videoStudioJob, work *videoStudioWork, cfg *config.Config) (*videoStudioArtifact, map[string]interface{}, error) {
	record, ok := m.mediaRecord(work.projectID, work.assetID)
	if !ok || record.Path != work.mediaPath {
		return nil, nil, fmt.Errorf("import record is unavailable")
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
	asset, err := videostudio.Probe(ctx, cfg.VideoStudio.FFmpegPath, staged)
	if err != nil {
		return nil, nil, err
	}
	asset.ID, asset.Name, asset.Path = record.Asset.ID, record.Asset.Name, record.Path
	record.Asset = asset
	record.CreatedAt = time.Now().UTC()
	if err := m.recordMedia(record); err != nil {
		return nil, nil, err
	}
	if err := m.mergeProjectAsset(ctx, svc, cfg, work.projectID, asset); err != nil {
		return nil, nil, err
	}
	return nil, map[string]interface{}{"asset": asset}, nil
}

func (m *videoStudioManager) processPreview(ctx context.Context, svc *desktop.Service, job *videoStudioJob, work *videoStudioWork, cfg *config.Config) (*videoStudioArtifact, map[string]interface{}, error) {
	record, ok := m.mediaRecord(work.projectID, work.request.AssetID)
	if !ok {
		return nil, nil, fmt.Errorf("asset is unavailable")
	}
	if record.Asset.Kind != videostudio.AssetVideo {
		return nil, nil, fmt.Errorf("previews require video assets")
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
	previewDir := filepath.Join(m.previewDir, job.ProjectID)
	if err := os.MkdirAll(previewDir, 0o700); err != nil {
		return nil, nil, err
	}
	_ = os.Chmod(previewDir, 0o700)
	output := filepath.Join(stageDir, "preview.mp4")
	if err := videostudio.PreparePreview(ctx, cfg.VideoStudio.FFmpegPath, staged, output); err != nil {
		_ = os.Remove(output)
		return nil, nil, err
	}
	info, err := os.Stat(output)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 512<<20 {
		_ = os.Remove(output)
		return nil, nil, fmt.Errorf("preview output is unavailable or exceeds 512 MiB")
	}
	projectLock := m.projectLock(job.ProjectID)
	projectLock.Lock()
	defer projectLock.Unlock()
	projectBytes, previewBytes, err := m.projectStorageUsage(ctx, svc, job.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	oldJobs := m.previewJobIDs(job.ProjectID, record.Asset.ID, job.ID)
	var oldBytes int64
	oldPaths := make([]string, 0, len(oldJobs))
	for _, oldID := range oldJobs {
		oldPath := filepath.Join(previewDir, oldID+".mp4")
		oldInfo, statErr := os.Lstat(oldPath)
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil || !oldInfo.Mode().IsRegular() {
			return nil, nil, fmt.Errorf("cached preview is not a regular file")
		}
		oldBytes += oldInfo.Size()
		oldPaths = append(oldPaths, oldPath)
	}
	maxProject := int64(cfg.VideoStudio.MaxProjectSizeMB) << 20
	if projectBytes > maxProject || previewBytes < oldBytes || info.Size() > maxProject-projectBytes || previewBytes-oldBytes > maxProject-projectBytes-info.Size() {
		return nil, nil, errVideoStudioProjectSizeLimit
	}
	if err := os.MkdirAll(previewDir, 0o700); err != nil {
		return nil, nil, err
	}
	finalPath := filepath.Join(previewDir, job.ID+".mp4")
	if _, statErr := os.Lstat(finalPath); !os.IsNotExist(statErr) {
		return nil, nil, fmt.Errorf("preview already exists")
	}
	if err := fileutil.PublishContext(ctx, func() error {
		if err := os.Rename(output, finalPath); err != nil {
			return err
		}
		for _, oldPath := range oldPaths {
			if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
				_ = os.Remove(finalPath)
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}
	m.expirePreviewJobs(job.ProjectID, record.Asset.ID, oldJobs)
	artifact := &videoStudioArtifact{Name: "preview.mp4", DownloadURL: fmt.Sprintf("/api/desktop/video-studio/projects/%s/previews/%s", job.ProjectID, job.ID), Size: info.Size()}
	return artifact, map[string]interface{}{"asset_id": record.Asset.ID}, nil
}

func videoStudioPrivateTreeSize(root string) (int64, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var total int64
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return 0, err
		}
		if info.IsDir() {
			size, err := videoStudioPrivateTreeSize(path)
			if err != nil {
				return 0, err
			}
			if size > int64(^uint64(0)>>1)-total {
				return 0, fmt.Errorf("preview cache size overflows")
			}
			total += size
		} else if info.Mode().IsRegular() {
			if info.Size() > int64(^uint64(0)>>1)-total {
				return 0, fmt.Errorf("preview cache size overflows")
			}
			total += info.Size()
		} else {
			return 0, fmt.Errorf("preview cache contains a non-regular file")
		}
	}
	return total, nil
}

func (m *videoStudioManager) processRender(ctx context.Context, svc *desktop.Service, job *videoStudioJob, work *videoStudioWork, cfg *config.Config) (*videoStudioArtifact, map[string]interface{}, error) {
	if err := validateVideoStudioProject(work.project); err != nil {
		return nil, nil, err
	}
	assets := m.canonicalProjectAssets(job.ProjectID, work.project.Assets)
	if len(assets) != len(work.project.Assets) {
		return nil, nil, fmt.Errorf("project references untrusted media")
	}
	active := videoStudioRenderAssetIDs(work.project)
	stageDir, cleanup, err := m.newStageDir(job.ID)
	if err != nil {
		return nil, nil, err
	}
	defer cleanup()
	paths := make(map[string]string, len(assets))
	var stagedTotal int64
	for _, asset := range work.project.Assets {
		if _, used := active[asset.ID]; !used {
			continue
		}
		record := assets[asset.ID]
		staged, size, err := m.stageAssetSized(ctx, svc, cfg, record, stageDir, stagedTotal)
		if err != nil {
			return nil, nil, err
		}
		stagedTotal += size
		paths[asset.ID] = staged
	}
	output := filepath.Join(stageDir, "render.mp4")
	err = videostudio.Render(ctx, cfg.VideoStudio.FFmpegPath, work.project, paths, output, func(progress float64) { m.updateProgress(job.ID, progress) })
	if err != nil {
		_ = os.Remove(output)
		return nil, nil, err
	}
	info, err := os.Stat(output)
	if err != nil || !info.Mode().IsRegular() {
		_ = os.Remove(output)
		return nil, nil, fmt.Errorf("render output is unavailable")
	}
	projectPath := videoStudioProjectPath(job.ProjectID)
	projectLock := m.projectLock(job.ProjectID)
	projectLock.Lock()
	defer projectLock.Unlock()
	used, err := m.projectStorageSize(ctx, svc, job.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	maxProject := int64(cfg.VideoStudio.MaxProjectSizeMB) << 20
	remaining := maxProject - used
	if remaining <= 0 {
		return nil, nil, errVideoStudioProjectSizeLimit
	}
	maxOutput := min64(min64(remaining, maxProject), 2<<30)
	if info.Size() > maxOutput {
		return nil, nil, errVideoStudioProjectSizeLimit
	}
	file, err := os.Open(output)
	if err != nil {
		return nil, nil, err
	}
	exportPath := projectPath + "/exports/" + job.ID + ".mp4"
	_, err = svc.WriteFileStreamConditional(ctx, exportPath, file, maxOutput, desktop.SourceUser, nil)
	_ = file.Close()
	if err != nil {
		return nil, nil, err
	}
	artifact := &videoStudioArtifact{Name: job.ID + ".mp4", Path: "exports/" + job.ID + ".mp4", DownloadURL: fmt.Sprintf("/api/desktop/video-studio/projects/%s/exports/%s.mp4", job.ProjectID, job.ID), Size: info.Size()}
	return artifact, map[string]interface{}{"duration_frames": videostudio.Duration(work.project), "width": work.project.Width, "height": work.project.Height}, nil
}

func videoStudioRenderAssetIDs(project videostudio.Project) map[string]struct{} {
	active := make(map[string]struct{})
	for _, track := range project.Tracks {
		if track.Hidden || track.Kind == videostudio.TrackAudio && track.Muted {
			continue
		}
		for _, clip := range track.Clips {
			if track.Kind == videostudio.TrackAudio && clip.Volume <= 0 {
				continue
			}
			active[clip.AssetID] = struct{}{}
		}
	}
	return active
}

func (m *videoStudioManager) processGenerate(ctx context.Context, svc *desktop.Service, job *videoStudioJob, work *videoStudioWork, cfg *config.Config) (artifact *videoStudioArtifact, metadata map[string]interface{}, runErr error) {
	providerReturnedVideo := false
	defer func() {
		if providerReturnedVideo && runErr != nil &&
			!errors.Is(runErr, errVideoStudioProjectSizeLimit) &&
			!errors.Is(runErr, errVideoStudioAssetSizeLimit) &&
			!errors.Is(runErr, errVideoStudioGenerationImport) {
			runErr = fmt.Errorf("%w: %w", errVideoStudioGenerationImport, runErr)
		}
	}()
	if !cfg.VideoGeneration.Enabled || cfg.VideoGeneration.APIKey == "" || cfg.VideoGeneration.ProviderType == "" {
		return nil, nil, fmt.Errorf("generation_unavailable")
	}
	if m.server.BudgetTracker != nil && m.server.BudgetTracker.IsBlocked("video_generation") {
		return nil, nil, fmt.Errorf("budget_blocked")
	}
	projectLock := m.projectLock(job.ProjectID)
	projectLock.Lock()
	assetCount := len(m.mediaForProject(job.ProjectID))
	used, err := m.projectStorageSize(ctx, svc, job.ProjectID)
	projectLock.Unlock()
	if err != nil {
		return nil, nil, fmt.Errorf("inspect project storage before generation: %w", err)
	}
	if assetCount >= videostudio.MaxAssetsPerProject {
		return nil, nil, fmt.Errorf("asset_count_limit")
	}
	if used >= int64(cfg.VideoStudio.MaxProjectSizeMB)<<20 {
		return nil, nil, errVideoStudioProjectSizeLimit
	}
	params := tools.VideoGenParams{Prompt: work.request.Prompt, NegativePrompt: work.request.NegativePrompt, DurationSeconds: work.request.DurationSeconds, Resolution: work.request.Resolution, AspectRatio: work.request.AspectRatio}
	provider := strings.ToLower(cfg.VideoGeneration.ProviderType)
	if provider == "agnes" && (work.request.FirstFrameAssetID != "" || work.request.LastFrameAssetID != "" || len(work.request.ReferenceAssetIDs) != 0) {
		return nil, nil, fmt.Errorf("provider_image_mode_unsupported")
	}
	image := func(id string) (string, error) {
		if id == "" {
			return "", nil
		}
		record, ok := m.mediaRecord(job.ProjectID, id)
		if !ok || record.Asset.Kind != videostudio.AssetImage {
			return "", fmt.Errorf("generation image must be a project image asset")
		}
		file, entry, _, err := svc.OpenPreviewFile(ctx, videoStudioProjectPath(job.ProjectID)+"/"+record.Path)
		if err != nil {
			return "", err
		}
		defer file.Close()
		const maxImageBytes = int64(20 << 20)
		if entry.Size <= 0 || entry.Size > maxImageBytes {
			return "", fmt.Errorf("generation image exceeds 20 MiB")
		}
		data, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
		if err != nil {
			return "", err
		}
		if int64(len(data)) > maxImageBytes {
			return "", fmt.Errorf("generation image exceeds 20 MiB")
		}
		digest := sha256.Sum256(data)
		if record.SHA256 == "" || hex.EncodeToString(digest[:]) != record.SHA256 {
			return "", fmt.Errorf("generation image changed after import")
		}
		return videoStudioGenerationImagePayload(provider, data)
	}
	params.FirstFrameImage, err = image(work.request.FirstFrameAssetID)
	if err != nil {
		return nil, nil, err
	}
	params.LastFrameImage, err = image(work.request.LastFrameAssetID)
	if err != nil {
		return nil, nil, err
	}
	for _, id := range work.request.ReferenceAssetIDs {
		value, err := image(id)
		if err != nil {
			return nil, nil, err
		}
		params.ReferenceImages = append(params.ReferenceImages, value)
	}
	if provider == "minimax" {
		// The helper formats all imported images for this provider; preserve that
		// convention if optional modes are added to a provider-specific request.
		if params.LastFrameImage != "" && !strings.HasPrefix(params.LastFrameImage, "data:image/png;base64,") {
			params.LastFrameImage = "data:image/png;base64," + params.LastFrameImage
		}
		for i := range params.ReferenceImages {
			if !strings.HasPrefix(params.ReferenceImages[i], "data:image/png;base64,") {
				params.ReferenceImages[i] = "data:image/png;base64," + params.ReferenceImages[i]
			}
		}
	}
	if err := m.markExternalStatusUnknownForWork(job.ID, work); err != nil {
		return nil, nil, err
	}
	result := tools.GenerateVideoResult(ctx, cfg, m.server.MediaRegistryDB, m.server.Logger, params)
	if result.Status != "ok" {
		return nil, nil, fmt.Errorf("provider_generation_failed")
	}
	providerReturnedVideo = true
	if m.server.BudgetTracker != nil && result.CostEstimate > 0 {
		m.server.BudgetTracker.RecordCostForCategory("video_generation", result.CostEstimate)
	}
	if err := m.markExternalStatusResolved(job.ID); err != nil {
		return nil, nil, err
	}
	outputRoot := filepath.Join(cfg.Directories.DataDir, "generated_videos")
	rootAbs, err := filepath.Abs(outputRoot)
	if err != nil {
		return nil, nil, err
	}
	resultAbs, err := filepath.Abs(result.FilePath)
	if err != nil {
		return nil, nil, err
	}
	rel, err := filepath.Rel(rootAbs, resultAbs)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return nil, nil, fmt.Errorf("generated video path is invalid")
	}
	info, err := os.Lstat(resultAbs)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return nil, nil, fmt.Errorf("generated video output is unavailable")
	}
	maxAsset := int64(cfg.VideoStudio.MaxAssetSizeMB) << 20
	if info.Size() > maxAsset {
		return nil, nil, errVideoStudioAssetSizeLimit
	}
	assetID := uidNew()
	name := safeVideoStudioFilename(result.Filename)
	mediaPath := "media/" + assetID + "_" + name
	input, err := os.Open(resultAbs)
	if err != nil {
		return nil, nil, err
	}
	projectLock = m.projectLock(job.ProjectID)
	projectLock.Lock()
	used, err = m.projectStorageSize(ctx, svc, job.ProjectID)
	if err != nil {
		projectLock.Unlock()
		_ = input.Close()
		return nil, nil, err
	}
	remaining := (int64(cfg.VideoStudio.MaxProjectSizeMB) << 20) - used
	if remaining <= 0 || info.Size() > remaining {
		projectLock.Unlock()
		_ = input.Close()
		return nil, nil, errVideoStudioProjectSizeLimit
	}
	if remaining < maxAsset {
		maxAsset = remaining
	}
	desktopPath := videoStudioProjectPath(job.ProjectID) + "/" + mediaPath
	_, err = svc.WriteFileStreamConditional(ctx, desktopPath, input, maxAsset, desktop.SourceUser, nil)
	_ = input.Close()
	projectLock.Unlock()
	if err != nil {
		return nil, nil, err
	}
	mediaPublished := true
	defer func() {
		if mediaPublished {
			if removeErr := m.removeMedia(job.ProjectID, assetID); removeErr != nil {
				videoStudioLog(m.server.Logger, "Video Studio generated media cleanup could not update manifest", "job_id", job.ID, "error", removeErr)
				return
			}
			if deleteErr := svc.DeletePath(context.Background(), desktopPath, desktop.SourceUser); deleteErr != nil {
				videoStudioLog(m.server.Logger, "Video Studio generated media file cleanup failed", "job_id", job.ID, "error", deleteErr)
			}
		}
	}()
	assetRecord := videoStudioMediaRecord{ProjectID: job.ProjectID, Asset: videostudio.Asset{ID: assetID, Name: name, Path: mediaPath}, Path: mediaPath, SHA256: fileSHA256(resultAbs), CreatedAt: time.Now().UTC()}
	stageDir, cleanup, err := m.newStageDir(job.ID + "-generated")
	if err != nil {
		return nil, nil, err
	}
	defer cleanup()
	staged, err := m.stageAsset(ctx, svc, cfg, assetRecord, stageDir, 0)
	if err != nil {
		return nil, nil, err
	}
	asset, err := videostudio.Probe(ctx, cfg.VideoStudio.FFmpegPath, staged)
	if err != nil {
		return nil, nil, err
	}
	asset.ID, asset.Name, asset.Path = assetID, name, mediaPath
	assetRecord.Asset = asset
	if err := m.recordMedia(assetRecord); err != nil {
		return nil, nil, err
	}
	if err := m.mergeProjectAsset(ctx, svc, cfg, job.ProjectID, asset); err != nil {
		return nil, nil, err
	}
	mediaPublished = false
	return nil, map[string]interface{}{"asset": asset, "provider": result.Provider, "model": result.Model, "cost_estimate": result.CostEstimate}, nil
}

func videoStudioGenerationImagePayload(provider string, data []byte) (string, error) {
	const maxImageBytes = int64(20 << 20)
	if len(data) == 0 || int64(len(data)) > maxImageBytes {
		return "", fmt.Errorf("generation image exceeds 20 MiB")
	}
	imageConfig, _, err := stdimage.DecodeConfig(bytes.NewReader(data))
	if err != nil || imageConfig.Width <= 0 || imageConfig.Height <= 0 || imageConfig.Width > 8192 || imageConfig.Height > 8192 || int64(imageConfig.Width)*int64(imageConfig.Height) > 16_000_000 {
		return "", fmt.Errorf("generation image dimensions or format are unsupported")
	}
	decoded, _, err := stdimage.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode local generation image: %w", err)
	}
	var normalized bytes.Buffer
	if err := png.Encode(&normalized, decoded); err != nil {
		return "", err
	}
	if normalized.Len() == 0 || int64(normalized.Len()) > maxImageBytes {
		return "", fmt.Errorf("normalized generation image exceeds 20 MiB")
	}
	payload := base64.StdEncoding.EncodeToString(normalized.Bytes())
	if strings.EqualFold(provider, "minimax") {
		payload = "data:image/png;base64," + payload
	}
	return payload, nil
}

func (m *videoStudioManager) stageAsset(ctx context.Context, svc *desktop.Service, cfg *config.Config, record videoStudioMediaRecord, dir string, already int64) (string, error) {
	path, _, err := m.stageAssetSized(ctx, svc, cfg, record, dir, already)
	return path, err
}

func (m *videoStudioManager) stageAssetSized(ctx context.Context, svc *desktop.Service, cfg *config.Config, record videoStudioMediaRecord, dir string, already int64) (string, int64, error) {
	if !videoStudioMediaPathValid(record.Path, record.Asset.ID) {
		return "", 0, fmt.Errorf("project asset path is invalid")
	}
	file, entry, _, err := svc.OpenPreviewFile(ctx, videoStudioProjectPath(record.ProjectID)+"/"+record.Path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	maxAsset := int64(cfg.VideoStudio.MaxAssetSizeMB) << 20
	if entry.Size <= 0 || entry.Size > maxAsset {
		return "", 0, fmt.Errorf("asset exceeds the configured size limit")
	}
	maxProject := int64(cfg.VideoStudio.MaxProjectSizeMB) << 20
	if already > maxProject || entry.Size > maxProject-already {
		return "", 0, fmt.Errorf("staged project assets exceed the configured project limit")
	}
	name := filepath.Join(dir, record.Asset.ID+filepath.Ext(record.Asset.Name))
	out, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", 0, err
	}
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(out, hasher), io.LimitReader(videoStudioContextReader{ctx: ctx, reader: file}, maxAsset+1))
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(name)
		return "", 0, copyErr
	}
	if syncErr != nil {
		_ = os.Remove(name)
		return "", 0, syncErr
	}
	if closeErr != nil {
		_ = os.Remove(name)
		return "", 0, closeErr
	}
	if written != entry.Size {
		_ = os.Remove(name)
		return "", 0, fmt.Errorf("asset changed while it was staged")
	}
	if record.SHA256 == "" || hex.EncodeToString(hasher.Sum(nil)) != record.SHA256 {
		_ = os.Remove(name)
		return "", 0, fmt.Errorf("asset content changed after import")
	}
	if err := ctx.Err(); err != nil {
		_ = os.Remove(name)
		return "", 0, err
	}
	return name, written, nil
}

func (m *videoStudioManager) newStageDir(suffix string) (string, func(), error) {
	base := filepath.Join(filepath.Dir(m.path), "staging")
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", func() {}, err
	}
	_ = os.Chmod(base, 0o700)
	dir, err := os.MkdirTemp(base, safeVideoStudioFilename(suffix)+"-")
	if err != nil {
		return "", func() {}, err
	}
	_ = os.Chmod(dir, 0o700)
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

func (m *videoStudioManager) mediaRecord(projectID, assetID string) (videoStudioMediaRecord, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := m.media[projectID]
	record, ok := items[assetID]
	return record, ok
}

func (m *videoStudioManager) recordMedia(record videoStudioMediaRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !videoStudioMediaPathValid(record.Path, record.Asset.ID) || record.ProjectID == "" {
		return fmt.Errorf("invalid media manifest entry")
	}
	records := m.media[record.ProjectID]
	previous, existed := records[record.Asset.ID]
	if records == nil {
		records = make(map[string]videoStudioMediaRecord)
		m.media[record.ProjectID] = records
	}
	records[record.Asset.ID] = record
	if err := m.persistLocked(); err != nil {
		if existed {
			records[record.Asset.ID] = previous
		} else {
			delete(records, record.Asset.ID)
			if len(records) == 0 {
				delete(m.media, record.ProjectID)
			}
		}
		return err
	}
	return nil
}

func (m *videoStudioManager) canonicalProjectAssets(projectID string, input []videostudio.Asset) map[string]videoStudioMediaRecord {
	assets := make(map[string]videoStudioMediaRecord, len(input))
	for _, asset := range input {
		record, ok := m.mediaRecord(projectID, asset.ID)
		if !ok {
			return nil
		}
		assets[asset.ID] = record
	}
	return assets
}

func (m *videoStudioManager) mergeProjectAsset(ctx context.Context, svc *desktop.Service, cfg *config.Config, projectID string, asset videostudio.Asset) error {
	lock := m.projectLock(projectID)
	lock.Lock()
	defer lock.Unlock()
	path := videoStudioProjectPath(projectID)
	for attempt := 0; attempt < 6; attempt++ {
		project, body, version, err := readVideoStudioProject(ctx, svc, path)
		if err != nil {
			return err
		}
		found := false
		for i := range project.Assets {
			if project.Assets[i].ID == asset.ID {
				project.Assets[i] = asset
				found = true
				break
			}
		}
		if !found {
			project.Assets = append(project.Assets, asset)
		}
		canonical, err := m.canonicalizeProject(projectID, project)
		if err != nil {
			return err
		}
		if err := validateVideoStudioProject(canonical); err != nil {
			return err
		}
		encoded, err := json.Marshal(canonical)
		if err != nil {
			return err
		}
		maxProject := int64(cfg.VideoStudio.MaxProjectSizeMB) << 20
		used, err := m.projectStorageSize(ctx, svc, projectID)
		if err != nil {
			return err
		}
		newTotal := used - int64(len(body)) + int64(len(encoded))
		if newTotal > maxProject || int64(len(encoded)) > maxProject {
			return errVideoStudioProjectSizeLimit
		}
		precondition := func(state desktop.FileWriteState) error {
			if state.Exists && state.Version != "" { /* byte writer does not populate Version */
			}
			current := desktop.NoteVersion(state.Data)
			if state.Exists && current == version {
				return nil
			}
			return &desktopFileConflict{Code: "file_conflict", Path: path, Version: current, Status: 412}
		}
		_, err = svc.WriteFileBytesConditional(ctx, path+"/project.json", encoded, desktop.SourceUser, precondition)
		if err == nil {
			return nil
		}
		var conflict *desktopFileConflict
		if !errors.As(err, &conflict) {
			return err
		}
	}
	return fmt.Errorf("project changed repeatedly while media metadata was being merged")
}

func (m *videoStudioManager) canonicalizeProject(projectID string, project videostudio.Project) (videostudio.Project, error) {
	canonical := make([]videostudio.Asset, 0, len(project.Assets))
	for _, item := range project.Assets {
		record, ok := m.mediaRecord(projectID, item.ID)
		if !ok {
			return project, fmt.Errorf("project references an asset that was not imported")
		}
		if !videoStudioMediaPathValid(record.Path, record.Asset.ID) {
			return project, fmt.Errorf("project asset path is not valid")
		}
		canonical = append(canonical, record.Asset)
	}
	project.Assets = canonical
	return project, nil
}

func validateVideoStudioProject(project videostudio.Project) error {
	return videostudio.Validate(project)
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

type videoStudioContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r videoStudioContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func fileSHA256(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return ""
	}
	return hex.EncodeToString(hash.Sum(nil))
}

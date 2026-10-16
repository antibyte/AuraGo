package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/fileutil"
	"aurago/internal/uid"
	"aurago/internal/videostudio"
)

// errVideoStudioDuplicateQuota reports a copy that would exceed the project storage limit.
var errVideoStudioDuplicateQuota = errors.New("duplicate_quota")

// videoStudioDuplicateHeadroom is the free space a copy must leave on the Desktop volume, as the renderer does.
const videoStudioDuplicateHeadroom = 64 << 20

// duplicateFile is one file a duplicate copies, relative to the project folder ("media/…" or "posters/…").
type duplicateFile struct {
	path string
	size int64
}

// duplicateSnapshot is what a duplicate copies, read under the source project's lock.
type duplicateSnapshot struct {
	project  videostudio.Project
	records  []videoStudioMediaRecord
	files    []duplicateFile
	document []byte // the copy's project.json, set by snapshotDuplicateSource
	missing  int    // media files of listed assets that are not on disk
	size     int64  // the project document plus every copied file
}

// videoStudioDirFiles lists the regular files of a desktop folder with their sizes; a missing folder is empty.
func videoStudioDirFiles(ctx context.Context, svc *desktop.Service, dir string) (map[string]int64, error) {
	entries, err := svc.ListFiles(ctx, dir)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]int64{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := make(map[string]int64, len(entries))
	for _, entry := range entries {
		if entry.Type == "file" {
			files[entry.Name] = entry.Size
		}
	}
	return files, nil
}

// copyableProjectFiles lists what a duplicate of the project copies: its document, the media files of the assets the
// saved project lists and their posters. Exports, versions, previews and media the document does not list stay
// behind, so they are not counted either. A poster that is gone is left out (the copied asset has none then); a media
// file that is gone is counted in missing. The caller holds the project lock.
func (m *videoStudioManager) copyableProjectFiles(ctx context.Context, svc *desktop.Service, projectID string, project videostudio.Project, projectBytes []byte) (duplicateSnapshot, error) {
	media, err := videoStudioDirFiles(ctx, svc, videoStudioProjectPath(projectID)+"/media")
	if err != nil {
		return duplicateSnapshot{}, err
	}
	posters, err := videoStudioDirFiles(ctx, svc, videoStudioProjectPath(projectID)+"/posters")
	if err != nil {
		return duplicateSnapshot{}, err
	}
	snap := duplicateSnapshot{project: project, size: int64(len(projectBytes))}
	snap.project.Assets = append([]videostudio.Asset(nil), project.Assets...)
	for i, asset := range snap.project.Assets {
		record, ok := m.mediaRecord(projectID, asset.ID)
		if !ok || !videoStudioMediaPathValid(record.Path, asset.ID) {
			return duplicateSnapshot{}, fmt.Errorf("media record is missing")
		}
		if size, ok := media[strings.TrimPrefix(record.Path, "media/")]; ok {
			snap.files = append(snap.files, duplicateFile{path: record.Path, size: size})
			snap.size += size
		} else {
			snap.missing++
		}
		if record.Asset.HasPoster {
			if size, ok := posters[asset.ID+".jpg"]; ok {
				snap.files = append(snap.files, duplicateFile{path: "posters/" + asset.ID + ".jpg", size: size})
				snap.size += size
			} else {
				record.Asset.HasPoster = false
				snap.project.Assets[i].HasPoster = false
			}
		}
		snap.records = append(snap.records, record)
	}
	return snap, nil
}

// duplicateAdmission checks, with the project lock held, that a copy fits the project limit and the Desktop volume.
// It returns the HTTP status, code and message to answer with, or an empty code when the copy may start. A media
// file that is already gone does not block here; the job fails with duplicate_failed instead. Unknown free space
// never blocks.
func (m *videoStudioManager) duplicateAdmission(ctx context.Context, svc *desktop.Service, cfg *config.Config, projectID string, project videostudio.Project, projectBytes []byte) (int, string, string) {
	snap, err := m.copyableProjectFiles(ctx, svc, projectID, project, projectBytes)
	if err != nil {
		return http.StatusNotFound, "project_not_found", "Project not found."
	}
	if snap.size > int64(cfg.VideoStudio.MaxProjectSizeMB)<<20 {
		return http.StatusRequestEntityTooLarge, "duplicate_quota", "The copy would exceed this project's storage limit."
	}
	if free, err := fileutil.FreeDiskBytes(svc.Config().WorkspaceDir); err == nil && free < snap.size+videoStudioDuplicateHeadroom {
		return http.StatusInsufficientStorage, "insufficient_disk_space", "There is not enough free disk space for the copy."
	}
	return 0, "", ""
}

// snapshotDuplicateSource holds the source lock only while it reads project.json, canonicalizes and validates it,
// measures the copy and lists the media records. The bytes are copied afterwards, outside the lock: deleting media is
// refused while a duplicate job is queued or running (projectHasActiveJobs), and imported files never change, so the
// listed files stay as they are. The copy's document (the snapshot under the new name) is encoded here, so the quota
// check measures exactly what will be written.
func (m *videoStudioManager) snapshotDuplicateSource(ctx context.Context, svc *desktop.Service, cfg *config.Config, sourceID, name string) (duplicateSnapshot, error) {
	lock := m.projectLock(sourceID)
	lock.Lock()
	defer lock.Unlock()
	project, projectBytes, _, err := readVideoStudioProject(ctx, svc, videoStudioProjectPath(sourceID))
	if err != nil {
		return duplicateSnapshot{}, err
	}
	project, err = m.canonicalizeProject(sourceID, project)
	if err != nil {
		return duplicateSnapshot{}, err
	}
	if err := videostudio.Validate(project); err != nil {
		return duplicateSnapshot{}, err
	}
	snap, err := m.copyableProjectFiles(ctx, svc, sourceID, project, projectBytes)
	if err != nil {
		return duplicateSnapshot{}, err
	}
	if snap.missing > 0 {
		return duplicateSnapshot{}, fmt.Errorf("%d media files of the project are missing", snap.missing)
	}
	snap.project.Name = name
	snap.document, err = json.Marshal(snap.project)
	if err != nil {
		return duplicateSnapshot{}, err
	}
	snap.size += int64(len(snap.document)) - int64(len(projectBytes))
	if snap.size > int64(cfg.VideoStudio.MaxProjectSizeMB)<<20 {
		return duplicateSnapshot{}, errVideoStudioDuplicateQuota
	}
	return snap, nil
}

// videoStudioStagingContext lets writes into a folder that is not a project yet bypass the job's publication gate:
// they publish nothing, so they must not mark the job committed (which would make it impossible to cancel).
// Cancelling the job still stops them, because the gate's commit checks the context first.
func videoStudioStagingContext(ctx context.Context) context.Context {
	return fileutil.WithPublicationGate(ctx, func(commit func() error) error { return commit() })
}

// copyVideoStudioProjectFile streams one file into the copy. The snapshot size is the budget: a file that grew,
// shrank or vanished since the snapshot fails the copy.
func copyVideoStudioProjectFile(ctx context.Context, svc *desktop.Service, from, to string, size int64) error {
	file, entry, _, err := svc.OpenPreviewFile(ctx, from)
	if err != nil {
		return err
	}
	defer file.Close()
	if entry.Size != size {
		return fmt.Errorf("a copied file changed after the snapshot")
	}
	written, err := svc.WriteFileStreamConditional(ctx, to, file, max(size, 1), desktop.SourceUser, nil)
	if err != nil {
		return err
	}
	if written.Size != size {
		return fmt.Errorf("a copied file changed while it was copied")
	}
	return nil
}

// verifyDuplicateCopy checks, before the copy's project.json is written, that every copied media file and poster is
// present with its snapshot size.
func verifyDuplicateCopy(ctx context.Context, svc *desktop.Service, destination string, snap duplicateSnapshot) error {
	listed := map[string]map[string]int64{}
	for _, file := range snap.files {
		dir := path.Dir(file.path)
		if listed[dir] == nil {
			files, err := videoStudioDirFiles(ctx, svc, destination+"/"+dir)
			if err != nil {
				return err
			}
			listed[dir] = files
		}
		if size, ok := listed[dir][path.Base(file.path)]; !ok || size != file.size {
			return fmt.Errorf("copied file %s is missing", file.path)
		}
	}
	return nil
}

// recordMediaBatch adds the media records of one project with a single manifest write; nothing is recorded when the
// write fails.
func (m *videoStudioManager) recordMediaBatch(projectID string, records []videoStudioMediaRecord) error {
	if len(records) == 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	previous := m.media[projectID]
	items := make(map[string]videoStudioMediaRecord, len(previous)+len(records))
	for id, record := range previous {
		items[id] = record
	}
	for _, record := range records {
		if record.ProjectID != projectID || !videoStudioMediaPathValid(record.Path, record.Asset.ID) {
			return fmt.Errorf("invalid media manifest entry")
		}
		items[record.Asset.ID] = record
	}
	m.media[projectID] = items
	if err := m.persistLocked(); err != nil {
		if previous == nil {
			delete(m.media, projectID)
		} else {
			m.media[projectID] = previous
		}
		return err
	}
	return nil
}

// setDuplicateCopy records (copyID set) or clears (copyID empty) the folder a duplicate job writes. Recording happens
// before anything is copied, so a restart in the middle of the copy still knows which folder to remove; clearing
// happens once the copy is complete or removed. The cleared state is best effort: a stale entry only makes the next
// start check a folder that holds project.json (and leaves it) or is already gone.
func (m *videoStudioManager) setDuplicateCopy(jobID, copyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[jobID]
	if job == nil {
		return fmt.Errorf("duplicate job is unknown")
	}
	previous := job.CopyProjectID
	job.CopyProjectID = copyID
	if err := m.persistLocked(); err != nil {
		job.CopyProjectID = previous
		return err
	}
	return nil
}

// cleanupInterruptedDuplicates removes what duplicate jobs left behind when the server stopped during their copy. A
// copy becomes a project only when its project.json is written last, so a recorded copy folder without one is
// incomplete: the folder and its media records are removed. A folder with project.json is a finished copy and is never
// touched. Each step is best effort and logged; a copy that could not be checked or removed is tried again at the next
// start.
func (m *videoStudioManager) cleanupInterruptedDuplicates() {
	type pendingCopy struct{ jobID, copyID string }
	var pending []pendingCopy
	m.mu.Lock()
	for _, job := range m.jobs {
		if job.Kind == "duplicate" && job.CopyProjectID != "" && job.Status != "queued" && job.Status != "running" {
			pending = append(pending, pendingCopy{jobID: job.ID, copyID: job.CopyProjectID})
		}
	}
	m.mu.Unlock()
	if len(pending) == 0 {
		return
	}
	svc, _, err := m.server.getDesktopService(m.ctx)
	if err != nil {
		videoStudioLog(m.server.Logger, "Video Studio could not check the folders of interrupted duplicates", "error", err)
		return
	}
	for _, item := range pending {
		if m.ctx.Err() != nil {
			return
		}
		if validVideoStudioProjectID(item.copyID) {
			folder := videoStudioProjectPath(item.copyID)
			files, err := videoStudioDirFiles(m.ctx, svc, folder)
			if err != nil {
				videoStudioLog(m.server.Logger, "Video Studio could not check an interrupted duplicate", "job_id", item.jobID, "project_id", item.copyID, "error", err)
				continue
			}
			if _, complete := files["project.json"]; !complete {
				if err := svc.DeletePath(m.ctx, folder, desktop.SourceUser); err != nil {
					videoStudioLog(m.server.Logger, "Video Studio could not remove an interrupted duplicate", "job_id", item.jobID, "project_id", item.copyID, "error", err)
					continue
				}
				m.removeProject(item.copyID)
				if m.server.Logger != nil {
					m.server.Logger.Info("Video Studio removed the incomplete copy of an interrupted duplicate", "job_id", item.jobID, "project_id", item.copyID)
				}
			}
		}
		if err := m.setDuplicateCopy(item.jobID, ""); err != nil {
			videoStudioLog(m.server.Logger, "Video Studio could not record a checked duplicate", "job_id", item.jobID, "error", err)
		}
	}
}

// copyDuplicate writes a snapshot as a new project: its media files and posters (same asset ids), the media records,
// and the project.json last, so the copy is not a project until it is complete. A copy that fails or is cancelled
// removes what it made. jobID, when set, receives progress and records the copy folder on the job before the copy
// starts (cleanupInterruptedDuplicates removes it after a restart).
func (m *videoStudioManager) copyDuplicate(ctx context.Context, svc *desktop.Service, jobID, sourceID string, snap duplicateSnapshot) (map[string]interface{}, error) {
	copyID := uid.New()
	destination := videoStudioProjectPath(copyID)
	staging := videoStudioStagingContext(ctx)
	if jobID != "" {
		if err := m.setDuplicateCopy(jobID, copyID); err != nil {
			return nil, err
		}
	}
	fail := func(err error) (map[string]interface{}, error) {
		// Without project.json the folder is not a project, so it is removed whatever state the job is in.
		deleteErr := svc.DeletePath(context.Background(), destination, desktop.SourceUser)
		m.removeProject(copyID)
		if deleteErr != nil {
			// The job keeps the copy id, so the next start removes the folder.
			videoStudioLog(m.server.Logger, "Video Studio could not remove a failed duplicate", "job_id", jobID, "project_id", copyID, "error", deleteErr)
		} else if jobID != "" {
			_ = m.setDuplicateCopy(jobID, "")
		}
		return nil, err
	}
	if err := svc.CreateDirectory(staging, destination+"/media", desktop.SourceUser); err != nil {
		return fail(err)
	}
	var copied int64
	for _, file := range snap.files {
		if err := copyVideoStudioProjectFile(staging, svc, videoStudioProjectPath(sourceID)+"/"+file.path, destination+"/"+file.path, file.size); err != nil {
			return fail(err)
		}
		copied += file.size
		if jobID != "" && snap.size > 0 {
			m.updateProgress(jobID, float64(copied)/float64(snap.size))
		}
	}
	if err := verifyDuplicateCopy(ctx, svc, destination, snap); err != nil {
		return fail(err)
	}
	records := make([]videoStudioMediaRecord, 0, len(snap.records))
	for _, record := range snap.records {
		record.ProjectID = copyID
		records = append(records, record)
	}
	if err := m.recordMediaBatch(copyID, records); err != nil {
		return fail(err)
	}
	document := snap.document
	if document == nil {
		encoded, err := json.Marshal(snap.project)
		if err != nil {
			return fail(err)
		}
		document = encoded
	}
	// This write publishes the copy and commits the job.
	if _, err := svc.WriteFileBytesConditional(ctx, destination+"/project.json", document, desktop.SourceUser, func(state desktop.FileWriteState) error {
		if state.Exists {
			return fmt.Errorf("project already exists")
		}
		return nil
	}); err != nil {
		return fail(err)
	}
	if jobID != "" {
		// The copy is a project now; a failure to record that only makes the next start look at it and leave it.
		_ = m.setDuplicateCopy(jobID, "")
	}
	return map[string]interface{}{"project_id": copyID, "name": snap.project.Name, "size_bytes": snap.size}, nil
}

// processDuplicate copies a saved project into a new one under the requested name: the document, its media and
// posters. Exports, versions and previews stay behind. The copy is checked against the project limit and the free
// space again when the job starts, because the source may have grown since the request was admitted.
func (m *videoStudioManager) processDuplicate(ctx context.Context, svc *desktop.Service, job *videoStudioJob, work *videoStudioWork, cfg *config.Config) (*videoStudioArtifact, map[string]interface{}, error) {
	snap, err := m.snapshotDuplicateSource(ctx, svc, cfg, job.ProjectID, work.request.Name)
	if err != nil {
		return nil, nil, err
	}
	if free, err := fileutil.FreeDiskBytes(svc.Config().WorkspaceDir); err == nil && free < snap.size+videoStudioDuplicateHeadroom {
		return nil, nil, fmt.Errorf("copy needs %d bytes: %w", snap.size, videostudio.ErrInsufficientDiskSpace)
	}
	result, err := m.copyDuplicate(ctx, svc, job.ID, job.ProjectID, snap)
	if err != nil {
		return nil, nil, err
	}
	return nil, result, nil
}

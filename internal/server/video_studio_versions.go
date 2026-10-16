package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"aurago/internal/desktop"
	"aurago/internal/uid"
	"aurago/internal/videostudio"
)

const (
	videoStudioVersionAuto          = "auto"
	videoStudioVersionManual        = "manual"
	videoStudioVersionBeforeRestore = "before_restore"
	videoStudioAutoVersionInterval  = 10 * time.Minute
	videoStudioAutoVersionLimit     = 20
	videoStudioManualVersionLimit   = 10
	// videoStudioVersionIndexReserve is the quota headroom kept for index.json when a snapshot is written.
	videoStudioVersionIndexReserve = 16 << 10
	videoStudioVersionLabelRunes   = 120
)

// videoStudioNow is the clock of version timestamps and automatic snapshots; tests replace it.
var videoStudioNow = func() time.Time { return time.Now().UTC() }

type videoStudioVersion struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Label     string    `json:"label,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	SizeBytes int64     `json:"size_bytes"`
	Name      string    `json:"name"`
}

type videoStudioVersionIndex struct {
	Versions []videoStudioVersion `json:"versions"`
}

func videoStudioVersionsDir(projectID string) string {
	return videoStudioProjectPath(projectID) + "/versions"
}

// validVideoStudioVersionID accepts the ids snapshotVideoStudioVersion creates (lowercase UUIDs from uid.New). Every
// version path is built from an id that passed it: index.json is a Desktop file, and a crafted id such as
// "../../<project>/project" would otherwise name a file outside the versions folder.
func validVideoStudioVersionID(value string) bool { return validVideoStudioProjectID(value) }

// videoStudioVersionDocPath is the document of a version; callers pass a validated id (validVideoStudioVersionID).
func videoStudioVersionDocPath(projectID, versionID string) string {
	return videoStudioVersionsDir(projectID) + "/" + versionID + ".json"
}

func videoStudioVersionIndexPath(projectID string) string {
	return videoStudioVersionsDir(projectID) + "/index.json"
}

// readVideoStudioVersionIndex returns the versions of a project, oldest first. A missing index is an empty list.
// Entries whose id is not a version id are dropped: they are never listed, read, restored or pruned, and the next
// index write leaves them out.
func readVideoStudioVersionIndex(ctx context.Context, svc *desktop.Service, projectID string) (videoStudioVersionIndex, error) {
	data, _, err := svc.ReadFileBytes(ctx, videoStudioVersionIndexPath(projectID))
	if errors.Is(err, os.ErrNotExist) {
		return videoStudioVersionIndex{}, nil
	}
	if err != nil {
		return videoStudioVersionIndex{}, err
	}
	var index videoStudioVersionIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return videoStudioVersionIndex{}, fmt.Errorf("decode version index: %w", err)
	}
	valid := index.Versions[:0]
	for _, entry := range index.Versions {
		if validVideoStudioVersionID(entry.ID) {
			valid = append(valid, entry)
		}
	}
	index.Versions = valid
	return index, nil
}

// readVideoStudioVersion finds a version in the index and reads its document; found is false when either is missing.
func readVideoStudioVersion(ctx context.Context, svc *desktop.Service, projectID, versionID string) (videoStudioVersion, []byte, bool, error) {
	if !validVideoStudioVersionID(versionID) {
		return videoStudioVersion{}, nil, false, nil
	}
	index, err := readVideoStudioVersionIndex(ctx, svc, projectID)
	if err != nil {
		return videoStudioVersion{}, nil, false, err
	}
	for _, entry := range index.Versions {
		if entry.ID != versionID {
			continue
		}
		data, _, err := svc.ReadFileBytes(ctx, videoStudioVersionDocPath(projectID, versionID))
		if errors.Is(err, os.ErrNotExist) {
			return videoStudioVersion{}, nil, false, nil
		}
		if err != nil {
			return videoStudioVersion{}, nil, false, err
		}
		return entry, data, true, nil
	}
	return videoStudioVersion{}, nil, false, nil
}

// versionPool groups the kinds for the count limits: automatic snapshots and restore safety copies share a pool.
func versionPool(kind string) string {
	if kind == videoStudioVersionManual {
		return videoStudioVersionManual
	}
	return videoStudioVersionAuto
}

// pruneVideoStudioVersions keeps the newest entries of each pool within its limit, in their original order, and
// returns the entries it removed.
func pruneVideoStudioVersions(versions []videoStudioVersion) (kept, removed []videoStudioVersion) {
	counts := map[string]int{}
	for i := len(versions) - 1; i >= 0; i-- {
		pool := versionPool(versions[i].Kind)
		limit := videoStudioAutoVersionLimit
		if pool == videoStudioVersionManual {
			limit = videoStudioManualVersionLimit
		}
		counts[pool]++
		if counts[pool] > limit {
			removed = append(removed, versions[i])
			continue
		}
		kept = append(kept, versions[i])
	}
	// kept was collected newest first; restore the index order (oldest first).
	for left, right := 0, len(kept)-1; left < right; left, right = left+1, right-1 {
		kept[left], kept[right] = kept[right], kept[left]
	}
	return kept, removed
}

// autoVersionDue reports whether a save should keep the document it replaces: no version exists yet, or the newest
// one (of any kind) is at least videoStudioAutoVersionInterval old.
func autoVersionDue(index videoStudioVersionIndex, now time.Time) bool {
	if len(index.Versions) == 0 {
		return true
	}
	newest := index.Versions[0].CreatedAt
	for _, version := range index.Versions[1:] {
		if version.CreatedAt.After(newest) {
			newest = version.CreatedAt
		}
	}
	return now.Sub(newest) >= videoStudioAutoVersionInterval
}

// snapshotVideoStudioVersion stores data as a new version of the project, prunes the pools and rewrites the index.
// The caller holds the project lock and has checked the quota.
func snapshotVideoStudioVersion(ctx context.Context, svc *desktop.Service, projectID, kind, label, name string, data []byte) (videoStudioVersion, error) {
	index, err := readVideoStudioVersionIndex(ctx, svc, projectID)
	if err != nil {
		return videoStudioVersion{}, err
	}
	entry := videoStudioVersion{ID: uid.New(), Kind: kind, Label: label, CreatedAt: videoStudioNow(), SizeBytes: int64(len(data)), Name: name}
	_ = svc.CreateDirectory(ctx, videoStudioVersionsDir(projectID), desktop.SourceUser)
	if err := svc.WriteFileBytes(ctx, videoStudioVersionDocPath(projectID, entry.ID), data, desktop.SourceUser); err != nil {
		return videoStudioVersion{}, err
	}
	kept, removed := pruneVideoStudioVersions(append(index.Versions, entry))
	encoded, err := json.Marshal(videoStudioVersionIndex{Versions: kept})
	if err == nil {
		err = svc.WriteFileBytes(ctx, videoStudioVersionIndexPath(projectID), encoded, desktop.SourceUser)
	}
	if err != nil {
		_ = svc.DeletePath(context.WithoutCancel(ctx), videoStudioVersionDocPath(projectID, entry.ID), desktop.SourceUser)
		return videoStudioVersion{}, err
	}
	for _, old := range removed {
		// The index read already dropped invalid ids; the check keeps this delete safe on its own.
		if validVideoStudioVersionID(old.ID) {
			_ = svc.DeletePath(ctx, videoStudioVersionDocPath(projectID, old.ID), desktop.SourceUser)
		}
	}
	return entry, nil
}

// decodeVideoStudioDocument decodes a stored project document and upgrades it to the current model.
func decodeVideoStudioDocument(data []byte) (videostudio.Project, error) {
	var project videostudio.Project
	if err := json.Unmarshal(data, &project); err != nil {
		return videostudio.Project{}, fmt.Errorf("decode version: %w", err)
	}
	return videostudio.Upgrade(project)
}

// videoStudioVersionLabel trims a manual version label; ok is false when it is too long or holds control characters.
func videoStudioVersionLabel(raw string) (string, bool) {
	label := strings.TrimSpace(raw)
	if utf8.RuneCountInString(label) > videoStudioVersionLabelRunes {
		return "", false
	}
	for _, r := range label {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return label, true
}

// handleVideoStudioVersions lists the versions of a project, oldest first (GET), or saves a manual version of the
// saved document (POST, optional body {"label": "..."}).
func handleVideoStudioVersions(s *Server, w http.ResponseWriter, r *http.Request, projectID string) {
	switch r.Method {
	case http.MethodGet:
		svc, _, _, ok := videoStudioReadAccess(s, w, r)
		if !ok {
			return
		}
		index, err := readVideoStudioVersionIndex(r.Context(), svc, projectID)
		if err != nil {
			writeVideoStudioError(w, http.StatusInternalServerError, "version_list_failed", "Versions could not be listed.")
			return
		}
		versions := index.Versions
		if versions == nil {
			versions = []videoStudioVersion{}
		}
		writeVideoStudioJSON(w, http.StatusOK, map[string]interface{}{"versions": versions})
	case http.MethodPost:
		svc, manager, cfg, ok := videoStudioMutationAccess(s, w, r, desktopScopeWrite)
		if !ok {
			return
		}
		var body struct {
			Label string `json:"label"`
		}
		// An empty body saves an unlabelled version.
		if err := decodeVideoStudioJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil && !errors.Is(err, io.EOF) {
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_request", "Version request is invalid.")
			return
		}
		label, ok := videoStudioVersionLabel(body.Label)
		if !ok {
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_request", "The version label is too long or contains control characters.")
			return
		}
		lock := manager.projectLock(projectID)
		lock.Lock()
		defer lock.Unlock()
		project, data, _, err := readVideoStudioProject(r.Context(), svc, videoStudioProjectPath(projectID))
		if err != nil {
			writeVideoStudioError(w, http.StatusNotFound, "project_not_found", "Project not found.")
			return
		}
		used, err := manager.projectStorageSize(r.Context(), svc, projectID)
		if err != nil {
			writeVideoStudioError(w, http.StatusNotFound, "project_not_found", "Project not found.")
			return
		}
		if used+int64(len(data))+videoStudioVersionIndexReserve > int64(cfg.VideoStudio.MaxProjectSizeMB)<<20 {
			writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "project_size_limit", "Project storage is full. Remove media or versions before saving another version.")
			return
		}
		entry, err := snapshotVideoStudioVersion(r.Context(), svc, projectID, videoStudioVersionManual, label, project.Name, data)
		if err != nil {
			writeVideoStudioError(w, http.StatusInternalServerError, "version_save_failed", "The version could not be saved.")
			return
		}
		writeVideoStudioJSON(w, http.StatusCreated, map[string]interface{}{"version": entry})
	default:
		writeVideoStudioMethodError(w)
	}
}

// handleVideoStudioVersion returns one version: its listing entry and the document it holds.
func handleVideoStudioVersion(s *Server, w http.ResponseWriter, r *http.Request, projectID, versionID string) {
	if r.Method != http.MethodGet {
		writeVideoStudioMethodError(w)
		return
	}
	if !validVideoStudioVersionID(versionID) {
		writeVideoStudioError(w, http.StatusNotFound, "version_not_found", "Version not found.")
		return
	}
	svc, _, _, ok := videoStudioReadAccess(s, w, r)
	if !ok {
		return
	}
	entry, data, found, err := readVideoStudioVersion(r.Context(), svc, projectID, versionID)
	if err != nil {
		writeVideoStudioError(w, http.StatusInternalServerError, "version_list_failed", "The version could not be read.")
		return
	}
	if !found {
		writeVideoStudioError(w, http.StatusNotFound, "version_not_found", "Version not found.")
		return
	}
	writeVideoStudioJSON(w, http.StatusOK, map[string]interface{}{"version": entry, "project": json.RawMessage(data)})
}

// handleVideoStudioVersionRestore replaces the saved project with a version. The saved document is kept first as a
// before_restore version, and the restored document goes through Upgrade, canonicalization and Validate like a save.
// Restoring needs If-Match, and it is refused while probe, generate, reverse or still jobs of the project are still
// importing, because their imports would land in the restored document.
func handleVideoStudioVersionRestore(s *Server, w http.ResponseWriter, r *http.Request, projectID, versionID string) {
	if r.Method != http.MethodPost {
		writeVideoStudioMethodError(w)
		return
	}
	svc, manager, cfg, ok := videoStudioMutationAccess(s, w, r, desktopScopeWrite)
	if !ok {
		return
	}
	if strings.TrimSpace(r.Header.Get("If-Match")) == "" {
		writeVideoStudioError(w, http.StatusPreconditionRequired, "precondition_required", "Restore requires the current project ETag in If-Match.")
		return
	}
	precondition, err := desktopFilePrecondition(r)
	if err != nil {
		writeVideoStudioFileError(w, err)
		return
	}
	if !validVideoStudioVersionID(versionID) {
		writeVideoStudioError(w, http.StatusNotFound, "version_not_found", "Version not found.")
		return
	}
	lock := manager.projectLock(projectID)
	lock.Lock()
	defer lock.Unlock()
	current, currentData, currentVersion, err := readVideoStudioProject(r.Context(), svc, videoStudioProjectPath(projectID))
	if err != nil {
		writeVideoStudioError(w, http.StatusNotFound, "project_not_found", "Project not found.")
		return
	}
	if err := precondition(desktop.FileWriteState{Exists: true, Data: currentData, Version: currentVersion}); err != nil {
		writeVideoStudioFileError(w, err)
		return
	}
	if manager.projectHasActiveJobs(projectID, "probe", "generate", "reverse", "still") {
		writeVideoStudioError(w, http.StatusConflict, "version_restore_dirty", "Media is still being imported. Restore a version when the import has finished.")
		return
	}
	_, data, found, err := readVideoStudioVersion(r.Context(), svc, projectID, versionID)
	if err != nil {
		writeVideoStudioError(w, http.StatusInternalServerError, "version_restore_failed", "The version could not be read.")
		return
	}
	if !found {
		writeVideoStudioError(w, http.StatusNotFound, "version_not_found", "Version not found.")
		return
	}
	restored, err := decodeVideoStudioDocument(data)
	if err != nil {
		writeVideoStudioError(w, http.StatusInternalServerError, "version_restore_failed", "The version could not be read.")
		return
	}
	restored, err = manager.canonicalizeProject(projectID, restored)
	if err != nil {
		writeVideoStudioError(w, http.StatusConflict, "project_asset_invalid", "This version uses media that was removed.")
		return
	}
	if err := videostudio.Validate(restored); err != nil {
		writeVideoStudioError(w, http.StatusBadRequest, "invalid_project", "The version does not meet the supported editing limits.")
		return
	}
	encoded, err := json.Marshal(restored)
	maxProject := int64(cfg.VideoStudio.MaxProjectSizeMB) << 20
	if err != nil || int64(len(encoded)) > maxProject {
		writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "project_size_limit", "Project document exceeds the configured project limit.")
		return
	}
	// The safety copy adds the current document; replacing project.json then swaps it for the restored one. The
	// storage must hold the larger of the two states plus the index headroom.
	used, err := manager.projectStorageSize(r.Context(), svc, projectID)
	if err != nil || used+max(int64(len(currentData)), int64(len(encoded)))+videoStudioVersionIndexReserve > maxProject {
		writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "project_size_limit", "Project storage is full. Remove media or versions before restoring.")
		return
	}
	if _, err := snapshotVideoStudioVersion(r.Context(), svc, projectID, videoStudioVersionBeforeRestore, "", current.Name, currentData); err != nil {
		writeVideoStudioError(w, http.StatusInternalServerError, "version_restore_failed", "The current state could not be kept before the restore.")
		return
	}
	entry, err := svc.WriteFileBytesConditional(r.Context(), videoStudioProjectPath(projectID)+"/project.json", encoded, desktop.SourceUser, precondition)
	if err != nil {
		writeVideoStudioFileError(w, err)
		return
	}
	version := desktop.NoteVersion(encoded)
	w.Header().Set("ETag", version)
	writeVideoStudioJSON(w, http.StatusOK, videoStudioProjectItem{ID: projectID, Project: restored, DesktopPath: videoStudioProjectPath(projectID), ETag: version, UpdatedAt: entry.Modified})
}

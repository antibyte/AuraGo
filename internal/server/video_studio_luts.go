package server

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/videostudio"
)

// LUT assets and built-in looks (spec §5.7, §9, §11; plan index §4.20). This file holds:
//   - the built-in look endpoint;
//   - the .cube branch of the media import, which buffers and parses before anything is written;
//   - the LUT references that decide deletion and render staging.

// handleVideoStudioBuiltinLUT serves a built-in look as .cube text for the preview's 3D texture:
// GET /api/desktop/video-studio/luts/builtin/{name}.cube, with the same access as every other Video Studio read.
// Only registry look ids are served. The bytes come from Go (videostudio.BuiltinLookCube) and are never read from
// disk.
func handleVideoStudioBuiltinLUT(s *Server, w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeVideoStudioMethodError(w)
		return
	}
	if _, _, _, ok := videoStudioReadAccess(s, w, r); !ok {
		return
	}
	id, isCube := strings.CutSuffix(name, ".cube")
	if _, known := videostudio.LookByID(id); !isCube || !known {
		writeVideoStudioError(w, http.StatusNotFound, "lut_not_found", "LUT not found.")
		return
	}
	data, ok := videostudio.BuiltinLookCube(id)
	if !ok {
		writeVideoStudioError(w, http.StatusNotFound, "lut_not_found", "LUT not found.")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, id+".cube", time.Time{}, bytes.NewReader(data))
}

// readVideoStudioLUTUpload buffers a .cube import:
//   - A Desktop source_path passes its stat size, and a file above MaxLUTBytes is refused before reading.
//   - At most MaxLUTBytes+1 bytes are read.
//   - The table is parsed before anything is written, so a refused file never reaches the project.
//
// On a refusal it answers the request itself and returns false:
//   - 413 lut_size_limit;
//   - 400 invalid_lut with the parser's line;
//   - the upload errors of a broken stream.
func readVideoStudioLUTUpload(w http.ResponseWriter, source io.Reader, expectedSize int64) ([]byte, bool) {
	if expectedSize > videostudio.MaxLUTBytes {
		writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "lut_size_limit", "LUT files may be at most 16 MiB.")
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(source, videostudio.MaxLUTBytes+1))
	var bodyLimit *http.MaxBytesError
	switch {
	case errors.As(err, &bodyLimit):
		writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "asset_size_limit", "Media file exceeds the configured asset limit.")
		return nil, false
	case err != nil:
		writeVideoStudioError(w, http.StatusBadRequest, "media_upload_failed", "The upload was interrupted or could not be read.")
		return nil, false
	case int64(len(data)) > videostudio.MaxLUTBytes:
		writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "lut_size_limit", "LUT files may be at most 16 MiB.")
		return nil, false
	}
	if _, err := videostudio.ParseCube(bytes.NewReader(data)); err != nil {
		var lutErr *videostudio.LUTError
		if errors.As(err, &lutErr) {
			writeVideoStudioErrorFields(w, http.StatusBadRequest, "invalid_lut", "The LUT file is invalid: "+lutErr.Reason+".", map[string]any{"line": lutErr.Line})
			return nil, false
		}
		writeVideoStudioError(w, http.StatusBadRequest, "invalid_lut", "The LUT file could not be read.")
		return nil, false
	}
	return data, true
}

// videoStudioLUTCount counts a project's LUT assets. It includes .cube imports whose probe has not finished (their
// record has no kind yet), so concurrent uploads cannot pass MaxLUTAssetsPerProject. The caller holds the project
// lock.
func videoStudioLUTCount(records []videoStudioMediaRecord) int {
	count := 0
	for _, record := range records {
		if record.Asset.Kind == videostudio.AssetLUT || record.Asset.Kind == "" && videostudio.IsLUTFileName(record.Asset.Name) {
			count++
		}
	}
	return count
}

// videoStudioEffectLUTRefs returns the project LUT asset ids that lut effect entries reference:
//   - all: every entry on any clip, which keeps the asset from deletion;
//   - rendered: on entries of enabled clips on visible visual tracks, which a render stages (the rule of
//     videostudio.collectRenderSources).
//
// Built-in refs and "" name no asset.
func videoStudioEffectLUTRefs(project videostudio.Project) (all, rendered map[string]struct{}) {
	all, rendered = make(map[string]struct{}), make(map[string]struct{})
	for _, track := range project.Tracks {
		for _, clip := range track.Clips {
			for _, entry := range clip.Effects {
				if entry.ID != "lut" {
					continue
				}
				ref, _ := entry.Params["lut"].(string)
				if _, assetID, ok := videostudio.ParseLUTRef(ref); ok && assetID != "" {
					all[assetID] = struct{}{}
					if entry.On && !clip.Disabled && !track.Hidden && track.Kind != videostudio.TrackAudio {
						rendered[assetID] = struct{}{}
					}
				}
			}
		}
	}
	return all, rendered
}

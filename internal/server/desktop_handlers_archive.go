package server

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/desktop"
)

func handleDesktopArchive(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		var body struct {
			Paths []string `json:"paths"`
			Dest  string   `json:"dest"`
		}
		if err := decodeDesktopJSON(w, r, &body, 10*1024*1024); err != nil { // 10MB limit
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if len(body.Paths) == 0 || body.Dest == "" {
			jsonError(w, "Missing paths or dest", http.StatusBadRequest)
			return
		}

		destResolved, err := svc.ResolvePath(body.Dest)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Ensure target directory exists
		if err := os.MkdirAll(filepath.Dir(destResolved), 0o755); err != nil {
			jsonError(w, fmt.Sprintf("Failed to create target directory: %v", err), http.StatusInternalServerError)
			return
		}

		zipFile, err := os.Create(destResolved)
		if err != nil {
			jsonError(w, fmt.Sprintf("Failed to create zip file: %v", err), http.StatusInternalServerError)
			return
		}
		defer zipFile.Close()

		zipWriter := zip.NewWriter(zipFile)
		defer zipWriter.Close()

		for _, srcPath := range body.Paths {
			srcResolved, err := svc.ResolvePath(srcPath)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}

			_, err = os.Stat(srcResolved)
			if err != nil {
				jsonError(w, fmt.Sprintf("Source file not found: %s", srcPath), http.StatusBadRequest)
				return
			}

			baseDir := filepath.Dir(srcResolved)

			err = filepath.Walk(srcResolved, func(path string, f os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if f.IsDir() {
					return nil
				}

				rel, err := filepath.Rel(baseDir, path)
				if err != nil {
					return err
				}

				// standard zip files use forward slashes
				rel = filepath.ToSlash(rel)

				header, err := zip.FileInfoHeader(f)
				if err != nil {
					return err
				}

				header.Name = rel
				header.Method = zip.Deflate

				writer, err := zipWriter.CreateHeader(header)
				if err != nil {
					return err
				}

				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()

				_, err = io.Copy(writer, file)
				return err
			})

			if err != nil {
				jsonError(w, fmt.Sprintf("Failed to add file to zip: %v", err), http.StatusInternalServerError)
				return
			}
		}

		// Broadcast desktop changed event
		event := desktop.Event{
			Type:      "desktop_changed",
			Payload:   map[string]interface{}{"operation": "create_file", "path": body.Dest},
			CreatedAt: time.Now().UTC(),
		}
		broadcastDesktopEvent(s, hub, event)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}
}

func handleDesktopExtract(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		var body struct {
			Path string `json:"path"`
			Dest string `json:"dest"`
		}
		if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if body.Path == "" || body.Dest == "" {
			jsonError(w, "Missing path or dest", http.StatusBadRequest)
			return
		}

		srcResolved, err := svc.ResolvePath(body.Path)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		destResolved, err := svc.ResolvePath(body.Dest)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		reader, err := zip.OpenReader(srcResolved)
		if err != nil {
			jsonError(w, fmt.Sprintf("Failed to open zip file: %v", err), http.StatusBadRequest)
			return
		}
		defer reader.Close()

		// Extraction
		for _, f := range reader.File {
			fpath := filepath.Join(destResolved, f.Name)
			if !strings.HasPrefix(filepath.Clean(fpath), filepath.Clean(destResolved)) {
				jsonError(w, fmt.Sprintf("Illegal file path in zip: %s", f.Name), http.StatusBadRequest)
				return
			}

			if f.FileInfo().IsDir() {
				_ = os.MkdirAll(fpath, os.ModePerm)
				continue
			}

			if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				jsonError(w, fmt.Sprintf("Failed to create folder: %v", err), http.StatusInternalServerError)
				return
			}

			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				jsonError(w, fmt.Sprintf("Failed to create file: %v", err), http.StatusInternalServerError)
				return
			}

			rc, err := f.Open()
			if err != nil {
				outFile.Close()
				jsonError(w, fmt.Sprintf("Failed to open zip entry: %v", err), http.StatusInternalServerError)
				return
			}

			_, err = io.Copy(outFile, rc)
			outFile.Close()
			rc.Close()

			if err != nil {
				jsonError(w, fmt.Sprintf("Failed to write file: %v", err), http.StatusInternalServerError)
				return
			}
		}

		// Broadcast desktop changed event
		event := desktop.Event{
			Type:      "desktop_changed",
			Payload:   map[string]interface{}{"operation": "extract_zip", "path": body.Dest},
			CreatedAt: time.Now().UTC(),
		}
		broadcastDesktopEvent(s, hub, event)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}
}

func handleDesktopArchiveList(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		zipPath := r.URL.Query().Get("path")
		if zipPath == "" {
			jsonError(w, "Missing path parameter", http.StatusBadRequest)
			return
		}

		srcResolved, err := svc.ResolvePath(zipPath)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		reader, err := zip.OpenReader(srcResolved)
		if err != nil {
			jsonError(w, fmt.Sprintf("Failed to open zip file: %v", err), http.StatusBadRequest)
			return
		}
		defer reader.Close()

		type zipEntry struct {
			Name           string `json:"name"`
			Size           int64  `json:"size"`
			CompressedSize int64  `json:"compressed_size"`
			IsDir          bool   `json:"is_dir"`
			ModTime        string `json:"mod_time"`
		}

		entries := make([]zipEntry, 0, len(reader.File))
		for _, f := range reader.File {
			entries = append(entries, zipEntry{
				Name:           f.Name,
				Size:           int64(f.UncompressedSize64),
				CompressedSize: int64(f.CompressedSize64),
				IsDir:          f.FileInfo().IsDir(),
				ModTime:        f.Modified.Format(time.RFC3339),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"entries": entries})
	}
}

func handleDesktopBatchRename(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		var body struct {
			Operations []struct {
				OldPath string `json:"old_path"`
				NewName string `json:"new_name"`
			} `json:"operations"`
		}
		if err := decodeDesktopJSON(w, r, &body, 10*1024*1024); err != nil { // 10MB limit
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		for _, op := range body.Operations {
			if op.OldPath == "" || op.NewName == "" {
				jsonError(w, "Invalid rename operation parameters", http.StatusBadRequest)
				return
			}

			cleanName := filepath.Base(filepath.Clean(op.NewName))
			if cleanName == "." || cleanName == ".." || cleanName == "/" || cleanName == "" {
				jsonError(w, "Invalid destination filename", http.StatusBadRequest)
				return
			}

			dir := filepath.Dir(op.OldPath)
			newPath := filepath.ToSlash(filepath.Join(dir, cleanName))

			err := svc.MovePath(r.Context(), op.OldPath, newPath, desktop.SourceUser)
			if err != nil {
				jsonError(w, fmt.Sprintf("Failed to rename %s to %s: %v", op.OldPath, cleanName, err), http.StatusBadRequest)
				return
			}

			event := desktop.Event{
				Type:      "desktop_changed",
				Payload:   map[string]interface{}{"operation": "move_path", "old_path": op.OldPath, "new_path": newPath},
				CreatedAt: time.Now().UTC(),
			}
			broadcastDesktopEvent(s, hub, event)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}
}

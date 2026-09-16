package gamemaker

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) WriteExport(ctx context.Context, projectID string, output io.Writer) (string, error) {
	project, err := s.GetProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	if project.CurrentRevision <= 0 {
		return "", fmt.Errorf("project has no playable revision")
	}
	// The workspace can be edited in Code Studio or replaced by publication.
	// Export one immutable published revision, including its original runtimes.
	rows, err := s.db.QueryContext(ctx, `SELECT f.path,f.content_hash,f.size
		FROM gm_revision_files f JOIN gm_revisions r ON r.id=f.revision_id
		WHERE r.project_id=? AND r.number=? ORDER BY f.path`, project.ID, project.CurrentRevision)
	if err != nil {
		return "", fmt.Errorf("read exported revision: %w", err)
	}
	defer rows.Close()
	var files []revisionFile
	included := make(map[string]bool)
	for rows.Next() {
		var file revisionFile
		if err := rows.Scan(&file.Path, &file.Hash, &file.Size); err != nil {
			return "", fmt.Errorf("read exported revision file: %w", err)
		}
		rel, err := safeRelativePath(file.Path, true)
		if err != nil || rel != file.Path {
			return "", fmt.Errorf("%w: non-canonical exported revision path", ErrInvalidPath)
		}
		if strings.HasPrefix(rel, ".") || strings.Contains(rel, "/.") {
			continue
		}
		hash, err := hex.DecodeString(file.Hash)
		if err != nil || len(hash) != sha256.Size || file.Size < 0 {
			return "", fmt.Errorf("invalid exported revision metadata for %s", rel)
		}
		included[rel] = true
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("read exported revision files: %w", err)
	}
	if err := rows.Close(); err != nil {
		return "", fmt.Errorf("close exported revision files: %w", err)
	}
	for _, required := range []string{"game.json", "index.html", "src/main.ts", "dist/game.js"} {
		if !included[required] {
			return "", fmt.Errorf("published revision is incomplete: missing %s", required)
		}
	}
	archive := zip.NewWriter(output)
	writeFile := func(name string, data []byte) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o644)
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("create export entry %s: %w", name, err)
		}
		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf("write export entry %s: %w", name, err)
		}
		return nil
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		data, err := os.ReadFile(filepath.Join(s.blobDir, file.Hash[:2], file.Hash))
		if err != nil {
			// Only the project-relative file name belongs in public diagnostics.
			var pathErr *os.PathError
			if errors.As(err, &pathErr) {
				err = pathErr.Err
			}
			return "", fmt.Errorf("read exported revision file %s: %w", file.Path, err)
		}
		sum := sha256.Sum256(data)
		if int64(len(data)) != file.Size || hex.EncodeToString(sum[:]) != file.Hash {
			return "", fmt.Errorf("exported revision integrity check failed for %s", file.Path)
		}
		if err := writeFile(file.Path, data); err != nil {
			return "", err
		}
	}
	for _, asset := range bundledRuntimeAssets(project.Dimension) {
		if included[asset.projectPath] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		data, err := runtimeFS.ReadFile(asset.embeddedPath)
		if err != nil {
			return "", fmt.Errorf("read exported game runtime %s: %w", asset.embeddedPath, err)
		}
		if err := writeFile(asset.projectPath, data); err != nil {
			return "", err
		}
	}
	if !included["README-AuraGo.txt"] {
		if err := writeFile("README-AuraGo.txt", []byte(exportInstructions)); err != nil {
			return "", err
		}
	}
	if err := archive.Close(); err != nil {
		return "", fmt.Errorf("finish game maker export: %w", err)
	}
	return project.Slug + ".zip", nil
}

const exportInstructions = `PLAYING THIS GAME / SPIEL STARTEN

Extract the complete ZIP, keeping its directory structure.
The game is already compiled. No AuraGo, npm, build step or internet access is
needed. Serve this folder with a static HTTP server. Do not double-click
index.html (file://): browsers block JavaScript modules and asset loading there.

For example, if Python 3 is installed, open a terminal in this folder and run:
  Windows:       py -3 -m http.server 8000 --bind 127.0.0.1
  Linux / macOS: python3 -m http.server 8000 --bind 127.0.0.1
Open http://127.0.0.1:8000/ in your browser. Stop the server with Ctrl+C.
Any other static HTTP(S) host also works; upload the whole folder, not only dist/.
Sound starts after a click or key press, as required by the browser.

Die ZIP vollstaendig entpacken. Das Spiel ist bereits kompiliert und benoetigt
weder AuraGo noch einen Build oder Internetzugang. Den Ordner ueber einen lokalen
Webserver wie oben starten und http://127.0.0.1:8000/ oeffnen. index.html nicht per
Doppelklick starten: file:// blockiert Module und das Laden von Spieldateien.

This bundle contains the last published revision. Unpublished edits, private
planning/history, validation reports and the Studio test driver are excluded.
See THIRD_PARTY_NOTICES.md and the licenses accompanying the imported assets.
`

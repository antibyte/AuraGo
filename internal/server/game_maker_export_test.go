package server

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/gamemaker"
)

func TestGameMakerExportFailureIsNotSuccessfulDownload(t *testing.T) {
	root := t.TempDir()
	svc, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "gm.db"), WorkspacePath: filepath.Join(root, "workspace"), Enabled: true, AllowCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	project, err := svc.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Unfinished", Dimension: "2d", Description: "Export must reject an unpublished game."})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: &config.Config{}, GameMaker: svc}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/game-maker/projects/"+project.ID+"/export", nil)
	handleGameMakerExport(w, r, s, project.ID)
	if w.Code < 400 || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") || w.Header().Get("Content-Disposition") != "" {
		t.Fatalf("export error became a successful ZIP download: status=%d headers=%v", w.Code, w.Header())
	}
	if !strings.Contains(w.Body.String(), "playable revision") {
		t.Fatalf("missing useful export error: %s", w.Body.String())
	}
}

func TestGameMakerExportDownloadAndLateFailure(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "gm.db")
	svc, err := gamemaker.NewService(gamemaker.Options{DBPath: dbPath, WorkspacePath: filepath.Join(root, "workspace"), Enabled: true, AllowCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	p, err := svc.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Published", Dimension: "2d", Description: "HTTP export fixture"})
	if err != nil {
		t.Fatal(err)
	}
	// Seed a published snapshot, not a live project tree. Service-level tests
	// exercise publication; this fixture verifies the actual HTTP response.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	result, err := db.Exec(`INSERT INTO gm_revisions(project_id,number,source,summary,file_count,total_bytes,created_at) VALUES(?,1,'test','export',4,0,CURRENT_TIMESTAMP)`, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	revisionID, _ := result.LastInsertId()
	var lastBlob string
	for _, name := range []string{"game.json", "index.html", "dist/game.js", "src/main.ts"} {
		data := []byte("export fixture: " + name)
		hash := sha256.Sum256(data)
		key := hex.EncodeToString(hash[:])
		lastBlob = filepath.Join(root, "game_maker_blobs", key[:2], key)
		if err := os.MkdirAll(filepath.Dir(lastBlob), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lastBlob, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO gm_revision_files(revision_id,path,content_hash,size) VALUES(?,?,?,?)`, revisionID, name, key, len(data)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE gm_projects SET current_revision=1 WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: &config.Config{}, GameMaker: svc}
	download := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handleGameMakerExport(w, httptest.NewRequest(http.MethodGet, "/export", nil), s, p.ID)
		return w
	}
	w := download()
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/zip" || !strings.Contains(w.Header().Get("Content-Disposition"), "published.zip") {
		t.Fatalf("export response: %d %v %s", w.Code, w.Header(), w.Body.String())
	}
	if w.Header().Get("Content-Length") != strconv.Itoa(w.Body.Len()) {
		t.Fatal("download length is not the completed archive size")
	}
	z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range z.File {
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(io.Discard, r)
		r.Close()
		if err != nil {
			t.Fatalf("download ZIP checksum %s: %v", file.Name, err)
		}
	}
	// The last source entry fails after several files have already been zipped.
	if err := os.WriteFile(lastBlob, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	w = download()
	if w.Code < 400 || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") || w.Header().Get("Content-Disposition") != "" || !strings.Contains(w.Body.String(), "integrity check") {
		t.Fatalf("late failure exposed a partial download: %d %v %s", w.Code, w.Header(), w.Body.String())
	}
}

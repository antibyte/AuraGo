package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/invasion"
	"aurago/internal/security"
)

func TestHandleInvasionArtifactUploadCompletesLocalArtifact(t *testing.T) {
	dataDir := t.TempDir()
	db, err := invasion.InitDB(filepath.Join(dataDir, "invasion.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	eggID, err := invasion.CreateEgg(db, invasion.EggRecord{Name: "Reporter", Active: true})
	if err != nil {
		t.Fatalf("CreateEgg: %v", err)
	}
	nestID, err := invasion.CreateNest(db, invasion.NestRecord{Name: "Nest", Active: true, EggID: eggID})
	if err != nil {
		t.Fatalf("CreateNest: %v", err)
	}
	sharedKey := strings.Repeat("a", 64)
	vault, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(dataDir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if err := vault.WriteSecret("egg_shared_"+nestID, sharedKey); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}

	sum := sha256.Sum256([]byte("hello"))
	token, artifact, err := invasion.CreateArtifactUpload(db, invasion.ArtifactUploadRequest{
		NestID:         nestID,
		EggID:          eggID,
		Filename:       "hello.txt",
		MIMEType:       "text/plain",
		ExpectedSize:   5,
		ExpectedSHA256: hex.EncodeToString(sum[:]),
		TTL:            time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateArtifactUpload: %v", err)
	}

	s := &Server{
		Cfg:        &config.Config{},
		InvasionDB: db,
		Vault:      vault,
		Logger:     slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil)),
	}
	s.Cfg.Directories.DataDir = dataDir

	req := httptest.NewRequest(http.MethodPost, "/api/invasion/artifacts/upload/"+token, strings.NewReader("hello"))
	signEggRequest(t, req, sharedKey, nil)
	req.Header.Set("X-AuraGo-Nest-ID", nestID)
	req.Header.Set("X-AuraGo-Egg-ID", eggID)
	rec := httptest.NewRecorder()
	handleInvasionArtifactUpload(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	completed, err := invasion.GetArtifact(db, artifact.ID)
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if completed.Status != invasion.ArtifactStatusCompleted {
		t.Fatalf("artifact status = %q, want completed", completed.Status)
	}
	if !strings.Contains(completed.StoragePath, filepath.Join("invasion_artifacts", nestID, artifact.ID)) {
		t.Fatalf("storage_path = %q", completed.StoragePath)
	}
}

func TestHandleInvasionArtifactUploadRejectsUnsignedUpload(t *testing.T) {
	dataDir := t.TempDir()
	db, err := invasion.InitDB(filepath.Join(dataDir, "invasion.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	sum := sha256.Sum256([]byte("hello"))
	token, _, err := invasion.CreateArtifactUpload(db, invasion.ArtifactUploadRequest{
		NestID:         "nest-1",
		EggID:          "egg-1",
		Filename:       "hello.txt",
		ExpectedSize:   5,
		ExpectedSHA256: hex.EncodeToString(sum[:]),
		TTL:            time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateArtifactUpload: %v", err)
	}

	s := &Server{
		Cfg:        &config.Config{},
		InvasionDB: db,
		Logger:     slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil)),
	}
	s.Cfg.Directories.DataDir = dataDir

	req := httptest.NewRequest(http.MethodPost, "/api/invasion/artifacts/upload/"+token, strings.NewReader("hello"))
	rec := httptest.NewRecorder()
	handleInvasionArtifactUpload(s)(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestHandleInvasionArtifactOfferAuthenticatesEggAndReturnsUploadToken(t *testing.T) {
	dataDir := t.TempDir()
	db, err := invasion.InitDB(filepath.Join(dataDir, "invasion.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	eggID, err := invasion.CreateEgg(db, invasion.EggRecord{Name: "Reporter", Active: true})
	if err != nil {
		t.Fatalf("CreateEgg: %v", err)
	}
	nestID, err := invasion.CreateNest(db, invasion.NestRecord{Name: "Nest", Active: true, EggID: eggID})
	if err != nil {
		t.Fatalf("CreateNest: %v", err)
	}
	sharedKey := strings.Repeat("a", 64)
	vault, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(dataDir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if err := vault.WriteSecret("egg_shared_"+nestID, sharedKey); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}

	s := &Server{
		Cfg:        &config.Config{},
		InvasionDB: db,
		Vault:      vault,
		Logger:     slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil)),
	}
	s.Cfg.Directories.DataDir = dataDir

	body, _ := json.Marshal(map[string]interface{}{
		"filename":        "report.txt",
		"mime_type":       "text/plain",
		"expected_size":   12,
		"expected_sha256": strings.Repeat("c", 64),
		"mission_id":      "mission-1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/invasion/artifacts/offer", bytes.NewReader(body))
	signEggRequest(t, req, sharedKey, body)
	req.Header.Set("X-AuraGo-Nest-ID", nestID)
	req.Header.Set("X-AuraGo-Egg-ID", eggID)

	rec := httptest.NewRecorder()
	handleInvasionArtifactOffer(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}
	if payload["upload_token"] == "" || payload["artifact_id"] == "" {
		t.Fatalf("missing upload response fields: %v", payload)
	}
}

func signEggRequest(t *testing.T, req *http.Request, sharedKey string, body []byte) {
	t.Helper()
	ts := time.Now().UTC().Format(time.RFC3339)
	key, err := hex.DecodeString(sharedKey)
	if err != nil {
		t.Fatalf("DecodeString: %v", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(req.Method))
	mac.Write([]byte("\n"))
	mac.Write([]byte(req.URL.Path))
	mac.Write([]byte("\n"))
	mac.Write([]byte(ts))
	mac.Write([]byte("\n"))
	mac.Write(body)
	req.Header.Set("X-AuraGo-Timestamp", ts)
	req.Header.Set("X-AuraGo-Signature", hex.EncodeToString(mac.Sum(nil)))
}

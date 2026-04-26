package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/invasion"
	"aurago/internal/tools"
)

const maxEggArtifactOfferBytes = 1 << 20

func handleInvasionArtifactOffer(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := readLimitedRequestBody(r, maxEggArtifactOfferBytes)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		nestID, eggID, err := verifyEggSignedRequest(s, r, body)
		if err != nil {
			jsonError(w, err.Error(), http.StatusUnauthorized)
			return
		}
		var req struct {
			MissionID      string                 `json:"mission_id"`
			TaskID         string                 `json:"task_id"`
			Filename       string                 `json:"filename"`
			MIMEType       string                 `json:"mime_type"`
			ExpectedSize   int64                  `json:"expected_size"`
			ExpectedSHA256 string                 `json:"expected_sha256"`
			Metadata       map[string]interface{} `json:"metadata"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		metadataJSON := ""
		if req.Metadata != nil {
			if b, err := json.Marshal(req.Metadata); err == nil {
				metadataJSON = string(b)
			}
		}
		token, artifact, err := invasion.CreateArtifactUpload(s.InvasionDB, invasion.ArtifactUploadRequest{
			NestID:         nestID,
			EggID:          eggID,
			MissionID:      req.MissionID,
			TaskID:         req.TaskID,
			Filename:       req.Filename,
			MIMEType:       req.MIMEType,
			ExpectedSize:   req.ExpectedSize,
			ExpectedSHA256: req.ExpectedSHA256,
			MetadataJSON:   metadataJSON,
			TTL:            15 * time.Minute,
		})
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to create artifact upload", "Artifact offer rejected", err, "nest_id", nestID, "egg_id", eggID)
			return
		}
		writeJSON(w, map[string]interface{}{
			"status":       "ready",
			"artifact_id":  artifact.ID,
			"upload_token": token,
			"upload_url":   "/api/invasion/artifacts/upload/" + token,
			"web_path":     artifact.WebPath,
			"expires_in":   900,
		})
	}
}

func handleInvasionArtifactUpload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		token := strings.TrimPrefix(r.URL.Path, "/api/invasion/artifacts/upload/")
		token = strings.Trim(token, "/")
		if token == "" {
			jsonError(w, "Missing upload token", http.StatusBadRequest)
			return
		}
		nestID, eggID, err := verifyEggSignedRequest(s, r, nil)
		if err != nil {
			jsonError(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if r.ContentLength > invasion.MaxArtifactSizeBytes {
			jsonError(w, fmt.Sprintf("artifact exceeds maximum size %d", invasion.MaxArtifactSizeBytes), http.StatusRequestEntityTooLarge)
			return
		}
		slot, err := invasion.GetArtifactUploadByToken(s.InvasionDB, token, time.Now())
		if err != nil {
			jsonError(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if slot.Artifact.NestID != nestID || slot.Artifact.EggID != eggID {
			jsonError(w, "upload token does not belong to authenticated egg", http.StatusUnauthorized)
			return
		}
		slot, err = invasion.ClaimArtifactUploadByToken(s.InvasionDB, token, time.Now())
		if err != nil {
			jsonError(w, err.Error(), http.StatusUnauthorized)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, invasion.MaxArtifactSizeBytes)
		storage := invasion.NewArtifactStorage(filepath.Join(s.Cfg.Directories.DataDir, "invasion_artifacts"))
		stored, err := storage.Save(slot.Artifact, r.Body, slot.ExpectedSize, slot.ExpectedSHA256)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to store artifact", "Artifact upload failed", err, "artifact_id", slot.Artifact.ID)
			return
		}
		if err := invasion.CompleteArtifactUpload(s.InvasionDB, token, stored.Path, stored.SizeBytes, stored.SHA256, time.Now()); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to complete artifact", "Artifact completion failed", err, "artifact_id", slot.Artifact.ID)
			return
		}
		s.Logger.Info("Egg artifact uploaded", "nest_id", slot.Artifact.NestID, "egg_id", slot.Artifact.EggID, "artifact_id", slot.Artifact.ID, "bytes", stored.SizeBytes)
		writeJSON(w, map[string]interface{}{
			"status":      "completed",
			"artifact_id": slot.Artifact.ID,
			"size_bytes":  stored.SizeBytes,
			"sha256":      stored.SHA256,
			"web_path":    slot.Artifact.WebPath,
		})
	}
}

func handleInvasionArtifactDownload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/invasion/artifacts/")
		id = strings.TrimSuffix(id, "/download")
		id = strings.Trim(id, "/")
		if id == "" {
			jsonError(w, "Missing artifact ID", http.StatusBadRequest)
			return
		}
		artifact, err := invasion.GetArtifact(s.InvasionDB, id)
		if err != nil {
			jsonError(w, "Artifact not found", http.StatusNotFound)
			return
		}
		if artifact.Status != invasion.ArtifactStatusCompleted || artifact.StoragePath == "" {
			jsonError(w, "Artifact is not available", http.StatusConflict)
			return
		}
		if _, err := os.Stat(artifact.StoragePath); err != nil {
			if os.IsNotExist(err) {
				jsonLoggedError(w, s.Logger, http.StatusNotFound, "Artifact file missing", "Artifact file missing", err, "artifact_id", artifact.ID, "path", artifact.StoragePath)
				return
			}
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to access artifact file", "Artifact stat failed", err, "artifact_id", artifact.ID)
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", artifact.Filename))
		if artifact.MIMEType != "" {
			w.Header().Set("Content-Type", artifact.MIMEType)
		}
		http.ServeFile(w, r, artifact.StoragePath)
	}
}

func handleInvasionEggMessage(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := readLimitedRequestBody(r, maxEggArtifactOfferBytes)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		nestID, eggID, err := verifyEggSignedRequest(s, r, body)
		if err != nil {
			jsonError(w, err.Error(), http.StatusUnauthorized)
			return
		}
		var req struct {
			MissionID       string   `json:"mission_id"`
			TaskID          string   `json:"task_id"`
			Severity        string   `json:"severity"`
			Title           string   `json:"title"`
			Body            string   `json:"body"`
			ArtifactIDs     []string `json:"artifact_ids"`
			DedupKey        string   `json:"dedup_key"`
			WakeupRequested bool     `json:"wakeup_requested"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		msg, err := invasion.RecordEggMessage(s.InvasionDB, invasion.EggMessageRecord{
			NestID:          nestID,
			EggID:           eggID,
			MissionID:       req.MissionID,
			TaskID:          req.TaskID,
			Severity:        req.Severity,
			Title:           req.Title,
			Body:            req.Body,
			ArtifactIDs:     req.ArtifactIDs,
			DedupKey:        req.DedupKey,
			WakeupRequested: req.WakeupRequested,
		}, invasion.EggMessageRatePolicy{Burst: 3, Window: time.Minute}, time.Now())
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to record egg message", "Egg message rejected", err, "nest_id", nestID, "egg_id", eggID)
			return
		}
		if msg.WakeupAllowed {
			s.scheduleEggMessageWakeup(msg)
		}
		writeJSON(w, map[string]interface{}{
			"status":         "stored",
			"message_id":     msg.ID,
			"wakeup_allowed": msg.WakeupAllowed,
		})
	}
}

func (s *Server) scheduleEggMessageWakeup(msg invasion.EggMessageRecord) {
	if s.BackgroundTasks == nil {
		return
	}
	prompt := fmt.Sprintf("An Egg sent a rate-limited message.\n\n<external_data>\nmessage_id: %s\nnest_id: %s\negg_id: %s\nseverity: %s\ntitle: %s\nbody: %s\nartifact_ids: %s\n</external_data>\n\nInspect the message and related artifacts with invasion_control before responding or acting.",
		msg.ID, msg.NestID, msg.EggID, msg.Severity, msg.Title, msg.Body, strings.Join(msg.ArtifactIDs, ","))
	_, err := s.BackgroundTasks.ScheduleFollowUp(prompt, tools.BackgroundTaskScheduleOptions{
		Source:      "invasion_egg_message",
		Description: "Egg message: " + msg.Title,
		Delay:       0,
		Timeout:     5 * time.Minute,
	})
	if err != nil && s.Logger != nil {
		s.Logger.Warn("Failed to schedule egg message wakeup", "message_id", msg.ID, "error", err)
	}
}

func verifyEggSignedRequest(s *Server, r *http.Request, body []byte) (string, string, error) {
	nestID := strings.TrimSpace(r.Header.Get("X-AuraGo-Nest-ID"))
	eggID := strings.TrimSpace(r.Header.Get("X-AuraGo-Egg-ID"))
	ts := strings.TrimSpace(r.Header.Get("X-AuraGo-Timestamp"))
	sig := strings.TrimSpace(r.Header.Get("X-AuraGo-Signature"))
	if nestID == "" || eggID == "" || ts == "" || sig == "" {
		return "", "", fmt.Errorf("missing egg authentication headers")
	}
	if s.Vault == nil {
		return "", "", fmt.Errorf("vault is unavailable")
	}
	nest, err := invasion.GetNest(s.InvasionDB, nestID)
	if err != nil {
		return "", "", err
	}
	if nest.EggID != "" && nest.EggID != eggID {
		return "", "", fmt.Errorf("egg is not assigned to nest")
	}
	when, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return "", "", fmt.Errorf("invalid timestamp")
	}
	if delta := time.Since(when); delta > 5*time.Minute || delta < -5*time.Minute {
		return "", "", fmt.Errorf("timestamp outside allowed window")
	}
	sharedKey, err := s.Vault.ReadSecret("egg_shared_" + nestID)
	if err != nil {
		return "", "", fmt.Errorf("shared key unavailable")
	}
	key, err := hex.DecodeString(sharedKey)
	if err != nil {
		return "", "", fmt.Errorf("shared key is invalid")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(r.Method))
	mac.Write([]byte("\n"))
	mac.Write([]byte(r.URL.Path))
	mac.Write([]byte("\n"))
	mac.Write([]byte(ts))
	mac.Write([]byte("\n"))
	mac.Write(body)
	expected := mac.Sum(nil)
	got, err := hex.DecodeString(sig)
	if err != nil || !hmac.Equal(got, expected) {
		return "", "", fmt.Errorf("invalid egg signature")
	}
	return nestID, eggID, nil
}

func readLimitedRequestBody(r *http.Request, maxBytes int64) ([]byte, error) {
	var buf bytes.Buffer
	limited := io.LimitReader(r.Body, maxBytes+1)
	if _, err := buf.ReadFrom(limited); err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	if int64(buf.Len()) > maxBytes {
		return nil, fmt.Errorf("request body too large")
	}
	return buf.Bytes(), nil
}

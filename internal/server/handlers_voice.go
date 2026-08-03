package server

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/speechlab"
	"aurago/internal/telegram"
	"aurago/internal/tools"
)

// VoiceUploadResponse represents the response from voice upload endpoint
type VoiceUploadResponse struct {
	Success            string `json:"success"`
	Transcription      string `json:"transcription"`
	Duration           int    `json:"duration"`
	SpeechLabTurnToken string `json:"speech_lab_turn_token,omitempty"`
}

// handleVoiceUpload receives audio recordings and transcribes them
func handleVoiceUpload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := s.ConfigSnapshot()
		if cfg != nil && cfg.SpeechLab.Active() && cfg.SpeechLab.ChatInputEnabled {
			handleSpeechLabVoiceUpload(s, w, r)
			return
		}

		// Limit upload size to 50MB
		if err := r.ParseMultipartForm(50 << 20); err != nil {
			s.Logger.Error("Failed to parse multipart form", "error", err)
			jsonError(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		// Get the audio file
		file, header, err := r.FormFile("audio")
		if err != nil {
			s.Logger.Error("Failed to get audio file", "error", err)
			jsonError(w, "No audio file uploaded", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Validate MIME type
		contentType := header.Header.Get("Content-Type")
		allowedTypes := []string{
			"audio/webm",
			"audio/ogg",
			"audio/wav",
			"audio/mp4",
			"audio/mpeg",
		}

		isValidType := false
		for _, t := range allowedTypes {
			if strings.HasPrefix(contentType, t) {
				isValidType = true
				break
			}
		}

		if !isValidType {
			s.Logger.Warn("Invalid audio MIME type", "type", contentType)
			jsonError(w, "Invalid audio format", http.StatusBadRequest)
			return
		}

		// Create temp directory for processing
		tempDir := filepath.Join(os.TempDir(), "aurago-voice")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			s.Logger.Error("Failed to create temp dir", "error", err)
			jsonError(w, "Server error", http.StatusInternalServerError)
			return
		}

		// Generate unique filename
		timestamp := time.Now().UnixNano()
		inputPath := filepath.Join(tempDir, fmt.Sprintf("voice_%d.webm", timestamp))
		outputPath := filepath.Join(tempDir, fmt.Sprintf("voice_%d.mp3", timestamp))

		// Save the uploaded file
		inputFile, err := os.Create(inputPath)
		if err != nil {
			s.Logger.Error("Failed to create input file", "error", err)
			jsonError(w, "Server error", http.StatusInternalServerError)
			return
		}

		if _, err := io.Copy(inputFile, file); err != nil {
			inputFile.Close()
			os.Remove(inputPath)
			s.Logger.Error("Failed to save audio file", "error", err)
			jsonError(w, "Failed to save audio", http.StatusInternalServerError)
			return
		}
		inputFile.Close()

		// Clean up temp files after processing
		defer func() {
			os.Remove(inputPath)
			os.Remove(outputPath)
		}()

		// Convert to MP3 using ffmpeg (same as Telegram voice processing)
		if err := telegram.ConvertOggToMp3(inputPath, outputPath); err != nil {
			// If conversion fails, try using the original file
			s.Logger.Warn("MP3 conversion failed, trying original format", "error", err)
			outputPath = inputPath
		}

		// Transcribe the application-owned temporary file in memory. The
		// agent-facing file API intentionally accepts workspace paths only.
		audioData, err := os.ReadFile(outputPath)
		if err != nil {
			s.Logger.Error("Failed to read audio for transcription", "error", err)
			jsonError(w, "Transcription failed", http.StatusInternalServerError)
			return
		}
		transcription, _, err := tools.TranscribeAudio(r.Context(), filepath.Base(outputPath), audioData, s.Cfg)
		if err != nil {
			s.Logger.Error("Transcription failed", "error", err)
			jsonError(w, "Transcription failed", http.StatusInternalServerError)
			return
		}

		s.Logger.Info("Voice transcription successful",
			"transcription_length", len(transcription),
			"content_type", contentType)

		// Return transcription
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VoiceUploadResponse{
			Success:       "true",
			Transcription: transcription,
			Duration:      0, // Could extract from ffmpeg if needed
		})
	}
}

func handleSpeechLabVoiceUpload(s *Server, w http.ResponseWriter, r *http.Request) {
	if s == nil || s.SpeechLab == nil {
		jsonError(w, "Speech Lab is unavailable", http.StatusServiceUnavailable)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, speechlab.MaxASRBytes+64*1024)
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		jsonError(w, "Expected a multipart WAV upload", http.StatusBadRequest)
		return
	}
	reader, err := r.MultipartReader()
	if err != nil {
		jsonError(w, "Invalid multipart upload", http.StatusBadRequest)
		return
	}
	var wav []byte
	for {
		part, nextErr := reader.NextPart()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			jsonError(w, "Invalid multipart upload", http.StatusBadRequest)
			return
		}
		if part.FormName() != "audio" {
			_ = part.Close()
			continue
		}
		if !isWAVMediaType(part.Header.Get("Content-Type")) {
			_ = part.Close()
			jsonError(w, "Speech Lab chat input requires WAV audio", http.StatusBadRequest)
			return
		}
		wav, err = io.ReadAll(io.LimitReader(part, speechlab.MaxASRBytes+1))
		_ = part.Close()
		if err != nil || len(wav) > speechlab.MaxASRBytes {
			jsonError(w, "Speech Lab audio exceeds 8 MiB", http.StatusRequestEntityTooLarge)
			return
		}
		break
	}
	duration, err := speechlab.PCM16WAVDuration(wav)
	if err != nil {
		jsonError(w, "Speech Lab chat input requires mono PCM16 WAV at 16 kHz", http.StatusBadRequest)
		return
	}
	if duration > 120*time.Second {
		jsonError(w, "Speech Lab chat input is limited to 120 seconds", http.StatusRequestEntityTooLarge)
		return
	}
	ready, err := s.SpeechLab.Require(r.Context(), true, false)
	if err != nil {
		writeSpeechLabError(w, err)
		return
	}
	result, err := s.SpeechLab.Transcribe(r.Context(), wav, s.ConfigSnapshot().SpeechLab.Language, ready.ASRID)
	if err != nil {
		writeSpeechLabError(w, err)
		return
	}
	if s.Logger != nil {
		s.Logger.Info("Speech Lab chat transcription succeeded", "transcription_length", len(result.Text), "asr_id", result.ASRID)
	}
	sessionID := strings.TrimSpace(r.Header.Get("X-Session-ID"))
	if sessionID == "" {
		sessionID = "default"
	}
	turnToken := s.speechLabTokens().Issue(sessionID, result.Text)
	writeSpeechLabJSON(w, http.StatusOK, VoiceUploadResponse{
		Success: "true", Transcription: result.Text, Duration: int(duration / time.Second), SpeechLabTurnToken: turnToken,
	})
}

func isWAVMediaType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	switch strings.ToLower(mediaType) {
	case "audio/wav", "audio/x-wav", "audio/wave", "audio/vnd.wave":
		return true
	default:
		return false
	}
}

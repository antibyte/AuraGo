package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/telegram"
)

// VoiceUploadResponse represents the response from voice upload endpoint
type VoiceUploadResponse struct {
	Success       string `json:"success"`
	Transcription string `json:"transcription"`
	Duration      int    `json:"duration"`
}

// handleVoiceUpload receives audio recordings and transcribes them
func handleVoiceUpload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Limit upload size to 50MB
		if err := r.ParseMultipartForm(50 << 20); err != nil {
			s.Logger.Error("Failed to parse multipart form", "error", err)
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		// Get the audio file
		file, header, err := r.FormFile("audio")
		if err != nil {
			s.Logger.Error("Failed to get audio file", "error", err)
			http.Error(w, "No audio file uploaded", http.StatusBadRequest)
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
			http.Error(w, "Invalid audio format", http.StatusBadRequest)
			return
		}

		// Create temp directory for processing
		tempDir := filepath.Join(os.TempDir(), "aurago-voice")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			s.Logger.Error("Failed to create temp dir", "error", err)
			http.Error(w, "Server error", http.StatusInternalServerError)
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
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}

		if _, err := io.Copy(inputFile, file); err != nil {
			inputFile.Close()
			os.Remove(inputPath)
			s.Logger.Error("Failed to save audio file", "error", err)
			http.Error(w, "Failed to save audio", http.StatusInternalServerError)
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

		// Transcribe using the same function as Telegram
		transcription, err := telegram.TranscribeMultimodal(outputPath, s.Cfg)
		if err != nil {
			s.Logger.Error("Transcription failed", "error", err)
			http.Error(w, "Transcription failed", http.StatusInternalServerError)
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

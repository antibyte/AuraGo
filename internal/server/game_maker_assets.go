package server

import (
	"net/http"
	"strings"

	"aurago/internal/gamemaker"
)

func handleGameMakerAssetPacks(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.GameMaker == nil {
			jsonError(w, "Game Maker service is unavailable", http.StatusServiceUnavailable)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, "/api/game-maker/asset-packs")
		if rel == "" || rel == "/" {
			packs, err := s.GameMaker.ListAssetPacks()
			if err != nil {
				handleGameMakerError(w, err)
				return
			}
			writeGameMakerJSON(w, http.StatusOK, map[string]any{"packs": packs})
			return
		}
		parts := strings.Split(strings.TrimPrefix(rel, "/"), "/")
		if len(parts) < 2 {
			handleGameMakerError(w, gamemaker.ErrNotFound)
			return
		}
		filename := strings.Join(parts[1:], "/")
		data, err := s.GameMaker.AssetPackFile(parts[0], filename)
		if err != nil {
			handleGameMakerError(w, err)
			return
		}
		contentType := "application/json"
		if strings.HasSuffix(filename, ".wav") {
			contentType = "audio/wav"
		}
		if strings.HasSuffix(filename, ".js") {
			contentType = "text/javascript; charset=utf-8"
		}
		if parts[1] == "sheet.png" {
			contentType = "image/png"
		}
		if strings.HasSuffix(filename, ".glb") {
			contentType = "model/gltf-binary"
		}
		if strings.HasSuffix(filename, ".webp") {
			contentType = "image/webp"
		}
		if strings.HasSuffix(filename, ".txt") {
			contentType = "text/plain; charset=utf-8"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, no-cache")
		_, _ = w.Write(data)
	}
}

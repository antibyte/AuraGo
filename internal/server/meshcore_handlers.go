package server

import (
	"aurago/internal/meshcore"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func registerMeshCoreRoutes(mux *http.ServeMux, s *Server) {
	mux.Handle("/api/meshcore/", requireAdmin(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := strings.TrimPrefix(r.URL.Path, "/api/meshcore/")
		read := action == "status" || action == "devices" || action == "contacts" || action == "channels" || action == "messages"
		method := http.MethodPost
		if read {
			method = http.MethodGet
		}
		if r.Method != method {
			jsonError(w, "Method not allowed", 405)
			return
		}
		if !read && !sameOriginOrNoOrigin(r) {
			jsonError(w, "Same origin required", 403)
			return
		}
		if s.MeshCore == nil {
			jsonError(w, "MeshCore unavailable", 503)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		var body struct {
			ID      string `json:"id"`
			Address string `json:"address"`
			PIN     string `json:"pin"`
		}
		if !read {
			d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
			d.DisallowUnknownFields()
			if err := d.Decode(&body); err != nil {
				jsonError(w, "Invalid request", 400)
				return
			}
			if d.Decode(&struct{}{}) != io.EOF {
				jsonError(w, "Invalid request", 400)
				return
			}
		}
		switch action {
		case "status":
			cfg := s.ConfigSnapshot()
			writeJSON(w, map[string]interface{}{"status": s.MeshCore.Status(), "config": cfg.MeshCore, "ble_supported": !cfg.Runtime.IsDocker && meshCoreBLESupported()})
		case "contacts":
			writeJSON(w, map[string]interface{}{"contacts": s.MeshCore.Status().Contacts})
		case "channels":
			writeJSON(w, map[string]interface{}{"channels": s.MeshCore.Status().Channels})
		case "devices":
			ports, err := meshcore.SerialPorts()
			if err != nil {
				jsonError(w, "Serial port enumeration unavailable", 503)
				return
			}
			writeJSON(w, map[string]interface{}{"ports": ports})
		case "messages":
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			messages, err := s.MeshCore.Messages(limit, offset)
			if err != nil {
				jsonError(w, "Inbox unavailable", 503)
				return
			}
			writeJSON(w, map[string]interface{}{"messages": messages})
		case "test":
			st, err := s.MeshCore.Test(ctx)
			if err != nil {
				jsonError(w, "MeshCore device is not ready; check saved connection settings", 503)
				return
			}
			writeJSON(w, map[string]interface{}{"status": st})
		case "recheck":
			if err := s.MeshCore.Recheck(body.ID); err != nil {
				jsonError(w, err.Error(), 409)
				return
			}
			writeJSON(w, map[string]interface{}{"status": "queued"})
		case "scan", "pair":
			cfg := s.ConfigSnapshot()
			if cfg.Runtime.IsDocker || !meshCoreBLESupported() || s.Bluetooth == nil {
				jsonError(w, "Bluetooth unavailable", 409)
				return
			}
			if action == "scan" {
				devices, err := s.Bluetooth.Discover(ctx, 10*time.Second)
				if err != nil {
					jsonError(w, "Bluetooth discovery unavailable", 409)
					return
				}
				writeJSON(w, map[string]interface{}{"devices": devices})
				return
			}
			if cfg.MeshCore.Transport != "ble" || body.Address != cfg.MeshCore.Address {
				jsonError(w, "Pair only the saved MeshCore device", 400)
				return
			}
			err := s.Bluetooth.Pair(ctx, body.Address, body.PIN)
			body.PIN = ""
			if err != nil {
				jsonError(w, "Pairing failed; check Bluetooth write permission and PIN", 409)
				return
			}
			writeJSON(w, map[string]interface{}{"status": "paired"})
		default:
			jsonError(w, "Unknown MeshCore action", 404)
		}
	})))
}

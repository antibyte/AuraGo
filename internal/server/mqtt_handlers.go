package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/mqtt"
)

// handleMQTTStatus returns the current MQTT connection status.
func handleMQTTStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if !s.Cfg.MQTT.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "disabled",
				"message": "MQTT integration is not enabled",
			})
			return
		}

		if s.Cfg.MQTT.Broker == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "no_broker",
				"message": "MQTT broker URL is not configured",
			})
			return
		}

		connected := mqtt.IsConnected()
		bufferLen := mqtt.BufferLen()

		response := map[string]interface{}{
			"status":      "disabled",
			"connected":   connected,
			"broker":      s.Cfg.MQTT.Broker,
			"client_id":   s.Cfg.MQTT.ClientID,
			"buffer_len":  bufferLen,
			"max_buffer":  s.Cfg.MQTT.Buffer.MaxMessages,
			"tls_enabled": s.Cfg.MQTT.TLS.Enabled,
		}

		if !s.Cfg.MQTT.Enabled {
			response["status"] = "disabled"
		} else if connected {
			response["status"] = "connected"
		} else {
			response["status"] = "disconnected"
		}

		json.NewEncoder(w).Encode(response)
	}
}

// handleMQTTTest tests the MQTT broker connection.
func handleMQTTTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if !s.Cfg.MQTT.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "MQTT integration is not enabled",
			})
			return
		}

		if s.Cfg.MQTT.Broker == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "MQTT broker URL is not configured",
			})
			return
		}

		connected := mqtt.IsConnected()

		if connected {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "success",
				"message": "MQTT broker connection is active",
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "MQTT broker connection is not active",
			})
		}
	}
}

// handleMQTTMessages returns buffered MQTT messages.
func handleMQTTMessages(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if !s.Cfg.MQTT.Enabled {
			jsonError(w, "MQTT integration is not enabled", http.StatusBadRequest)
			return
		}

		topic := r.URL.Query().Get("topic")
		limit := 50

		messages := mqtt.GetMessages(topic, limit)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "success",
			"messages": messages,
			"count":    len(messages),
		})
	}
}

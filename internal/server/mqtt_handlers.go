package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/config"
	"aurago/internal/mqtt"
	"aurago/internal/security"
)

// handleMQTTStatus returns the current MQTT connection status.
func handleMQTTStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		cfg := s.ConfigSnapshot()
		if cfg == nil {
			jsonError(w, "MQTT configuration is unavailable", http.StatusServiceUnavailable)
			return
		}
		runtime := mqtt.MQTTStatus{State: "disabled"}
		if s.MQTTController != nil {
			runtime = s.MQTTController.Status()
		} else if cfg.MQTT.Enabled {
			runtime.State = "disconnected"
		}
		runtime.LastError = security.Scrub(runtime.LastError)
		for i := range runtime.Subscriptions {
			runtime.Subscriptions[i].LastError = security.Scrub(runtime.Subscriptions[i].LastError)
		}
		bufferLen := mqtt.BufferLen()
		source, err := config.ResolveMQTTPasswordSource(s.Vault)
		if err != nil {
			source = "unavailable"
		}

		response := map[string]interface{}{
			"status":                  "disabled",
			"connected":               runtime.Connected,
			"broker":                  cfg.MQTT.Broker,
			"client_id":               cfg.MQTT.ClientID,
			"active_broker":           runtime.ActiveBroker,
			"active_client_id":        runtime.ActiveClientID,
			"connection_state":        runtime.State,
			"effective_tls":           runtime.EffectiveTLS,
			"transport_scheme":        runtime.ActiveTransport,
			"config_revision":         runtime.DesiredRevision,
			"applied_config_revision": runtime.ActiveRevision,
			"credential_source":       source,
			"runtime":                 runtime,
			"buffer_len":              bufferLen,
			"max_buffer":              cfg.MQTT.Buffer.MaxMessages,
			"max_age_hours":           cfg.MQTT.Buffer.MaxAgeHours,
			"max_payload_bytes":       cfg.MQTT.Buffer.MaxPayloadBytes,
			"tls_enabled":             runtime.EffectiveTLS,
			"stats":                   mqttSanitizedStats(),
		}

		if !cfg.MQTT.Enabled || cfg.EggMode.Enabled {
			response["status"] = "disabled"
		} else if cfg.MQTT.Broker == "" {
			response["status"] = "no_broker"
		} else if runtime.Connected {
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

		cfg := s.ConfigSnapshot()
		if cfg == nil || !cfg.MQTT.Enabled || cfg.EggMode.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "MQTT integration is not enabled",
			})
			return
		}

		if cfg.MQTT.Broker == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "MQTT broker URL is not configured",
			})
			return
		}

		if err := mqtt.TestConnectionContext(r.Context(), cfg, s.Logger); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": security.Scrub(err.Error()),
				"stats":   mqttSanitizedStats(),
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "MQTT broker connection test succeeded",
			"stats":   mqttSanitizedStats(),
		})
	}
}

func mqttSanitizedStats() map[string]interface{} {
	stats := mqtt.RuntimeStats()
	if message, ok := stats["last_error"].(string); ok {
		stats["last_error"] = security.Scrub(message)
	}
	return stats
}

// handleMQTTMessages returns buffered MQTT messages.
func handleMQTTMessages(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		cfg := s.ConfigSnapshot()
		if cfg == nil || !cfg.MQTT.Enabled || cfg.EggMode.Enabled {
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

package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/push"
)

// handlePushVAPIDPublicKey returns the VAPID public key needed by the browser
// to create a push subscription.
func handlePushVAPIDPublicKey(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		mgr := push.GlobalManager
		if mgr == nil {
			http.Error(w, "push notifications not available", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"public_key": mgr.GetPublicKey()})
	}
}

// handlePushSubscribe saves a new Web Push subscription sent by the browser.
func handlePushSubscribe(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		mgr := push.GlobalManager
		if mgr == nil {
			http.Error(w, "push notifications not available", http.StatusServiceUnavailable)
			return
		}

		var sub push.PushSubscription
		if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := mgr.Subscribe(sub); err != nil {
			s.Logger.Error("Failed to save push subscription", "error", err)
			http.Error(w, "failed to save subscription", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

// handlePushUnsubscribe removes a push subscription by endpoint.
func handlePushUnsubscribe(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		mgr := push.GlobalManager
		if mgr == nil {
			http.Error(w, "push notifications not available", http.StatusServiceUnavailable)
			return
		}

		var req struct {
			Endpoint string `json:"endpoint"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Endpoint == "" {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := mgr.Unsubscribe(req.Endpoint); err != nil {
			s.Logger.Error("Failed to remove push subscription", "error", err)
			http.Error(w, "failed to remove subscription", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// handlePushStatus returns whether push is available and how many subscriptions exist.
func handlePushStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		mgr := push.GlobalManager
		if mgr == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"available": false, "subscriptions": 0})
			return
		}
		count := mgr.CountSubscriptions()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"available": true, "subscriptions": count})
	}
}

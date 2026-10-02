package server

import (
	"context"
	"encoding/json"
	"net/http"

	"aurago/internal/bluetooth"
)

// realtimeHeadsets is the Bluetooth side of Live Speech headsets.
type realtimeHeadsets interface {
	List(ctx context.Context) ([]bluetooth.HeadsetDevice, string)
	Open(ctx context.Context, address string) (realtimeHeadsetLink, error)
}

// realtimeHeadsetLink is what the bridge needs from bluetooth.HeadsetLink.
type realtimeHeadsetLink interface {
	Frames() <-chan []byte
	Events() <-chan bluetooth.HeadsetEvent
	Write(stream int, pcm []byte) error
	Flush(stream int)
	Close() error
}

type managerHeadsets struct{ s *Server }

func (h managerHeadsets) List(ctx context.Context) ([]bluetooth.HeadsetDevice, string) {
	if h.s == nil || h.s.Bluetooth == nil {
		return []bluetooth.HeadsetDevice{}, "Bluetooth is not available."
	}
	return h.s.Bluetooth.HeadsetDevices(ctx)
}

func (h managerHeadsets) Open(ctx context.Context, address string) (realtimeHeadsetLink, error) {
	if h.s == nil || h.s.Bluetooth == nil {
		return nil, &bluetooth.CodedError{Code: bluetooth.ErrorHeadsetAudioUnavailable, Message: "Bluetooth is not available."}
	}
	link, err := h.s.Bluetooth.OpenHeadset(ctx, address)
	if err != nil {
		return nil, err
	}
	return link, nil
}

// handleRealtimeSpeechAudioDevices lists the server's Bluetooth headsets with
// a microphone for the Live Speech audio field.
func handleRealtimeSpeechAudioDevices(headsets realtimeHeadsets) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		devices, reason := headsets.List(r.Context())
		if devices == nil {
			devices = []bluetooth.HeadsetDevice{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"devices": devices, "reason": reason})
	}
}

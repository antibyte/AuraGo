package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"aurago/internal/bluetooth"
	"aurago/internal/realtimespeech"
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

const (
	realtimeHeadsetReadLimit   = 64 * 1024
	realtimeHeadsetWriteWait   = 5 * time.Second
	realtimeHeadsetSessionPoll = 2 * time.Second
)

var realtimeHeadsetErrorCodes = map[string]string{
	bluetooth.ErrorHeadsetBusy:               "headset_busy",
	bluetooth.ErrorHeadsetUnknown:            "headset_unknown",
	bluetooth.ErrorInvalidArgument:           "headset_unknown",
	bluetooth.ErrorHeadsetProfileUnavailable: "headset_profile_unavailable",
	bluetooth.ErrorHeadsetAudioUnavailable:   "headset_audio_unavailable",
}

func realtimeHeadsetErrorCode(err error) string {
	if code, ok := realtimeHeadsetErrorCodes[bluetooth.ErrorCode(err)]; ok {
		return code
	}
	return "headset_audio_unavailable"
}

type realtimeHeadsetControl struct {
	Type   string `json:"type"`
	Stream int    `json:"stream"`
}

// handleRealtimeSpeechHeadset bridges one Live Speech session to a server
// headset over a WebSocket: microphone PCM flows to the browser as binary
// frames, provider audio comes back as [stream byte] + PCM. The bridge ends
// with the WebSocket or the session.
func handleRealtimeSpeechHeadset(registry *realtimespeech.Registry, headsets realtimeHeadsets) http.HandlerFunc {
	return newRealtimeHeadsetHandler(registry, headsets, realtimeHeadsetSessionPoll)
}

func newRealtimeHeadsetHandler(registry *realtimespeech.Registry, headsets realtimeHeadsets, sessionPoll time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		sessionID := strings.TrimSpace(query.Get("session"))
		clientID := strings.TrimSpace(query.Get("client"))
		address := strings.TrimSpace(query.Get("device"))
		if sessionID == "" || clientID == "" || address == "" {
			jsonError(w, "session, client and device are required", http.StatusBadRequest)
			return
		}
		if _, ok := registry.Get(sessionID, clientID); !ok {
			jsonError(w, "Live speech session not found", http.StatusForbidden)
			return
		}
		conn, err := desktopWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetReadLimit(realtimeHeadsetReadLimit)
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		link, err := headsets.Open(ctx, address)
		if err != nil {
			_ = writeRealtimeHeadsetJSON(conn, map[string]interface{}{"type": "error", "code": realtimeHeadsetErrorCode(err)})
			return
		}
		defer link.Close()
		if err := writeRealtimeHeadsetJSON(conn, map[string]interface{}{
			"type": "ready", "input_rate": bluetooth.HeadsetInputRate, "output_rate": bluetooth.HeadsetOutputRate,
		}); err != nil {
			return
		}
		go readRealtimeHeadsetBrowser(conn, link, cancel)

		poll := time.NewTicker(sessionPoll)
		defer poll.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-poll.C:
				if _, ok := registry.Get(sessionID, clientID); !ok {
					return
				}
			case frame := <-link.Frames():
				_ = conn.SetWriteDeadline(time.Now().Add(realtimeHeadsetWriteWait))
				if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
					return
				}
			case event := <-link.Events():
				if err := writeRealtimeHeadsetJSON(conn, realtimeHeadsetEventMessage(event)); err != nil {
					return
				}
			}
		}
	}
}

func realtimeHeadsetEventMessage(event bluetooth.HeadsetEvent) map[string]interface{} {
	switch event.Type {
	case bluetooth.HeadsetReady:
		return map[string]interface{}{"type": "device_ready"}
	case bluetooth.HeadsetLost:
		return map[string]interface{}{"type": "device_lost"}
	default:
		return map[string]interface{}{"type": "error", "code": realtimeHeadsetErrorCode(&bluetooth.CodedError{Code: event.Code})}
	}
}

// readRealtimeHeadsetBrowser forwards browser audio and flush requests; it
// cancels the bridge when the WebSocket closes.
func readRealtimeHeadsetBrowser(conn *websocket.Conn, link realtimeHeadsetLink, cancel context.CancelFunc) {
	defer cancel()
	for {
		kind, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		switch kind {
		case websocket.BinaryMessage:
			if len(data) >= 2 {
				_ = link.Write(int(data[0]), data[1:])
			}
		case websocket.TextMessage:
			var control realtimeHeadsetControl
			if json.Unmarshal(data, &control) == nil && control.Type == "flush" {
				link.Flush(control.Stream)
			}
		}
	}
}

func writeRealtimeHeadsetJSON(conn *websocket.Conn, payload interface{}) error {
	_ = conn.SetWriteDeadline(time.Now().Add(realtimeHeadsetWriteWait))
	return conn.WriteJSON(payload)
}

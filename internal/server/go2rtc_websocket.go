package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/sdp/v3"
)

type go2RTCWireMessage struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value,omitempty"`
}

func validateGo2RTCOffer(raw string, talk bool) error {
	if len(raw) > 128<<10 {
		return fmt.Errorf("SDP is too large")
	}
	var offer sdp.SessionDescription
	if err := offer.Unmarshal([]byte(raw)); err != nil || len(offer.MediaDescriptions) == 0 {
		return fmt.Errorf("invalid SDP offer")
	}
	direction := func(attrs []sdp.Attribute, fallback string) string {
		for _, a := range attrs {
			switch a.Key {
			case "recvonly", "sendonly", "sendrecv", "inactive":
				return a.Key
			}
		}
		return fallback
	}
	sessionDirection := direction(offer.Attributes, "sendrecv")
	for _, media := range offer.MediaDescriptions {
		if media.MediaName.Media != "audio" && media.MediaName.Media != "video" {
			return fmt.Errorf("unsupported media")
		}
		d := direction(media.Attributes, sessionDirection)
		if talk {
			if media.MediaName.Media != "audio" || (d != "sendonly" && d != "inactive") {
				return fmt.Errorf("talk sessions permit microphone audio only")
			}
		} else if d != "recvonly" && d != "inactive" {
			return fmt.Errorf("viewer sessions may only receive media")
		}
	}
	return nil
}

func validateGo2RTCMessage(data []byte, talk bool) (go2RTCWireMessage, error) {
	var msg go2RTCWireMessage
	if len(data) > 256<<10 || json.Unmarshal(data, &msg) != nil {
		return msg, fmt.Errorf("invalid player message")
	}
	var value string
	if len(msg.Value) > 0 && json.Unmarshal(msg.Value, &value) != nil {
		return msg, fmt.Errorf("player value must be a string")
	}
	switch msg.Type {
	case "webrtc/offer":
		return msg, validateGo2RTCOffer(value, talk)
	case "webrtc/candidate":
		if len(value) > 4096 || strings.ContainsAny(value, "\r\n") {
			return msg, fmt.Errorf("invalid ICE candidate")
		}
	case "mse", "hls", "mp4":
		if talk || len(value) > 2048 {
			return msg, fmt.Errorf("unsupported player message")
		}
		for _, ch := range value {
			if ch < 32 || ch > 126 {
				return msg, fmt.Errorf("invalid codec list")
			}
		}
	case "mjpeg":
		if talk {
			return msg, fmt.Errorf("unsupported player message")
		}
	default:
		return msg, fmt.Errorf("unsupported player message")
	}
	return msg, nil
}

// bridgeGo2RTCWebSocket terminates the public protocol boundary: no caller
// parameters or arbitrary upstream JSON are forwarded without validation.
func bridgeGo2RTCWebSocket(s *Server, w http.ResponseWriter, r *http.Request, alias string) {
	target, err := s.Go2RTC.ProxyTarget()
	if err != nil {
		jsonError(w, "Camera upstream unavailable", 503)
		return
	}
	user, password, err := s.Go2RTC.ProxyCredentials()
	if err != nil {
		jsonError(w, "Camera credential unavailable", 503)
		return
	}
	target.Scheme = "ws"
	target.Path = "/api/go2rtc/proxy/api/ws"
	target.RawQuery = url.Values{"src": {alias}}.Encode()
	headers := make(http.Header)
	auth := &http.Request{Header: headers}
	auth.SetBasicAuth(user, password)
	headers.Set("Origin", "http://"+target.Host)
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	upstream, response, err := dialer.DialContext(r.Context(), target.String(), headers)
	if response != nil && response.Body != nil {
		response.Body.Close()
	}
	if err != nil {
		jsonError(w, "Camera signaling unavailable", 502)
		return
	}
	defer upstream.Close()
	upgrade := websocket.Upgrader{CheckOrigin: sameOriginOrNoOrigin}
	client, err := upgrade.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer client.Close()
	client.SetReadLimit(256 << 10)
	upstream.SetReadLimit(20 << 20)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() { <-ctx.Done(); client.Close(); upstream.Close() }()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer cancel()
		for {
			kind, data, err := client.ReadMessage()
			if err != nil || kind != websocket.TextMessage {
				return
			}
			msg, err := validateGo2RTCMessage(data, false)
			if err != nil {
				return
			}
			clean, _ := json.Marshal(msg)
			upstream.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if upstream.WriteMessage(kind, clean) != nil {
				return
			}
		}
	}()
	for {
		kind, data, err := upstream.ReadMessage()
		if err != nil {
			break
		}
		if kind == websocket.TextMessage {
			var msg go2RTCWireMessage
			if json.Unmarshal(data, &msg) != nil {
				break
			}
			if msg.Type == "error" {
				msg.Value = json.RawMessage(`"Camera media unavailable"`)
				data, _ = json.Marshal(msg)
			}
		}
		client.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if client.WriteMessage(kind, data) != nil {
			break
		}
	}
	cancel()
	<-done
}

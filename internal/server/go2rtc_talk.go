package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"aurago/internal/tools"
)

const go2RTCTalkScope = "go2rtc.talk"

type go2RTCTalkSession struct {
	id        string
	identity  string
	lease     *tools.Go2RTCMediaLease
	server    *Server
	timer     *time.Timer
	hardTimer *time.Timer
	connected bool
	once      sync.Once
}

func go2RTCRequestCanTalk(s *Server, r *http.Request) bool {
	if token, bearer := bearerCredential(r.Header.Get("Authorization")); bearer {
		tm := s.currentTokenManager()
		if tm == nil || token == "" {
			return false
		}
		if _, ok := tm.Validate(token, "admin"); ok {
			return true
		}
		_, ok := tm.Validate(token, go2RTCTalkScope)
		return ok
	}
	return go2RTCRequestIsAdmin(s, r)
}

func go2RTCTalkIdentity(r *http.Request) string {
	identity := ""
	if token, bearer := bearerCredential(r.Header.Get("Authorization")); bearer {
		identity = "bearer:" + token
	} else if cookie, err := r.Cookie(sessionCookieName); err == nil {
		identity = "session:" + cookie.Value
	} else {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		identity = "local:" + host
	}
	hash := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(hash[:])
}

func (session *go2RTCTalkSession) close() {
	session.once.Do(func() {
		s := session.server
		session.lease.Release()
		s.go2RTCTalkMu.Lock()
		if s.go2RTCTalks[session.lease.StreamID] == session {
			delete(s.go2RTCTalks, session.lease.StreamID)
		}
		if session.timer != nil {
			session.timer.Stop()
		}
		if session.hardTimer != nil {
			session.hardTimer.Stop()
		}
		s.go2RTCTalkMu.Unlock()
	})
}

func (session *go2RTCTalkSession) heartbeat(r *http.Request) bool {
	if !go2RTCRequestCanTalk(session.server, r) || session.identity != go2RTCTalkIdentity(r) || session.lease.Context.Err() != nil {
		return false
	}
	cfg := session.server.Go2RTC.Config()
	enabled := false
	for _, camera := range cfg.Streams {
		if camera.ID == session.lease.StreamID {
			enabled = camera.Enabled && camera.TalkEnabled
		}
	}
	if !cfg.Enabled || !cfg.WebRTC.Enabled || !enabled {
		return false
	}
	session.server.go2RTCTalkMu.Lock()
	defer session.server.go2RTCTalkMu.Unlock()
	if session.server.go2RTCTalks[session.lease.StreamID] != session {
		return false
	}
	session.timer.Reset(3 * time.Second)
	return true
}

func handleGo2RTCTalk(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !sameOriginOrNoOrigin(r) || !go2RTCRequestCanTalk(s, r) {
			jsonError(w, "go2rtc.talk permission is required", 403)
			return
		}
		manager := go2RTCManager(s, w)
		if manager == nil {
			return
		}
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/go2rtc/talk/"), "/"), "/")
		if len(parts) < 1 || len(parts) > 2 || tools.ValidateGo2RTCStreamID(parts[0]) != nil {
			jsonError(w, "Invalid camera", 400)
			return
		}
		id := parts[0]
		enabled := false
		cfg := manager.Config()
		for _, camera := range cfg.Streams {
			if camera.ID == id && camera.Enabled {
				enabled = camera.TalkEnabled
			}
		}
		if !cfg.Enabled || !cfg.WebRTC.Enabled || !enabled {
			jsonError(w, "Camera talk requires enabled LAN WebRTC and camera talk", 412)
			return
		}
		if !manager.SupportsBoundWebRTC() {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			_, _ = manager.Test(ctx)
			cancel()
			if !manager.SupportsBoundWebRTC() {
				jsonError(w, "Talk requires a sidecar with ws-close-v1", 412)
				return
			}
		}
		identity := go2RTCTalkIdentity(r)
		if r.Method == http.MethodPost && len(parts) == 1 {
			var body struct{}
			if err := decodeGo2RTCAppJSON(w, r, &body); err != nil {
				jsonError(w, "Invalid talk request", 400)
				return
			}
			lease, err := manager.AcquireMedia(context.Background(), id, "main", false)
			if err != nil {
				jsonError(w, "Camera is unavailable", 503)
				return
			}
			var token [24]byte
			if _, err := rand.Read(token[:]); err != nil {
				lease.Release()
				jsonError(w, "Could not create talk session", 500)
				return
			}
			session := &go2RTCTalkSession{id: hex.EncodeToString(token[:]), identity: identity, lease: lease, server: s}
			s.go2RTCTalkMu.Lock()
			if s.go2RTCTalks == nil {
				s.go2RTCTalks = map[string]*go2RTCTalkSession{}
			}
			if old := s.go2RTCTalks[id]; old != nil && old.lease.Context.Err() == nil {
				s.go2RTCTalkMu.Unlock()
				lease.Release()
				jsonError(w, "Camera talk is busy", 409)
				return
			}
			s.go2RTCTalks[id] = session
			session.timer = time.AfterFunc(10*time.Second, session.close)
			session.hardTimer = time.AfterFunc(60*time.Second, session.close)
			s.go2RTCTalkMu.Unlock()
			context.AfterFunc(lease.Context, session.close)
			writeGo2RTCJSON(w, map[string]interface{}{"status": "ok", "session": session.id, "revision": lease.Revision, "ws_url": fmt.Sprintf("/api/go2rtc/talk/%s/ws?session=%s", id, session.id)})
			return
		}
		s.go2RTCTalkMu.Lock()
		session := s.go2RTCTalks[id]
		if session == nil || session.identity != identity || session.id != r.URL.Query().Get("session") || session.lease.Context.Err() != nil {
			s.go2RTCTalkMu.Unlock()
			jsonError(w, "Talk session expired", 403)
			return
		}
		if r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "ws" {
			if session.connected {
				s.go2RTCTalkMu.Unlock()
				jsonError(w, "Talk session is already connected", 409)
				return
			}
			session.connected = true
			session.timer.Reset(3 * time.Second)
			s.go2RTCTalkMu.Unlock()
			defer session.close()
			bridgeGo2RTCWebSocket(s, w, r.WithContext(session.lease.Context), session.lease, session)
			return
		}
		s.go2RTCTalkMu.Unlock()
		if r.Method == http.MethodDelete && len(parts) == 1 {
			session.close()
			writeGo2RTCJSON(w, map[string]string{"status": "ok"})
			return
		}
		jsonError(w, "Method not allowed", 405)
	}
}

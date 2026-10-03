package server

import (
	"net/http"
	"os"
	"strings"
	"time"

	"aurago/internal/security"
)

func castMediaAssetHandler(dir, prefix string, requireTicket bool) http.Handler {
	active := make(chan struct{}, 32)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", 405)
			return
		}
		if requireTicket && !security.ValidCastMediaTicket(r.URL, time.Now()) {
			http.Error(w, "forbidden", 403)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, prefix)
		if !strings.HasPrefix(r.URL.Path, prefix) || name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\:\x00") {
			http.NotFound(w, r)
			return
		}
		select {
		case active <- struct{}{}:
			defer func() { <-active }()
		default:
			http.Error(w, "busy", 503)
			return
		}
		root, err := os.OpenRoot(dir)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer root.Close()
		info, err := root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		f, err := root.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err = f.Stat()
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, no-store")
		if prefix == "/tts/" {
			w.Header().Set("Content-Type", chatVoiceAudioMIMEType(name))
		} else {
			w.Header().Set("Content-Type", castMediaContentType(name))
		}
		http.ServeContent(w, r, name, info.ModTime(), f)
	})
}

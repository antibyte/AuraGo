// This server runs only inside the opt-in, isolated Gotenberg Docker test network.
package main

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

func main() {
	var public, private atomic.Int64
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stats":
			_ = json.NewEncoder(w).Encode(map[string]int64{"public": public.Load(), "private": private.Load()})
		case "/private":
			private.Add(1)
			_, _ = w.Write([]byte("private target must stay unreachable"))
		case "/public":
			public.Add(1)
			w.Header().Set("Content-Type", "text/css")
			_, _ = w.Write([]byte("h1 { color: green; }"))
		case "/redirect":
			http.Redirect(w, r, "http://192.168.245.2:8080/private", http.StatusFound)
		default:
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<!doctype html><html><body><h1>Isolated document fixture</h1></body></html>"))
		}
	})
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}

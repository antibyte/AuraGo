package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gamemaker"
)

func TestGameMakerPreviewCannotWriteProjectAPI(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("auth_enabled_%v", enabled), func(t *testing.T) {
			root := t.TempDir()
			svc, err := gamemaker.NewService(gamemaker.Options{
				DBPath: filepath.Join(root, "gm.db"), WorkspacePath: filepath.Join(root, "workspace"),
				Enabled: true, AllowCreate: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer svc.Close()
			s := &Server{Cfg: &config.Config{}, GameMaker: svc}
			s.Cfg.Auth.Enabled = enabled
			s.Cfg.Auth.PasswordHash = "synthetic-test-configured-password"
			s.Cfg.Auth.SessionSecret = "synthetic-preview-audit-session"
			requestOrigins := make(chan string, 2)
			mux := http.NewServeMux()
			mux.HandleFunc("/api/game-maker/projects", func(w http.ResponseWriter, r *http.Request) {
				requestOrigins <- r.Header.Get("Origin")
				handleGameMakerProjects(s)(w, r)
			})
			mux.HandleFunc("/api/game-maker/preview/audit/index.html", func(w http.ResponseWriter, r *http.Request) {
				setGameMakerPreviewHeaders(w)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprint(w, `<!doctype html><script>
        fetch('/api/game-maker/projects', {method:'POST',mode:'no-cors',headers:{'Content-Type':'text/plain'},
          body:JSON.stringify({name:'Preview injected project',description:'Synthetic audit fixture',dimension:'2d'})})
          .then(()=>parent.postMessage({done:true},'*')).catch(e=>parent.postMessage({done:true,error:String(e)},'*'));
        </script>`)
			})
			mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `<!doctype html><script>window.auditResult=null;addEventListener('message',e=>window.auditResult={...e.data,origin:e.origin});</script><iframe sandbox="allow-scripts allow-pointer-lock" src="/api/game-maker/preview/audit/index.html"></iframe>`)
			})
			authenticated := authMiddleware(s, mux)
			host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/fixture" {
					http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour)), Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
					mux.ServeHTTP(w, r)
					return
				}
				authenticated.ServeHTTP(w, r)
			}))
			defer host.Close()
			page := previewGatewayBrowser(t).MustPage(host.URL + "/fixture").Timeout(20 * time.Second).MustWaitLoad()
			defer page.Close()
			page.MustWait(`()=>window.auditResult?.done===true`)
			result := page.MustEval(`()=>JSON.stringify(window.auditResult)`).Str()
			projects, err := svc.ListProjects(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			origin := "not received"
			select {
			case origin = <-requestOrigins:
			default:
			}
			t.Logf("opaque frame result=%s API Origin=%q created projects=%d", result, origin, len(projects))
			if len(projects) != 0 {
				t.Fatalf("sandboxed game mutated the real project API: %d projects created", len(projects))
			}
			status := page.MustEval(`async()=>{const r=await fetch('/api/game-maker/projects',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:'Parent created project',description:'Synthetic fixture',dimension:'2d'})});return r.status}`).Int()
			if status != http.StatusCreated {
				t.Fatalf("same-origin parent create returned %d", status)
			}
			projects, err = svc.ListProjects(context.Background())
			if err != nil || len(projects) != 1 {
				t.Fatalf("same-origin create: count=%d err=%v", len(projects), err)
			}
		})
	}
}

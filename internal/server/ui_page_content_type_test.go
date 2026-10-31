package server

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

// Every page, served through the gzip middleware, must be text/html and the
// rendered markup must start with the doctype (no whitespace-only first chunk).
func TestUIPagesServeAsHTMLThroughGzip(t *testing.T) {
	for _, c := range uiPageHeaderCases {
		c := c
		t.Run(c.file, func(t *testing.T) {
			body := renderUIPageForTest(t, c)
			if !strings.HasPrefix(body, "<!DOCTYPE html>") {
				t.Fatalf("%s output starts with %q, want <!DOCTYPE html>", c.file, body[:min(40, len(body))])
			}
			tmpl, err := parseUIPage(os.DirFS("../../ui"), c.file)
			if c.file == "dashboard.html" {
				tmpl, err = parseDashboardTemplate(os.DirFS("../../ui"))
			}
			if err != nil {
				t.Fatal(err)
			}
			h := gzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := executeUIPageTemplate(w, tmpl, uiTemplateData("en", c.page)); err != nil {
					t.Error(err)
				}
			}))
			req := httptest.NewRequest(http.MethodGet, "/page", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
				t.Fatalf("Content-Type = %q", ct)
			}
		})
	}
}

// Guard: no handler may execute a page template straight into the response.
func TestUIPageHandlersUseExecuteUIPageTemplate(t *testing.T) {
	direct := regexp.MustCompile(`\b[A-Za-z0-9]*[Tt]mpl\.Execute\(w,`)
	for _, f := range []string{"server_routes_ui.go", "auth_handlers.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if m := direct.Find(src); m != nil {
			t.Errorf("%s executes a template directly (%s); use executeUIPageTemplate", f, m)
		}
	}
}

func TestExecuteUIPageTemplateErrorLeavesResponseUntouched(t *testing.T) {
	tmpl := template.Must(template.New("p").Parse("<!DOCTYPE html>{{template \"nope\"}}"))
	rec := httptest.NewRecorder()
	if err := executeUIPageTemplate(rec, tmpl, nil); err == nil {
		t.Fatal("expected execution error")
	}
	if rec.Body.Len() != 0 || rec.Header().Get("Content-Type") != "" {
		t.Fatalf("partial output written: %q", rec.Body.String())
	}
}

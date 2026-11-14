package tools

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/testutil"
)

func TestRenderHTMLPDFChoosesTheRenderer(t *testing.T) {
	oldGotenberg, oldChromium := htmlPDFGotenberg, htmlPDFChromium
	defer func() { htmlPDFGotenberg, htmlPDFChromium = oldGotenberg, oldChromium }()
	var calls []string
	var browserPage string
	var gotenbergErr, chromiumErr error
	htmlPDFGotenberg = func(ctx context.Context, cfg *config.GotenbergConfig, page string) ([]byte, error) {
		calls = append(calls, "gotenberg "+cfg.URL)
		return []byte("%PDF-gotenberg"), gotenbergErr
	}
	htmlPDFChromium = func(ctx context.Context, page string) ([]byte, error) {
		calls = append(calls, "browser")
		browserPage = page
		return []byte("%PDF-browser"), chromiumErr
	}
	page := `<!doctype html><html><head><title>x</title></head><body>pages</body></html>`
	withGotenberg := &config.Config{}
	withGotenberg.Tools.DocumentCreator.Enabled = true
	withGotenberg.Tools.DocumentCreator.Backend = "gotenberg"
	withGotenberg.Tools.DocumentCreator.Gotenberg.URL = "http://gotenberg:3000"
	withMaroto := &config.Config{}
	withMaroto.Tools.DocumentCreator.Enabled = true
	withMaroto.Tools.DocumentCreator.Backend = "maroto"
	disabled := &config.Config{}
	disabled.Tools.DocumentCreator.Backend = "gotenberg"

	for _, test := range []struct {
		name       string
		cfg        *config.Config
		gotenberg  error
		chromium   error
		want       string
		wantCalls  string
		wantErr    bool
		wantErrHas []string
	}{
		{name: "no config", want: "%PDF-browser", wantCalls: "browser"},
		{name: "another backend", cfg: withMaroto, want: "%PDF-browser", wantCalls: "browser"},
		{name: "document creator off", cfg: disabled, want: "%PDF-browser", wantCalls: "browser"},
		{name: "gotenberg", cfg: withGotenberg, want: "%PDF-gotenberg", wantCalls: "gotenberg http://gotenberg:3000"},
		{name: "gotenberg fails", cfg: withGotenberg, gotenberg: errors.New("refused"), want: "%PDF-browser", wantCalls: "gotenberg http://gotenberg:3000,browser"},
		{name: "both fail", cfg: withGotenberg, gotenberg: errors.New("refused"), chromium: errors.New("no binary"), wantCalls: "gotenberg http://gotenberg:3000,browser", wantErr: true, wantErrHas: []string{"gotenberg: refused", "browser: no binary"}},
		{name: "browser fails alone", chromium: errors.New("no binary"), wantCalls: "browser", wantErr: true, wantErrHas: []string{"browser: no binary"}},
	} {
		calls, gotenbergErr, chromiumErr = nil, test.gotenberg, test.chromium
		data, err := RenderHTMLPDF(context.Background(), test.cfg, page)
		if got := strings.Join(calls, ","); got != test.wantCalls {
			t.Fatalf("%s: renderers %q, want %q", test.name, got, test.wantCalls)
		}
		if test.wantErr {
			if !errors.Is(err, ErrHTMLPDFUnavailable) {
				t.Fatalf("%s: error %v is not ErrHTMLPDFUnavailable", test.name, err)
			}
			for _, part := range test.wantErrHas {
				if !strings.Contains(err.Error(), part) {
					t.Fatalf("%s: error %q lacks %q", test.name, err, part)
				}
			}
			continue
		}
		if err != nil || string(data) != test.want {
			t.Fatalf("%s: %q, %v", test.name, data, err)
		}
	}

	// The built-in browser gets the page behind the policy; Gotenberg's renderer applies it itself.
	if at, title := strings.Index(browserPage, remoteContentCSPMeta), strings.Index(browserPage, "<title>"); at < 0 || at > title {
		t.Fatalf("the browser page has no policy in front of its head content: %s", browserPage)
	}

	// A cancelled request is not "no renderer".
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls, gotenbergErr, chromiumErr = nil, errors.New("refused"), nil
	if _, err := RenderHTMLPDF(ctx, withGotenberg, page); !errors.Is(err, context.Canceled) || errors.Is(err, ErrHTMLPDFUnavailable) || len(calls) != 1 {
		t.Fatalf("cancelled render: %v after %v", err, calls)
	}
}

// The built-in browser keeps the page's own sheet sizes and loads nothing the page points at.
func TestRenderHTMLPDFBrowserKeepsPageSizesAndLoadsNothing(t *testing.T) {
	bin := testutil.RequireBrowserBin(t)
	previous := htmlPDFBrowserBin
	htmlPDFBrowserBin = bin
	defer func() { htmlPDFBrowserBin = previous }()
	var launched *security.EgressBrowser
	restore := security.SetEgressLaunchObserver(func(eb *security.EgressBrowser) { launched = eb })
	defer restore()
	var hits atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer remote.Close()
	page := `<!doctype html><html><head><style>@page{size:595pt 842pt;margin:0}@page wide{size:842pt 595pt;margin:0}div{break-after:page}.wide{page:wide;break-after:auto}</style></head><body><div>Portrait</div><div class="wide">Landscape<img src="` +
		remote.URL + `/picture.png"><script>fetch("` + remote.URL + `/script")</script><iframe srcdoc="&lt;meta http-equiv=refresh content=&quot;0;url=` + remote.URL + `/frame&quot;&gt;&lt;img src=&quot;` + remote.URL + `/nested.png&quot;&gt;"></iframe></div></body></html>`
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	data, err := RenderHTMLPDF(ctx, nil, page)
	if err != nil {
		t.Fatal(err)
	}
	portrait, landscape := 0, 0
	for _, box := range regexp.MustCompile(`/MediaBox \[0 0 ([0-9.]+) ([0-9.]+)\]`).FindAllSubmatch(data, -1) {
		width, _ := strconv.ParseFloat(string(box[1]), 64)
		height, _ := strconv.ParseFloat(string(box[2]), 64)
		if width > height {
			landscape++
		} else {
			portrait++
		}
	}
	if portrait != 1 || landscape != 1 {
		t.Fatalf("the PDF has %d portrait and %d landscape pages instead of 1 and 1", portrait, landscape)
	}
	if hits.Load() != 0 {
		t.Fatalf("the page reached its remote host %d times", hits.Load())
	}
	if launched == nil {
		t.Fatal("no egress browser was launched")
	}
	host := strings.TrimPrefix(remote.URL, "http://")
	for _, seen := range append(launched.ProxyDialed(), launched.ProxyRefused()...) {
		if seen == host {
			t.Fatalf("a request for %s got as far as the proxy", host)
		}
	}
}

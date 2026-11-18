package media

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"aurago/internal/uid"
)

var httpClientExplicitMedia = newExplicitMediaClient()

func newExplicitMediaClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	dialer := &net.Dialer{Timeout: 15 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
		target, err := netip.ParseAddrPort(address)
		if err != nil {
			return fmt.Errorf("invalid media download address: %w", err)
		}
		ip := target.Addr().Unmap()
		if !ip.IsGlobalUnicast() && !ip.IsLoopback() {
			return fmt.Errorf("media download address is blocked: %s", ip)
		}
		return nil
	}}
	transport.DialContext = dialer.DialContext
	return &http.Client{Timeout: httpClientMedia.Timeout, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 || req.URL.User != nil {
			return fmt.Errorf("media download redirect refused")
		}
		req.Header.Del("Referer")
		return nil
	}}
}

// SaveExplicitMediaContext saves a document, audio, or video URL. Callers must
// authorize network requests before invoking it. This client permits LAN targets.
func SaveExplicitMediaContext(ctx context.Context, rawURL, destDir string) (string, error) {
	target, err := url.Parse(rawURL)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Hostname() == "" || target.User != nil {
		return "", fmt.Errorf("media download requires an HTTP(S) URL without credentials")
	}
	target.Fragment, target.RawFragment = "", ""
	ext := strings.ToLower(filepath.Ext(SanitizeFileName(target.Path)))
	if url.PathEscape(ext) != ext {
		ext = ""
	}
	return saveAttachmentURLContext(ctx, httpClientExplicitMedia, target.String(), uid.New()+ext, destDir, true)
}

package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"strings"
	"sync"
	"time"

	"aurago/internal/agentmail"
	"aurago/internal/httporigin"
	"aurago/internal/mailer"
)

type mailProbeGate struct {
	mu        sync.Mutex
	active    int
	lastStart time.Time
}

func (g *mailProbeGate) begin(now time.Time) (func(), bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active >= 2 || !g.lastStart.IsZero() && now.Sub(g.lastStart) < time.Second {
		return nil, false
	}
	g.active++
	g.lastStart = now
	return func() { g.mu.Lock(); g.active--; g.mu.Unlock() }, true
}

func (s *Server) beginMailProbe(w http.ResponseWriter) (func(), bool) {
	release, ok := s.mailTests.begin(time.Now())
	if !ok {
		w.Header().Set("Retry-After", "1")
		mailerJSON(w, http.StatusTooManyRequests, map[string]string{"status": "error", "code": "mail_test_busy", "message": "mail_test_busy"})
	}
	return release, ok
}

// Public outcomes never contain a server-controlled response or request value.
func mailConnectionErrorCode(err error) string {
	var api *agentmail.APIError
	var certificate *tls.CertificateVerificationError
	var unknownCA x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalidCert x509.CertificateInvalidError
	var netErr net.Error
	var smtpErr *textproto.Error
	switch {
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, mailer.ErrMailTarget), errors.Is(err, httporigin.ErrCrossOriginRedirect), strings.Contains(err.Error(), "SSRF protection"):
		return "target_blocked"
	case errors.As(err, &certificate), errors.As(err, &unknownCA), errors.As(err, &hostname), errors.As(err, &invalidCert):
		return "certificate_invalid"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return "timeout"
	case errors.As(err, &api):
		switch api.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return "auth_failed"
		case http.StatusTooManyRequests:
			return "rate_limited"
		default:
			return "provider_rejected"
		}
	case errors.As(err, &smtpErr) && (smtpErr.Code == 530 || smtpErr.Code == 534 || smtpErr.Code == 535):
		return "auth_failed"
	case errors.As(err, &netErr), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.ErrClosedPipe), errors.Is(err, net.ErrClosed):
		return "connection_failed"
	case errors.Is(err, mailer.ErrMailAuthentication):
		return "auth_failed"
	default:
		return "connection_failed"
	}
}

func writeMailConnectionError(w http.ResponseWriter, code string) {
	mailerJSON(w, http.StatusBadGateway, map[string]string{"status": "error", "code": code, "message": code})
}

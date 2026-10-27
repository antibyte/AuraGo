package dockerhost

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	"aurago/internal/dockerutil"
)

// Host states of a HostStatus.
const (
	StateOK      = "ok"
	StateError   = "error"
	StateUnknown = "unknown"
)

// Codes that classify why a host is unreachable.
const (
	CodeDNS           = "dns"
	CodeRefused       = "refused"
	CodeTimeout       = "timeout"
	CodeTLS           = "tls"
	CodeAuth          = "auth"
	CodeSSHHostKey    = "ssh_host_key"
	CodeSocketMissing = "socket_missing"
	CodeAPITooOld     = "api_too_old"
	CodeUnknown       = "unknown"
)

// minAPIMinor is the oldest Engine API (1.41, Docker 20.10) the Docker Manager works with.
const minAPIMinor = 41

// HostStatus is the result of the last connection test of a host.
type HostStatus struct {
	State         string    `json:"state"`          // "ok" | "error" | "unknown"
	Code          string    `json:"code,omitempty"` // dns, refused, timeout, tls, auth, ssh_host_key, socket_missing, api_too_old, unknown
	Message       string    `json:"message,omitempty"`
	CheckedAt     time.Time `json:"checked_at,omitempty"`
	EngineVersion string    `json:"engine_version,omitempty"`
	APIVersion    string    `json:"api_version,omitempty"`
	OS            string    `json:"os,omitempty"`
	Arch          string    `json:"arch,omitempty"`
}

// classifiedError carries a code chosen where the failure is understood best.
type classifiedError struct {
	code string
	err  error
}

func (e *classifiedError) Error() string { return e.err.Error() }
func (e *classifiedError) Unwrap() error { return e.err }

func withCode(code string, err error) error { return &classifiedError{code: code, err: err} }

// ClassifyError maps a connection or Engine error to one of the status codes:
// dns, refused, timeout, tls, auth, ssh_host_key, socket_missing, api_too_old,
// or unknown. nil gives "".
func ClassifyError(err error) string {
	if err == nil {
		return ""
	}
	var coded *classifiedError
	if errors.As(err, &coded) {
		return coded.code
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return CodeDNS
	}
	if status := StatusOf(err); status == http.StatusUnauthorized || status == http.StatusForbidden {
		return CodeAuth
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "knownhosts:") || strings.Contains(text, "host key"):
		return CodeSSHHostKey
	case strings.Contains(text, "unable to authenticate") || strings.Contains(text, "no supported methods remain") ||
		strings.Contains(text, "permission denied") || strings.Contains(text, "access denied"):
		return CodeAuth
	case strings.Contains(text, "no supported version") || strings.Contains(text, "unsupported docker api version"):
		return CodeAPITooOld
	case isTLSError(err, text):
		return CodeTLS
	case errors.Is(err, context.DeadlineExceeded) || isNetTimeout(err) ||
		strings.Contains(text, "timeout") || strings.Contains(text, "deadline exceeded") || strings.Contains(text, "timed out"):
		return CodeTimeout
	case strings.Contains(text, "docker over ssh: open "): // sshd refused to forward the Engine socket
		return CodeSocketMissing
	case errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(text, "connection refused") || strings.Contains(text, "actively refused"):
		return CodeRefused
	case strings.Contains(text, "no such host") || strings.Contains(text, "server misbehaving") ||
		strings.Contains(text, "name resolution") || strings.Contains(text, "resolv.conf"):
		// Before the socket test: a resolver that cannot open /etc/resolv.conf
		// reports "open /...: no such file or directory", which is a DNS problem.
		return CodeDNS
	case strings.Contains(text, "open /") || strings.Contains(text, "no such file or directory") ||
		strings.Contains(text, "cannot find the file") || strings.Contains(text, "cannot find the path"):
		return CodeSocketMissing
	}
	return CodeUnknown
}

func isTLSError(err error, text string) bool {
	var verify *tls.CertificateVerificationError
	var authority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalidCert x509.CertificateInvalidError
	var record tls.RecordHeaderError
	if errors.As(err, &verify) || errors.As(err, &authority) || errors.As(err, &hostname) ||
		errors.As(err, &invalidCert) || errors.As(err, &record) {
		return true
	}
	return strings.Contains(text, "tls:") || strings.Contains(text, "x509:") || strings.Contains(text, "docker_tls") ||
		strings.Contains(text, "certificate") || strings.Contains(text, "http response to https client")
}

func isNetTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// errorStatus is the HostStatus of a failed test.
func errorStatus(err error) HostStatus {
	return HostStatus{
		State:     StateError,
		Code:      ClassifyError(err),
		Message:   dockerutil.SanitizeOneLine(err.Error(), 300),
		CheckedAt: time.Now().UTC(),
	}
}

// apiMinor reads the minor number of an Engine API version such as "1.45".
func apiMinor(version string) (int, bool) {
	major, minor, found := strings.Cut(strings.TrimSpace(version), ".")
	if !found || major != "1" {
		return 0, false
	}
	n, err := strconv.Atoi(minor)
	return n, err == nil
}

// probeHost tests the connection of a client: GET /version for the Engine and
// API version (an Engine below API 1.41 is refused), then GET /info for the
// operating system and architecture. A failing /info (a socket proxy may block
// it) keeps the host ok with the /version values.
func probeHost(ctx context.Context, c *dockerutil.Client) HostStatus {
	var version struct {
		Version    string `json:"Version"`
		APIVersion string `json:"ApiVersion"`
		OS         string `json:"Os"`
		Arch       string `json:"Arch"`
	}
	if err := engineJSON(ctx, c, http.MethodGet, "version", &version); err != nil {
		return errorStatus(err)
	}
	minor, ok := apiMinor(version.APIVersion)
	if !ok {
		return errorStatus(withCode(CodeUnknown, fmt.Errorf(
			"the endpoint did not report a usable Docker Engine API version (got %q); is it a Docker Engine?", version.APIVersion)))
	}
	if minor < minAPIMinor {
		return errorStatus(withCode(CodeAPITooOld, fmt.Errorf(
			"Docker Engine API %q is too old; the Docker Manager needs API 1.%d or newer (Docker 20.10+)", version.APIVersion, minAPIMinor)))
	}
	status := HostStatus{
		State:         StateOK,
		CheckedAt:     time.Now().UTC(),
		EngineVersion: version.Version,
		APIVersion:    version.APIVersion,
		OS:            version.OS,
		Arch:          version.Arch,
	}
	var info struct {
		OperatingSystem string `json:"OperatingSystem"`
		OSType          string `json:"OSType"`
		Architecture    string `json:"Architecture"`
	}
	if err := engineJSON(ctx, c, http.MethodGet, "info", &info); err == nil {
		if info.OperatingSystem != "" {
			status.OS = info.OperatingSystem
		} else if info.OSType != "" {
			status.OS = info.OSType
		}
		if info.Architecture != "" {
			status.Arch = info.Architecture
		}
	}
	return status
}

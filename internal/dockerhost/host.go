// Package dockerhost owns the Docker Manager's hosts: the registry of the
// local Engine and of remote Engines (plain TCP, TLS, mutual TLS, SSH), their
// vault secrets, one Engine client per host, connection health, and the Engine
// reads and container actions the Docker Manager API serves.
package dockerhost

import (
	"errors"
	"fmt"
	"net"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"aurago/internal/dockerhost/transport"
)

// Host kinds.
const (
	KindLocal = "local"
	KindTCP   = "tcp"
	KindTLS   = "tls"
	KindMTLS  = "mtls"
	KindSSH   = "ssh"
)

// LocalHostID is the id of the built-in local host. It is never stored.
const LocalHostID = "local"

// Defaults for the per-host settings.
const (
	DefaultStacksDir      = "/opt/stacks"
	DefaultHostSocketPath = "/var/run/docker.sock"
)

// Default ports per kind when a host stores port 0.
const (
	defaultTCPPort = 2375
	defaultTLSPort = 2376
	defaultSSHPort = 22
)

// Colors are the host colors the UI maps to theme tokens.
var Colors = []string{"blue", "teal", "purple", "coral", "pink", "amber", "green", "gray"}

// Host is one Docker Engine the Docker Manager manages.
type Host struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Kind           string    `json:"kind"`
	Address        string    `json:"address"` // local: engine endpoint (from docker.host, read-only); others: hostname/IP (IPv6 may be bracketed)
	Port           int       `json:"port"`    // 0 = default: 2375 tcp, 2376 tls/mtls, 22 ssh
	StacksDir      string    `json:"stacks_dir"`
	HostSocketPath string    `json:"host_socket_path"`
	Color          string    `json:"color"`
	SSHUser        string    `json:"ssh_user,omitempty"`
	Sort           int       `json:"sort"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// SecretInput is write-only. nil fields keep what is stored; Clear removes both
// secrets first. A blank SSHSecret or an empty TLS field also keeps the stored
// value, so a form that leaves the secret fields untouched changes nothing.
//
// One exception protects the SSH secret: it is kept only while the host's
// address, port and user are unchanged (and the host stays an SSH host). When an
// edit or a draft test moves the host to another address, port or user, the
// stored SSH secret is not reused and SSHSecret must be set again, so a stored
// password is never sent to a machine the user did not enter it for. TLS
// material keeps merging, because it only verifies the Engine and signs the
// handshake.
type SecretInput struct {
	SSHSecret *string                // ssh only
	TLS       *transport.TLSMaterial // tls/mtls only
	Clear     bool
}

var (
	// ErrHostNotFound is returned for an id that is not in the registry.
	ErrHostNotFound = errors.New("docker host not found")
	// ErrLocalHostFixed is returned when the built-in local host is edited or deleted.
	ErrLocalHostFixed = errors.New("the local Docker host cannot be changed or deleted")
)

// ValidationError names the field of a host that failed validation.
type ValidationError struct{ Field, Message string }

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

func invalid(field, format string, args ...any) error {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// Normalized returns the host with surrounding blanks trimmed, the color
// lowercased, paths cleaned, empty paths defaulted and the SSH user dropped for
// kinds that do not use one. ValidateHost judges a normalized host.
func (h Host) Normalized() Host {
	h.Name = strings.TrimSpace(h.Name)
	h.Kind = strings.ToLower(strings.TrimSpace(h.Kind))
	h.Address = strings.TrimSpace(h.Address)
	h.Color = strings.ToLower(strings.TrimSpace(h.Color))
	h.SSHUser = strings.TrimSpace(h.SSHUser)
	h.StacksDir = normalizePath(h.StacksDir, DefaultStacksDir)
	h.HostSocketPath = normalizePath(h.HostSocketPath, DefaultHostSocketPath)
	if h.Kind != KindSSH {
		h.SSHUser = ""
	}
	return h
}

// normalizePath trims p and cleans an absolute path. A path with ".." segments
// is left as typed so ValidateHost rejects it instead of Clean resolving it.
func normalizePath(p, fallback string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return fallback
	}
	if strings.HasPrefix(p, "/") && !hasDotDot(p) {
		return path.Clean(p)
	}
	return p
}

func hasDotDot(p string) bool {
	return slices.Contains(strings.Split(p, "/"), "..")
}

// ValidateHost checks a normalized host that is about to be stored or tested:
// name 1..64 runes without control characters; kind one of tcp, tls, mtls, ssh
// (the local host is built in); a host name or IP address without scheme, path
// or port; port 0..65535; stacks_dir and host_socket_path absolute POSIX paths
// without ".." segments, not the root, and without blanks, control characters
// or any of : , = ' " ` and \ (they end up in bind-mount strings, where those
// would split or reshape the mount); a color from Colors; ssh_user for ssh.
func ValidateHost(h Host) error {
	if n := utf8.RuneCountInString(h.Name); n < 1 || n > 64 {
		return invalid("name", "must be 1 to 64 characters")
	}
	if strings.IndexFunc(h.Name, unicode.IsControl) >= 0 {
		return invalid("name", "must not contain control characters")
	}
	switch h.Kind {
	case KindTCP, KindTLS, KindMTLS, KindSSH:
	case KindLocal:
		return invalid("kind", "the local host is built in and cannot be added")
	default:
		return invalid("kind", "must be one of tcp, tls, mtls, ssh")
	}
	if !validAddress(h.Address) {
		return invalid("address", "must be a host name or IP address without scheme, path or port")
	}
	if h.Port < 0 || h.Port > 65535 {
		return invalid("port", "must be between 0 and 65535")
	}
	if err := validateAbsPath("stacks_dir", h.StacksDir); err != nil {
		return err
	}
	if err := validateAbsPath("host_socket_path", h.HostSocketPath); err != nil {
		return err
	}
	if !slices.Contains(Colors, h.Color) {
		return invalid("color", "must be one of %s", strings.Join(Colors, ", "))
	}
	if h.Kind == KindSSH {
		if h.SSHUser == "" || utf8.RuneCountInString(h.SSHUser) > 64 || strings.ContainsAny(h.SSHUser, " \t\r\n") {
			return invalid("ssh_user", "an SSH user name of up to 64 characters without blanks is required")
		}
	}
	return nil
}

func validateAbsPath(field, p string) error {
	switch {
	case p == "" || !strings.HasPrefix(p, "/"):
		return invalid(field, "must be an absolute path")
	case len(p) > 4096 || strings.ContainsAny(p, "\x00\\"):
		return invalid(field, "must be a POSIX path without backslashes")
	case strings.IndexFunc(p, unsafePathRune) >= 0:
		return invalid(field, "must not contain blanks, control characters or any of : , = quotes")
	case hasDotDot(p):
		return invalid(field, "must not contain .. segments")
	case path.Clean(p) == "/":
		return invalid(field, "must not be the filesystem root")
	}
	return nil
}

// unsafePathRune reports a rune that must not appear in a stacks directory or
// socket path: it would split or reshape a bind-mount string
// ("src:dst:opts", comma separated --mount options) or the shell and compose
// lines built from it.
func unsafePathRune(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune(":,=\"'`\\", r)
}

// validAddress accepts a DNS name, an IPv4 address, or an IPv6 address bare or
// in brackets. A scheme, path, query, user info or port is rejected.
func validAddress(a string) bool {
	if a == "" || len(a) > 253 {
		return false
	}
	if strings.HasPrefix(a, "[") {
		if !strings.HasSuffix(a, "]") {
			return false
		}
		ip := net.ParseIP(a[1 : len(a)-1])
		return ip != nil && ip.To4() == nil
	}
	if net.ParseIP(a) != nil {
		return true
	}
	for _, label := range strings.Split(a, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
				return false
			}
		}
	}
	return true
}

// bareAddress strips the brackets of an IPv6 literal.
func bareAddress(a string) string {
	if strings.HasPrefix(a, "[") && strings.HasSuffix(a, "]") {
		return a[1 : len(a)-1]
	}
	return a
}

// ReachesEngineAt reports whether the host is the Engine AuraGo itself uses,
// reached over the network: a tcp, tls or mtls host whose address and effective
// port equal those of dockerHost (the configured docker.host: tcp://host:port,
// http:// or https:// likewise, or a bare host:port; a missing port is the
// default of the scheme). Names compare ignoring case and a trailing dot,
// addresses by value (IPv6 bare or bracketed), and every loopback name or
// address (localhost, 127.0.0.1, ::1) counts as the same one. Sockets, named
// pipes and SSH hosts never match: they have no address to compare.
func (h Host) ReachesEngineAt(dockerHost string) bool {
	switch h.Kind {
	case KindTCP, KindTLS, KindMTLS:
	default:
		return false
	}
	endpoint := strings.TrimSpace(dockerHost)
	lower := strings.ToLower(endpoint)
	if strings.HasPrefix(lower, "unix://") || strings.HasPrefix(lower, "npipe://") {
		return false
	}
	port := defaultTCPPort
	for _, scheme := range []string{"tcp://", "http://", "https://"} {
		if strings.HasPrefix(lower, scheme) {
			endpoint = endpoint[len(scheme):]
			if scheme == "https://" {
				port = defaultTLSPort
			}
			break
		}
	}
	endpoint = strings.TrimRight(endpoint, "/")
	host := endpoint
	if name, portText, err := net.SplitHostPort(endpoint); err == nil {
		n, convErr := strconv.Atoi(portText)
		if convErr != nil {
			return false
		}
		host, port = name, n
	}
	host = strings.TrimSuffix(bareAddress(host), ".")
	if host == "" || port != effectivePort(h) {
		return false
	}
	own := strings.TrimSuffix(bareAddress(h.Address), ".")
	if isLoopbackName(host) && isLoopbackName(own) {
		return true
	}
	return sameAddress(host, own)
}

// isLoopbackName reports localhost or a loopback IP address.
func isLoopbackName(name string) bool {
	if strings.EqualFold(name, "localhost") {
		return true
	}
	ip := net.ParseIP(name)
	return ip != nil && ip.IsLoopback()
}

// effectivePort is the stored port or the default of the kind.
func effectivePort(h Host) int {
	if h.Port > 0 {
		return h.Port
	}
	switch h.Kind {
	case KindTLS, KindMTLS:
		return defaultTLSPort
	case KindSSH:
		return defaultSSHPort
	default:
		return defaultTCPPort
	}
}

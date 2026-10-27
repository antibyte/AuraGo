package dockerhost

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestClassifyError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"dns error type", &net.DNSError{Err: "no such host", Name: "nas.lan", IsNotFound: true}, CodeDNS},
		{"dns timeout is still dns", &net.DNSError{Err: "i/o timeout", Name: "nas.lan", IsTimeout: true}, CodeDNS},
		{"dns text", errors.New("dial tcp: lookup nas.lan: no such host"), CodeDNS},
		{"dns resolver without resolv.conf", errors.New("dial tcp: lookup nas.lan: open /etc/resolv.conf: no such file or directory"), CodeDNS},
		{"refused errno", fmt.Errorf("Docker request: %w", &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}), CodeRefused},
		{"refused text", errors.New("dial tcp 10.0.0.5:2375: connect: connection refused"), CodeRefused},
		{"refused windows text", errors.New("dial tcp 127.0.0.1:2375: connectex: No connection could be made because the target machine actively refused it."), CodeRefused},
		{"deadline", fmt.Errorf("Docker request: %w", context.DeadlineExceeded), CodeTimeout},
		{"i/o timeout text", errors.New("dial tcp 10.0.0.9:2375: i/o timeout"), CodeTimeout},
		{"http client timeout", errors.New("Client.Timeout exceeded while awaiting headers"), CodeTimeout},
		{"unknown authority", fmt.Errorf("Docker request: %w", x509.UnknownAuthorityError{}), CodeTLS},
		{"tls text", errors.New("tls: failed to verify certificate: x509: certificate signed by unknown authority"), CodeTLS},
		{"plain engine on a tls port", errors.New("http: server gave HTTP response to HTTPS client"), CodeTLS},
		{"unusable tls material", errors.New("docker_tls_ca contains no PEM certificate"), CodeTLS},
		{"ssh authentication", errors.New("docker over SSH: ssh handshake failed: ssh: handshake failed: ssh: unable to authenticate, attempted methods [none password], no supported methods remain"), CodeAuth},
		{"engine 401", &StatusError{Status: 401, Err: errors.New("Docker API returned 401: unauthorized")}, CodeAuth},
		{"engine 403 from a socket proxy", &StatusError{Status: 403, Err: errors.New("Docker API returned 403: forbidden")}, CodeAuth},
		{"socket permission", errors.New("dial unix /var/run/docker.sock: connect: permission denied"), CodeAuth},
		{"ssh unknown host key", errors.New("docker over SSH: ssh handshake failed: ssh: handshake failed: knownhosts: key is unknown"), CodeSSHHostKey},
		{"ssh missing known_hosts", errors.New("docker over SSH: failed to get ssh config: SSH host key verification failed: open known_hosts"), CodeSSHHostKey},
		{"ssh socket open rejected", errors.New("docker over SSH: open /var/run/docker.sock: ssh: rejected: connect failed (Connection refused)"), CodeSocketMissing},
		{"unix socket missing", errors.New("dial unix /var/run/docker.sock: connect: no such file or directory"), CodeSocketMissing},
		{"named pipe missing", errors.New("open //./pipe/docker_engine: The system cannot find the file specified."), CodeSocketMissing},
		{"api below the supported range", errors.New("negotiate Docker API: Docker API has no supported version (server 1.24, minimum 1.12, client v1.45)"), CodeAPITooOld},
		{"coded error wins", withCode(CodeAPITooOld, errors.New("connection refused")), CodeAPITooOld},
		{"anything else", errors.New("something odd happened"), CodeUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyError(tc.err); got != tc.want {
				t.Fatalf("ClassifyError(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestProbeHostRefusesEnginesBelowAPI141(t *testing.T) {
	for api, want := range map[string]string{"1.40": CodeAPITooOld, "1.41": "", "1.45": ""} {
		fe := newFakeEngine(t)
		fe.setVersion(api, "20.10.0")
		status := probeHost(context.Background(), fe.client(t))
		if want == "" && status.State != StateOK {
			t.Fatalf("API %s: status = %+v, want ok", api, status)
		}
		if want != "" && (status.State != StateError || status.Code != want) {
			t.Fatalf("API %s: status = %+v, want an error with code %s", api, status, want)
		}
	}
}

func TestProbeHostSaysSoWhenTheEngineReportsNoAPIVersion(t *testing.T) {
	fe := newFakeEngine(t)
	// The unversioned probe of the transport sees API 1.45; the versioned answer lacks the field.
	fe.json("GET", "/version", 200, `{"Version":"27.0.0"}`)
	status := probeHost(context.Background(), fe.client(t))
	if status.State != StateError || status.Code != CodeUnknown {
		t.Fatalf("status = %+v, want an unknown error, not api_too_old", status)
	}
	if strings.Contains(status.Message, "too old") || !strings.Contains(status.Message, "did not report") {
		t.Fatalf("message = %q, want a text that says no version was reported", status.Message)
	}
}

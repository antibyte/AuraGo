// Package transport holds the Docker Engine connection building blocks shared
// by Invasion and the Docker Manager: the TLS client configuration for HTTPS
// Engines and the dialer that reaches an Engine socket through SSH.
package transport

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
)

// Docker TLS modes. The empty mode is the plain-HTTP transport.
const (
	TLSOff    = ""
	TLSServer = "tls"
	TLSMutual = "mtls"
)

// TLSMaterial holds the PEM blocks of an encrypted Docker Engine connection.
// It is stored as JSON in the vault and never in a database.
type TLSMaterial struct {
	CA   string `json:"ca,omitempty"`
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

// String and GoString never print the PEM blocks, so a stray %v, %+v or %#v of
// the material (or of a struct holding it) cannot leak the private key into a
// log or an error message.
func (m TLSMaterial) String() string {
	return fmt.Sprintf("TLSMaterial{ca:%s cert:%s key:%s}", presence(m.CA), presence(m.Cert), presence(m.Key))
}

// GoString implements fmt.GoStringer; see String.
func (m TLSMaterial) GoString() string { return m.String() }

func presence(pem string) string {
	if strings.TrimSpace(pem) == "" {
		return "unset"
	}
	return "set"
}

// TLSClientConfig builds the client TLS configuration of an encrypted Engine
// connection: TLS 1.2 or newer, the stored CA (system roots when none is
// stored) and, for mtls, the client certificate. Verification is never
// disabled. The error texts are shared with Invasion's nest validation.
func TLSClientConfig(mode string, material TLSMaterial) (*tls.Config, error) {
	mode = strings.TrimSpace(mode)
	switch mode {
	case TLSServer, TLSMutual:
	case TLSOff:
		return nil, fmt.Errorf("docker_tls is off; no TLS configuration applies")
	default:
		return nil, fmt.Errorf("invalid docker_tls %q (must be empty, tls or mtls)", mode)
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if ca := strings.TrimSpace(material.CA); ca != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(ca)) {
			return nil, fmt.Errorf("docker_tls_ca contains no PEM certificate")
		}
		cfg.RootCAs = pool
	}
	cert, key := strings.TrimSpace(material.Cert), strings.TrimSpace(material.Key)
	switch mode {
	case TLSMutual:
		if cert == "" || key == "" {
			return nil, fmt.Errorf("docker_tls mtls requires docker_tls_cert and docker_tls_key")
		}
		pair, err := tls.X509KeyPair([]byte(cert), []byte(key))
		if err != nil {
			return nil, fmt.Errorf("docker_tls_cert/docker_tls_key: %w", err)
		}
		cfg.Certificates = []tls.Certificate{pair}
	case TLSServer:
		if cert != "" || key != "" {
			return nil, fmt.Errorf("a client certificate requires docker_tls mtls")
		}
	}
	return cfg, nil
}

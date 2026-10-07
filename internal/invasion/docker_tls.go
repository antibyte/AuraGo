package invasion

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
)

// Docker TLS modes for docker_remote nests. The empty mode is the original
// plain-HTTP transport and stays the default for every existing nest.
const (
	DockerTLSOff    = ""
	DockerTLSServer = "tls"
	DockerTLSMutual = "mtls"
)

// DockerTLSMaterial holds the PEM blocks of an encrypted docker_remote nest.
// It lives in the vault under DockerTLSVaultKey, never in the invasion DB.
type DockerTLSMaterial struct {
	CA   string `json:"ca,omitempty"`
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

// DockerTLSVaultKey names the vault entry that holds a nest's Docker TLS PEMs.
// The nest_ prefix keeps it out of Python tool secret access.
func DockerTLSVaultKey(nestID string) string {
	return "nest_docker_tls_" + nestID
}

// DockerRemoteUsesTLS reports whether a nest talks to its Docker Engine over HTTPS.
func DockerRemoteUsesTLS(nest NestRecord) bool {
	return nest.DeployMethod == "docker_remote" && strings.TrimSpace(nest.DockerTLS) != DockerTLSOff
}

// ValidateDockerTLS checks a mode and its PEM material without contacting anything.
func ValidateDockerTLS(mode string, material DockerTLSMaterial) error {
	if strings.TrimSpace(mode) == DockerTLSOff {
		return nil
	}
	_, err := dockerTLSClientConfig(mode, material)
	return err
}

// dockerTLSClientConfig builds the client TLS configuration of a docker_remote
// nest: TLS 1.2 or newer, the stored CA (system roots when none is stored)
// and, for mtls, the client certificate. Verification is never disabled.
func dockerTLSClientConfig(mode string, material DockerTLSMaterial) (*tls.Config, error) {
	mode = strings.TrimSpace(mode)
	switch mode {
	case DockerTLSServer, DockerTLSMutual:
	case DockerTLSOff:
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
	case DockerTLSMutual:
		if cert == "" || key == "" {
			return nil, fmt.Errorf("docker_tls mtls requires docker_tls_cert and docker_tls_key")
		}
		pair, err := tls.X509KeyPair([]byte(cert), []byte(key))
		if err != nil {
			return nil, fmt.Errorf("docker_tls_cert/docker_tls_key: %w", err)
		}
		cfg.Certificates = []tls.Certificate{pair}
	case DockerTLSServer:
		if cert != "" || key != "" {
			return nil, fmt.Errorf("a client certificate requires docker_tls mtls")
		}
	}
	return cfg, nil
}

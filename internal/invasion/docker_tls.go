package invasion

import (
	"crypto/tls"
	"strings"

	"aurago/internal/dockerhost/transport"
)

// Docker TLS modes for docker_remote nests. The empty mode is the original
// plain-HTTP transport and stays the default for every existing nest.
const (
	DockerTLSOff    = transport.TLSOff
	DockerTLSServer = transport.TLSServer
	DockerTLSMutual = transport.TLSMutual
)

// DockerTLSMaterial holds the PEM blocks of an encrypted docker_remote nest.
// It lives in the vault under DockerTLSVaultKey, never in the invasion DB.
// The type is shared with the Docker Manager's host registry.
type DockerTLSMaterial = transport.TLSMaterial

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
// and, for mtls, the client certificate. Verification is never disabled. The
// work is done by transport.TLSClientConfig, which the Docker Manager shares.
func dockerTLSClientConfig(mode string, material DockerTLSMaterial) (*tls.Config, error) {
	return transport.TLSClientConfig(mode, material)
}

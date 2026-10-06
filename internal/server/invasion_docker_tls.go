package server

import (
	"encoding/json"
	"errors"
	"strings"

	"aurago/internal/invasion"
	"aurago/internal/security"
)

// nestDockerTLSRequest is embedded in the nest create and update requests.
// DockerTLS is a pointer so an update that omits the field (an older client)
// keeps the nest's current mode instead of downgrading it to plain HTTP.
type nestDockerTLSRequest struct {
	DockerTLS     *string `json:"docker_tls"`
	DockerTLSCA   string  `json:"docker_tls_ca"`
	DockerTLSCert string  `json:"docker_tls_cert"`
	DockerTLSKey  string  `json:"docker_tls_key"`
}

// resolveNestDockerTLS decides a nest's TLS mode and vault material after a
// create or update. Non-docker_remote nests always end in plain mode. Empty
// PEM fields keep stored values. A nil material means "nothing to store".
// Errors are client errors (HTTP 400).
func resolveNestDockerTLS(deployMethod, currentMode string, stored invasion.DockerTLSMaterial, req nestDockerTLSRequest) (string, *invasion.DockerTLSMaterial, error) {
	mode := strings.TrimSpace(currentMode)
	if req.DockerTLS != nil {
		mode = strings.TrimSpace(*req.DockerTLS)
	}
	ca := strings.TrimSpace(req.DockerTLSCA)
	cert := strings.TrimSpace(req.DockerTLSCert)
	key := strings.TrimSpace(req.DockerTLSKey)
	if deployMethod != "docker_remote" {
		if req.DockerTLS != nil && mode != invasion.DockerTLSOff {
			return "", nil, errors.New("docker_tls requires deploy_method docker_remote")
		}
		if ca != "" || cert != "" || key != "" {
			return "", nil, errors.New("docker_tls_ca, docker_tls_cert and docker_tls_key require deploy_method docker_remote")
		}
		return invasion.DockerTLSOff, nil, nil
	}
	if mode == invasion.DockerTLSOff {
		if ca != "" || cert != "" || key != "" {
			return "", nil, errors.New("docker_tls_ca, docker_tls_cert and docker_tls_key require docker_tls tls or mtls")
		}
		return invasion.DockerTLSOff, nil, nil
	}
	material := stored
	if ca != "" {
		material.CA = ca
	}
	if cert != "" {
		material.Cert = cert
	}
	if key != "" {
		material.Key = key
	}
	if mode == invasion.DockerTLSServer {
		if cert != "" || key != "" {
			return "", nil, errors.New("a client certificate requires docker_tls mtls")
		}
		material.Cert, material.Key = "", ""
	}
	if err := invasion.ValidateDockerTLS(mode, material); err != nil {
		return "", nil, err
	}
	if material == (invasion.DockerTLSMaterial{}) {
		return mode, nil, nil
	}
	return mode, &material, nil
}

// errDockerTLSMaterialUnreadable reports a vault entry that does not decode as
// Docker TLS material. The decoder's own error can quote stored bytes, so it is
// never wrapped.
var errDockerTLSMaterialUnreadable = errors.New("stored Docker TLS material is unreadable")

// dockerTLSUnreadableMessage is the client message for an update that would
// keep relying on unreadable stored material.
const dockerTLSUnreadableMessage = "stored Docker TLS material is unreadable; paste the CA again or switch Docker TLS off"

// dockerTLSUpdateReplacesStoredMaterial reports whether an update can proceed
// although the stored Docker TLS material is unreadable. A tls nest has a vault
// entry only when a CA was pinned, so the update must not silently fall back to
// the system roots. It proceeds only when it switches Docker TLS off, moves the
// nest away from docker_remote, or sends docker_tls_ca (plus the client
// certificate and key for mtls); the entry is then removed or rewritten.
func dockerTLSUpdateReplacesStoredMaterial(deployMethod, currentMode string, req nestDockerTLSRequest) bool {
	if deployMethod != "docker_remote" {
		return true
	}
	mode := strings.TrimSpace(currentMode)
	if req.DockerTLS != nil {
		mode = strings.TrimSpace(*req.DockerTLS)
	}
	if mode == invasion.DockerTLSOff {
		return true
	}
	if strings.TrimSpace(req.DockerTLSCA) == "" {
		return false
	}
	if mode == invasion.DockerTLSMutual {
		return strings.TrimSpace(req.DockerTLSCert) != "" && strings.TrimSpace(req.DockerTLSKey) != ""
	}
	return true // tls; resolveNestDockerTLS rejects unknown modes
}

// loadNestDockerTLS reads a nest's Docker TLS material; a missing entry is
// empty material (TLS against the system roots).
func (s *Server) loadNestDockerTLS(nestID string) (invasion.DockerTLSMaterial, error) {
	var material invasion.DockerTLSMaterial
	if s.Vault == nil {
		return material, errors.New("vault is not available")
	}
	raw, err := s.Vault.ReadSecret(invasion.DockerTLSVaultKey(nestID))
	if errors.Is(err, security.ErrSecretNotFound) {
		return material, nil
	}
	if err != nil {
		return material, err
	}
	if err := json.Unmarshal([]byte(raw), &material); err != nil {
		return invasion.DockerTLSMaterial{}, errDockerTLSMaterialUnreadable
	}
	return material, nil
}

func (s *Server) storeNestDockerTLS(nestID string, material invasion.DockerTLSMaterial) error {
	if s.Vault == nil {
		return errors.New("vault is not available")
	}
	raw, err := json.Marshal(material)
	if err != nil {
		return err
	}
	return s.Vault.WriteSecret(invasion.DockerTLSVaultKey(nestID), string(raw))
}

func (s *Server) deleteNestDockerTLS(nestID string) error {
	if s.Vault == nil {
		return nil
	}
	return s.Vault.DeleteSecret(invasion.DockerTLSVaultKey(nestID))
}

// applyNestDockerTLS writes changed material or removes material that is no
// longer used. previous is what the vault held before the request.
func (s *Server) applyNestDockerTLS(nestID string, previous invasion.DockerTLSMaterial, next *invasion.DockerTLSMaterial) error {
	switch {
	case next != nil && *next != previous:
		return s.storeNestDockerTLS(nestID, *next)
	case next == nil && previous != (invasion.DockerTLSMaterial{}):
		return s.deleteNestDockerTLS(nestID)
	}
	return nil
}

// invasionTransportSecret returns the credential a nest's deploy transport
// uses: the Docker TLS material for an encrypted docker_remote nest,
// otherwise the nest's vault secret exactly as before (nil when none is
// stored). Vault errors are returned unwrapped so callers keep their messages.
func (s *Server) invasionTransportSecret(nest invasion.NestRecord) ([]byte, error) {
	if invasion.DockerRemoteUsesTLS(nest) {
		material, err := s.loadNestDockerTLS(nest.ID)
		if err != nil {
			return nil, err
		}
		return json.Marshal(material)
	}
	if nest.VaultSecretID == "" {
		return nil, nil
	}
	secret, err := s.Vault.ReadSecret(nest.VaultSecretID)
	if err != nil {
		return nil, err
	}
	return []byte(secret), nil
}

// nestDockerTLSSnapshot is the raw vault entry a nest update is about to
// replace. taken is false when it could not be read; nothing is restored then.
type nestDockerTLSSnapshot struct {
	taken, exists bool
	raw           string
}

func (s *Server) snapshotNestDockerTLS(nestID string) nestDockerTLSSnapshot {
	if s.Vault == nil {
		return nestDockerTLSSnapshot{}
	}
	raw, err := s.Vault.ReadSecret(invasion.DockerTLSVaultKey(nestID))
	switch {
	case err == nil:
		return nestDockerTLSSnapshot{taken: true, exists: true, raw: raw}
	case errors.Is(err, security.ErrSecretNotFound):
		return nestDockerTLSSnapshot{taken: true}
	}
	return nestDockerTLSSnapshot{}
}

// restoreNestDockerTLSSnapshot puts back the entry a failed nest update
// replaced, so the unchanged mode keeps the material it was saved with. Best
// effort: on failure the nest keeps the new material and fails closed if it
// does not fit the mode.
func (s *Server) restoreNestDockerTLSSnapshot(nestID string, snap nestDockerTLSSnapshot) {
	if !snap.taken || s.Vault == nil {
		return
	}
	key := invasion.DockerTLSVaultKey(nestID)
	var err error
	if snap.exists {
		err = s.Vault.WriteSecret(key, snap.raw)
	} else {
		err = s.Vault.DeleteSecret(key)
	}
	if err != nil && s.Logger != nil {
		s.Logger.Warn("Failed to restore invasion nest Docker TLS material after a failed update", "nest_id", nestID, "error", err)
	}
}

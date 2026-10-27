package server

import (
	"errors"
	"net/http"
	"strings"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerhost/transport"
	"aurago/internal/invasion"
	"aurago/internal/security"
)

// ── Invasion nest import ────────────────────────────────────────────────────
// A Docker nest (deploy method docker_remote or docker_ssh) becomes a Docker
// Manager host with one click. The vault secret is copied, never shared, so
// deleting either side leaves the other intact.

var (
	errNestLocalEngine = errors.New("this nest uses the local Docker engine, which is already the Local host")
	errNestNotDocker   = errors.New("this nest does not deploy through Docker")
)

// dockerNestView is one row of GET /nests.
type dockerNestView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DeployMethod string `json:"deploy_method"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	DockerTLS    string `json:"docker_tls"`
}

func isDockerNest(nest invasion.NestRecord) bool {
	switch nest.DeployMethod {
	case "docker_local", "docker_remote", "docker_ssh":
		return true
	}
	return false
}

// nestList serves GET /nests: the Invasion nests that deploy through Docker.
// Without Invasion Control the list is empty.
func (dm *dmRequest) nestList() {
	if !dm.allow(http.MethodGet) {
		return
	}
	views := []dockerNestView{}
	if dm.s.InvasionDB != nil {
		nests, err := invasion.ListNests(dm.s.InvasionDB)
		if err != nil {
			dm.internalFailure("list invasion nests", err)
			return
		}
		for _, nest := range nests {
			if !isDockerNest(nest) {
				continue
			}
			views = append(views, dockerNestView{
				ID: nest.ID, Name: nest.Name, DeployMethod: nest.DeployMethod,
				Host: nest.Host, Port: nest.Port, DockerTLS: nest.DockerTLS,
			})
		}
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"nests": views})
}

// findNest looks the nest up in the full list; GetNest has no typed
// not-found error.
func (dm *dmRequest) findNest(id string) (invasion.NestRecord, bool) {
	if dm.s.InvasionDB == nil {
		dmWriteError(dm.w, http.StatusNotFound, dmCodeNestNotFound, "Invasion Control is not enabled", nil)
		return invasion.NestRecord{}, false
	}
	nests, err := invasion.ListNests(dm.s.InvasionDB)
	if err != nil {
		dm.internalFailure("list invasion nests", err)
		return invasion.NestRecord{}, false
	}
	for _, nest := range nests {
		if nest.ID == id {
			return nest, true
		}
	}
	dmWriteError(dm.w, http.StatusNotFound, dmCodeNestNotFound, "nest not found", nil)
	return invasion.NestRecord{}, false
}

// dockerHostFromNest maps a nest onto a host draft; the registry fills the
// color and validates the result. docker_local is refused: it points at the
// local engine. An unsupported docker_tls value is a validation error.
func dockerHostFromNest(nest invasion.NestRecord) (dockerhost.Host, error) {
	name := dmTrimRunes(nest.Name, 64)
	if name == "" {
		name = dmTrimRunes(nest.ID, 64)
	}
	host := dockerhost.Host{
		Name:           name,
		Address:        strings.TrimSpace(nest.Host),
		Port:           nest.Port,
		StacksDir:      dockerhost.DefaultStacksDir,
		HostSocketPath: dockerhost.DefaultHostSocketPath,
	}
	switch nest.DeployMethod {
	case "docker_local":
		return dockerhost.Host{}, errNestLocalEngine
	case "docker_remote":
		switch strings.TrimSpace(nest.DockerTLS) {
		case invasion.DockerTLSOff:
			host.Kind = dockerhost.KindTCP
		case invasion.DockerTLSServer:
			host.Kind = dockerhost.KindTLS
		case invasion.DockerTLSMutual:
			host.Kind = dockerhost.KindMTLS
		default:
			return dockerhost.Host{}, &dockerhost.ValidationError{Field: "kind", Message: "the nest uses an unsupported docker_tls mode"}
		}
	case "docker_ssh":
		host.Kind = dockerhost.KindSSH
		host.SSHUser = strings.TrimSpace(nest.Username)
	default:
		return dockerhost.Host{}, errNestNotDocker
	}
	return host, nil
}

func dmTrimRunes(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) > limit {
		runes = runes[:limit]
	}
	return string(runes)
}

// nestSecrets copies the nest's credential: the TLS bundle of an encrypted
// docker_remote nest, or the SSH secret of a docker_ssh nest. A missing vault
// entry copies nothing.
func (dm *dmRequest) nestSecrets(nest invasion.NestRecord, kind string) (dockerhost.SecretInput, error) {
	var sec dockerhost.SecretInput
	if dm.s.Vault == nil {
		return sec, nil
	}
	switch kind {
	case dockerhost.KindSSH:
		if nest.VaultSecretID == "" {
			return sec, nil
		}
		value, err := dm.s.Vault.ReadSecret(nest.VaultSecretID)
		if errors.Is(err, security.ErrSecretNotFound) {
			return sec, nil
		}
		if err != nil {
			return sec, err
		}
		if value != "" {
			sec.SSHSecret = &value
		}
	case dockerhost.KindTLS, dockerhost.KindMTLS:
		material, err := dm.s.loadNestDockerTLS(nest.ID)
		if err != nil {
			return sec, err
		}
		if material != (invasion.DockerTLSMaterial{}) {
			copied := transport.TLSMaterial(material)
			sec.TLS = &copied
		}
	}
	return sec, nil
}

// nestImport serves POST /hosts/import-nest/{nestID}.
func (dm *dmRequest) nestImport(nestID string) {
	if !dm.allow(http.MethodPost) {
		return
	}
	nest, ok := dm.findNest(nestID)
	if !ok {
		return
	}
	host, err := dockerHostFromNest(nest)
	switch {
	case errors.Is(err, errNestLocalEngine), errors.Is(err, errNestNotDocker):
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeNestNotDocker, err.Error(), nil)
		return
	case err != nil:
		dm.hostFailure(err)
		return
	}
	ctx, cancel := dm.context(dmRegistryTimeout)
	defer cancel()
	sec, err := dm.nestSecrets(nest, host.Kind)
	if errors.Is(err, errDockerTLSMaterialUnreadable) {
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeInvalidHost, dockerTLSUnreadableMessage, dmFields{"field": "tls"})
		return
	}
	if err != nil {
		dm.internalFailure("read the nest credential", err)
		return
	}
	created, err := dm.registry().Create(ctx, host, sec)
	if err != nil {
		dm.hostFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusCreated, dmFields{"host": dm.hostView(created, dm.registry().Status(created.ID))})
}

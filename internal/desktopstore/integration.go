package desktopstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"aurago/internal/security"
)

// IntegrationSettings is stored only in the Vault. Legacy integrations keep
// their credential in their existing Vault key, never in Credential.
type IntegrationSettings struct {
	Revision               string   `json:"revision"`
	URL                    string   `json:"url"`
	Username               string   `json:"username,omitempty"`
	Credential             string   `json:"credential,omitempty"`
	Read                   bool     `json:"read"`
	Execute                bool     `json:"execute"`
	Create                 bool     `json:"create"`
	Process                bool     `json:"process"`
	Sign                   bool     `json:"sign"`
	Notify                 bool     `json:"notify"`
	AllowedIDs             []string `json:"allowed_ids,omitempty"`
	Source                 string   `json:"source,omitempty"`
	CertificateFingerprint string   `json:"certificate_fingerprint,omitempty"`
	AdminPort              string   `json:"admin_port,omitempty"`
	DNSAddress             string   `json:"dns_address,omitempty"`
	DockerNetwork          string   `json:"docker_network,omitempty"`
}

var IntegrationApps = []string{"adguard-home", "uptime-kuma", "n8n", "node-red", "olivetin", "open-webui", "bytestash", "beszel", "filebrowser-quantum", "stirling-pdf", "ziro-designer"}

func HasIntegration(id string) bool {
	for _, app := range IntegrationApps {
		if app == id {
			return true
		}
	}
	return false
}

func IntegrationKey(id string) string { return "desktop_store_integration_" + id }

// DNS sockets belong to the LAN profile, never to the HTTP preview/proxy.
func IsHTTPPort(port PortBinding) bool {
	return (port.Protocol == "" || port.Protocol == "tcp") && port.ID != "dns-tcp" && port.ID != "dns-udp"
}

func (s *Service) integrationAdminReady(ctx context.Context, app InstalledApp) bool {
	if app.AppID != "adguard-home" {
		return false
	}
	for _, port := range app.Ports {
		if port.ID != "admin" {
			continue
		}
		host, number := port.HostIP, port.HostPort
		if s.cfg.Containerized {
			host, number = app.ContainerName, port.ContainerPort
		}
		if s.portProbe(ctx, host, number) {
			return true
		}
	}
	return false
}

func ReadIntegration(secrets SecretStore, id string) (IntegrationSettings, error) {
	var settings IntegrationSettings
	if !HasIntegration(id) || secrets == nil {
		return settings, errors.New("store integration unavailable")
	}
	raw, err := secrets.ReadSecret(IntegrationKey(id))
	if err != nil {
		if errors.Is(err, security.ErrSecretNotFound) {
			return settings, nil
		}
		return settings, errors.New("cannot read store integration settings")
	}
	if raw == "" {
		return settings, nil
	}
	if json.Unmarshal([]byte(raw), &settings) != nil {
		return settings, errors.New("invalid store integration settings")
	}
	return settings, nil
}

// IntegrationOperation reserves the same per-app exclusion as lifecycle work.
// Its completion must run even when a connection test or persistence fails.
func (s *Service) IntegrationOperation(ctx context.Context, id string) (func(error), error) {
	if !HasIntegration(id) {
		return nil, errors.New("unsupported store integration")
	}
	op, err := s.createOperation(ctx, "integration", id, OperationRequest{})
	if err != nil {
		return nil, err
	}
	return func(cause error) {
		status, message := OperationSucceeded, ""
		if cause != nil {
			status, message = OperationFailed, "Store integration configuration failed"
		}
		finishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.updateOperation(finishCtx, op.ID, status, "", message)
	}, nil
}

// IntegrationIdle is also checked at dispatch, so a newly admitted operation
// blocks old agent snapshots before its background worker starts.
func (s *Service) IntegrationIdle(ctx context.Context, id string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rejectActiveOperationLocked(ctx, id, time.Now().UTC())
}

// IntegrationURL resolves only a known published port. Remote Docker and
// container networking are validated by the server against the actual daemon.
func IntegrationURL(app InstalledApp, portID string, containerized bool) (string, error) {
	ports := app.Ports
	if len(ports) == 0 {
		ports = []PortBinding{{ID: "main", HostIP: app.HostIP, HostPort: app.HostPort, ContainerPort: app.ContainerPort, Protocol: app.Protocol}}
	}
	for i, p := range ports {
		if (portID == "" && i == 0) || p.ID == portID {
			if p.Protocol != "tcp" || p.HostPort < 1 {
				break
			}
			host, port := p.HostIP, p.HostPort
			if host == "" || host == "0.0.0.0" || host == "::" {
				host = "127.0.0.1"
			}
			if containerized {
				host, port = app.ContainerName, p.ContainerPort
			}
			return "http://" + net.JoinHostPort(host, strconv.Itoa(port)), nil
		}
	}
	return "", fmt.Errorf("Store update required: admin port is not published")
}

const storeNetworkEnv = "AURAGO_STORE_DOCKER_NETWORK="

func IntegrationNetwork(app InstalledApp) string {
	for _, env := range app.Env {
		if strings.HasPrefix(env, storeNetworkEnv) {
			return strings.TrimPrefix(env, storeNetworkEnv)
		}
	}
	return ""
}

func (s *Service) applyIntegrationRuntime(app *InstalledApp, network string) error {
	if !HasIntegration(app.AppID) {
		return nil
	}
	settings, err := ReadIntegration(s.cfg.Secrets, app.AppID)
	if err != nil && s.cfg.Secrets != nil {
		return err
	}
	if settings.DockerNetwork != "" {
		network = settings.DockerNetwork
	}
	if !validPrintGuardNetwork(network) {
		return errors.New("invalid shared Docker network")
	}
	filtered := app.Env[:0]
	for _, value := range app.Env {
		if !strings.HasPrefix(value, storeNetworkEnv) {
			filtered = append(filtered, value)
		}
	}
	app.Env = filtered
	if network != "" {
		app.Env = append(app.Env, storeNetworkEnv+network)
	}
	if app.AppID == "adguard-home" && settings.DNSAddress != "" {
		ip := net.ParseIP(settings.DNSAddress)
		if ip == nil || !ip.IsGlobalUnicast() || ip.IsLoopback() {
			return errors.New("DNS publishing requires a specific LAN address")
		}
		for _, protocol := range []string{"tcp", "udp"} {
			app.Ports = append(app.Ports, PortBinding{ID: "dns-" + protocol, Name: "DNS", ContainerPort: 53, HostPort: 53, HostIP: ip.String(), Protocol: protocol})
		}
	}
	return nil
}

package desktopstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"aurago/internal/security"
)

const PrintGuardAppID = "printguard"
const PrintGuardMCPName = "printguard-store"
const printGuardVaultKey = "desktop_store_printguard_config"
const printGuardRevisionEnv = "AURAGO_PRINTGUARD_REVISION"

// A previous process's successful probe cannot authorize this process's connection.
var printGuardAgentEpoch = newID("pg-auth")
var printGuardRevocations atomic.Uint64

// Release 2.6.3 registry manifests, resolved anonymously from GHCR on 2026-10-23.
var printGuardImages = map[string]string{
	"cpu":    "ghcr.io/oliverbravery/printguard:2.6.3@sha256:c70168c1703c29a447d19917d3296ef7093ebb8d2fbe267879e8cecc284a11a3",
	"intel":  "ghcr.io/oliverbravery/printguard:2.6.3-intel@sha256:afa75b109c030c291cf006334f4ee754698aefd02c116b2d714b96d00e7c2192",
	"nvidia": "ghcr.io/oliverbravery/printguard:2.6.3-nvidia@sha256:c7cdf6c6a33982b0f7f8500273f8140f401d11b68a159279b3d4d9e149e7300f",
}

type printGuardRuntime struct {
	Revision      string   `json:"revision"`
	Acceleration  string   `json:"acceleration"`
	Image         string   `json:"image"`
	DockerNetwork string   `json:"docker_network,omitempty"`
	Origins       []string `json:"origins"`
}

type printGuardSettings struct {
	Desired   printGuardRuntime `json:"desired"`
	Previous  printGuardRuntime `json:"previous"`
	AgentMode string            `json:"agent_mode"`
	APIToken  string            `json:"api_token"`
	Verified  string            `json:"verified"`
}

type PrintGuardConfigUpdate struct {
	Acceleration  *string  `json:"acceleration,omitempty"`
	DockerNetwork *string  `json:"docker_network,omitempty"`
	AgentMode     *string  `json:"agent_mode,omitempty"`
	APIToken      string   `json:"api_token,omitempty"`
	ClearAPIToken bool     `json:"clear_api_token,omitempty"`
	Origins       []string `json:"-"`
}

type PrintGuardConfigStatus struct {
	Installed          bool   `json:"installed"`
	Acceleration       string `json:"acceleration"`
	ActiveAcceleration string `json:"active_acceleration,omitempty"`
	DockerNetwork      string `json:"docker_network"`
	AgentMode          string `json:"agent_mode"`
	TokenConfigured    bool   `json:"token_configured"`
	AgentVerified      bool   `json:"agent_verified"`
	Pending            bool   `json:"pending"`
}

type printGuardSecretReader interface{ ReadSecret(string) (string, error) }

func readPrintGuardSettings(vault printGuardSecretReader) (printGuardSettings, error) {
	settings := printGuardSettings{Desired: printGuardRuntime{Acceleration: "cpu"}, AgentMode: "off"}
	if vault == nil {
		return settings, fmt.Errorf("PrintGuard requires the AuraGo vault")
	}
	raw, err := vault.ReadSecret(printGuardVaultKey)
	if errors.Is(err, security.ErrSecretNotFound) || (err == nil && raw == "") {
		return settings, nil
	}
	if err != nil {
		return settings, fmt.Errorf("read PrintGuard settings from vault")
	}
	if json.Unmarshal([]byte(raw), &settings) != nil || printGuardImages[settings.Desired.Acceleration] == "" || !slices.Contains([]string{"off", "read", "control"}, settings.AgentMode) {
		return settings, fmt.Errorf("invalid PrintGuard vault settings")
	}
	security.RegisterSensitive(settings.APIToken)
	return settings, nil
}

func (s *Service) writePrintGuardSettings(settings printGuardSettings) error {
	security.RegisterSensitive(settings.APIToken)
	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	if err := s.cfg.Secrets.WriteSecret(printGuardVaultKey, string(raw)); err != nil {
		return fmt.Errorf("save PrintGuard settings in vault")
	}
	return nil
}

func (settings printGuardSettings) runtimeFor(app InstalledApp) (printGuardRuntime, error) {
	revision, _ := envValue(app.Env, printGuardRevisionEnv)
	for _, cfg := range []printGuardRuntime{settings.Desired, settings.Previous} {
		if revision != "" && revision == cfg.Revision && cfg.Image != "" && cfg.Image == app.Image {
			return cfg, nil
		}
	}
	return printGuardRuntime{}, fmt.Errorf("PrintGuard active configuration is unavailable")
}

func (s *Service) PrintGuardConfiguration(ctx context.Context) (PrintGuardConfigStatus, error) {
	settings, err := readPrintGuardSettings(s.cfg.Secrets)
	if err != nil {
		return PrintGuardConfigStatus{}, err
	}
	status := PrintGuardConfigStatus{Acceleration: settings.Desired.Acceleration, DockerNetwork: settings.Desired.DockerNetwork, AgentMode: settings.AgentMode, TokenConfigured: settings.APIToken != ""}
	app, exists, err := s.GetInstalled(ctx, PrintGuardAppID)
	if err != nil {
		return status, err
	}
	status.Installed = exists
	if exists {
		active, err := settings.runtimeFor(app)
		if err != nil {
			return status, err
		}
		status.ActiveAcceleration = active.Acceleration
		status.Pending = active.Revision != settings.Desired.Revision
		status.AgentVerified = settings.Verified != "" && settings.Verified == printGuardAgentSignature(settings, app)
	}
	return status, nil
}

func (s *Service) ConfigurePrintGuard(ctx context.Context, req PrintGuardConfigUpdate) (Operation, error) {
	if req.Acceleration != nil && printGuardImages[*req.Acceleration] == "" {
		return Operation{}, fmt.Errorf("unsupported PrintGuard acceleration profile")
	}
	if req.AgentMode != nil && !slices.Contains([]string{"off", "read", "control"}, *req.AgentMode) {
		return Operation{}, fmt.Errorf("unsupported PrintGuard agent mode")
	}
	if len(req.APIToken) > 4096 || strings.IndexFunc(req.APIToken, unicode.IsControl) >= 0 || (req.ClearAPIToken && req.APIToken != "") {
		return Operation{}, fmt.Errorf("invalid PrintGuard API token")
	}
	if req.DockerNetwork != nil && !validPrintGuardNetwork(*req.DockerNetwork) {
		return Operation{}, fmt.Errorf("invalid PrintGuard Docker network")
	}
	op, err := s.createOperation(ctx, OperationConfigure, PrintGuardAppID, OperationRequest{})
	if err != nil {
		return Operation{}, err
	}
	fail := func(err error) (Operation, error) {
		_ = s.updateOperation(ctx, op.ID, OperationFailed, "", err.Error())
		return Operation{}, err
	}
	s.printGuardMu.Lock()
	defer s.printGuardMu.Unlock()
	app, exists, err := s.GetInstalled(ctx, PrintGuardAppID)
	if err != nil || !exists {
		return fail(fmt.Errorf("PrintGuard is not installed"))
	}
	settings, err := readPrintGuardSettings(s.cfg.Secrets)
	if err != nil {
		return fail(err)
	}
	origins := settings.Desired.Origins
	if req.Origins != nil {
		origins, err = validateGodsEyeOrigins(req.Origins)
	}
	if err != nil || len(origins) == 0 {
		return fail(fmt.Errorf("PrintGuard requires exact AuraGo and app origins"))
	}
	active, err := settings.runtimeFor(app)
	if err != nil {
		return fail(err)
	}
	next := settings.Desired
	if req.Acceleration != nil {
		next.Acceleration = *req.Acceleration
	}
	if req.DockerNetwork != nil {
		next.DockerNetwork = *req.DockerNetwork
	}
	next.Image = printGuardImages[next.Acceleration]
	next.Origins = origins
	if next.Image != active.Image || next.Acceleration != active.Acceleration || next.DockerNetwork != active.DockerNetwork || !slices.Equal(next.Origins, active.Origins) {
		next.Revision = newID("pg")
	} else {
		next.Revision = active.Revision
	}
	settings.Previous, settings.Desired = active, next
	if req.AgentMode != nil {
		settings.AgentMode = *req.AgentMode
	}
	if token := strings.TrimSpace(req.APIToken); token != "" {
		settings.APIToken = token
	}
	if req.ClearAPIToken {
		settings.APIToken = ""
		settings.AgentMode = "off"
	}
	if settings.AgentMode != "off" && settings.APIToken == "" {
		return fail(fmt.Errorf("create an API token in PrintGuard before enabling agent access"))
	}
	settings.Verified = ""
	if err := s.writePrintGuardSettings(settings); err != nil {
		return fail(err)
	}
	return op, nil
}

func validPrintGuardNetwork(network string) bool {
	if network == "" {
		return true
	}
	return len(network) <= 128 && network != "host" && network != "none" && network != "bridge" && strings.IndexFunc(network, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("_.-", r))
	}) < 0
}

func (s *Service) preparePrintGuardInstall(req InstallRequest) error {
	s.printGuardMu.Lock()
	defer s.printGuardMu.Unlock()
	settings, err := readPrintGuardSettings(s.cfg.Secrets)
	if err != nil {
		return err
	}
	settings.Desired.Origins, err = validateGodsEyeOrigins(req.AllowedOrigins)
	if err != nil || len(settings.Desired.Origins) == 0 {
		return fmt.Errorf("PrintGuard requires protected app origins")
	}
	settings.Desired.DockerNetwork = req.DockerNetwork
	settings.Desired.Acceleration = "cpu"
	settings.Desired.Image = printGuardImages["cpu"]
	settings.Desired.Revision = newID("pg")
	settings.Verified = ""
	return s.writePrintGuardSettings(settings)
}

func (s *Service) printGuardEnvironment() ([]string, []SecretRef, error) {
	settings, err := readPrintGuardSettings(s.cfg.Secrets)
	if err != nil {
		return nil, nil, err
	}
	return []string{printGuardRevisionEnv + "=" + settings.Desired.Revision}, nil, nil
}

func (s *Service) applyPrintGuardImage(app *InstalledApp) error {
	settings, err := readPrintGuardSettings(s.cfg.Secrets)
	if err != nil {
		return err
	}
	app.Image = settings.Desired.Image
	return nil
}

func (s *Service) preparePrintGuardUpdate(app InstalledApp) error {
	s.printGuardMu.Lock()
	defer s.printGuardMu.Unlock()
	settings, err := readPrintGuardSettings(s.cfg.Secrets)
	if err != nil {
		return err
	}
	active, err := settings.runtimeFor(app)
	if err != nil {
		return err
	}
	image := printGuardImages[settings.Desired.Acceleration]
	if settings.Desired.Image == image {
		return nil
	}
	settings.Previous = active
	settings.Desired.Image, settings.Desired.Revision = image, newID("pg")
	settings.Verified = ""
	return s.writePrintGuardSettings(settings)
}

func (s *Service) printGuardContainerSpec(app InstalledApp, spec ContainerSpec) (ContainerSpec, error) {
	settings, err := readPrintGuardSettings(s.cfg.Secrets)
	if err != nil {
		return spec, err
	}
	cfg, err := settings.runtimeFor(app)
	if err != nil {
		return spec, err
	}
	if app.BindMode != BindModeLocal || app.HostIP != "127.0.0.1" || app.TailscaleEnabled || !validPrintGuardNetwork(cfg.DockerNetwork) {
		return spec, fmt.Errorf("PrintGuard requires a private protected runtime")
	}
	spec.NetworkMode = cfg.DockerNetwork
	spec.Env = append(spec.Env, "PRINTGUARD_ORIGINS="+strings.Join(cfg.Origins, ","))
	switch cfg.Acceleration {
	case "intel":
		spec.Devices = []DeviceMapping{{PathOnHost: "/dev/dri", PathInContainer: "/dev/dri", CgroupPermissions: "rwm"}}
	case "nvidia":
		spec.DeviceRequests = []DeviceRequest{{Driver: "nvidia", Count: -1, Capabilities: [][]string{{"gpu", "compute", "utility"}}}}
	}
	return spec, nil
}

func (s *Service) configurePrintGuardRuntime(ctx context.Context, op Operation) error {
	status, err := s.PrintGuardConfiguration(ctx)
	if err != nil {
		return err
	}
	if !status.Pending {
		return nil
	}
	return s.update(ctx, op)
}

func appReadinessTimeoutFor(app InstalledApp) time.Duration {
	if app.AppID == PrintGuardAppID {
		return 5 * time.Minute
	}
	return appReadinessTimeout
}

// PrintGuardAgentBinding contains only a proven Store runtime; never serialize its token.
type PrintGuardAgentBinding struct {
	Mode          string
	Token         string `json:"-"`
	HostPort      int
	ContainerName string
	DockerNetwork string
	Signature     string
	Verified      bool
}

func printGuardAgentSignature(settings printGuardSettings, app InstalledApp) string {
	revision, _ := envValue(app.Env, printGuardRevisionEnv)
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%s\x00%s\x00%s\x00%d\x00%s", printGuardAgentEpoch, printGuardRevocations.Load(), settings.AgentMode, settings.APIToken, revision, app.HostPort, app.ContainerID)))
	return hex.EncodeToString(sum[:])
}

// ReadPrintGuardAgentBinding is passive: no migration, recovery or Docker mutation.
func ReadPrintGuardAgentBinding(ctx context.Context, dbPath string, vault printGuardSecretReader) (PrintGuardAgentBinding, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return PrintGuardAgentBinding{}, fmt.Errorf("PrintGuard is not installed")
	}
	settings, err := readPrintGuardSettings(vault)
	if err != nil {
		return PrintGuardAgentBinding{}, err
	}
	if settings.AgentMode == "off" || settings.APIToken == "" {
		return PrintGuardAgentBinding{}, fmt.Errorf("PrintGuard agent access is disabled")
	}
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		return PrintGuardAgentBinding{}, err
	}
	path := filepath.ToSlash(abs)
	if runtime.GOOS == "windows" {
		path = "/" + path
	}
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}).String())
	if err != nil {
		return PrintGuardAgentBinding{}, err
	}
	defer db.Close()
	var app InstalledApp
	var env string
	err = db.QueryRowContext(ctx, `SELECT app_id, container_name, container_id, image, status, host_port, env_json, bind_mode, host_ip, tailscale_enabled FROM desktop_store_apps WHERE app_id = ?`, PrintGuardAppID).Scan(&app.AppID, &app.ContainerName, &app.ContainerID, &app.Image, &app.Status, &app.HostPort, &env, &app.BindMode, &app.HostIP, &app.TailscaleEnabled)
	if err != nil || app.Status != AppStatusRunning || app.ContainerName != ContainerName(PrintGuardAppID) || app.ContainerID == "" || app.HostPort < 1 || app.HostPort > 65535 || app.BindMode != BindModeLocal || app.HostIP != "127.0.0.1" || app.TailscaleEnabled || json.Unmarshal([]byte(env), &app.Env) != nil {
		return PrintGuardAgentBinding{}, fmt.Errorf("PrintGuard runtime is unavailable")
	}
	var busy int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM desktop_store_operations WHERE app_id = ? AND status IN (?, ?)`, PrintGuardAppID, OperationPending, OperationRunning).Scan(&busy); err != nil || busy != 0 {
		return PrintGuardAgentBinding{}, fmt.Errorf("PrintGuard operation is in progress")
	}
	active, err := settings.runtimeFor(app)
	if err != nil {
		return PrintGuardAgentBinding{}, fmt.Errorf("PrintGuard configuration is pending")
	}
	signature := printGuardAgentSignature(settings, app)
	return PrintGuardAgentBinding{Mode: settings.AgentMode, Token: settings.APIToken, HostPort: app.HostPort, ContainerName: app.ContainerName, DockerNetwork: active.DockerNetwork, Signature: signature, Verified: signature == settings.Verified}, nil
}

func (s *Service) VerifyPrintGuardAgent(ctx context.Context, expected PrintGuardAgentBinding) error {
	s.printGuardMu.Lock()
	defer s.printGuardMu.Unlock()
	current, err := ReadPrintGuardAgentBinding(ctx, s.cfg.DBPath, s.cfg.Secrets)
	if err != nil || current.Signature != expected.Signature {
		return fmt.Errorf("PrintGuard configuration changed during verification")
	}
	settings, err := readPrintGuardSettings(s.cfg.Secrets)
	if err != nil {
		return err
	}
	settings.Verified = current.Signature
	return s.writePrintGuardSettings(settings)
}

// PrintGuardAPIToken is for the server's bounded, private status probe only.
func (s *Service) PrintGuardAPIToken() (string, error) {
	settings, err := readPrintGuardSettings(s.cfg.Secrets)
	return settings.APIToken, err
}

func (s *Service) RevokePrintGuardVerification() error {
	printGuardRevocations.Add(1)
	s.printGuardMu.Lock()
	defer s.printGuardMu.Unlock()
	settings, err := readPrintGuardSettings(s.cfg.Secrets)
	if err != nil {
		return err
	}
	if settings.Verified == "" {
		return nil
	}
	settings.Verified = ""
	return s.writePrintGuardSettings(settings)
}

package desktopstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"aurago/internal/security"
)

const GodsEyeAppID = "gods-eye-view"
const godsEyeVaultKey = "desktop_store_gods-eye-view_config"
const godsEyeRevisionEnv = "AURAGO_GEV_REVISION"

var godsEyeKeys = []string{"CESIUM_ION_TOKEN", "GOOGLE_MAPS_API_KEY", "OPENAI_API_KEY", "AISSTREAM_API_KEY", "FIRMS_MAP_KEY", "TOMTOM_API_KEY", "OPENSKY_CLIENT_ID", "OPENSKY_CLIENT_SECRET", "LL2_API_TOKEN"}

// GodsEyeConfigUpdate never enters the operation ledger or installed-app record.
type GodsEyeConfigUpdate struct {
	Keys           map[string]string `json:"keys"`
	Clear          []string          `json:"clear"`
	AllowedOrigins *[]string         `json:"allowed_origins,omitempty"`
}

type GodsEyeConfigStatus struct {
	Configured     map[string]bool `json:"configured"`
	AllowedOrigins []string        `json:"allowed_origins"`
	Pending        bool            `json:"pending"`
}

type godsEyeConfig struct {
	Revision string            `json:"revision"`
	Keys     map[string]string `json:"keys"`
	Origins  []string          `json:"origins"`
}

// Keep only desired and last active settings so failed replacement can restore
// the exact old environment without persisting credentials in SQLite.
type godsEyeSettings struct {
	Desired  godsEyeConfig `json:"desired"`
	Previous godsEyeConfig `json:"previous"`
}

func validateGodsEyeOrigins(origins []string) ([]string, error) {
	if len(origins) > 8 {
		return nil, fmt.Errorf("at most eight AuraGo origins are allowed")
	}
	out := make([]string, 0, len(origins))
	for _, raw := range origins {
		raw = strings.TrimSpace(raw)
		u, err := url.Parse(raw)
		if err != nil || len(raw) > 512 || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") || strings.ContainsAny(raw, "*;'\"\\\t\r\n ") {
			return nil, fmt.Errorf("allowed origins must be exact HTTP(S) origins without paths or wildcards")
		}
		if port := u.Port(); port != "" {
			n, err := strconv.Atoi(port)
			if err != nil || n < 1 || n > 65535 {
				return nil, fmt.Errorf("invalid AuraGo origin port")
			}
		} else if strings.HasSuffix(u.Host, ":") {
			return nil, fmt.Errorf("invalid AuraGo origin port")
		}
		origin := strings.ToLower(u.Scheme + "://" + u.Host)
		if !slices.Contains(out, origin) {
			out = append(out, origin)
		}
	}
	return out, nil
}

func (s *Service) readGodsEyeSettings() (godsEyeSettings, error) {
	if s.cfg.Secrets == nil {
		return godsEyeSettings{}, fmt.Errorf("God's Eye View requires the AuraGo vault")
	}
	raw, err := s.cfg.Secrets.ReadSecret(godsEyeVaultKey)
	if errors.Is(err, security.ErrSecretNotFound) || (err == nil && raw == "") {
		return godsEyeSettings{}, nil
	}
	if err != nil {
		return godsEyeSettings{}, fmt.Errorf("read God's Eye View settings from vault")
	}
	var settings godsEyeSettings
	if json.Unmarshal([]byte(raw), &settings) != nil {
		return settings, fmt.Errorf("invalid God's Eye View vault settings")
	}
	return settings, nil
}

func (s *Service) writeGodsEyeSettings(settings godsEyeSettings) error {
	raw, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("encode God's Eye View settings")
	}
	for _, cfg := range []godsEyeConfig{settings.Desired, settings.Previous} {
		for _, value := range cfg.Keys {
			security.RegisterSensitive(value)
		}
	}
	if err := s.cfg.Secrets.WriteSecret(godsEyeVaultKey, string(raw)); err != nil {
		return fmt.Errorf("save God's Eye View settings in vault")
	}
	return nil
}

func (settings godsEyeSettings) configFor(app InstalledApp) (godsEyeConfig, error) {
	revision, _ := envValue(app.Env, godsEyeRevisionEnv)
	for _, cfg := range []godsEyeConfig{settings.Desired, settings.Previous} {
		if revision != "" && revision == cfg.Revision {
			return cfg, nil
		}
	}
	return godsEyeConfig{}, fmt.Errorf("God's Eye View active configuration is unavailable")
}

func (s *Service) GodsEyeConfiguration(ctx context.Context) (GodsEyeConfigStatus, error) {
	app, exists, err := s.GetInstalled(ctx, GodsEyeAppID)
	if err != nil || !exists {
		return GodsEyeConfigStatus{}, fmt.Errorf("God's Eye View is not installed")
	}
	settings, err := s.readGodsEyeSettings()
	if err != nil {
		return GodsEyeConfigStatus{}, err
	}
	status := GodsEyeConfigStatus{Configured: map[string]bool{}, AllowedOrigins: settings.Desired.Origins}
	for _, key := range godsEyeKeys {
		status.Configured[key] = settings.Desired.Keys[key] != ""
	}
	revision, _ := envValue(app.Env, godsEyeRevisionEnv)
	status.Pending = revision != settings.Desired.Revision
	return status, nil
}

func (s *Service) ConfigureGodsEye(ctx context.Context, req GodsEyeConfigUpdate) (Operation, error) {
	for key, value := range req.Keys {
		if !slices.Contains(godsEyeKeys, key) || len(value) > 512 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return Operation{}, fmt.Errorf("invalid provider key or value")
		}
	}
	for _, key := range req.Clear {
		if !slices.Contains(godsEyeKeys, key) || req.Keys[key] != "" {
			return Operation{}, fmt.Errorf("invalid or conflicting provider key removal")
		}
	}
	if req.AllowedOrigins != nil {
		origins, err := validateGodsEyeOrigins(*req.AllowedOrigins)
		if err != nil {
			return Operation{}, err
		}
		req.AllowedOrigins = &origins
	}
	app, exists, err := s.GetInstalled(ctx, GodsEyeAppID)
	if err != nil || !exists {
		return Operation{}, fmt.Errorf("God's Eye View is not installed")
	}
	// Reserve the existing exclusive lifecycle slot before touching the vault.
	op, err := s.createOperation(ctx, OperationConfigure, GodsEyeAppID, OperationRequest{})
	if err != nil {
		return Operation{}, err
	}
	fail := func(err error) (Operation, error) {
		_ = s.updateOperation(ctx, op.ID, OperationFailed, "", err.Error())
		return Operation{}, err
	}
	// Refresh after reservation: an update/uninstall may have completed while
	// we were acquiring the lifecycle slot.
	app, exists, err = s.GetInstalled(ctx, GodsEyeAppID)
	if err != nil || !exists {
		return fail(fmt.Errorf("God's Eye View is not installed"))
	}
	settings, err := s.readGodsEyeSettings()
	if err != nil {
		return fail(err)
	}
	active, err := settings.configFor(app)
	if err != nil {
		return fail(err)
	}
	keys := map[string]string{}
	for key, value := range settings.Desired.Keys {
		keys[key] = value
	}
	for key, value := range req.Keys {
		if value = strings.TrimSpace(value); value != "" {
			keys[key] = value
		}
	}
	for _, key := range req.Clear {
		delete(keys, key)
	}
	if (keys["OPENSKY_CLIENT_ID"] == "") != (keys["OPENSKY_CLIENT_SECRET"] == "") {
		return fail(fmt.Errorf("OpenSky client ID and secret must be configured or removed together"))
	}
	settings.Previous = active
	settings.Desired.Keys = keys
	settings.Desired.Revision = newID("cfg")
	if req.AllowedOrigins != nil {
		settings.Desired.Origins = *req.AllowedOrigins
	}
	if err := s.writeGodsEyeSettings(settings); err != nil {
		return fail(err)
	}
	return op, nil
}

func (s *Service) prepareGodsEyeInstall(req InstallRequest) error {
	settings, err := s.readGodsEyeSettings()
	if err != nil {
		return err
	}
	origins, err := validateGodsEyeOrigins(append(settings.Desired.Origins, req.AllowedOrigins...))
	if err != nil {
		return err
	}
	settings.Desired.Origins = origins
	settings.Desired.Revision = newID("cfg")
	return s.writeGodsEyeSettings(settings)
}

func (s *Service) godsEyeEnvironment() ([]string, []SecretRef, error) {
	settings, err := s.readGodsEyeSettings()
	if err != nil {
		return nil, nil, err
	}
	return []string{godsEyeRevisionEnv + "=" + settings.Desired.Revision}, nil, nil
}

func (s *Service) runtimeContainerSpec(app InstalledApp) (ContainerSpec, error) {
	spec := containerSpecFromRecord(app)
	if app.AppID != GodsEyeAppID {
		return spec, nil
	}
	settings, err := s.readGodsEyeSettings()
	if err != nil {
		return spec, err
	}
	cfg, err := settings.configFor(app)
	if err != nil {
		return spec, err
	}
	origins, _ := json.Marshal(cfg.Origins)
	spec.Env = append(spec.Env, "AURAGO_FRAME_ORIGINS="+string(origins))
	for _, key := range godsEyeKeys {
		if value := cfg.Keys[key]; value != "" {
			security.RegisterSensitive(value)
			spec.Env = append(spec.Env, key+"="+value)
		}
	}
	return spec, nil
}

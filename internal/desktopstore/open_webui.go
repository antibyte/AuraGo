package desktopstore

import (
	"aurago/internal/security"
	"context"
	"errors"
	"strings"
)

func hasOpenWebUIRelay(app InstalledApp) bool {
	for _, ref := range app.SecretRefs {
		if ref.Key == deepTutorTokenKey {
			return true
		}
	}
	return false
}

func (s *Service) requiresRelay(ctx context.Context, id, operation string) bool {
	if id == DeepTutorAppID {
		return true
	}
	if id != OpenWebUIAppID {
		return false
	}
	if operation == OperationInstall || operation == OperationUpdate {
		return true
	}
	app, exists, err := s.GetInstalled(ctx, id)
	return err != nil || (exists && hasOpenWebUIRelay(app))
}

func (s *Service) applyRelayNetwork(ctx context.Context, app *InstalledApp) error {
	if !IsRelayApp(app.AppID) {
		return nil
	}
	info, err := s.deepTutorRelayInfo(ctx, IntegrationNetwork(*app))
	if err != nil {
		return err
	}
	if info.DockerNetwork != "" && IntegrationNetwork(*app) != info.DockerNetwork {
		filtered := app.Env[:0]
		for _, env := range app.Env {
			if !strings.HasPrefix(env, storeNetworkEnv) {
				filtered = append(filtered, env)
			}
		}
		app.Env = append(filtered, storeNetworkEnv+info.DockerNetwork)
	}
	return nil
}

func (s *Service) openWebUIContainerSpec(ctx context.Context, spec ContainerSpec) (ContainerSpec, error) {
	info, err := s.deepTutorRelayInfo(ctx, spec.NetworkMode)
	if err != nil {
		return spec, err
	}
	if s.cfg.Secrets == nil {
		return spec, errors.New("Open WebUI needs the AuraGo Vault")
	}
	token, err := s.cfg.Secrets.ReadSecret(RelayTokenKey(OpenWebUIAppID))
	if err != nil || token == "" {
		return spec, errors.New("Open WebUI relay token missing; update the Store app")
	}
	security.RegisterSensitive(token)
	// Open WebUI uses these only as initial defaults; persistent user settings win.
	spec.Env = append(spec.Env, "OPENAI_API_BASE_URL="+info.BaseURL+RelayPath(OpenWebUIAppID), "OPENAI_API_KEY="+token, "DEFAULT_MODELS=aurago")
	spec.ExtraHosts = append(spec.ExtraHosts, info.ExtraHosts...)
	return spec, nil
}

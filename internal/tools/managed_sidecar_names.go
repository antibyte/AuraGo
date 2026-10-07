package tools

import (
	"strings"

	"aurago/internal/config"
)

// ManagedSidecarContainerNames returns the container names of AuraGo's
// managed sidecars, which their managers find (and several reuse) by name:
// Gotenberg, the managed and the embeddings Ollama, Piper, Supertonic,
// Ansible, browser automation, go2rtc, Space Agent, Manifest (and its
// Postgres), OmniRoute, the Dograh services and cloudflared, each with its
// default name and the configured one. Agent dispatch reserves them while
// docker.allow_host_access is off (F-C13). Code Studio and OpenSCAD stay
// usable (K8), and the names other checks already reserve for everyone
// (Garage, homepage, the app container, the security proxy, local LLM,
// ACE-Step) are not listed.
func ManagedSidecarContainerNames(cfg *config.Config) []string {
	if cfg == nil {
		cfg = &config.Config{}
	}
	var names []string
	seen := map[string]bool{}
	add := func(candidates ...string) {
		for _, name := range candidates {
			name = strings.TrimPrefix(strings.TrimSpace(name), "/")
			if key := strings.ToLower(name); name != "" && !seen[key] {
				seen[key] = true
				names = append(names, name)
			}
		}
	}
	add(gotenbergContainerName, ollamaManagedContainerName, ollamaEmbContainerName, piperContainerName)
	add(defaultSupertonicContainerName, cfg.TTS.Supertonic.ContainerName)
	add(ansibleContainerName, cfg.Ansible.ContainerName)
	add(browserAutomationContainerName, cfg.BrowserAutomation.ContainerName)
	inDocker := cfg.Runtime.IsDocker || browserAutomationRunsInDocker()
	if managedHost := browserAutomationManagedURLHost(cfg.BrowserAutomation.URL, cfg.BrowserAutomation.ContainerName, inDocker); managedHost != "" {
		add(browserAutomationEffectiveContainerName(BrowserAutomationSidecarConfig{ContainerName: cfg.BrowserAutomation.ContainerName}, managedHost))
	}
	add("aurago_go2rtc", cfg.Go2RTC.ContainerName)
	add(spaceAgentDefaultContainerName, cfg.SpaceAgent.ContainerName)
	add(manifestDefaultContainerName, cfg.Manifest.ContainerName, manifestDefaultPostgresContainerName, cfg.Manifest.PostgresContainerName)
	add(omniRouteDefaultContainerName, cfg.OmniRoute.ContainerName)
	add(dograhDefaultAPIContainerName, cfg.Dograh.APIContainerName, dograhDefaultUIContainerName, cfg.Dograh.UIContainerName,
		dograhDefaultPostgresContainerName, cfg.Dograh.PostgresContainerName, dograhDefaultRedisContainerName, cfg.Dograh.RedisContainerName,
		dograhDefaultMinioContainerName, cfg.Dograh.MinioContainerName, dograhDefaultCoturnContainerName, cfg.Dograh.CoturnContainerName)
	add(cfdContainerName)
	return names
}

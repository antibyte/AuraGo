package desktop

// CapabilityProvider reports host capabilities that gate hardware-bound
// built-in apps, for example "bluetooth".
type CapabilityProvider interface {
	HasCapability(name string) bool
}

// SetCapabilityProvider installs the host capability source. Without a
// provider every requirement is unmet, so hardware-bound apps stay hidden.
func (s *Service) SetCapabilityProvider(provider CapabilityProvider) {
	s.capabilityMu.Lock()
	s.capabilities = provider
	s.capabilityMu.Unlock()
}

func (s *Service) capabilityProvider() CapabilityProvider {
	s.capabilityMu.RLock()
	defer s.capabilityMu.RUnlock()
	return s.capabilities
}

// AppRequirementsMet reports whether provider satisfies every requirement of app.
func AppRequirementsMet(app AppManifest, provider CapabilityProvider) bool {
	for _, requirement := range app.Requires {
		if provider == nil || !provider.HasCapability(requirement) {
			return false
		}
	}
	return true
}

// FilterAvailableApps returns the apps whose requirements are met.
func FilterAvailableApps(apps []AppManifest, provider CapabilityProvider) []AppManifest {
	out := make([]AppManifest, 0, len(apps))
	for _, app := range apps {
		if AppRequirementsMet(app, provider) {
			out = append(out, app)
		}
	}
	return out
}

// filterUnavailableAppShortcuts omits, without deleting, app shortcuts whose
// target is a built-in app with unmet requirements.
func filterUnavailableAppShortcuts(shortcuts []Shortcut, apps []AppManifest, provider CapabilityProvider) []Shortcut {
	hidden := map[string]bool{}
	for _, app := range apps {
		if !AppRequirementsMet(app, provider) {
			hidden[app.ID] = true
		}
	}
	if len(hidden) == 0 {
		return shortcuts
	}
	out := make([]Shortcut, 0, len(shortcuts))
	for _, shortcut := range shortcuts {
		if shortcut.TargetType == ShortcutTargetApp && hidden[shortcut.TargetID] {
			continue
		}
		out = append(out, shortcut)
	}
	return out
}

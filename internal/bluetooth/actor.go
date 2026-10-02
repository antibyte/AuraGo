package bluetooth

// Actor identifies who requests a Bluetooth change. The configuration limits
// readonly and allow_playback bind only the LLM agent; authenticated operators
// using the admin UI or the desktop app are not restricted by them.
type Actor int

const (
	// ActorAgent is the zero value so an unset actor is always the restricted one.
	ActorAgent Actor = iota
	ActorOperator
)

func (m *Manager) currentOptions() Options {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.options
}

func (m *Manager) requireWritableFor(actor Actor) (Options, Status, error) {
	options, status, err := m.requireUsable()
	if err != nil {
		return Options{}, status, err
	}
	if actor == ActorAgent && options.ReadOnly {
		return Options{}, status, codedError(ErrorReadOnly, "Bluetooth is in read-only mode; pairing and connection changes are disabled.", nil)
	}
	return options, status, nil
}

func (m *Manager) requirePlaybackFor(actor Actor) (Options, Status, error) {
	options, status, err := m.requireUsable()
	if err != nil {
		return Options{}, status, err
	}
	if actor == ActorAgent && !options.AllowPlayback {
		return Options{}, status, codedError(ErrorPlaybackDisabled, "Bluetooth playback is disabled in the AuraGo configuration.", nil)
	}
	if !status.Audio.Usable {
		return Options{}, status, codedError(ErrorAudioTargetUnavailable, status.Audio.Reason, nil)
	}
	return options, status, nil
}

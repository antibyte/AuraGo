package desktop

import "time"

// RemoteProxyOptions controls browser-to-device SSH/VNC proxy lifetime limits.
type RemoteProxyOptions struct {
	MaxSessionDuration time.Duration
	IdleTimeout        time.Duration
}

// RemoteProxyOptionsFromConfig converts user-facing minute settings into proxy durations.
func RemoteProxyOptionsFromConfig(cfg Config) RemoteProxyOptions {
	return normalizeRemoteProxyOptions(RemoteProxyOptions{
		MaxSessionDuration: time.Duration(cfg.RemoteMaxSessionMinutes) * time.Minute,
		IdleTimeout:        time.Duration(cfg.RemoteIdleTimeoutMinutes) * time.Minute,
	})
}

func normalizeRemoteProxyOptions(options ...RemoteProxyOptions) RemoteProxyOptions {
	normalized := RemoteProxyOptions{
		MaxSessionDuration: remoteProxyMaxSessionDuration,
		IdleTimeout:        remoteProxyIdleTimeout,
	}
	if len(options) == 0 {
		return normalized
	}
	if options[0].MaxSessionDuration > 0 {
		normalized.MaxSessionDuration = options[0].MaxSessionDuration
	}
	if options[0].IdleTimeout > 0 {
		normalized.IdleTimeout = options[0].IdleTimeout
	}
	return normalized
}

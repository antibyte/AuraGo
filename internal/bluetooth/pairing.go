package bluetooth

import (
	"context"
	"log/slog"
	"strings"
)

// agentHost is the platform seam of the interactive BlueZ agent.
type agentHost interface {
	// Acquire registers the interactive agent; asDefault additionally claims the
	// default-agent role for incoming requests. release undoes exactly this call.
	Acquire(ctx context.Context, asDefault bool) (release func(), err error)
	// Pair pairs devicePath over the agent's own connection so BlueZ routes the
	// pairing questions to the interactive agent.
	Pair(ctx context.Context, devicePath string) error
	Close() error
}

// deviceResolver names devices for pairing questions.
type deviceResolver interface {
	deviceByPath(path string) (Device, bool)
}

type agentHostFactory func(*interactionBroker, deviceResolver, *slog.Logger) (agentHost, error)

var serviceNames = map[string]string{
	"00001105": "Object Push",
	"00001108": "Headset",
	"0000110a": "Audio Source",
	"0000110b": "Audio Sink",
	"0000110e": "Remote Control",
	"0000111e": "Handsfree",
	"0000111f": "Handsfree Audio Gateway",
	"00001124": "Human Interface Device",
	"0000112f": "Phonebook Access",
	"00001812": "Human Interface Device",
}

// serviceName returns a readable profile name for a Bluetooth service UUID.
func serviceName(uuid string) string {
	normalized := strings.ToLower(strings.TrimSpace(uuid))
	if len(normalized) >= 8 {
		if name, ok := serviceNames[normalized[:8]]; ok {
			return name
		}
	}
	return uuid
}

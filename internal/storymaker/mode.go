package storymaker

import "fmt"

// Modes of a project: where the pictures of its films come from.
const (
	// ModeCatalog builds scenes from the catalog (people, props, environments). It is the default.
	ModeCatalog = "catalog"
	// ModeFree has the agent draw and animate everything itself, in code scenes only.
	ModeFree = "free"
)

// EffectiveMode maps a stored or requested value to a mode. Anything but "free" is the catalog mode, so a project made before
// modes existed keeps its behaviour.
func EffectiveMode(stored string) string {
	if stored == ModeFree {
		return ModeFree
	}
	return ModeCatalog
}

// NormalizeMode validates a requested mode. An empty request is the catalog mode.
func NormalizeMode(requested string) (string, error) {
	switch requested {
	case "", ModeCatalog:
		return ModeCatalog, nil
	case ModeFree:
		return ModeFree, nil
	}
	return "", fmt.Errorf("%w: mode must be %s or %s", ErrInvalidArgument, ModeCatalog, ModeFree)
}

// storedMode is what project.json keeps: nothing for the catalog mode, so catalog projects stay byte-identical to the
// projects made before modes.
func storedMode(mode string) string {
	if mode == ModeFree {
		return ModeFree
	}
	return ""
}

package homedash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

// RuntimeConfig is the immutable snapshot the server builds from config.Config. Adapters never read
// config.Config. Source Enabled is already "homedash.sources.<s>.enabled && integration enabled".
type RuntimeConfig struct {
	Enabled, ReadOnly, DesktopReadOnly, EggMode bool
	KioskEnabled, KioskAllowControl             bool
	Limits                                      Limits
	History                                     HistoryConfig
	Sources                                     map[SourceID]SourceSettings
}

type Limits struct{ MaxDashboards, MaxStreams, MaxKioskStreams, MaxSubscriptions, CommandsPerMinute int }
type HistoryConfig struct{ RingPoints, RingEntities, SampleSeconds int }

// SourceSettings is the per-source part of the snapshot. ConfigKey is the key under
// homedash.sources (for example "home_assistant") and names the blocking switch in blocked_by.
// Control false refuses every command of the source regardless of ConfigKey; a source without a
// control switch in the config block is built with Control false, and a source missing from the
// settings map is refused as well. Params and Lists carry adapter settings (url, token, readonly,
// allowed_services ...).
type SourceSettings struct {
	Enabled     bool
	Control     bool
	PollSeconds int
	ConfigKey   string
	Params      map[string]string
	Lists       map[string][]string
}

func (c RuntimeConfig) active() bool { return c.Enabled && !c.EggMode }

func (c RuntimeConfig) withDefaults() RuntimeConfig {
	set := func(p *int, def int) {
		if *p <= 0 {
			*p = def
		}
	}
	set(&c.Limits.MaxDashboards, 50)
	set(&c.Limits.MaxStreams, 32)
	set(&c.Limits.MaxKioskStreams, 16)
	set(&c.Limits.MaxSubscriptions, 300)
	set(&c.Limits.CommandsPerMinute, 60)
	set(&c.History.RingPoints, 360)
	set(&c.History.RingEntities, 500)
	set(&c.History.SampleSeconds, 60)
	return c
}

// Fingerprint changes whenever anything a source runtime depends on changes.
func (s SourceSettings) Fingerprint() string {
	h := sha256.New()
	fmt.Fprintf(h, "%t|%t|%d|%s|", s.Enabled, s.Control, s.PollSeconds, s.ConfigKey)
	for _, k := range sortedKeys(s.Params) {
		fmt.Fprintf(h, "p:%q=%q|", k, s.Params[k])
	}
	for _, k := range sortedKeys(s.Lists) {
		fmt.Fprintf(h, "l:%q=%q|", k, s.Lists[k])
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

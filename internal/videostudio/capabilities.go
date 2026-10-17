package videostudio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrFeatureUnavailable reports an export that needs an FFmpeg filter or xfade transition the binary lacks.
var ErrFeatureUnavailable = errors.New("FFmpeg lacks a filter this project needs")

// CapabilityError names the project feature and the missing FFmpeg capability behind ErrFeatureUnavailable.
type CapabilityError struct {
	Feature string // "transition:circleopen", "effect:lut", "blend:multiply"
	Missing string // "xfade:circleopen", "filter:lut3d"
}

func (e *CapabilityError) Error() string {
	return ErrFeatureUnavailable.Error() + ": " + e.Feature + " needs " + e.Missing
}

func (e *CapabilityError) Unwrap() error { return ErrFeatureUnavailable }

// phase2Filters are the FFmpeg filters phase 2 uses for colour, LUTs, keys, blur, sharpen, vignette, masks, blend
// modes and runtime commands (spec §7.9). They are optional: a missing one makes the features that need it
// unavailable. The readiness filters of hasRequiredRenderFilters stay a hard requirement.
var phase2Filters = []string{"lutrgb", "colorchannelmixer", "lut3d", "colorkey", "despill", "gblur", "cas", "vignette", "geq", "alphamerge", "maskedmerge", "blend", "sendcmd"}

const (
	capabilityProbeTimeout = 10 * time.Second
	// capabilityMemoLimit bounds the memo; it is cleared when one more binary (path, size or time) appears.
	capabilityMemoLimit = 8
)

type capabilityKey struct {
	path    string
	size    int64
	modTime int64
}

// capabilityMemo holds probe results per resolved binary path, size and modification time, so the status endpoint and
// every render share one probe per FFmpeg binary. Failed probes are not stored. Two concurrent first probes of the
// same binary may both run; they store the same result.
var capabilityMemo = struct {
	sync.Mutex
	entries map[capabilityKey][]string
}{entries: make(map[capabilityKey][]string)}

// capabilityCommand runs one FFmpeg query of the probe; tests replace it.
var capabilityCommand = func(ctx context.Context, ffmpeg string, args ...string) ([]byte, error) {
	return runSmallCommand(ctx, ffmpeg, args, commandOutputLimit)
}

// ProbeCapabilities lists what the binary lacks: "filter:<name>" for phase 2 filters missing from `-filters`, and
// "xfade:<name>" for registry transitions missing from `-h filter=xfade`. The result is memoized per resolved binary
// path, size and modification time. Entries follow capabilityFilters order, then registry transition order.
func ProbeCapabilities(ctx context.Context, ffmpegPath string) ([]string, error) {
	ffmpeg, err := resolveBinary(ffmpegPath, "ffmpeg")
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(ffmpeg)
	if err != nil {
		return nil, fmt.Errorf("inspect FFmpeg binary: %w", err)
	}
	key := capabilityKey{path: ffmpeg, size: info.Size(), modTime: info.ModTime().UnixNano()}
	capabilityMemo.Lock()
	cached, ok := capabilityMemo.entries[key]
	capabilityMemo.Unlock()
	if ok {
		return slices.Clone(cached), nil
	}

	probeCtx, cancel := context.WithTimeout(ctx, capabilityProbeTimeout)
	defer cancel()
	listing, err := capabilityCommand(probeCtx, ffmpeg, "-hide_banner", "-filters")
	if err != nil {
		return nil, fmt.Errorf("list FFmpeg filters: %w", err)
	}
	present := ffmpegFilterNames(string(listing))
	missing := []string{}
	for _, name := range capabilityFilters() {
		if !present[name] {
			missing = append(missing, "filter:"+name)
		}
	}
	transitions := map[string]bool{}
	if present["xfade"] {
		help, err := capabilityCommand(probeCtx, ffmpeg, "-hide_banner", "-h", "filter=xfade")
		if err != nil {
			return nil, fmt.Errorf("read FFmpeg xfade help: %w", err)
		}
		transitions = xfadeTransitionNames(string(help))
	}
	reported := map[string]bool{}
	for _, spec := range registryTransitions {
		if !transitions[spec.XFade] && !reported[spec.XFade] {
			reported[spec.XFade] = true
			missing = append(missing, "xfade:"+spec.XFade)
		}
	}

	capabilityMemo.Lock()
	if len(capabilityMemo.entries) >= capabilityMemoLimit {
		clear(capabilityMemo.entries)
	}
	capabilityMemo.entries[key] = missing
	capabilityMemo.Unlock()
	return slices.Clone(missing), nil
}

// capabilityFilters lists phase2Filters followed by every filter an EffectSpec needs, each once.
func capabilityFilters() []string {
	seen := map[string]bool{}
	var names []string
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for _, name := range phase2Filters {
		add(name)
	}
	for _, effect := range registryEffects {
		for _, name := range effect.Filters {
			add(name)
		}
	}
	return names
}

// ffmpegFilterNames returns the filter names of `ffmpeg -filters` output. A filter line starts with the flags column
// followed by the name, for example " TSC xfade VV->V Cross fade ..." (FFmpeg 5.1: three flags, timeline, slice
// threading, command) or " TS xfade VV->V Cross fade ..." (FFmpeg 8: two flags); legend lines (" T.. = Timeline
// support", " A = Audio input/output") are skipped.
func ffmpegFilterNames(output string) map[string]bool {
	names := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !filterFlagsColumn(fields[0]) || fields[1] == "=" {
			continue
		}
		names[fields[1]] = true
	}
	return names
}

// filterFlagsColumn reports whether field is a flags column: two or three of 'T', 'S', 'C' and '.'. FFmpeg 5.1 lists
// three flags per filter, FFmpeg 8 two.
func filterFlagsColumn(field string) bool {
	if len(field) < 2 || len(field) > 3 {
		return false
	}
	for _, r := range field {
		if r != 'T' && r != 'S' && r != 'C' && r != '.' {
			return false
		}
	}
	return true
}

// xfadeTransitionNames returns the transition constants listed under the "transition" option of
// `ffmpeg -h filter=xfade`. Each constant line holds the name, its integer value and flags
// ("     fade            0            ..FV....... fade transition"). The list ends at the next option (a line whose
// second field is a "<type>") or at a blank line.
func xfadeTransitionNames(output string) map[string]bool {
	names := make(map[string]bool)
	inside := false
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			if inside {
				break
			}
			continue
		}
		if len(fields) >= 2 && strings.HasPrefix(fields[1], "<") {
			if inside {
				break
			}
			inside = fields[0] == "transition"
			continue
		}
		if !inside || len(fields) < 2 {
			continue
		}
		if _, err := strconv.Atoi(fields[1]); err == nil {
			names[fields[0]] = true
		}
	}
	return names
}

// CheckProjectCapabilities returns a *CapabilityError for the first feature of project that needs an entry of
// missing (as reported by ProbeCapabilities), or nil. Only what renders counts: transitions on hidden tracks, of
// disabled clips or before a disabled clip are skipped, as in segmentLayers. 2b adds the keyframes check (sendcmd),
// which 2c extends to crop and effect keyframes (clipHasKeyframes). 2c also adds the effect checks; 2d adds blend and
// mask checks.
func CheckProjectCapabilities(project Project, missing []string) error {
	if len(missing) == 0 {
		return nil
	}
	lacking := make(map[string]bool, len(missing))
	for _, entry := range missing {
		lacking[entry] = true
	}
	for _, track := range project.Tracks {
		if !visualClip(track) {
			continue
		}
		// Keyframes drive opacity, crops and effect parameters through sendcmd (index §4.11). Disabled clips never render.
		if lacking["filter:sendcmd"] {
			for _, clip := range track.Clips {
				if !clip.Disabled && clipHasKeyframes(clip) {
					return &CapabilityError{Feature: "keyframes", Missing: "filter:sendcmd"}
				}
			}
		}
		// An effect that is on needs every filter of its registry row. The preview still draws it, the inspector badges
		// it, and the export refuses it here (index §4.3).
		for _, clip := range track.Clips {
			if clip.Disabled {
				continue
			}
			for _, entry := range clip.Effects {
				if !entry.On {
					continue
				}
				spec, ok := EffectByID(entry.ID)
				if !ok {
					continue // Validate rejects unknown effects.
				}
				for _, filter := range spec.Filters {
					if need := "filter:" + filter; lacking[need] {
						return &CapabilityError{Feature: "effect:" + entry.ID, Missing: need}
					}
				}
			}
		}
		ordered := sortedClips(track.Clips)
		for i, clip := range ordered {
			if clip.Transition == nil || clip.Disabled || i+1 >= len(ordered) || ordered[i+1].Disabled {
				continue
			}
			spec, ok := TransitionByID(clip.Transition.Type)
			if !ok {
				continue // Validate rejects unknown transitions.
			}
			if need := "xfade:" + spec.XFade; lacking[need] {
				return &CapabilityError{Feature: "transition:" + string(spec.ID), Missing: need}
			}
		}
	}
	return nil
}

// clipHasKeyframes reports keyframes the export may drive through sendcmd: base and crop paths (Clip.Keyframes) and the
// keyframes of the clip's effect entries that are on.
func clipHasKeyframes(clip Clip) bool {
	if len(clip.Keyframes) > 0 {
		return true
	}
	for _, entry := range clip.Effects {
		if entry.On && len(entry.Keyframes) > 0 {
			return true
		}
	}
	return false
}

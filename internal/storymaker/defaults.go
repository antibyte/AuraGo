package storymaker

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Defaults of the story format (design spec, section Defaults).
const (
	DefaultEnvironment = "env.plain-stage"
	DefaultAspect      = "16:9"
	DefaultFPS         = 30
	DefaultPalette     = "sunny"
	DefaultFont        = "default"
)

// actorSlots are the default spots of several actors, assigned in cast order.
var actorSlots = []string{"left", "right", "far-left", "far-right"}

// ApplyDefaults fills the static defaults of the spec's Defaults table into s, only for fields
// the author omitted, and lists each filled field. Timing (delay, duration, ease) is never
// defaulted: the player's compile step owns it. Calling it again changes nothing.
//
// Beyond the table it completes the document-level fields a bare story may lack: schema,
// format (16:9, 30 fps), style (sunny, default font, captions on) and the show-beat instance id.
//
// With several actors, each actor without a position takes the slot left, right, far-left,
// far-right (wrapping) that matches its rank in the story's cast list, so a character keeps its
// side across scenes; a single actor stands at center.
func ApplyDefaults(s *Story) []DefaultApplied {
	var out []DefaultApplied
	set := func(path string, value any) { out = append(out, DefaultApplied{Path: path, Value: value}) }

	if s.Schema == "" {
		s.Schema = Schema
		set("/schema", Schema)
	}
	if s.Format.Aspect == "" {
		s.Format.Aspect = DefaultAspect
		set("/format/aspect", DefaultAspect)
	}
	if s.Format.FPS == 0 {
		s.Format.FPS = DefaultFPS
		set("/format/fps", DefaultFPS)
	}
	if s.Style == (Style{}) {
		s.Style = Style{Palette: DefaultPalette, Font: DefaultFont, Captions: true}
		set("/style", map[string]any{"palette": DefaultPalette, "font": DefaultFont, "captions": true})
	} else {
		if s.Style.Palette == "" {
			s.Style.Palette = DefaultPalette
			set("/style/palette", DefaultPalette)
		}
		if s.Style.Font == "" {
			s.Style.Font = DefaultFont
			set("/style/font", DefaultFont)
		}
	}
	for i := range s.Cast {
		if s.Cast[i].Name == "" && s.Cast[i].ID != "" {
			s.Cast[i].Name = s.Cast[i].ID
			set(jp("cast", i, "name"), s.Cast[i].ID)
		}
	}

	prevEnv := ""
	for i := range s.Scenes {
		sc := &s.Scenes[i]
		if sc.Environment == "" {
			sc.Environment = prevEnv
			if sc.Environment == "" {
				sc.Environment = DefaultEnvironment
			}
			set(jp("scenes", i, "environment"), sc.Environment)
		}
		prevEnv = sc.Environment
		if sc.Variant == "" {
			sc.Variant = "day"
			set(jp("scenes", i, "variant"), "day")
		}
		if sc.Speaker == "" {
			sc.Speaker = "narrator"
			set(jp("scenes", i, "speaker"), "narrator")
		}
		if sc.Transition == "" {
			sc.Transition = "fade"
			if i == 0 {
				sc.Transition = "cut"
			}
			set(jp("scenes", i, "transition"), sc.Transition)
		}
		if sc.Duration == nil {
			sc.Duration = AutoDuration()
			set(jp("scenes", i, "duration"), "auto")
		}
		if sc.Camera == nil {
			sc.Camera = &Camera{Framing: "wide"}
			set(jp("scenes", i, "camera"), map[string]any{"framing": "wide"})
		} else if sc.Camera.Framing == "" {
			sc.Camera.Framing = "wide"
			set(jp("scenes", i, "camera", "framing"), "wide")
		}

		ranks := actorRanks(s.Cast, sc.Actors)
		for j := range sc.Actors {
			a := &sc.Actors[j]
			if a.At.IsZero() {
				slot := "center"
				if len(sc.Actors) > 1 {
					slot = actorSlots[ranks[j]%len(actorSlots)]
				}
				a.At = SpotAt(slot)
				set(jp("scenes", i, "actors", j, "at"), slot)
			}
			if a.Pose == "" {
				a.Pose = "stand"
				set(jp("scenes", i, "actors", j, "pose"), "stand")
			}
			if a.Facing == "" {
				a.Facing = "auto"
				set(jp("scenes", i, "actors", j, "facing"), "auto")
			}
			if a.Expression == "" {
				a.Expression = "neutral"
				set(jp("scenes", i, "actors", j, "expression"), "neutral")
			}
			if a.Scale == 0 {
				a.Scale = 1
				set(jp("scenes", i, "actors", j, "scale"), 1)
			}
		}
		for j := range sc.Props {
			if sc.Props[j].Layer == "" {
				sc.Props[j].Layer = "front"
				set(jp("scenes", i, "props", j, "layer"), "front")
			}
		}
		for j, id := range deriveShowIDs(sc) {
			if id != "" {
				sc.Beats[j].ID = id
				set(jp("scenes", i, "beats", j, "id"), id)
			}
		}
	}
	return out
}

// actorRanks returns, per actor, its rank in cast order (actors of unknown cast members come
// last, in scene order).
func actorRanks(cast []CastMember, actors []Actor) []int {
	idx := make([]int, len(actors))
	for j := range idx {
		idx[j] = j
	}
	order := func(j int) int {
		for k, c := range cast {
			if c.ID == actors[j].Cast {
				return k
			}
		}
		return len(cast)
	}
	sort.SliceStable(idx, func(a, b int) bool { return order(idx[a]) < order(idx[b]) })
	ranks := make([]int, len(actors))
	for rank, j := range idx {
		ranks[j] = rank
	}
	return ranks
}

// deriveShowIDs returns, aligned with sc.Beats, the instance id a show beat without an id
// would get (<item-slug>-<n>), and "" for every other beat. It skips ids already taken by props
// and by beats with an explicit id, so ids stay unique.
func deriveShowIDs(sc *Scene) []string {
	if len(sc.Beats) == 0 {
		return nil
	}
	used := map[string]bool{}
	for _, p := range sc.Props {
		if p.ID != "" {
			used[p.ID] = true
		}
	}
	for _, b := range sc.Beats {
		if b.ID != "" {
			used[b.ID] = true
		}
	}
	counters := map[string]int{}
	out := make([]string, len(sc.Beats))
	for j, b := range sc.Beats {
		if b.Do != "show" || b.ID != "" {
			continue
		}
		slug := showSlug(b)
		n := counters[slug] + 1
		id := slug + "-" + strconv.Itoa(n)
		for used[id] {
			n++
			id = slug + "-" + strconv.Itoa(n)
		}
		counters[slug] = n
		used[id] = true
		out[j] = id
	}
	return out
}

// showSlug names what a show beat shows: the item without its prefix, or text, icon or chart.
func showSlug(b Beat) string {
	switch {
	case b.Item != "":
		return assetSlug(b.Item)
	case b.Text != "":
		return "text"
	case b.Icon != "":
		return cleanSlug(b.Icon, "icon")
	case b.Chart != "":
		return "chart"
	}
	return "show"
}

// assetSlug turns prop.lock into lock and my.prop.red-box into red-box.
func assetSlug(id string) string {
	parts := strings.Split(id, ".")
	if len(parts) > 1 {
		if (parts[0] == "my" || parts[0] == "lib") && len(parts) > 2 {
			parts = parts[2:]
		} else {
			parts = parts[1:]
		}
	}
	return cleanSlug(strings.Join(parts, "-"), "item")
}

// cleanSlug lower-cases s, replaces runs of other characters by one dash and limits the length
// to 40 characters so that "<slug>-<n>" stays a valid id.
func cleanSlug(s, fallback string) string {
	var b strings.Builder
	dash := true
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	if out == "" {
		return fallback
	}
	return out
}

// jp builds a JSON Pointer (RFC 6901) from path segments.
func jp(parts ...any) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteByte('/')
		switch v := p.(type) {
		case int:
			b.WriteString(strconv.Itoa(v))
		case string:
			b.WriteString(escapePointer(v))
		default:
			b.WriteString(escapePointer(fmt.Sprint(v)))
		}
	}
	return b.String()
}

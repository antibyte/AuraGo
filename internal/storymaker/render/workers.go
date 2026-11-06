package render

import (
	"io/fs"
	"math"
	"strconv"
	"strings"

	"aurago/internal/storymaker"
)

// parallelChunk is the chunk size C of the parallel path (measured, spec start value).
const parallelChunk = 8

// maxRenderWorkers caps N: N=6 adds about 10 percent over N=4 for about 60 MB, and more adds nothing.
const maxRenderWorkers = 6

// renderPreRoll is how many frames a worker paints and drops before a chunk. Every frame was
// byte-identical with and without it, so it is off. It is a variable so the parity test can set it to 2.
var renderPreRoll = 0

// ChooseWorkers is N, the number of player workers of a render. Worker encoding exists only for PNG,
// so every other transport stays serial. Each code scene has its own worker, which N cannot bound, so
// its memory comes off the budget of the player workers. freeMB 0 means unknown and sets no bound.
func ChooseWorkers(transport string, logicalCPUs int, freeMB int64, width, height, chunk int, cjk bool, codeScenes int) int {
	if transport != TransportPNG {
		return 1
	}
	n := (logicalCPUs - 2) / 3
	if n < 1 {
		n = 1
	}
	if n > maxRenderWorkers {
		n = maxRenderWorkers
	}
	if freeMB <= 0 {
		return n
	}
	frameMB := 0.6 * float64(width) * float64(height) / (1920 * 1080)
	perWorker := 60 + 2*float64(chunk)*frameMB
	codeMB := 0.0
	if cjk {
		perWorker += 120
		codeMB = float64(codeScenes) * 120
	}
	if bound := int(math.Floor((float64(freeMB) - codeMB) / perWorker)); bound < n {
		n = bound
	}
	if n < 1 {
		n = 1
	}
	return n
}

// BoundMemoryToCgroup is the smaller of hostMB, the available memory of the machine, and the room left
// under a cgroup memory limit. Inside a container hostMB still describes the whole host, so a plan made
// from it alone gets Chrome OOM-killed. root is the cgroup mount, /sys/fs/cgroup in production. Without
// a limit to read hostMB stays as it is, and 0 still means unknown. A full cgroup gives 1 and not 0,
// because 0 would lift the bound where it matters most.
func BoundMemoryToCgroup(hostMB int64, root fs.FS) int64 {
	roomMB, ok := cgroupRoomMB(root)
	if !ok {
		return hostMB
	}
	if roomMB < 1 {
		roomMB = 1
	}
	if hostMB > 0 && hostMB < roomMB {
		return hostMB
	}
	return roomMB
}

// cgroupRoomMB is the limit minus the memory in use, from the cgroup v2 files or else the v1 files. ok
// is false when neither gives a limit: v2 writes "max", which does not parse. v1 writes a huge number,
// and the minimum in BoundMemoryToCgroup drops it. Inactive file cache does not count as in use, as in
// `docker stats`: the kernel reclaims it before it kills anything, and without this a container that
// has been up for a while reads as full and renders serially for no reason.
func cgroupRoomMB(root fs.FS) (int64, bool) {
	for _, files := range []struct{ limit, usage, stat, inactive string }{
		{"memory.max", "memory.current", "memory.stat", "inactive_file"},
		{"memory/memory.limit_in_bytes", "memory/memory.usage_in_bytes", "memory/memory.stat", "total_inactive_file"},
	} {
		limit, ok := cgroupFileBytes(root, files.limit)
		if !ok {
			continue
		}
		used, ok := cgroupFileBytes(root, files.usage)
		if !ok {
			continue
		}
		used -= cgroupStatBytes(root, files.stat, files.inactive)
		if used < 0 {
			used = 0
		}
		return (limit - used) >> 20, true
	}
	return 0, false
}

func cgroupFileBytes(root fs.FS, name string) (int64, bool) {
	data, err := fs.ReadFile(root, name)
	if err != nil {
		return 0, false
	}
	return parseCgroupBytes(string(data))
}

// cgroupStatBytes reads one key of a memory.stat file, and 0 when it is missing.
func cgroupStatBytes(root fs.FS, name, key string) int64 {
	data, err := fs.ReadFile(root, name)
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if value, found := strings.CutPrefix(line, key+" "); found {
			n, _ := parseCgroupBytes(value)
			return n
		}
	}
	return 0
}

func parseCgroupBytes(text string) (int64, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// storyUsesCJK reports whether a render loads the CJK face: a drawn text resolves to sm-cjk, or a code
// scene has the language base zh or ja. It walks the same sources as the glyph check of Validate.
func storyUsesCJK(s *storymaker.Story, g *storymaker.Glyphs) bool {
	for i := range s.Scenes {
		sc := &s.Scenes[i]
		if sc.Kind == "code" && codeSceneIsCJK(s, sc) {
			return true
		}
		if g == nil {
			continue
		}
		sayWithoutText := false
		for j := range sc.Beats {
			b := &sc.Beats[j]
			if b.Do != "show" && b.Do != "say" {
				continue
			}
			if b.Text == "" {
				sayWithoutText = sayWithoutText || b.Do == "say"
				continue
			}
			if textUsesCJK(g, b.Text) {
				return true
			}
		}
		if !sc.Silent && (s.Style.Captions || sayWithoutText) && textUsesCJK(g, sc.Narration) {
			return true
		}
	}
	return false
}

// codeSceneIsCJK looks at the project voice and at the voice of the scene's speaker, so the memory
// reservation errs on the safe side.
func codeSceneIsCJK(s *storymaker.Story, sc *storymaker.Scene) bool {
	if s.Voice != nil && cjkLanguage(s.Voice.Language) {
		return true
	}
	for i := range s.Cast {
		c := &s.Cast[i]
		if c.ID == sc.Speaker && c.Voice != nil && cjkLanguage(c.Voice.Language) {
			return true
		}
	}
	return false
}

func cjkLanguage(tag string) bool {
	base := storymaker.LanguageBase(tag)
	return base == "zh" || base == "ja"
}

func textUsesCJK(g *storymaker.Glyphs, text string) bool {
	for _, r := range text {
		if i := g.FaceOf(r); i >= 0 && g.Faces[i].ID == "sm-cjk" {
			return true
		}
	}
	return false
}

// codeSceneCount counts the scenes of kind code.
func codeSceneCount(s *storymaker.Story) int {
	n := 0
	for i := range s.Scenes {
		if s.Scenes[i].Kind == "code" {
			n++
		}
	}
	return n
}

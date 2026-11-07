{% raw %}
# Part W: Parallel render workers (droppable)

Batch 2D. Run the tasks in order. `outline.md` holds the batch rules, the test matrices and the execution rules ("outline section N"). `interfaces.md` holds the frozen interfaces ("frozen section N"), and its section 10 and `reconcile.md` win over them. A group id such as `G-A-5` names a block of tasks inside a part; the index lists them.

<!-- G-W-1 -->

### Task W1: `ChooseWorkers`, settings, boot. Wire: Go to boot document
**Batch:** 2D  **Depends on:** none
**Files:**
- Create: `internal/storymaker/render/workers.go`
- Modify: `internal/storymaker/render/render.go`
- Modify: `internal/storymaker/render/listener.go`
- Modify: `internal/server/storymaker_service.go`
- Test: `internal/storymaker/render/workers_test.go`
- Test: `internal/storymaker/render/listener_test.go`

The page ignores the new boot fields until Task W4, so this commit changes no render.

- [ ] **Step 1: Impact**

Run from the main checkout, once per symbol: `Settings`, `PageParams`, `bootDoc`, `boot`, `execute`, `storymakerSettings`.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "Settings" --direction upstream --repo .
```

Handle HIGH, CRITICAL and UNKNOWN by the GitNexus rule of outline section 5.

- [ ] **Step 2: Write the failing test**

Create `internal/storymaker/render/workers_test.go`.

```go
package render

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"testing/fstest"

	"aurago/internal/storymaker"
)

func TestChooseWorkers(t *testing.T) {
	const fullW, fullH = 1920, 1080
	cases := []struct {
		name          string
		transport     string
		cpus          int
		freeMB        int64
		width, height int
		chunk         int
		cjk           bool
		codeScenes    int
		want          int
	}{
		// Worker encoding exists only for PNG.
		{"webp stays serial", TransportWebP, 32, 0, fullW, fullH, 8, false, 0, 1},
		{"raw stays serial", TransportRaw, 32, 0, fullW, fullH, 8, false, 0, 1},
		{"screenshot stays serial", TransportScreenshot, 32, 0, fullW, fullH, 8, false, 0, 1},

		// CPU rule: clamp(floor((cpus-2)/3), 1, 6).
		{"16 CPUs: 14/3 floors to 4", TransportPNG, 16, 0, fullW, fullH, 8, false, 0, 4},
		{"8 CPUs: 6/3 is 2", TransportPNG, 8, 0, fullW, fullH, 8, false, 0, 2},
		{"4 CPUs: 2/3 floors to 0, clamped to 1", TransportPNG, 4, 0, fullW, fullH, 8, false, 0, 1},
		{"1 CPU: clamped to 1", TransportPNG, 1, 0, fullW, fullH, 8, false, 0, 1},
		{"unknown CPU count: clamped to 1", TransportPNG, 0, 0, fullW, fullH, 8, false, 0, 1},
		{"32 CPUs: 10 capped at 6", TransportPNG, 32, 0, fullW, fullH, 8, false, 0, 6},

		// Memory rule: floor((freeMB-codeMB)/(60+2*chunk*frameMB+(cjk?120:0))), frameMB = 0.6*w*h/(1920*1080).
		{"300 MB at 1080p: per worker 60+2*8*0.6 = 69.6, 300/69.6 floors to 4", TransportPNG, 32, 300, fullW, fullH, 8, false, 0, 4},
		{"300 MB at 4K: frameMB 2.4, per worker 98.4, 300/98.4 floors to 3", TransportPNG, 32, 300, 3840, 2160, 8, false, 0, 3},
		{"chunk 16: per worker 60+2*16*0.6 = 79.2, 300/79.2 floors to 3", TransportPNG, 32, 300, fullW, fullH, 16, false, 0, 3},
		{"CJK: per worker 69.6+120 = 189.6, 1000/189.6 floors to 5", TransportPNG, 32, 1000, fullW, fullH, 8, true, 0, 5},
		{"CJK and 5 code scenes: (1000-600)/189.6 floors to 2", TransportPNG, 32, 1000, fullW, fullH, 8, true, 5, 2},
		{"5 code scenes without CJK reserve nothing: 1000/69.6 floors to 14, the CPU cap 6 wins", TransportPNG, 32, 1000, fullW, fullH, 8, false, 5, 6},
		{"10 MB: 10/69.6 floors to 0, at least 1", TransportPNG, 32, 10, fullW, fullH, 8, false, 0, 1},
		{"code scenes eat all memory: (1000-1200)/189.6 is negative, at least 1", TransportPNG, 32, 1000, fullW, fullH, 8, true, 10, 1},
		{"unknown memory has no bound", TransportPNG, 32, 0, fullW, fullH, 8, true, 5, 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ChooseWorkers(tc.transport, tc.cpus, tc.freeMB, tc.width, tc.height, tc.chunk, tc.cjk, tc.codeScenes)
			if got != tc.want {
				t.Errorf("ChooseWorkers = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestBoundMemoryToCgroup(t *testing.T) {
	text := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }
	mib := func(n int64) *fstest.MapFile { return text(strconv.FormatInt(n<<20, 10) + "\n") }
	v2 := func(limit, used, inactive int64) fstest.MapFS {
		return fstest.MapFS{
			"memory.max":     mib(limit),
			"memory.current": mib(used),
			"memory.stat":    text(fmt.Sprintf("anon 1\ninactive_file %d\nactive_file 2\n", inactive<<20)),
		}
	}
	v1 := func(limit, used, inactive int64) fstest.MapFS {
		return fstest.MapFS{
			"memory/memory.limit_in_bytes": mib(limit),
			"memory/memory.usage_in_bytes": mib(used),
			"memory/memory.stat":           text(fmt.Sprintf("inactive_file 1\ntotal_inactive_file %d\n", inactive<<20)),
		}
	}
	const host = 64000
	cases := []struct {
		name string
		host int64
		root fstest.MapFS
		want int64
	}{
		{"no cgroup files keep the host figure", host, fstest.MapFS{}, host},
		{"no cgroup files and an unknown host figure stay unknown", 0, fstest.MapFS{}, 0},
		{"v2 max means no limit", host, fstest.MapFS{"memory.max": text("max\n"), "memory.current": mib(500)}, host},
		{"v2 room: 1024-300", host, v2(1024, 300, 0), 724},
		{"v2 inactive file cache can be reclaimed: 1024-(1000-800)", host, v2(1024, 1000, 800), 824},
		{"v2 without memory.stat counts all usage", host, fstest.MapFS{"memory.max": mib(1024), "memory.current": mib(300)}, 724},
		{"v2 without memory.current keeps the host figure", host, fstest.MapFS{"memory.max": mib(1024)}, host},
		{"a host figure below the room wins", 500, v2(1024, 300, 0), 500},
		{"an unknown host figure takes the room", 0, v2(1024, 300, 0), 724},
		{"a full cgroup is 1, because 0 would mean unknown and lift the bound", host, v2(1024, 1024, 0), 1},
		{"usage above the limit is 1 too", host, v2(1024, 2048, 0), 1},
		{"v1 room uses total_inactive_file: 1024-(1000-800)", host, v1(1024, 1000, 800), 824},
		{"v1 without a limit is a huge number and keeps the host figure", host, fstest.MapFS{"memory/memory.limit_in_bytes": text("9223372036854771712\n"), "memory/memory.usage_in_bytes": mib(500)}, host},
		{"an unreadable limit keeps the host figure", host, fstest.MapFS{"memory.max": text("garbage\n"), "memory.current": mib(500)}, host},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BoundMemoryToCgroup(tc.host, tc.root); got != tc.want {
				t.Errorf("BoundMemoryToCgroup = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestStoryUsesCJK(t *testing.T) {
	cat, err := storymaker.LoadCatalog(os.DirFS(uiDirForTests), "img/storymaker/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	glyphs := cat.Glyphs()
	if glyphs == nil {
		t.Fatal("the real catalog names no glyphs file")
	}
	voice := func(lang string) *storymaker.Voice {
		return &storymaker.Voice{Provider: "phonon", Voice: "v", Language: lang}
	}
	show := func(text string) []storymaker.Beat { return []storymaker.Beat{{Do: "show", Text: text}} }
	on := storymaker.Style{Captions: true}
	off := storymaker.Style{}
	cases := []struct {
		name  string
		story storymaker.Story
		want  bool
	}{
		{"latin show text", storymaker.Story{Style: on, Scenes: []storymaker.Scene{{ID: "s1", Beats: show("Hallo")}}}, false},
		{"han show text", storymaker.Story{Style: off, Scenes: []storymaker.Scene{{ID: "s1", Beats: show("日本語")}}}, true},
		{"han say text", storymaker.Story{Style: off, Scenes: []storymaker.Scene{{ID: "s1", Beats: []storymaker.Beat{{Do: "say", Who: "anna", Text: "日本語"}}}}}, true},
		{"han narration with captions on", storymaker.Story{Style: on, Scenes: []storymaker.Scene{{ID: "s1", Narration: "日本語"}}}, true},
		{"han narration with captions off", storymaker.Story{Style: off, Scenes: []storymaker.Scene{{ID: "s1", Narration: "日本語"}}}, false},
		{"han narration in a silent scene", storymaker.Story{Style: on, Scenes: []storymaker.Scene{{ID: "s1", Silent: true, Narration: "日本語"}}}, false},
		{"captions off, a say without text draws the narration", storymaker.Story{Style: off, Scenes: []storymaker.Scene{{ID: "s1", Narration: "日本語", Beats: []storymaker.Beat{{Do: "say", Who: "anna"}}}}}, true},
		{"code scene in zh-CN", storymaker.Story{Style: off, Voice: voice("zh-CN"), Scenes: []storymaker.Scene{{ID: "s1", Kind: "code"}}}, true},
		{"code scene in ja", storymaker.Story{Style: off, Voice: voice("ja"), Scenes: []storymaker.Scene{{ID: "s1", Kind: "code"}}}, true},
		{"code scene in de", storymaker.Story{Style: off, Voice: voice("de"), Scenes: []storymaker.Scene{{ID: "s1", Kind: "code"}}}, false},
		{"code scene with a cast speaker in ja", storymaker.Story{Style: off, Voice: voice("de"), Cast: []storymaker.CastMember{{ID: "anna", Voice: voice("ja-JP")}}, Scenes: []storymaker.Scene{{ID: "s1", Kind: "code", Speaker: "anna"}}}, true},
		{"japanese voice on a storyboard scene with latin text", storymaker.Story{Style: off, Voice: voice("ja"), Scenes: []storymaker.Scene{{ID: "s1", Beats: show("Hallo")}}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := storyUsesCJK(&tc.story, glyphs); got != tc.want {
				t.Errorf("storyUsesCJK = %v, want %v", got, tc.want)
			}
		})
	}
	// The mini catalog has no glyph data: drawn text cannot resolve to a face, the language rule still holds.
	if storyUsesCJK(&storymaker.Story{Scenes: []storymaker.Scene{{ID: "s1", Beats: show("日本語")}}}, nil) {
		t.Error("nil glyphs: drawn text must not count")
	}
	if !storyUsesCJK(&storymaker.Story{Voice: voice("ja"), Scenes: []storymaker.Scene{{ID: "s1", Kind: "code"}}}, nil) {
		t.Error("nil glyphs: the code scene language must still count")
	}
}

func TestCodeSceneCount(t *testing.T) {
	s := storymaker.Story{Scenes: []storymaker.Scene{{ID: "a", Kind: "code"}, {ID: "b"}, {ID: "c", Kind: "code"}}}
	if got := codeSceneCount(&s); got != 2 {
		t.Errorf("codeSceneCount = %d, want 2", got)
	}
}
```

In `internal/storymaker/render/listener_test.go`, replace this text.

```go
func TestStartListenerRejectsWeakPrefixes(t *testing.T) {
```

With this text.

```go
func TestBootCarriesWorkerFieldsOnlyWhenSet(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	read := func(params PageParams) map[string]any {
		t.Helper()
		prefix, err := NewPrefix()
		if err != nil {
			t.Fatal(err)
		}
		token, err := NewPrefix() // 32 hex characters, the size StartListener asks of a token
		if err != nil {
			t.Fatal(err)
		}
		params.Width, params.Height, params.FPS, params.Transport, params.FrameLimit, params.Token = 1920, 1080, 30, TransportPNG, 27000, token
		l, err := StartListener(prefix, &Snapshot{}, params, Hooks{})
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		res, err := client.Get(l.Base() + "/boot.json")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var doc map[string]any
		if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
			t.Fatal(err)
		}
		return doc
	}
	absent := read(PageParams{})
	for _, key := range []string{"workers", "chunk", "pre_roll"} {
		if _, ok := absent[key]; ok {
			t.Errorf("boot.%s is present without a value: %v", key, absent[key])
		}
	}
	set := read(PageParams{Workers: 4, Chunk: 8, PreRoll: 2})
	if set["workers"] != float64(4) || set["chunk"] != float64(8) || set["pre_roll"] != float64(2) {
		t.Errorf("boot = %v", set)
	}
}

func TestStartListenerRejectsWeakPrefixes(t *testing.T) {
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/render -run '(?i)(ChooseWorkers|BoundMemoryToCgroup|StoryUsesCJK|CodeSceneCount|BootCarriesWorker)' -count=1
```

Expected: the package does not compile, with `undefined: ChooseWorkers`, `undefined: BoundMemoryToCgroup`, `undefined: storyUsesCJK`, `undefined: codeSceneCount` and `unknown field Workers in struct literal`.

- [ ] **Step 4: Implement**

Create `internal/storymaker/render/workers.go`.

```go
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
```

All old texts below are lines that the rows of N18, N19, N20, A17 and A19 do not list, so they read the same when this task runs. Those tasks do change the lines around them.

In `internal/storymaker/render/listener.go`, replace this text.

```go
	// FrameDeadlineMS is the per-frame render deadline the page may use; a render is not real time,
```

With this text.

```go
	// Workers, Chunk and PreRoll drive the parallel path of the page. Zero values stay out of boot.json,
	// and the page then renders serially.
	Workers, Chunk, PreRoll int
	// FrameDeadlineMS is the per-frame render deadline the page may use; a render is not real time,
```

In the same file, replace this text.

```go
}

func (l *Listener) boot(w http.ResponseWriter, r *http.Request) {
```

With this text.

```go
	Workers         int               `json:"workers,omitempty"`
	Chunk           int               `json:"chunk,omitempty"`
	PreRoll         int               `json:"pre_roll,omitempty"`
}

func (l *Listener) boot(w http.ResponseWriter, r *http.Request) {
```

In the same file, replace this text.

```go
	ids := make([]string, 0, len(l.snap.CodeBundles))
```

With this text.

```go
	doc.Workers, doc.Chunk, doc.PreRoll = l.params.Workers, l.params.Chunk, l.params.PreRoll
	ids := make([]string, 0, len(l.snap.CodeBundles))
```

In `internal/storymaker/render/render.go`, replace this text.

```go
	Limits             storymaker.Limits
```

With this text.

```go
	Limits             storymaker.Limits
	LogicalCPUs        int   // runtime.NumCPU() in production
	FreeMemoryMB       int64 // available memory in MiB; 0 means unknown, no memory bound
```

In the same file, replace this text.

```go
	h.SetPrefixHash(HashPrefix(prefix))
```

With this text.

```go
	h.SetPrefixHash(HashPrefix(prefix))
	workers := ChooseWorkers(run.transport, run.st.LogicalCPUs, run.st.FreeMemoryMB, run.width, run.height, parallelChunk,
		storyUsesCJK(&run.plan.Story, r.deps.Catalog.Glyphs()), codeSceneCount(&run.plan.Story))
```

In the same file, replace this text.

```go
FrameDeadlineMS: frameDeadlineMS
```

With this text.

```go
FrameDeadlineMS: frameDeadlineMS, Workers: workers, Chunk: parallelChunk, PreRoll: renderPreRoll
```

In `internal/server/storymaker_service.go`, replace this text.

```go
	"path/filepath"
```

With this text.

```go
	"os"
	"path/filepath"
	"runtime"
```

In the same file, replace this text.

```go
	"aurago/internal/videostudio"
```

With this text.

```go
	"aurago/internal/videostudio"

	"github.com/shirou/gopsutil/v4/mem"
```

In the same file, replace this text.

```go
		Limits:             storymakerLimits(cfg),
```

With this text.

```go
		Limits:             storymakerLimits(cfg),
		LogicalCPUs:        runtime.NumCPU(),
		FreeMemoryMB:       storymakerFreeMemoryMB(),
```

In the same file, replace this text.

```go
// storymakerAvailable is the capability behind Requires: ["storymaker"]: the feature flag and an
```

With this text.

```go
// storymakerFreeMemoryMB is the available memory in MiB, bounded by the cgroup limit of a container. 0
// means unknown, and ChooseWorkers then sets no memory bound.
func storymakerFreeMemoryMB() int64 {
	var hostMB int64
	if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
		hostMB = int64(vm.Available >> 20)
	}
	return render.BoundMemoryToCgroup(hostMB, os.DirFS("/sys/fs/cgroup"))
}

// storymakerAvailable is the capability behind Requires: ["storymaker"]: the feature flag and an
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
gofmt -w internal/storymaker/render/render.go internal/storymaker/render/listener.go internal/server/storymaker_service.go
go test ./internal/storymaker/render -run '(?i)(ChooseWorkers|BoundMemoryToCgroup|StoryUsesCJK|CodeSceneCount|BootCarriesWorker)' -count=1
go test ./internal/storymaker/render -count=1
go build ./internal/server
gofmt -l internal/storymaker internal/server
```

Expected: both test lines pass, the build is clean and `gofmt -l` prints nothing.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5. A PARTIAL answer on the new file needs a grep confirmation in the commit body.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/render/workers.go internal/storymaker/render/workers_test.go internal/storymaker/render/render.go internal/storymaker/render/listener.go internal/storymaker/render/listener_test.go internal/server/storymaker_service.go
git commit -m "feat(storymaker): choose render workers and carry them in the boot document" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task W2: PNG in the worker, helper engines
**Batch:** 2D  **Depends on:** none
**Files:**
- Modify: `ui/js/storymaker/worker-player.js`
- Modify: `ui/js/storymaker/host-core.js`
- Modify (generated): `ui/js/storymaker/host.bundle.js`, by `npm run build:ui`
- Modify (generated): `ui/js/storymaker/player.bundle.js`, by `npm run build:ui`
- Test: `scripts/storymaker-tests/07-workers.mjs`

- [ ] **Step 1: Impact**

Run from the main checkout, once per symbol: `handle`, `load`, `renderNow`.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "renderNow" --direction upstream --repo .
```

Handle HIGH, CRITICAL and UNKNOWN by the GitNexus rule of outline section 5.

- [ ] **Step 2: Write the failing test**

In `scripts/storymaker-tests/07-workers.mjs`, replace this text.

```js
import { t } from '../storymaker-test-context.mjs';
```

With this text.

```js
import { loadHost, TEST_LIMITS } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';
```

In the same file, replace this text.

```js
check('worker realm: the determinism shadows are installed before any scene code runs', async () => {
```

With this text.

```js
check('player worker: render with png answers a blob from convertToBlob, and no bitmap', async () => {
    const w = new InProcessWorker('player', ''), c = client(w), types = [];
    // env.makeCanvas reads OffscreenCanvas when the first frame draws, so this stand-in sees the encode call.
    w.context.OffscreenCanvas = class extends w.context.OffscreenCanvas {
        async convertToBlob(options) { types.push(options.type); return super.convertToBlob(options); }
    };
    await until(() => c.hello === 'player', 'hello');
    await c.call({ type: 'compile', story: minimal, catalog, audioInfo: {}, opts: {} });
    await c.call({ type: 'assets', assets: allAssets(), fonts: [font] });
    const frame = await c.call({ type: 'render', n: 3, codeFrames: {}, codeErrors: {}, png: true });
    assert.equal(frame.ok, true, JSON.stringify(frame.error));
    assert.equal(frame.bitmap, undefined, 'a png render hands over no bitmap');
    assert.ok(frame.blob.size > 0, 'the fake blob counts the draw calls of the frame');
    assert.deepEqual(types, ['image/png']);
    w.terminate();
});

check('engine: shared_code_scenes builds no scene workers and compiles the same film; render with png returns the blob', async () => {
    const roles = [], errors = [];
    const H = loadHost().StorymakerHost;
    const env = H.defaultEnv('http://unused', {
        hasWorkers: true,
        createWorker(role, text) { roles.push(role); return new InProcessWorker(role, text, {}); },
        setTimer: (fn, ms) => setTimeout(fn, ms), clearTimer: h => clearTimeout(h), now: () => Date.now(), limits: TEST_LIMITS,
        onError: e => errors.push(e.code)
    });
    const load = extra => ({ story: firewall, catalog, audio_info: {}, assets: allAssets(), fonts: [font], format: { fps: 30, width: 192, height: 108 }, ...extra });
    const main = H.createEngine(env), mainLoaded = await main.load(load({ code_bundles: { s3: s3Bundle } }));
    assert.deepEqual(roles, ['player', 'scene']);
    roles.length = 0;
    const helper = H.createEngine(env), helperLoaded = await helper.load(load({ code_bundles: {}, shared_code_scenes: main.codeScenesInfo }));
    assert.deepEqual(roles, ['player'], 'a helper engine builds no scene worker');
    assert.deepEqual(errors, [], 'a helper engine reports no missing bundle');
    assert.equal(helperLoaded.report.frame_count, mainLoaded.report.frame_count);
    assert.equal(JSON.stringify(helper.schedule), JSON.stringify(main.schedule));
    await helperLoaded.prepare();
    const blob = await helper.render(0, { png: true });
    assert.ok(blob && blob.size > 0, 'render with png returns the blob');
    const bitmap = await helper.render(1);
    assert.ok(bitmap.digest, 'without png the answer stays a bitmap');
    main.dispose();
    helper.dispose();
});

check('worker realm: the determinism shadows are installed before any scene code runs', async () => {
```

- [ ] **Step 3: Run it**

```powershell
npm run test:storymaker
```

Expected: two `FAIL` lines.
- `player worker: render with png ...` fails because the reply still holds a bitmap.
- `engine: shared_code_scenes ...` fails because the helper reports `code_error` for the missing bundle of `s3`.

- [ ] **Step 4: Implement**

`worker-player.js` changes in 2B only at its FontFace line, so the text below is unchanged. In `ui/js/storymaker/worker-player.js`, replace this text.

```js
                var bitmap = typeof main.canvas.transferToImageBitmap === 'function' ? main.canvas.transferToImageBitmap() : main.canvas;
```

With this text.

```js
                if (msg.png) {   // the parallel path of the render page: the PNG is encoded here, so the host main thread stays free
                    var blob = await main.canvas.convertToBlob({ type: 'image/png' });
                    var painted = msg.codeFrames || {};
                    for (var cid in painted) if (painted[cid] && typeof painted[cid].close === 'function') painted[cid].close();
                    return { payload: { blob: blob, width: width, height: height } };
                }
                var bitmap = typeof main.canvas.transferToImageBitmap === 'function' ? main.canvas.transferToImageBitmap() : main.canvas;
```

`host-core.js` changes in 2B at `fontDescriptor` and the `buildScene` options, not in the lines below. In `ui/js/storymaker/host-core.js`, replace this text.

```js
    // load(msg): msg = {story, catalog, audio_info, assets, fonts, code_bundles, format}. Resolves with the compile report
```

With this text.

```js
    // load(msg): msg = {story, catalog, audio_info, assets, fonts, code_bundles, format, shared_code_scenes?}. Resolves with the compile report
```

In the same file, replace this text.

```js
            if (self.mode === 'fallback') {
```

With this text.

```js
            if (msg.shared_code_scenes) {   // a helper engine of the parallel path: the main engine built the scene, this one only takes its duration
                codeScenes[cid] = msg.shared_code_scenes[cid] || {};
                return;
            }
            if (self.mode === 'fallback') {
```

In the same file, replace this text.

```js
var compiled = await this.send(
```

With this text.

```js
this.codeScenesInfo = codeScenes;
        var compiled = await this.send(
```

In the same file, replace this text.

```js
    // render(n, opts): the frame as an ImageBitmap (a canvas in the fallback). Calls are serialised.
```

With this text.

```js
    // render(n, opts): the frame as an ImageBitmap (a canvas in the fallback), or with opts.png as a PNG Blob encoded in the worker. Calls are serialised.
```

In the same file, replace this text.

```js
            var out = await this.send({ type: 'render', n: n, width: width, codeFrames: codeFrames, codeErrors: this.codeErrors }, ms, transfer);
```

With this text.

```js
            var out = await this.send({ type: 'render', n: n, width: width, codeFrames: codeFrames, codeErrors: this.codeErrors, png: !!opts.png }, ms, transfer);
```

In the same file, replace this text.

```js
            return out.bitmap;
```

With this text.

```js
            return opts.png ? out.blob : out.bitmap;
```

- [ ] **Step 5: Run**

```powershell
npm run build:ui
npm run check:ui
npm run test:storymaker
```

Expected: `check:ui` passes and the suite ends with `0 failed`. `build:ui` also rewrites `ui/js/desktop/bundles/main.bundle.js`. Leave that file unstaged, as outline section 2 says for 2D.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/storymaker/worker-player.js ui/js/storymaker/host-core.js ui/js/storymaker/host.bundle.js ui/js/storymaker/player.bundle.js scripts/storymaker-tests/07-workers.mjs
git commit -m "feat(storymaker): encode PNG in the player worker and add helper engines" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task W3: `host-pool.js`
**Batch:** 2D  **Depends on:** W2
**Files:**
- Create: `ui/js/storymaker/host-pool.js`
- Modify: `scripts/storymaker-bundle-parts.js`
- Modify (generated): `ui/js/storymaker/host.bundle.js`, by `npm run build:ui`
- Test: `scripts/storymaker-tests/22-host-pool.mjs`
- Test: `scripts/storymaker-tests/09-host-render.mjs`

- [ ] **Step 1: Impact**

Run from the main checkout.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "storymakerHostParts" --direction upstream --repo .
```

Handle HIGH, CRITICAL and UNKNOWN by the GitNexus rule of outline section 5. The known callers are `scripts/build-ui-bundles.js`, `scripts/storymaker-test-support.mjs` and the guards.

- [ ] **Step 2: Write the failing test**

Create `scripts/storymaker-tests/22-host-pool.mjs`.

```js
// Task W3: the parallel scheduler of the render page: in-order posting, the reorder window, the fixed deal of chunks, restart and crash.
import assert from 'node:assert/strict';
import { loadHost, plain, tick, settle, until } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check } = t;
const runPool = opts => loadHost().StorymakerHost.runPool(opts);

// One storyboard scene over the whole film unless a test passes scenes.
const scheduleOf = (limit, scenes) => ({
    fps: 30, frame_count: limit,
    scenes: scenes || [{ id: 'a', kind: 'storyboard', start_frame: 0, end_frame: limit, tf: 0, transition: 'cut' }]
});

// Fake engines: render(n) yields once, logs who drew what, and hands back a blob that names the frame.
function fakeEngines(count, schedule, log, failAt) {
    return Array.from({ length: count }, (_, w) => ({
        schedule,
        async render(n, o) {
            log.push({ w, n, png: o && o.png });
            await tick();
            if (failAt === n) throw { code: 'deadline', message: 'worker ' + w + ' overran frame ' + n };
            return { frame: n };
        }
    }));
}

check('pool: frames post in order with one POST in flight, each drawn by its worker in the fixed deal', async () => {
    // 3 workers, chunks of 4, 29 frames: frame f is in chunk floor(f/4), and chunk j goes to worker j % 3.
    const limit = 29, log = [], posted = [];
    let inFlight = 0, most = 0;
    const outcome = await runPool({
        engines: fakeEngines(3, scheduleOf(limit), log), limit, chunk: 4, preRoll: 0, status: {},
        post: async (n, blob) => {
            inFlight++;
            most = Math.max(most, inFlight);
            assert.equal(blob.frame, n);
            posted.push(n);
            await tick();
            inFlight--;
            return n === limit - 1 ? { ok: true, done: true } : { ok: true };
        }
    });
    assert.equal(plain(outcome).phase, 'done');
    assert.deepEqual(posted, [...Array(limit).keys()]);
    assert.equal(most, 1);
    assert.equal(log.length, limit, 'without pre-roll each frame renders once');
    for (const e of log) {
        assert.equal(e.w, Math.floor(e.n / 4) % 3, 'worker of frame ' + e.n);
        assert.equal(e.png, true, 'the PNG is encoded in the worker');
    }
});

check('pool: a worker waits while its chunk starts N*C*2 frames or more past the next frame to post', async () => {
    // 2 workers, chunks of 2: the window is 2*2*2 = 8. The first POST hangs, so the next frame stays 0.
    // Chunks starting at 0, 2, 4 and 6 may run, and they end at frame 7. A chunk starting at 8 may not.
    const limit = 40, log = [];
    let release;
    const hold = new Promise(resolve => { release = resolve; });
    const done = runPool({
        engines: fakeEngines(2, scheduleOf(limit), log), limit, chunk: 2, preRoll: 0, status: {},
        post: async n => {
            if (n === 0) await hold;
            return n === limit - 1 ? { ok: true, done: true } : { ok: true };
        }
    });
    await until(() => log.length >= 8, 'the window to fill');
    await settle(40);
    assert.equal(Math.max(...log.map(e => e.n)), 7, 'nothing past the window runs while frame 0 is in flight');
    release();
    assert.equal(plain(await done).phase, 'done');
    assert.equal(Math.max(...log.map(e => e.n)), limit - 1);
});

check('pool: a chunk with code-scene frames goes to worker 0, also for the outgoing layer of a transition', async () => {
    // 2 workers, chunks of 4, 40 frames. Scene b (code) runs 16..27. Scene c starts at 28 with a 2-frame fade, so frames 28
    // and 29 still draw b. Chunks 4, 5, 6 and 7 hold code frames and go to worker 0. Without code, 5 and 7 would go to worker 1.
    const limit = 40, log = [];
    const schedule = scheduleOf(limit, [
        { id: 'a', kind: 'storyboard', start_frame: 0, end_frame: 16, tf: 0, transition: 'cut' },
        { id: 'b', kind: 'code', start_frame: 16, end_frame: 28, tf: 0, transition: 'cut' },
        { id: 'c', kind: 'storyboard', start_frame: 28, end_frame: 40, tf: 2, transition: 'fade' }
    ]);
    await runPool({
        engines: fakeEngines(2, schedule, log), limit, chunk: 4, preRoll: 0, status: {},
        post: async n => (n === limit - 1 ? { ok: true, done: true } : { ok: true })
    });
    const byFrame = new Map(log.map(e => [e.n, e.w]));
    for (let f = 16; f < 32; f++) assert.equal(byFrame.get(f), 0, 'code chunk frame ' + f);
    for (const f of [4, 12, 36]) assert.equal(byFrame.get(f), 1, 'chunks 1, 3 and 9 keep the deal j % 2, frame ' + f);
    for (const f of [0, 8, 32]) assert.equal(byFrame.get(f), 0, 'chunks 0, 2 and 8 keep the deal j % 2, frame ' + f);
});

check('pool: a restart abandons the generation and runs the same engines again from frame 0', async () => {
    const limit = 12, log = [], posted = [], status = {};
    let restarted = false;
    const outcome = await runPool({
        engines: fakeEngines(2, scheduleOf(limit), log), limit, chunk: 4, preRoll: 0, status,
        post: async n => {
            posted.push(n);
            if (n === 5 && !restarted) { restarted = true; return { restart: true }; }
            return n === limit - 1 ? { ok: true, done: true } : { ok: true };
        }
    });
    assert.equal(plain(outcome).phase, 'done');
    assert.deepEqual(posted, [0, 1, 2, 3, 4, 5, ...Array(limit).keys()]);
    assert.equal(status.restarts, 1);
    assert.equal(log.filter(e => e.n === 0).length, 2, 'frame 0 is drawn again');
});

check('pool: a worker that fails rejects the run and stops the poster', async () => {
    // 2 workers, chunks of 4: frame 6 is in chunk 1 on worker 1 and fails, so no frame past 5 can ever post.
    const limit = 24, log = [], posted = [];
    const run = runPool({
        engines: fakeEngines(2, scheduleOf(limit), log, 6), limit, chunk: 4, preRoll: 0, status: {},
        post: async n => { posted.push(n); return { ok: true }; }
    });
    await assert.rejects(run, e => e.code === 'deadline');
    assert.ok(posted.every(n => n <= 5), JSON.stringify(posted));
});
```

- [ ] **Step 3: Run it**

```powershell
npm run test:storymaker
```

Expected: five `FAIL pool: ...` lines with `runPool is not a function`, because the part does not exist yet.

- [ ] **Step 4: Implement**

Create `ui/js/storymaker/host-pool.js`.

```js
// Storymaker host (document realm): the parallel frame scheduler of the render page. Chunks of C frames go to the player
// engines in a fixed deal, the frames come back as PNG blobs encoded in the workers, and one poster sends them in frame order
// with one POST in flight, so the ingest rule of Go stays as it is. Classic script; publishes StorymakerHost.runPool.
(function (root) {
    'use strict';
    var H = root.StorymakerHost || (root.StorymakerHost = {});

    // frameHasCode(schedule, f): true when frame f draws a code scene, also as the outgoing layer of a transition.
    function frameHasCode(schedule, f) {
        var layers = root.StorymakerCore.schedule.framePlan(schedule, f).layers;
        for (var i = 0; i < layers.length; i++) {
            if (schedule.scenes[layers[i].i].kind === 'code') return true;
        }
        return false;
    }

    // dealChunks: chunk j goes to worker j % N, and a chunk with code-scene frames to worker 0, the only one with scene workers.
    function dealChunks(schedule, limit, chunk, workers) {
        var deal = [], w, j, f;
        for (w = 0; w < workers; w++) deal.push([]);
        for (j = 0; j * chunk < limit; j++) {
            var hasCode = false, end = Math.min(limit, (j + 1) * chunk);
            for (f = j * chunk; f < end && !hasCode; f++) hasCode = frameHasCode(schedule, f);
            deal[hasCode ? 0 : j % workers].push(j);
        }
        return deal;
    }

    function wake(gen) {
        var waiting = gen.waiters;
        gen.waiters = [];
        for (var i = 0; i < waiting.length; i++) waiting[i]();
    }
    function sleepOn(gen) {
        return new Promise(function (resolve) { gen.waiters.push(resolve); });
    }
    function abort(gen, err) {
        if (err && !gen.err) gen.err = err;
        gen.abort = true;
        wake(gen);
    }

    // renderChunks: one worker's loop. It waits while its next chunk starts span frames or more past the next frame to post.
    // The worker that owns the next frame never waits, so the window cannot deadlock.
    async function renderChunks(gen, o, engine, chunks, span) {
        try {
            for (var i = 0; i < chunks.length; i++) {
                var start = chunks[i] * o.chunk, end = Math.min(o.limit, start + o.chunk), f;
                while (!gen.abort && start - gen.next >= span) await sleepOn(gen);
                if (gen.abort) return;
                for (f = Math.max(0, start - o.preRoll); f < start; f++) {
                    await engine.render(f, { png: true });   // pre-roll: painted, dropped
                    if (gen.abort) return;
                }
                for (f = start; f < end; f++) {
                    var blob = await engine.render(f, { png: true });
                    if (gen.abort) return;
                    gen.buffer[f] = blob;
                    wake(gen);
                }
            }
        } catch (err) {
            abort(gen, err);
        }
    }

    // postFrames: the one in-order poster. A reply without stop, restart or done moves it to the next frame.
    async function postFrames(gen, o) {
        try {
            while (!gen.abort) {
                var n = gen.next, blob = gen.buffer[n];
                if (blob === undefined) { await sleepOn(gen); continue; }
                delete gen.buffer[n];
                var reply = await o.post(n, blob);
                o.status.frame = n;
                if (reply.stop) { gen.outcome = { phase: 'stopped', error: reply.error || '', message: reply.message || '' }; break; }
                if (reply.restart) break;
                if (reply.done) { gen.outcome = { phase: 'done' }; break; }
                gen.next = n + 1;
                if (gen.next >= o.limit) throw H.fail('render_failed', 'The page ran past the last frame (' + o.limit + ') without a done answer.');
                wake(gen);
            }
            abort(gen);
        } catch (err) {
            abort(gen, err);
        }
    }

    // runPool(o): o = {engines, limit, chunk, preRoll, post(n, blob), status}. engines[0] holds the code scenes. Resolves with
    // {phase: 'done'} or {phase: 'stopped', error, message}. A crash, a deadline, a failed POST or an overrun rejects. On a
    // restart answer the generation is abandoned, its buffer goes with it, and the same engines run again from frame 0.
    H.runPool = async function (o) {
        var workers = o.engines.length, span = workers * o.chunk * 2;
        var deal = dealChunks(o.engines[0].schedule, o.limit, o.chunk, workers);
        o.status.workers = workers;
        for (;;) {
            var gen = { abort: false, next: 0, buffer: {}, waiters: [], err: null, outcome: null };
            var tasks = [postFrames(gen, o)];
            for (var w = 0; w < workers; w++) tasks.push(renderChunks(gen, o, o.engines[w], deal[w], span));
            await Promise.all(tasks);
            if (gen.err) throw gen.err;
            if (gen.outcome) return gen.outcome;
            o.status.restarts = (o.status.restarts || 0) + 1;
        }
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

No earlier task adds a host part, so the texts below are as they stand on main. In `scripts/storymaker-bundle-parts.js`, replace this text.

```js
  'ui/js/storymaker/host-render.js',
```

With this text.

```js
  'ui/js/storymaker/host-pool.js',
  'ui/js/storymaker/host-render.js',
```

In `scripts/storymaker-tests/09-host-render.mjs`, replace this text.

```js
    assert.ok(storymakerHostParts.length === 5 && storymakerHostParts[0].endsWith('schedule.js'));
```

With this text.

```js
    assert.ok(storymakerHostParts.length === 6 && storymakerHostParts[0].endsWith('schedule.js'));
```

- [ ] **Step 5: Run**

```powershell
npm run build:ui
npm run check:ui
npm run test:storymaker
```

Expected: `check:ui` passes and the suite ends with `0 failed`. Leave `ui/js/desktop/bundles/main.bundle.js` unstaged.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5. A PARTIAL answer on the new file needs a grep confirmation in the commit body.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/storymaker/host-pool.js ui/js/storymaker/host.bundle.js scripts/storymaker-bundle-parts.js scripts/storymaker-tests/22-host-pool.mjs scripts/storymaker-tests/09-host-render.mjs
git commit -m "feat(storymaker): add the parallel frame scheduler of the render page" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task W4: Parallel path in the render page. Wire: Go+JS
**Batch:** 2D  **Depends on:** W1, W3
**Files:**
- Modify: `ui/js/storymaker/host-render.js`
- Modify (generated): `ui/js/storymaker/host.bundle.js`, by `npm run build:ui`
- Test: `scripts/storymaker-tests/09-host-render.mjs`

- [ ] **Step 1: Impact**

Run from the main checkout.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "mountRender" --direction upstream --repo .
```

Handle HIGH, CRITICAL and UNKNOWN by the GitNexus rule of outline section 5.

- [ ] **Step 2: Write the failing test**

The test helpers `runRender` and `makeServer` are as N18 and A19 leave them. Those tasks do not touch the lines quoted here. In `scripts/storymaker-tests/09-host-render.mjs`, replace this text.

```js
    const workers = [];
```

With this text.

```js
    const workers = opts.workers || [];
```

In the same file, replace this text.

```js
        createWorker(role, text) { const w = new InProcessWorker(role, text, {}); workers.push(w); return w; },
```

With this text.

```js
        createWorker(role, text) { const w = new InProcessWorker(role, text, {}); workers.push(w); if (opts.onWorker) opts.onWorker(w, workers.length); return w; },
```

In the same file, replace this text.

```js
check('host bundle is small and self-contained: no codescene, rig or compile in the document realm', () => {
```

With this text.

```js
check('render mode: with boot.workers the helpers load and prepare before the report, and frames post in order', async () => {
    const workers = [];
    let atReport = null;
    const r = await runRender({
        workers, boot: { workers: 3, chunk: 8 },
        report: () => {
            atReport = workers.filter(w => w.role === 'player').map(w => w.received.includes('assets'));
            return { start: true };
        }
    });
    assert.equal(r.status.phase, 'done', JSON.stringify(r.status));
    assert.deepEqual(atReport, [true, true, true], 'all three player workers hold their assets when the report goes out');
    assert.equal(workers.filter(w => w.role === 'scene').length, 1, 'the code scene has one worker, in the main engine');
    const count = r.server.posts.find(p => p.path === '/report').report.frame_count;
    assert.deepEqual(r.frames.map(f => f.n), [...Array(count).keys()]);
    assert.ok(r.frames.every(f => typeof f.body.size === 'number' && f.headers['Content-Type'] === 'text/plain'), 'the blobs come from the workers, as CORS-simple requests');
    assert.equal(r.status.workers, 3);
});

check('render mode: a helper that compiles another frame count ends the job with render_failed before the report', async () => {
    const r = await runRender({
        story: minimal, noCode: true, boot: { workers: 2, chunk: 8 },
        // The second worker of a story without code scenes is the helper: its compile answer gains one frame.
        onWorker(w, count) {
            if (count !== 2) return;
            const deliver = w.deliverToHost.bind(w);
            w.deliverToHost = data => deliver(data && data.report ? { ...data, report: { ...data.report, frame_count: data.report.frame_count + 1 } } : data);
        }
    });
    assert.equal(r.status.phase, 'error');
    assert.equal(r.status.error, 'render_failed');
    assert.equal(r.server.posts.filter(p => p.path === '/report').length, 0);
    assert.equal(r.server.posts.find(p => p.path === '/error').error.code, 'render_failed');
});

check('render mode: workers 1 and every other transport keep the serial path', async () => {
    const one = [], webp = [];
    const single = await runRender({ story: minimal, noCode: true, workers: one, boot: { workers: 1, chunk: 8 } });
    assert.equal(single.status.phase, 'done');
    assert.equal(one.filter(w => w.role === 'player').length, 1);
    assert.equal(single.frames[0].body.type, 'image/png', 'encoded on the visible canvas');
    const other = await runRender({ transport: 'webp', story: minimal, noCode: true, workers: webp, boot: { workers: 3, chunk: 8 } });
    assert.equal(other.status.phase, 'done');
    assert.equal(webp.filter(w => w.role === 'player').length, 1);
    assert.equal(other.frames[0].body.type, 'image/webp');
});

check('host bundle is small and self-contained: no codescene, rig or compile in the document realm', () => {
```

- [ ] **Step 3: Run it**

```powershell
npm run test:storymaker
```

Expected: two `FAIL` lines.
- `with boot.workers ...` fails because `atReport` holds one worker, not three.
- `a helper that compiles ...` fails because the phase is `done`.

The third new check already passes, because the serial path is all there is so far.

- [ ] **Step 4: Implement**

The lines quoted below are not in the line lists of N18 (33, 42, 55, 173-174), N19 (72-76) or A19 (105-110, 154). The new post call follows the form that N18 keeps at the serial path: `post('/frame?n=' + n, body, ...)`.

In `ui/js/storymaker/host-render.js`, replace this text.

```js
        var engine = null;
```

With this text.

```js
        var engine = null, helpers = [];
```

In the same file, replace this text.

```js
            var rep = loaded.report;
```

With this text.

```js
            var rep = loaded.report;
            // Parallel path: only PNG is encoded in the worker, and a main engine in the document-realm fallback stays serial.
            // The helpers start before the report, so their start-up cannot trip the 10 s frame watchdog of Go.
            var parallel = boot.transport === 'png' && boot.workers > 1 && boot.chunk > 0 && engine.mode === 'workers';
            if (parallel) {
                for (var hi = 1; hi < boot.workers; hi++) helpers.push(H.createEngine(env));
                await Promise.all(helpers.map(async function (helper) {
                    var made = await helper.load({
                        story: story, catalog: catalog, audio_info: boot.audio_info || {}, assets: {}, fonts: [], code_bundles: {},
                        format: { fps: boot.fps, width: boot.width, height: boot.height }, shared_code_scenes: engine.codeScenesInfo
                    });
                    if (helper.mode !== 'workers') throw H.fail('render_failed', 'A helper worker could not start.');
                    if (made.report.frame_count !== rep.frame_count || JSON.stringify(helper.schedule) !== JSON.stringify(engine.schedule)) {
                        throw H.fail('render_failed', 'A helper worker compiled ' + made.report.frame_count + ' frames, not ' + rep.frame_count + '.');
                    }
                    await made.prepare();
                }));
            }
```

In the same file, replace this text.

```js
            status.phase = 'rendering';
```

With this text.

```js
            status.phase = 'rendering';
            if (parallel) {
                var outcome = await H.runPool({
                    engines: [engine].concat(helpers), limit: limit, chunk: boot.chunk, preRoll: boot.pre_roll > 0 ? boot.pre_roll : 0, status: status,
                    post: function (f, blob) { return post('/frame?n=' + f, blob, { 'Content-Type': 'text/plain' }); }
                });
                helpers.forEach(function (helper) { helper.dispose(); });
                status.phase = outcome.phase;
                if (outcome.phase === 'stopped') { status.error = outcome.error; status.message = outcome.message; }
                return status;
            }
```

In the same file, replace this text.

```js
            if (engine) engine.dispose();
```

With this text.

```js
            if (engine) engine.dispose();
            helpers.forEach(function (helper) { helper.dispose(); });
```

- [ ] **Step 5: Run**

```powershell
npm run build:ui
npm run check:ui
npm run test:storymaker
```

Expected: `check:ui` passes and the suite ends with `0 failed`. Leave `ui/js/desktop/bundles/main.bundle.js` unstaged.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/storymaker/host-render.js ui/js/storymaker/host.bundle.js scripts/storymaker-tests/09-host-render.mjs
git commit -m "feat(storymaker): render with parallel player workers when boot.json asks for them" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task W5: Parallel parity and forced encoder failure
**Batch:** 2D  **Depends on:** W4, S11
**Files:**
- Modify: `internal/storymaker/testdata/stories/speech.json`
- Test: `internal/storymaker/render/browser_smoke_test.go`

- [ ] **Step 1: Write the failing test**

A24 added the `storymakertest` import to this file, so only `videostudio` is new. In `internal/storymaker/render/browser_smoke_test.go`, replace this text.

```go
	"aurago/internal/storymaker/storymakertest"
```

With this text.

```go
	"aurago/internal/storymaker/storymakertest"
	"aurago/internal/videostudio"
```

In the same file, replace this text.

```go
// The render browser must not open a DevTools port. Such a port listens on loopback, which every
```

With this text.

```go
// pageEndSpy keeps the last __smStatus of the render page. execute closes the page when the job ends, so
// the wrapper reads the status first.
type pageEndSpy struct {
	mu   sync.Mutex
	last string
}

type pageEndPage struct {
	browserPage
	spy *pageEndSpy
}

func (p *pageEndPage) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	status := p.browserPage.Status(ctx)
	cancel()
	p.spy.mu.Lock()
	p.spy.last = status
	p.spy.mu.Unlock()
	p.browserPage.Close()
}

// end decodes what the parallel path leaves in __smStatus. A serial render reports no workers.
func (s *pageEndSpy) end(t *testing.T) (workers, restarts int) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	var doc struct {
		Workers  int `json:"workers"`
		Restarts int `json:"restarts"`
	}
	if err := json.Unmarshal([]byte(s.last), &doc); err != nil {
		t.Fatalf("page status %q: %v", s.last, err)
	}
	return doc.Workers, doc.Restarts
}

// newParallelFixture is a real-browser fixture on a stated CPU count, with no memory bound.
func newParallelFixture(t *testing.T, chrome string, cpus int, observer func(int, string, []byte)) (*runFixture, *pageEndSpy) {
	t.Helper()
	fx := newRealFixture(t, chrome)
	fx.settings.LogicalCPUs = cpus
	fx.settings.TimeoutSeconds = 600
	fx.deps.FrameObserver = observer
	spy := &pageEndSpy{}
	fx.deps.OpenPage = func(ctx context.Context, o sessionOptions) (browserPage, error) {
		p, err := openSession(ctx, o)
		if err != nil {
			return nil, err
		}
		return &pageEndPage{browserPage: p, spy: spy}, nil
	}
	fx.rebuild(t)
	return fx, spy
}

// loadParallelStory saves a story fixture, gives each code scene the s3 source and seeds the narration, so
// the narration_incomplete gate passes. SeedNarration writes missing pieces only.
func (fx *runFixture) loadParallelStory(t *testing.T, name string) storymaker.Story {
	t.Helper()
	ctx := context.Background()
	lf := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
	data, err := os.ReadFile("../testdata/stories/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, lf(data), ""); err != nil {
		t.Fatal(err)
	}
	story, _, err := storymaker.DecodeStory(lf(data), fx.settings.Limits)
	if err != nil {
		t.Fatal(err)
	}
	storymaker.ApplyDefaults(&story)
	src, err := os.ReadFile("../testdata/scenes/s3.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, sc := range story.Scenes {
		if sc.Kind != "code" {
			continue
		}
		if _, res, err := fx.projects.SaveCodeScene(ctx, fx.projectID, sc.ID, lf(src), ""); err != nil || len(res.Errors) != 0 {
			t.Fatalf("SaveCodeScene %s: %v %+v", sc.ID, err, res)
		}
	}
	if err := storymakertest.SeedNarration(ctx, fx.projects, fx.projectID, &story, 1.5); err != nil {
		t.Fatal(err)
	}
	return story
}

// requireSpeechCoverage keeps the parity test honest: the speech fixture must hold a bubble and a sliding
// text in the CJK face, the case where a wrap could depend on the frames a worker drew before.
func requireSpeechCoverage(t *testing.T, fx *runFixture, story *storymaker.Story) {
	t.Helper()
	glyphs := fx.deps.Catalog.Glyphs()
	if glyphs == nil {
		t.Fatal("the real catalog names no glyphs file")
	}
	bubble, sliding := false, false
	for _, sc := range story.Scenes {
		for _, b := range sc.Beats {
			if b.Do == "say" && b.Text != "" {
				bubble = true
			}
			if b.Do != "show" || b.Entry != "slide" {
				continue
			}
			for _, r := range b.Text {
				if i := glyphs.FaceOf(r); i >= 0 && glyphs.Faces[i].ID == "sm-cjk" {
					sliding = true
				}
			}
		}
	}
	if !bubble {
		t.Fatal("speech.json has no say with text, so the parity test would not cover a bubble")
	}
	if !sliding {
		t.Fatal("speech.json has no sliding text in the CJK face, so the parity test would not cover the layout memo at moving widths")
	}
}

type parallelRun struct {
	hashes            *frameHashes
	workers, restarts int
}

func renderParallelStory(t *testing.T, chrome, name string, cpus int) parallelRun {
	t.Helper()
	hashes := newFrameHashes()
	fx, spy := newParallelFixture(t, chrome, cpus, hashes.observe(t))
	story := fx.loadParallelStory(t, name)
	if name == "speech" {
		requireSpeechCoverage(t, fx, &story)
	}
	job := fx.render(t)
	if job.Status != StatusSucceeded {
		t.Fatalf("%s on %d CPUs: %+v %+v", name, cpus, job, job.Error)
	}
	if hashes.count() != job.FramesTotal {
		t.Fatalf("%s on %d CPUs: observed %d of %d frames", name, cpus, hashes.count(), job.FramesTotal)
	}
	workers, restarts := spy.end(t)
	return parallelRun{hashes: hashes, workers: workers, restarts: restarts}
}

// firstFrameDifference is the first frame whose decoded RGBA differs, or -1.
func firstFrameDifference(a, b *frameHashes) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	b.mu.Lock()
	defer b.mu.Unlock()
	for n := 0; n < max(len(a.hashes), len(b.hashes)); n++ {
		if a.hashes[n] != b.hashes[n] {
			return n
		}
	}
	return -1
}

func distinctFrames(f *frameHashes) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := map[string]bool{}
	for _, h := range f.hashes {
		seen[h] = true
	}
	return len(seen)
}

func TestParallelParityBrowser(t *testing.T) {
	chrome := browserSmoke(t)
	old := renderPreRoll
	t.Cleanup(func() { renderPreRoll = old })
	// 14 CPUs give floor((14-2)/3) = 4 workers with no memory bound. 1 CPU clamps to 1, the serial path.
	for _, name := range []string{"firewall", "speech"} {
		t.Run(name, func(t *testing.T) {
			renderPreRoll = 0
			serial := renderParallelStory(t, chrome, name, 1)
			if serial.workers != 0 {
				t.Fatalf("the serial run reports %d pool workers", serial.workers)
			}
			if distinctFrames(serial.hashes) < 2 {
				t.Fatalf("all frames of %s are the same, the comparison proves nothing", name)
			}
			for _, preRoll := range []int{0, 2} {
				renderPreRoll = preRoll
				run := renderParallelStory(t, chrome, name, 14)
				if run.workers != 4 {
					t.Fatalf("pre-roll %d: the page ran %d pool workers, want 4", preRoll, run.workers)
				}
				if at := firstFrameDifference(serial.hashes, run.hashes); at >= 0 {
					t.Errorf("pre-roll %d: frame %d differs from the serial render", preRoll, at)
				}
			}
		})
	}
}

func TestParallelEncoderFailureBrowser(t *testing.T) {
	chrome := browserSmoke(t)
	var mu sync.Mutex
	arrivals := map[int]int{}
	hashes := newFrameHashes()
	observe := hashes.observe(t)
	// 14 CPUs give floor((14-2)/3) = 4 workers.
	fx, spy := newParallelFixture(t, chrome, 14, func(n int, transport string, data []byte) {
		mu.Lock()
		arrivals[n]++
		mu.Unlock()
		observe(n, transport, data)
	})
	// The missing device makes the hardware encoder fail once. recoverEncoder then restarts with libx264.
	fx.settings.Encoder = videostudio.Encoder{Codec: videostudio.CodecVAAPI, Device: "/dev/dri/does-not-exist"}
	fx.settings.EncoderName = videostudio.CodecVAAPI
	fx.loadParallelStory(t, "firewall")
	job := fx.render(t)
	if job.Status != StatusSucceeded {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	workers, restarts := spy.end(t)
	if workers != 4 || restarts != 1 {
		t.Fatalf("the page ran %d pool workers and %d restarts, want 4 and 1", workers, restarts)
	}
	mu.Lock()
	first := arrivals[0]
	mu.Unlock()
	// One failure, one restart: frame 0 reaches Go once before the restart and once after it.
	if first != 2 {
		t.Errorf("frame 0 arrived %d times, want 2", first)
	}
	if hashes.count() != job.FramesTotal {
		t.Errorf("observed %d of %d frames", hashes.count(), job.FramesTotal)
	}
	meta, _ := fx.pub.get("Documents/Storymaker/" + fx.projectID + "/exports/" + job.ID + ".json")
	if !strings.Contains(string(meta), `"encoder": "libx264"`) {
		t.Errorf("record = %s", meta)
	}
}

// The render browser must not open a DevTools port. Such a port listens on loopback, which every
```

- [ ] **Step 2: Run it**

Run browser lines through `go test` with the shell sandbox off.

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/storymaker/render -run '^TestParallelParityBrowser$/^speech$' -count=1 -timeout 5m; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
```

Expected: `--- FAIL: TestParallelParityBrowser/speech` with `speech.json has no sliding text in the CJK face`. The fixture of S11 has no such beat yet.

- [ ] **Step 3: Implement**

S11 owns the fixture text, so this step edits it through a node script. Write it with the Write tool as `add-sliding-cjk.mjs` in your session scratchpad, not in the project.

```js
// Adds one sliding Han text to the speech fixture. The parity test needs it to cover the layout memo at moving widths.
import fs from 'node:fs';

const TEXT = '滑入的文字';
const file = process.argv[2];
const story = JSON.parse(fs.readFileSync(file, 'utf8'));
// A scene with an explicit duration is never extended, and the beat could fall off its end.
const scene = story.scenes.find(s => s.kind !== 'code' && s.duration === undefined);
if (!scene) throw new Error('speech.json has no storyboard scene with an automatic length');
scene.beats = scene.beats || [];
if (!scene.beats.some(b => b.text === TEXT)) {
    scene.beats.push({ do: 'show', text: TEXT, style: 'caption', at: { x: 50, y: 30 }, entry: 'slide' });
}
fs.writeFileSync(file, JSON.stringify(story, null, 1) + '\n');
```

Run it from the worktree. Replace `<scratchpad>` with your session scratchpad path.

```powershell
node <scratchpad>/add-sliding-cjk.mjs internal/storymaker/testdata/stories/speech.json
```

- [ ] **Step 4: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
npm run test:storymaker
go test ./internal/storymaker -count=1
$env:AURAGO_RUN_BROWSER_SMOKE='1'
go test ./internal/storymaker/render -run '^TestParallelParityBrowser$' -count=1 -timeout 20m
go test ./internal/storymaker/render -run '^TestParallelEncoderFailureBrowser$' -count=1 -timeout 15m
Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
gofmt -l internal/storymaker
```

Expected: the suite ends with `0 failed`, S11's real-catalog check included. Both browser tests pass and `gofmt -l` prints nothing.

A frame difference in the parity test is a real finding. Send it to debug-agent with the frame number and do not pin around it. If it cannot be fixed, outline section 1 says how W is dropped.

- [ ] **Step 5: Commit**

Run `detect_changes` per outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/render/browser_smoke_test.go internal/storymaker/testdata/stories/speech.json
git commit -m "test(storymaker): prove parallel render parity and the encoder restart in Chrome" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

{% endraw %}

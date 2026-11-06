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

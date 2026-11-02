package storymaker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

const testRoot = "Documents/Storymaker"

// newTestStore returns a ProjectStore on a MemStore with a controllable clock and sequential ids.
func newTestStore(t *testing.T) (*ProjectStore, *MemStore, *time.Time) {
	t.Helper()
	mem := NewMemStore()
	ps := NewProjectStore(mem, testRoot)
	clock := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	ps.now = func() time.Time { return clock }
	n := 0
	ps.newID = func() string {
		n++
		return fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
	}
	return ps, mem, &clock
}

func TestProjectCreateLayoutAndDefaults(t *testing.T) {
	ps, mem, _ := newTestStore(t)
	ctx := context.Background()
	proj, story, etag, err := ps.Create(ctx, "  Was ist eine Firewall?  ", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !ValidProjectID(proj.ID) || proj.Title != "Was ist eine Firewall?" || proj.Aspect != "16:9" || proj.FPS != 30 ||
		!proj.CreatedAt.Equal(proj.UpdatedAt) || proj.CreatedAt.IsZero() {
		t.Fatalf("project: %+v", proj)
	}
	if len(story.Scenes) != 1 || story.Scenes[0].ID != "s1" || story.Scenes[0].Environment != "env.plain-stage" ||
		story.Style.Palette != "sunny" || !story.Style.Captions || story.Format.FPS != 30 || story.Voice != nil || story.Music != nil {
		t.Fatalf("story: %+v", story)
	}
	if issues := Validate(&story, loadMini(t), DefaultLimits()); len(issues) != 0 {
		t.Fatalf("a new story must be valid:%s", dump(issues))
	}
	base := testRoot + "/" + proj.ID
	if got, want := mem.Paths(), []string{base + "/project.json", base + "/story.json"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("files: %v", got)
	}
	dirs := strings.Join(mem.Dirs(), " ")
	for _, d := range []string{"assets", "audio/narration", "audio/music", "exports"} {
		if !strings.Contains(dirs, base+"/"+d) {
			t.Errorf("directory %s not created (have %s)", d, dirs)
		}
	}
	data, gotEtag, err := ps.LoadStory(ctx, proj.ID)
	if err != nil || gotEtag != etag || ETag(data) != etag {
		t.Fatalf("LoadStory: %v %q %q", err, gotEtag, etag)
	}
	again, _, err := DecodeStory(data, DefaultLimits())
	if err != nil || again.Title != story.Title {
		t.Fatalf("stored story: %v %+v", err, again)
	}
	if !strings.HasPrefix(etag, `"`) || len(etag) != 66 {
		t.Fatalf("etag must be the quoted sha256 of the notes: %s", etag)
	}

	// Format options and invalid arguments.
	p2, s2, _, err := ps.Create(ctx, "Hochformat", "9:16", 24)
	if err != nil || p2.Aspect != "9:16" || p2.FPS != 24 || s2.Format != (Format{Aspect: "9:16", FPS: 24}) {
		t.Fatalf("9:16: %v %+v", err, p2)
	}
	for _, bad := range []struct {
		title, aspect string
		fps           int
	}{{"", "", 0}, {"   ", "", 0}, {strings.Repeat("x", 121), "", 0}, {"T", "4:3", 0}, {"T", "", 60}} {
		if _, _, _, err := ps.Create(ctx, bad.title, bad.aspect, bad.fps); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("Create(%q,%q,%d) = %v", bad.title, bad.aspect, bad.fps, err)
		}
	}
}

func TestProjectGetListDelete(t *testing.T) {
	ps, mem, clock := newTestStore(t)
	ctx := context.Background()
	if got, err := ps.List(ctx); err != nil || got != nil {
		t.Fatalf("list of a missing root: %v %v", got, err)
	}
	a, _, _, _ := ps.Create(ctx, "A", "", 0)
	*clock = clock.Add(time.Hour)
	b, _, _, _ := ps.Create(ctx, "B", "", 0)
	// Junk next to the projects is skipped.
	_, _ = mem.WriteFile(ctx, testRoot+"/Library/library.json", []byte("{}"), "")
	_, _ = mem.WriteFile(ctx, testRoot+"/00000000-0000-4000-8000-999999999999/note.txt", []byte("x"), "")
	list, err := ps.List(ctx)
	if err != nil || len(list) != 2 || list[0].ID != b.ID || list[1].ID != a.ID {
		t.Fatalf("list (newest first): %v %+v", err, list)
	}
	got, err := ps.Get(ctx, a.ID)
	if err != nil || got.Title != "A" || !got.CreatedAt.Equal(a.CreatedAt) {
		t.Fatalf("get: %v %+v", err, got)
	}
	if _, err := ps.Get(ctx, "00000000-0000-4000-8000-000000000042"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing project: %v", err)
	}
	for _, id := range []string{"", "../x", "ABCDEF00-0000-4000-8000-000000000001", "00000000-0000-1000-8000-000000000001", "a/b"} {
		if _, err := ps.Get(ctx, id); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Get(%q) = %v", id, err)
		}
		if err := ps.Delete(ctx, id); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Delete(%q) = %v", id, err)
		}
	}
	if err := ps.Delete(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := ps.Delete(ctx, a.ID); err != nil {
		t.Fatalf("deleting a missing project must succeed: %v", err)
	}
	for _, p := range mem.Paths() {
		if strings.Contains(p, a.ID) {
			t.Errorf("left behind: %s", p)
		}
	}
	if list, _ := ps.List(ctx); len(list) != 1 || list[0].ID != b.ID {
		t.Fatalf("list after delete: %+v", list)
	}
	if _, _, err := ps.LoadStory(ctx, a.ID); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("LoadStory after delete: %v", err)
	}
}

func TestSaveStoryETagPreconditions(t *testing.T) {
	ps, _, clock := newTestStore(t)
	ctx := context.Background()
	proj, story, etag1, _ := ps.Create(ctx, "Erste Version", "", 0)

	story.Title = "Zweite Version"
	story.Format.FPS = 25
	data, _ := CanonicalJSON(story)
	*clock = clock.Add(time.Minute)
	etag2, err := ps.SaveStory(ctx, proj.ID, data, etag1)
	if err != nil || etag2 == etag1 || etag2 != ETag(data) {
		t.Fatalf("save with the current etag: %v %q", err, etag2)
	}
	// The project record follows the story: title, fps and updated_at.
	got, _ := ps.Get(ctx, proj.ID)
	if got.Title != "Zweite Version" || got.FPS != 25 || !got.UpdatedAt.Equal(*clock) || !got.CreatedAt.Equal(proj.CreatedAt) {
		t.Fatalf("project after save: %+v", got)
	}

	// A stale etag fails and changes nothing.
	story.Title = "Dritte Version"
	data3, _ := CanonicalJSON(story)
	if _, err := ps.SaveStory(ctx, proj.ID, data3, etag1); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale etag: %v", err)
	}
	if cur, etag, _ := ps.LoadStory(ctx, proj.ID); etag != etag2 || !strings.Contains(string(cur), "Zweite Version") {
		t.Fatal("a failed save changed the story")
	}
	if _, err := ps.SaveStory(ctx, proj.ID, data3, MustNotExist); !errors.Is(err, ErrConflict) {
		t.Fatalf("MustNotExist on an existing story: %v", err)
	}
	// An unconditional write is allowed.
	if _, err := ps.SaveStory(ctx, proj.ID, data3, ""); err != nil {
		t.Fatalf("unconditional save: %v", err)
	}

	// Size limit, unknown and invalid projects.
	if _, err := ps.SaveStory(ctx, proj.ID, make([]byte, DefaultLimits().MaxStoryBytes+1), ""); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversize story: %v", err)
	}
	if _, err := ps.SaveStory(ctx, "00000000-0000-4000-8000-000000000099", data3, ""); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unknown project: %v", err)
	}
	if _, err := ps.SaveStory(ctx, "nope", data3, ""); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("invalid id: %v", err)
	}
}

func TestSaveCodeSceneLifecycle(t *testing.T) {
	ps, mem, _ := newTestStore(t)
	ctx := context.Background()
	proj, _, _, _ := ps.Create(ctx, "Code", "", 0)
	good := readFixture(t, "testdata/scenes/s3.js")
	scenes := testRoot + "/" + proj.ID + "/scenes/"

	// Create-only, bundle written, no errors file.
	etag, res, err := ps.SaveCodeScene(ctx, proj.ID, "s3", good, MustNotExist)
	if err != nil || len(res.Errors) != 0 || res.Code == nil || etag != ETag(good) {
		t.Fatalf("first save: %v %+v %q", err, res.Errors, etag)
	}
	if got := mem.Paths(); !contains(got, scenes+"s3.js") || !contains(got, scenes+"s3.bundle.js") || contains(got, scenes+"s3.errors.json") {
		t.Fatalf("files after a good save: %v", got)
	}
	bundle, err := ps.LoadCodeBundle(ctx, proj.ID, "s3")
	if err != nil || string(bundle) != string(res.Code) {
		t.Fatalf("bundle: %v", err)
	}
	src, srcEtag, err := ps.LoadCodeSource(ctx, proj.ID, "s3")
	if err != nil || string(src) != string(good) || srcEtag != etag {
		t.Fatalf("source: %v", err)
	}
	status, err := ps.CodeSceneStatus(ctx, proj.ID)
	if err != nil || !reflect.DeepEqual(status, []CodeSceneStatus{{SceneID: "s3", OK: true, Errors: []BundleError{}}}) {
		t.Fatalf("status: %v %+v", err, status)
	}

	// Creating again with MustNotExist, or updating with a stale etag, conflicts and keeps the old state.
	if _, _, err := ps.SaveCodeScene(ctx, proj.ID, "s3", good, MustNotExist); !errors.Is(err, ErrConflict) {
		t.Fatalf("create over existing: %v", err)
	}
	bad := []byte("export default function scene(api) {\n  eval('1');\n}\n")
	if _, _, err := ps.SaveCodeScene(ctx, proj.ID, "s3", bad, `"stale"`); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale etag: %v", err)
	}
	if b, _ := ps.LoadCodeBundle(ctx, proj.ID, "s3"); string(b) != string(res.Code) {
		t.Fatal("a conflicting save changed the bundle")
	}

	// A failing update keeps the source, drops the bundle and records the errors.
	etag2, res2, err := ps.SaveCodeScene(ctx, proj.ID, "s3", bad, etag)
	if err != nil || res2.Code != nil || len(res2.Errors) != 1 || res2.Errors[0].Line != 2 || res2.Errors[0].Column != 3 || etag2 != ETag(bad) {
		t.Fatalf("failing save: %v %+v", err, res2)
	}
	if _, err := ps.LoadCodeBundle(ctx, proj.ID, "s3"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("bundle must be gone: %v", err)
	}
	if got := mem.Paths(); !contains(got, scenes+"s3.js") || contains(got, scenes+"s3.bundle.js") || !contains(got, scenes+"s3.errors.json") {
		t.Fatalf("files after a failing save: %v", got)
	}
	status, _ = ps.CodeSceneStatus(ctx, proj.ID)
	if len(status) != 1 || status[0].OK || !reflect.DeepEqual(status[0].Errors, res2.Errors) {
		t.Fatalf("status after failure: %+v", status)
	}
	var wire []map[string]any
	data, _ := json.Marshal(status)
	_ = json.Unmarshal(data, &wire)
	if _, ok := wire[0]["errors"].([]any)[0].(map[string]any)["column"]; !ok {
		t.Fatalf("status JSON: %s", data)
	}

	// Fixing it again restores the bundle and removes the errors file.
	if _, res3, err := ps.SaveCodeScene(ctx, proj.ID, "s3", good, etag2); err != nil || res3.Code == nil {
		t.Fatalf("fix: %v %+v", err, res3)
	}
	if got := mem.Paths(); !contains(got, scenes+"s3.bundle.js") || contains(got, scenes+"s3.errors.json") {
		t.Fatalf("files after the fix: %v", got)
	}

	// A second scene sorts after; sources over the limit and invalid ids are refused before writing.
	if _, _, err := ps.SaveCodeScene(ctx, proj.ID, "intro", good, ""); err != nil {
		t.Fatal(err)
	}
	if status, _ := ps.CodeSceneStatus(ctx, proj.ID); len(status) != 2 || status[0].SceneID != "intro" || status[1].SceneID != "s3" {
		t.Fatalf("two scenes: %+v", status)
	}
	big := []byte("export default function scene(api) {}\n" + strings.Repeat("// x\n", 20000))
	if _, _, err := ps.SaveCodeScene(ctx, proj.ID, "big", big, ""); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversize: %v", err)
	}
	if contains(mem.Paths(), scenes+"big.js") {
		t.Fatal("an oversize source was written")
	}
	for _, id := range []string{"", "../x", "A", "a b", strings.Repeat("a", 49)} {
		if _, _, err := ps.SaveCodeScene(ctx, proj.ID, id, good, ""); !errors.Is(err, ErrInvalidID) {
			t.Errorf("scene id %q: %v", id, err)
		}
	}
	if _, _, err := ps.SaveCodeScene(ctx, "00000000-0000-4000-8000-000000000099", "s1", good, ""); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unknown project: %v", err)
	}
}

func TestCodeSceneStatusWithoutScenes(t *testing.T) {
	ps, _, _ := newTestStore(t)
	proj, _, _, _ := ps.Create(context.Background(), "Leer", "", 0)
	status, err := ps.CodeSceneStatus(context.Background(), proj.ID)
	if err != nil || len(status) != 0 {
		t.Fatalf("%v %+v", err, status)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestMemStoreContract(t *testing.T) {
	ctx := context.Background()
	m := NewMemStore()
	if names, err := m.List(ctx, "a/b"); names != nil || err != nil {
		t.Fatalf("missing dir: %v %v", names, err)
	}
	if _, _, err := m.ReadFile(ctx, "a/x"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file: %v", err)
	}
	e1, err := m.WriteFile(ctx, "a/b/x.txt", []byte("one"), MustNotExist)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.WriteFile(ctx, "a/b/x.txt", []byte("two"), MustNotExist); !errors.Is(err, ErrConflict) {
		t.Fatalf("must-not-exist: %v", err)
	}
	if _, err := m.WriteFile(ctx, "a/b/new.txt", []byte("two"), e1); !errors.Is(err, ErrConflict) {
		t.Fatalf("etag of a missing file: %v", err)
	}
	if _, err := m.WriteFile(ctx, "a/b/x.txt", []byte("two"), e1); err != nil {
		t.Fatal(err)
	}
	_, _ = m.WriteFile(ctx, "a/c.txt", []byte("c"), "")
	_ = m.MkdirAll(ctx, "a/empty/deeper")
	names, _ := m.List(ctx, "a")
	if !reflect.DeepEqual(names, []string{"b", "c.txt", "empty"}) {
		t.Fatalf("list: %v", names)
	}
	if err := m.Remove(ctx, "a/b"); err != nil {
		t.Fatal(err)
	}
	if err := m.Remove(ctx, "a/b"); err != nil {
		t.Fatalf("remove twice: %v", err)
	}
	if names, _ := m.List(ctx, "a"); !reflect.DeepEqual(names, []string{"c.txt", "empty"}) {
		t.Fatalf("list after remove: %v", names)
	}
}

func TestCodeSceneIssues(t *testing.T) {
	ps, _, _ := newTestStore(t)
	ctx := context.Background()
	proj, _, _, _ := ps.Create(ctx, "Code", "", 0)
	s := storyOf(t, `{"id":"s1"},{"id":"missing","kind":"code"},{"id":"broken","kind":"code"},{"id":"fine","kind":"code"}`)
	good := readFixture(t, "testdata/scenes/s3.js")
	_, _, _ = ps.SaveCodeScene(ctx, proj.ID, "fine", good, "")
	_, _, _ = ps.SaveCodeScene(ctx, proj.ID, "broken", []byte("export default function scene(api) {\n  fetch('x');\n}\n"), "")
	_, _, _ = ps.SaveCodeScene(ctx, proj.ID, "unused", good, "") // not in the story: ignored
	status, err := ps.CodeSceneStatus(ctx, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	issues := CodeSceneIssues(&s, status)
	if len(issues) != 2 {
		t.Fatalf("issues: %s", dump(issues))
	}
	m := expectIssue(t, issues, CodeCodeError, SeverityError, "/scenes/1/kind")
	b := expectIssue(t, issues, CodeCodeError, SeverityError, "/scenes/2/kind")
	if m.SceneID != "missing" || !strings.Contains(m.Message, "no source") || !strings.Contains(m.Fix, "scenes/missing.js") {
		t.Errorf("missing: %+v", m)
	}
	if b.SceneID != "broken" || !strings.Contains(b.Message, "line 2, column 3") || !strings.Contains(b.Message, "fetch") {
		t.Errorf("broken: %+v", b)
	}
}

func TestSaveCodeSceneConcurrentSavesStayConsistent(t *testing.T) {
	ps, _, _ := newTestStore(t)
	ctx := context.Background()
	proj, _, _, _ := ps.Create(ctx, "Code", "", 0)
	a := []byte("export default function scene(api) { api.duration(1); }\n")
	b := []byte("export default function scene(api) { api.duration(2); }\n")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		src := a
		if i%2 == 1 {
			src = b
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := ps.SaveCodeScene(ctx, proj.ID, "s1", src, ""); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	src, _, _ := ps.LoadCodeSource(ctx, proj.ID, "s1")
	bundle, err := ps.LoadCodeBundle(ctx, proj.ID, "s1")
	if err != nil || !strings.Contains(string(bundle), strings.TrimSpace(strings.TrimSuffix(strings.SplitN(string(src), "{", 2)[1], "}\n"))) {
		t.Fatalf("bundle does not belong to the last source: %v\nsource: %s\nbundle: %s", err, src, bundle)
	}
}

// faultStore fails WriteFile or Remove for paths that end in a chosen suffix.
type faultStore struct {
	*MemStore
	failWrite  string
	failRemove string
}

func (f *faultStore) WriteFile(ctx context.Context, p string, data []byte, ifMatch string) (string, error) {
	if f.failWrite != "" && strings.HasSuffix(p, f.failWrite) {
		return "", errors.New("injected write failure")
	}
	return f.MemStore.WriteFile(ctx, p, data, ifMatch)
}

func (f *faultStore) Remove(ctx context.Context, p string) error {
	if f.failRemove != "" && strings.HasSuffix(p, f.failRemove) {
		return errors.New("injected remove failure")
	}
	return f.MemStore.Remove(ctx, p)
}

// A save that fails half way must never leave a bundle that was built from older source reported
// as ok: render loads the bundle of every scene whose status is ok.
func TestSaveCodeSceneFailureNeverLeavesAStaleBundle(t *testing.T) {
	v1 := []byte("export default function scene(api) { api.duration(1); }\n")
	v2 := []byte("export default function scene(api) { api.duration(2); }\n")
	bad := []byte("export default function scene(api) {\n  eval('1');\n}\n")
	cases := []struct {
		name       string
		next       []byte
		failWrite  string
		failRemove string
		mustFail   bool
	}{
		{"bundle write fails", v2, "/s1.bundle.js", "", true},
		{"old bundle cannot be removed before a failing build", bad, "", "/s1.bundle.js", true},
		{"old bundle cannot be removed before a good build", v2, "", "/s1.bundle.js", false},
		{"source write fails", v2, "/scenes/s1.js", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			faults := &faultStore{MemStore: NewMemStore()}
			ps := NewProjectStore(faults, testRoot)
			ctx := context.Background()
			proj, _, _, err := ps.Create(ctx, "Code", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			etag, _, err := ps.SaveCodeScene(ctx, proj.ID, "s1", v1, MustNotExist)
			if err != nil {
				t.Fatal(err)
			}
			faults.failWrite, faults.failRemove = tc.failWrite, tc.failRemove
			if _, _, err := ps.SaveCodeScene(ctx, proj.ID, "s1", tc.next, etag); err == nil && tc.mustFail {
				t.Fatal("the injected failure must be reported")
			}
			faults.failWrite, faults.failRemove = "", ""
			status, err := ps.CodeSceneStatus(ctx, proj.ID)
			if err != nil || len(status) != 1 {
				t.Fatalf("status: %v %+v", err, status)
			}
			if !status[0].OK {
				return // not bundled: render is blocked, which is the safe side
			}
			src, _, err := ps.LoadCodeSource(ctx, proj.ID, "s1")
			if err != nil {
				t.Fatal(err)
			}
			bundle, err := ps.LoadCodeBundle(ctx, proj.ID, "s1")
			if err != nil {
				t.Fatalf("status ok but no bundle: %v", err)
			}
			if want := BundleCodeScene(src, "s1", DefaultLimits().MaxCodeBytes).Code; string(bundle) != string(want) {
				t.Fatalf("status is ok but the bundle was not built from the current source\nsource: %s\nbundle: %s", src, bundle)
			}
		})
	}
}

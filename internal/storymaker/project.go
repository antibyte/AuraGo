package storymaker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/desktop"
	"github.com/google/uuid"
)

// Store errors.
var (
	// ErrConflict reports a failed precondition: a stale ETag, or MustNotExist on an existing file.
	ErrConflict = errors.New("storymaker: precondition failed")
	// ErrInvalidID reports a project or scene id that is not in the accepted form.
	ErrInvalidID = errors.New("storymaker: invalid project or scene id")
	// ErrInvalidArgument reports an unusable title, aspect or frame rate.
	ErrInvalidArgument = errors.New("storymaker: invalid argument")
	// ErrTooLarge reports story or code content over its size limit.
	ErrTooLarge = errors.New("storymaker: content too large")
)

// MustNotExist is the ifMatch value of Store.WriteFile that creates a file only when it is new.
const MustNotExist = "-"

// Store is the file access ProjectStore needs. The server implements it over the desktop
// workspace; NewMemStore is the in-memory implementation for tests. Paths are rooted desktop
// paths such as "Documents/Storymaker/<id>/story.json".
//
// Semantics: an etag is ETag(bytes). ReadFile of a missing file wraps fs.ErrNotExist.
// WriteFile creates missing parent directories and takes an ifMatch of "" (unconditional),
// MustNotExist (create only) or an etag (the file must exist and still have it); a failed
// precondition returns ErrConflict. Remove behaves like os.RemoveAll: a missing path is not an
// error and a directory is removed with everything below it. List returns the base names of
// the direct children of dir (files and directories); a missing directory is (nil, nil). Open
// reads a file of any size; a missing file wraps fs.ErrNotExist.
type Store interface {
	ReadFile(ctx context.Context, path string) (data []byte, etag string, err error)
	WriteFile(ctx context.Context, path string, data []byte, ifMatch string) (etag string, err error)
	Remove(ctx context.Context, path string) error
	List(ctx context.Context, dir string) ([]string, error)
	MkdirAll(ctx context.Context, dir string) error
	Open(ctx context.Context, path string) (io.ReadSeekCloser, FileInfo, error)
}

// FileInfo describes a file opened with Store.Open.
type FileInfo struct {
	Name    string
	Size    int64
	ModTime time.Time
	MIME    string
}

// ETag is the strong version of a file: the quoted SHA-256 of its bytes, the same format the
// desktop notes use.
func ETag(data []byte) string { return desktop.NoteVersion(data) }

// Project is the project record in project.json and in the API.
type Project struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Aspect        string    `json:"aspect"`
	FPS           int       `json:"fps"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	TargetSeconds int       `json:"target_seconds,omitempty"`
	Audience      string    `json:"audience,omitempty"`
	Template      string    `json:"template,omitempty"`
}

// CodeSceneStatus is the bundle state of one code scene.
type CodeSceneStatus struct {
	SceneID string        `json:"scene_id"`
	OK      bool          `json:"ok"`
	Errors  []BundleError `json:"errors"`
}

// ProjectStore keeps projects in <root>/<id>/ on top of a Store:
//
//	story.json            the story document (ETag-protected writes)
//	project.json          the project record
//	scenes/<id>.js        code-scene source
//	scenes/<id>.bundle.js the bundled classic script, written on a successful save
//	scenes/<id>.errors.json the bundle errors of the last failed save
//	assets/  audio/narration/  audio/music/  exports/   created empty by Create
type ProjectStore struct {
	st    Store
	root  string
	lim   Limits
	now   func() time.Time
	newID func() string
	// codeMu serialises SaveCodeScene, so two saves cannot leave a bundle that belongs to
	// the other save's source.
	codeMu sync.Mutex
}

// NewProjectStore returns a store rooted at root, for example "Documents/Storymaker".
func NewProjectStore(st Store, root string) *ProjectStore {
	root = strings.Trim(path.Clean(strings.ReplaceAll(root, "\\", "/")), "/")
	return &ProjectStore{
		st: st, root: root, lim: DefaultLimits(),
		now:   func() time.Time { return time.Now().UTC().Truncate(time.Second) },
		newID: func() string { return uuid.NewString() },
	}
}

func (p *ProjectStore) dir(id string) string { return path.Join(p.root, id) }

func (p *ProjectStore) file(id string, parts ...string) string {
	return path.Join(append([]string{p.root, id}, parts...)...)
}

func (p *ProjectStore) checkID(id string) error {
	if !ValidProjectID(id) {
		return fmt.Errorf("%w: project %q", ErrInvalidID, clip(id, 40))
	}
	return nil
}

func (p *ProjectStore) checkScene(id, sceneID string) error {
	if err := p.checkID(id); err != nil {
		return err
	}
	if !validSlug(sceneID) {
		return fmt.Errorf("%w: scene %q", ErrInvalidID, clip(sceneID, 40))
	}
	return nil
}

// Create makes a project. An empty aspect means 16:9 and fps 0 means 30. The new story has the
// title, the format, the default style and one empty scene s1 on env.plain-stage, so the editor
// opens on something that renders; defaults are applied before the first write. It returns the
// project, the story and the etag of story.json.
func (p *ProjectStore) Create(ctx context.Context, title, aspect string, fps int) (Project, Story, string, error) {
	return p.CreateWithVoice(ctx, title, aspect, fps, nil)
}

// cleanup removes a half-created project; a failure to clean up is not reported.
func (p *ProjectStore) cleanup(id string) {
	_ = p.st.Remove(context.Background(), p.dir(id))
}

func encodeProject(pr Project) ([]byte, error) {
	b, err := json.MarshalIndent(pr, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("storymaker: encode project: %w", err)
	}
	return append(b, '\n'), nil
}

// List returns the projects, most recently updated first. Folders that are not valid project
// ids or whose project.json cannot be read are skipped.
func (p *ProjectStore) List(ctx context.Context) ([]Project, error) {
	names, err := p.st.List(ctx, p.root)
	if err != nil {
		return nil, fmt.Errorf("storymaker: list projects: %w", err)
	}
	var out []Project
	for _, name := range names {
		id := path.Base(name)
		if !ValidProjectID(id) {
			continue
		}
		pr, err := p.Get(ctx, id)
		if err != nil {
			continue
		}
		out = append(out, pr)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// Get reads project.json. A missing project wraps fs.ErrNotExist.
func (p *ProjectStore) Get(ctx context.Context, id string) (Project, error) {
	pr, _, err := p.readProject(ctx, id)
	return pr, err
}

func (p *ProjectStore) readProject(ctx context.Context, id string) (Project, string, error) {
	if err := p.checkID(id); err != nil {
		return Project{}, "", err
	}
	data, etag, err := p.st.ReadFile(ctx, p.file(id, "project.json"))
	if err != nil {
		return Project{}, "", err
	}
	var pr Project
	if err := json.Unmarshal(data, &pr); err != nil {
		return Project{}, "", fmt.Errorf("storymaker: project %s: %w", id, err)
	}
	pr.ID = id
	return pr, etag, nil
}

// LoadStory returns the bytes of story.json and their etag.
func (p *ProjectStore) LoadStory(ctx context.Context, id string) ([]byte, string, error) {
	if err := p.checkID(id); err != nil {
		return nil, "", err
	}
	return p.st.ReadFile(ctx, p.file(id, "story.json"))
}

// SaveStory writes story.json (data is stored as given: the caller decodes, validates and
// canonicalises first) and returns the new etag. ifMatch is "" for an unconditional write or
// the etag the caller read; a stale etag returns ErrConflict. A successful write updates
// project.json: updated_at, and the title, aspect and fps read from the story.
func (p *ProjectStore) SaveStory(ctx context.Context, id string, data []byte, ifMatch string) (string, error) {
	if err := p.checkID(id); err != nil {
		return "", err
	}
	if p.lim.MaxStoryBytes > 0 && len(data) > p.lim.MaxStoryBytes {
		return "", fmt.Errorf("%w: story.json is %d bytes, the limit is %d", ErrTooLarge, len(data), p.lim.MaxStoryBytes)
	}
	if _, _, err := p.st.ReadFile(ctx, p.file(id, "project.json")); err != nil {
		return "", err
	}
	etag, err := p.st.WriteFile(ctx, p.file(id, "story.json"), data, ifMatch)
	if err != nil {
		return "", err
	}
	p.touch(ctx, id, data)
	return etag, nil
}

// touch bumps updated_at and mirrors the story's title and format into project.json. It is
// best effort: the story is already saved, so a failure here is not reported.
func (p *ProjectStore) touch(ctx context.Context, id string, story []byte) {
	var head struct {
		Title  string `json:"title"`
		Format struct {
			Aspect string `json:"aspect"`
			FPS    int    `json:"fps"`
		} `json:"format"`
	}
	_ = json.Unmarshal(story, &head)
	for attempt := 0; attempt < 3; attempt++ {
		pr, etag, err := p.readProject(ctx, id)
		if err != nil {
			return
		}
		pr.UpdatedAt = p.now()
		if t := strings.TrimSpace(head.Title); t != "" && runeLen(t) <= 120 {
			pr.Title = t
		}
		if head.Format.Aspect != "" {
			pr.Aspect = head.Format.Aspect
		}
		if head.Format.FPS != 0 {
			pr.FPS = head.Format.FPS
		}
		b, err := encodeProject(pr)
		if err != nil {
			return
		}
		if _, err := p.st.WriteFile(ctx, p.file(id, "project.json"), b, etag); !errors.Is(err, ErrConflict) {
			return
		}
	}
}

// SetAgentMeta records the agent's length target, audience and template in project.json. These
// three live in the project record, not in story.json. It retries a write that lost a race.
func (p *ProjectStore) SetAgentMeta(ctx context.Context, id string, targetSeconds int, audience, template string) error {
	if targetSeconds < 0 {
		return fmt.Errorf("%w: the target length must not be negative", ErrInvalidArgument)
	}
	for attempt := 0; attempt < 3; attempt++ {
		pr, etag, err := p.readProject(ctx, id)
		if err != nil {
			return err
		}
		pr.TargetSeconds, pr.Audience, pr.Template = targetSeconds, audience, template
		b, err := encodeProject(pr)
		if err != nil {
			return err
		}
		_, err = p.st.WriteFile(ctx, p.file(id, "project.json"), b, etag)
		if !errors.Is(err, ErrConflict) {
			return err
		}
	}
	return ErrConflict
}

// SaveCodeScene writes the source of a code scene and bundles it. ifMatch follows
// Store.WriteFile: "" unconditional, MustNotExist for a new scene, or the source etag.
// On success it writes scenes/<id>.bundle.js and removes scenes/<id>.errors.json. When bundling
// fails it keeps the source, removes the bundle and writes scenes/<id>.errors.json; the error
// list is in the result and the call still succeeds, so the editor can show it. Source over the
// code size limit is refused with ErrTooLarge before anything is written.
//
// The previous bundle is removed before the source changes: a render loads the bundle of every
// scene whose status is ok, so a save that fails half way must leave the old source with its own
// bundle (the removal failed) or the new source without a bundle (a later step failed), never a
// bundle that was built from older source. A refused precondition is checked first, so it keeps
// the old bundle.
func (p *ProjectStore) SaveCodeScene(ctx context.Context, id, sceneID string, source []byte, ifMatch string) (string, BundleResult, error) {
	if err := p.checkScene(id, sceneID); err != nil {
		return "", BundleResult{}, err
	}
	p.codeMu.Lock()
	defer p.codeMu.Unlock()
	if p.lim.MaxCodeBytes > 0 && len(source) > p.lim.MaxCodeBytes {
		return "", BundleResult{}, fmt.Errorf("%w: the code scene is %d bytes, the limit is %d", ErrTooLarge, len(source), p.lim.MaxCodeBytes)
	}
	if _, _, err := p.st.ReadFile(ctx, p.file(id, "project.json")); err != nil {
		return "", BundleResult{}, err
	}
	if err := p.st.MkdirAll(ctx, p.file(id, "scenes")); err != nil {
		return "", BundleResult{}, fmt.Errorf("storymaker: create scenes folder: %w", err)
	}
	if err := p.checkCodePrecondition(ctx, id, sceneID, ifMatch); err != nil {
		return "", BundleResult{}, err
	}
	if err := p.st.Remove(ctx, p.codePath(id, sceneID, ".bundle.js")); err != nil {
		return "", BundleResult{}, fmt.Errorf("storymaker: remove bundle: %w", err)
	}
	etag, err := p.st.WriteFile(ctx, p.codePath(id, sceneID, ".js"), source, ifMatch)
	if err != nil {
		return "", BundleResult{}, err
	}
	res := BundleCodeScene(source, sceneID, p.lim.MaxCodeBytes)
	if len(res.Errors) == 0 {
		if _, err := p.st.WriteFile(ctx, p.codePath(id, sceneID, ".bundle.js"), res.Code, ""); err != nil {
			return etag, res, fmt.Errorf("storymaker: write bundle: %w", err)
		}
		if err := p.st.Remove(ctx, p.codePath(id, sceneID, ".errors.json")); err != nil {
			return etag, res, fmt.Errorf("storymaker: remove errors: %w", err)
		}
		return etag, res, nil
	}
	b, err := json.MarshalIndent(res.Errors, "", "  ")
	if err == nil {
		_, err = p.st.WriteFile(ctx, p.codePath(id, sceneID, ".errors.json"), append(b, '\n'), "")
	}
	if err != nil {
		return etag, res, fmt.Errorf("storymaker: write errors: %w", err)
	}
	return etag, res, nil
}

// checkCodePrecondition applies the ifMatch rule of Store.WriteFile to the source file, so that a
// refused save is refused before the old bundle is touched. The write repeats the check under the
// store's own lock.
func (p *ProjectStore) checkCodePrecondition(ctx context.Context, id, sceneID, ifMatch string) error {
	if ifMatch == "" {
		return nil
	}
	_, cur, err := p.st.ReadFile(ctx, p.codePath(id, sceneID, ".js"))
	exists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("storymaker: read code scene: %w", err)
	}
	if ifMatch == MustNotExist {
		if exists {
			return ErrConflict
		}
		return nil
	}
	if !exists || cur != ifMatch {
		return ErrConflict
	}
	return nil
}

func (p *ProjectStore) codePath(id, sceneID, ext string) string {
	return p.file(id, "scenes", sceneID+ext)
}

// LoadCodeSource returns the source of a code scene and its etag.
func (p *ProjectStore) LoadCodeSource(ctx context.Context, id, sceneID string) ([]byte, string, error) {
	if err := p.checkScene(id, sceneID); err != nil {
		return nil, "", err
	}
	return p.st.ReadFile(ctx, p.codePath(id, sceneID, ".js"))
}

// LoadCodeBundle returns the bundled script of a code scene; fs.ErrNotExist when the last save
// failed or the scene was never saved.
func (p *ProjectStore) LoadCodeBundle(ctx context.Context, id, sceneID string) ([]byte, error) {
	if err := p.checkScene(id, sceneID); err != nil {
		return nil, err
	}
	data, _, err := p.st.ReadFile(ctx, p.codePath(id, sceneID, ".bundle.js"))
	return data, err
}

// CodeSceneStatus lists every code scene that has a source file, sorted by scene id, with the
// result of its last save.
func (p *ProjectStore) CodeSceneStatus(ctx context.Context, id string) ([]CodeSceneStatus, error) {
	if err := p.checkID(id); err != nil {
		return nil, err
	}
	names, err := p.st.List(ctx, p.file(id, "scenes"))
	if err != nil {
		return nil, fmt.Errorf("storymaker: list scenes: %w", err)
	}
	have := map[string]bool{}
	for _, n := range names {
		have[path.Base(n)] = true
	}
	var out []CodeSceneStatus
	for name := range have {
		if !strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".bundle.js") {
			continue
		}
		sceneID := strings.TrimSuffix(name, ".js")
		if !validSlug(sceneID) {
			continue
		}
		st := CodeSceneStatus{SceneID: sceneID, Errors: []BundleError{}}
		switch {
		case have[sceneID+".bundle.js"]:
			st.OK = true
		case have[sceneID+".errors.json"]:
			if data, _, err := p.st.ReadFile(ctx, p.codePath(id, sceneID, ".errors.json")); err == nil {
				_ = json.Unmarshal(data, &st.Errors)
			}
		}
		if !st.OK && len(st.Errors) == 0 {
			st.Errors = []BundleError{{Line: 1, Column: 1, Message: "The scene has not been bundled yet; save it again."}}
		}
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SceneID < out[j].SceneID })
	return out, nil
}

// CodeSceneIssues reports, for every scene of kind "code", a code_error issue when the scene has
// no source file or its last bundle failed (first error with line and column). Sources of scenes
// that are not in the story are ignored. A render needs this list to be empty.
func CodeSceneIssues(s *Story, status []CodeSceneStatus) []Issue {
	byID := make(map[string]CodeSceneStatus, len(status))
	for _, st := range status {
		byID[st.SceneID] = st
	}
	var out []Issue
	for i, sc := range s.Scenes {
		if sc.Kind != "code" {
			continue
		}
		st, ok := byID[sc.ID]
		switch {
		case !ok:
			out = append(out, Issue{Severity: SeverityError, SceneID: sc.ID, Path: jp("scenes", i, "kind"), Code: CodeCodeError,
				Message: "The code scene has no source yet.",
				Fix:     "Write scenes/" + sc.ID + ".js with export default function scene(api) { ... }, or set kind to \"storyboard\"."})
		case !st.OK:
			msg := "The code scene did not bundle."
			if len(st.Errors) > 0 {
				e := st.Errors[0]
				msg = fmt.Sprintf("The code scene did not bundle (line %d, column %d): %s", e.Line, e.Column, e.Message)
			}
			out = append(out, Issue{Severity: SeverityError, SceneID: sc.ID, Path: jp("scenes", i, "kind"), Code: CodeCodeError,
				Message: msg, Fix: "Fix the reported line in the code scene and save it again."})
		}
	}
	return out
}

// Delete removes a project folder with everything in it. A missing project is not an error.
func (p *ProjectStore) Delete(ctx context.Context, id string) error {
	if err := p.checkID(id); err != nil {
		return err
	}
	return p.st.Remove(ctx, p.dir(id))
}

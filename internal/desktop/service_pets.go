package desktop

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"aurago/internal/webassets"
)

const petsDirName = "Pets"

var petIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// PetJSON is the on-disk metadata format compatible with OpenPets.
type PetJSON struct {
	ID              string `json:"id"`
	DisplayName     string `json:"displayName"`
	Description     string `json:"description,omitempty"`
	SpritesheetPath string `json:"spritesheetPath"`
	Category        string `json:"category,omitempty"`
	Subcategory     string `json:"subcategory,omitempty"`
}

type bundledPet struct {
	Manifest    PetJSON
	Spritesheet []byte
}

// ListPets returns all pets discovered in the workspace Pets directory.
func (s *Service) ListPets(ctx context.Context) ([]PetManifest, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	cfg := s.Config()
	return s.listPetsWithDefaultRepair(cfg.WorkspaceDir)
}

// GetPet returns a single pet by ID.
func (s *Service) GetPet(ctx context.Context, id string) (PetManifest, error) {
	if !petIDPattern.MatchString(id) {
		return PetManifest{}, fmt.Errorf("invalid pet id %q", id)
	}
	if err := s.ensureReady(ctx); err != nil {
		return PetManifest{}, err
	}
	cfg := s.Config()
	return getPetInDir(cfg.WorkspaceDir, id)
}

// SetActivePet stores the active pet ID in desktop settings.
func (s *Service) SetActivePet(ctx context.Context, id string) error {
	if id != "" && !petIDPattern.MatchString(id) {
		return fmt.Errorf("invalid pet id %q", id)
	}
	return s.SetSetting(ctx, "pet.active_id", id, SourceAgent)
}

// GetActivePetID reads the active pet ID from desktop settings.
func (s *Service) GetActivePetID(ctx context.Context) (string, error) {
	settings, err := s.listSettings(ctx)
	if err != nil {
		return "", err
	}
	return settings["pet.active_id"], nil
}

// InstallPet writes a pet package into the workspace Pets directory.
// files maps relative paths (e.g. "pet.json", "spritesheet.webp") to content.
func (s *Service) InstallPet(ctx context.Context, id string, files map[string][]byte) error {
	if !petIDPattern.MatchString(id) {
		return fmt.Errorf("invalid pet id %q", id)
	}
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	cfg := s.Config()
	if cfg.ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}

	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()

	petDir := filepath.Join(cfg.WorkspaceDir, petsDirName, id)
	if err := os.MkdirAll(petDir, 0o700); err != nil {
		return fmt.Errorf("create pet directory: %w", err)
	}

	for relPath, data := range files {
		cleanRel := filepath.ToSlash(filepath.Clean(relPath))
		if cleanRel == "." || strings.Contains(cleanRel, "..") || filepath.IsAbs(cleanRel) {
			return fmt.Errorf("invalid pet file path %q", relPath)
		}
		target := filepath.Join(petDir, cleanRel)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return fmt.Errorf("create pet file directory: %w", err)
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			return fmt.Errorf("write pet file %s: %w", cleanRel, err)
		}
	}

	// Validate the installed package.
	if _, err := getPetInDir(cfg.WorkspaceDir, id); err != nil {
		return fmt.Errorf("installed pet is invalid: %w", err)
	}

	s.invalidateBootstrapCache()
	_ = s.Audit(ctx, "install_pet", id, nil, SourceAgent)
	return nil
}

// InstallPetFromZip extracts a pet ZIP into the workspace.
func (s *Service) InstallPetFromZip(ctx context.Context, id string, r io.Reader, size int64) error {
	if !petIDPattern.MatchString(id) {
		return fmt.Errorf("invalid pet id %q", id)
	}
	files, err := extractPetZip(r, size)
	if err != nil {
		return err
	}
	return s.InstallPet(ctx, id, files)
}

// DeletePet removes a pet from the workspace.
func (s *Service) DeletePet(ctx context.Context, id string) error {
	if !petIDPattern.MatchString(id) {
		return fmt.Errorf("invalid pet id %q", id)
	}
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	cfg := s.Config()
	if cfg.ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}

	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()

	petDir := filepath.Join(cfg.WorkspaceDir, petsDirName, id)
	if _, err := os.Stat(petDir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("pet %q not found", id)
		}
		return fmt.Errorf("stat pet directory: %w", err)
	}
	if err := os.RemoveAll(petDir); err != nil {
		return fmt.Errorf("remove pet directory: %w", err)
	}

	s.invalidateBootstrapCache()
	_ = s.Audit(ctx, "delete_pet", id, nil, SourceAgent)

	// Clear active pet if it was this one (outside the mutation lock to avoid deadlock).
	if active, _ := s.GetActivePetID(ctx); active == id {
		_ = s.SetActivePet(ctx, "")
	}
	return nil
}

// PetSpritesheetPath returns the absolute filesystem path to a pet's spritesheet.
func (s *Service) PetSpritesheetPath(id string) (string, error) {
	if !petIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid pet id %q", id)
	}
	cfg := s.Config()
	petDir := filepath.Join(cfg.WorkspaceDir, petsDirName, id)
	data, err := os.ReadFile(filepath.Join(petDir, "pet.json"))
	if err != nil {
		return "", fmt.Errorf("read pet.json: %w", err)
	}
	var pet PetJSON
	if err := json.Unmarshal(data, &pet); err != nil {
		return "", fmt.Errorf("parse pet.json: %w", err)
	}
	spritesheet := strings.TrimSpace(pet.SpritesheetPath)
	if spritesheet == "" {
		spritesheet = "spritesheet.webp"
	}
	cleanSpritesheet := filepath.ToSlash(filepath.Clean(spritesheet))
	if strings.Contains(cleanSpritesheet, "..") {
		return "", fmt.Errorf("invalid spritesheet path")
	}
	return filepath.Join(petDir, cleanSpritesheet), nil
}

func listPetsInDir(workspaceDir string) ([]PetManifest, error) {
	petsDir := filepath.Join(workspaceDir, petsDirName)
	entries, err := os.ReadDir(petsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		slog.Warn("desktop: pets directory is not readable", "error", err)
		return nil, errors.New("pets directory is not readable")
	}
	var pets []PetManifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pet, err := getPetInDir(workspaceDir, entry.Name())
		if err != nil {
			continue
		}
		pets = append(pets, pet)
	}
	sort.Slice(pets, func(i, j int) bool {
		return strings.ToLower(pets[i].DisplayName) < strings.ToLower(pets[j].DisplayName)
	})
	return pets, nil
}

func (s *Service) listPetsWithDefaultRepair(workspaceDir string) ([]PetManifest, error) {
	// The check is cheap when each default already loads (getPetInDir only).
	// Only take the mutation lock when a bundled pet is genuinely missing;
	// a refused entry (link, special file, invalid manifest) is never repaired.
	needRepair := false
	for _, pet := range bundledDefaultPets() {
		if bundledPetNeedsRepair(workspaceDir, pet.Manifest.ID, false) {
			needRepair = true
			break
		}
	}
	if needRepair {
		desktopMutationMu.Lock()
		err := ensureBundledDefaultPets(workspaceDir)
		desktopMutationMu.Unlock()
		if err != nil {
			return nil, err
		}
	}
	return listPetsInDir(workspaceDir)
}

// cleanPetRelPath cleans a manifest-supplied file path inside a pet directory.
// Pet packages are portable, so on every OS the path must be relative and
// slash-separated: absolute paths, ".." escapes, backslashes, colons (drive
// letters, NTFS streams) and names filepath.Localize refuses are rejected.
func cleanPetRelPath(raw string) (string, bool) {
	if strings.ContainsAny(raw, `:\`) {
		return "", false
	}
	clean := path.Clean(raw)
	if clean == "." || !fs.ValidPath(clean) {
		return "", false
	}
	if _, err := filepath.Localize(clean); err != nil {
		return "", false
	}
	return clean, true
}

// petRegularFile reports whether the slash-separated rel names a regular file
// inside the pet directory id under root (the Pets directory). Every
// component, the pet directory included, is checked with Lstat: intermediate
// ones must be real directories (symlinks and Windows junctions are not), the
// last one a regular file, so links never redirect a pet lookup.
func petRegularFile(root *os.Root, id, rel string) bool {
	parts := strings.Split(id+"/"+rel, "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return false
		}
		if last := i == len(parts)-1; (!last && !info.IsDir()) || (last && !info.Mode().IsRegular()) {
			return false
		}
	}
	return true
}

// getPetInDir loads Pets/<id>/pet.json and checks its spritesheet. Errors
// name the pet id only, never a resolved host path, because they reach HTTP
// and agent tool responses.
func getPetInDir(workspaceDir, id string) (PetManifest, error) {
	if !petIDPattern.MatchString(id) {
		return PetManifest{}, fmt.Errorf("invalid pet id %q", id)
	}
	root, err := os.OpenRoot(filepath.Join(workspaceDir, petsDirName))
	if err != nil {
		return PetManifest{}, fmt.Errorf("pet %q not found", id)
	}
	defer root.Close()
	if !petRegularFile(root, id, "pet.json") {
		return PetManifest{}, fmt.Errorf("pet %q not found", id)
	}
	data, err := root.ReadFile(id + "/pet.json")
	if err != nil {
		return PetManifest{}, fmt.Errorf("pet %q is not readable", id)
	}
	var pet PetJSON
	if err := json.Unmarshal(data, &pet); err != nil {
		return PetManifest{}, fmt.Errorf("parse pet %q: %w", id, err)
	}
	spritesheet := strings.TrimSpace(pet.SpritesheetPath)
	if spritesheet == "" {
		spritesheet = "spritesheet.webp"
	}
	spritesheet, ok := cleanPetRelPath(spritesheet)
	if !ok {
		return PetManifest{}, fmt.Errorf("pet %q has an invalid spritesheet path", id)
	}
	if !petRegularFile(root, id, spritesheet) {
		return PetManifest{}, fmt.Errorf("pet %q spritesheet missing", id)
	}
	displayName := strings.TrimSpace(pet.DisplayName)
	if displayName == "" {
		displayName = id
	}
	return PetManifest{
		ID:          id,
		DisplayName: displayName,
		Description: strings.TrimSpace(pet.Description),
		Category:    strings.TrimSpace(pet.Category),
		Subcategory: strings.TrimSpace(pet.Subcategory),
		Spritesheet: spritesheet,
	}, nil
}

func extractPetZip(r io.Reader, size int64) (map[string][]byte, error) {
	const maxSize = 50 * 1024 * 1024
	const maxFiles = 100
	if size > maxSize {
		return nil, fmt.Errorf("pet zip too large")
	}
	data, err := io.ReadAll(io.LimitReader(r, maxSize+1))
	if err != nil {
		return nil, fmt.Errorf("read pet zip: %w", err)
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("pet zip too large")
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open pet zip: %w", err)
	}
	files := make(map[string][]byte)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.ToSlash(filepath.Clean(f.Name))
		if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
			continue
		}
		if len(files) >= maxFiles {
			return nil, fmt.Errorf("pet zip contains too many files")
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open pet zip entry %s: %w", name, err)
		}
		content, err := io.ReadAll(io.LimitReader(rc, maxSize+1))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read pet zip entry %s: %w", name, err)
		}
		if int64(len(content)) > maxSize {
			return nil, fmt.Errorf("pet zip entry %s too large", name)
		}
		files[name] = content
	}
	if _, ok := files["pet.json"]; !ok {
		return nil, fmt.Errorf("pet zip missing pet.json")
	}
	return files, nil
}

// ParsePetScale parses the pet scale setting into a float.
func ParsePetScale(raw string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || v < 0.25 || v > 3.0 {
		return 1.0
	}
	return v
}

// PetScaleString formats a scale value for storage.
func PetScaleString(scale float64) string {
	if scale < 0.25 {
		scale = 0.25
	}
	if scale > 3.0 {
		scale = 3.0
	}
	return strconv.FormatFloat(scale, 'f', 2, 64)
}

func bundledDefaultPets() []bundledPet {
	return []bundledPet{
		{
			Manifest: PetJSON{
				ID:              "openpets-default",
				DisplayName:     "OpenPets Default",
				Description:     "The built-in OpenPets companion (MIT licensed).",
				SpritesheetPath: "spritesheet.webp",
				Category:        "mascot",
			},
		},
		{
			Manifest: PetJSON{
				ID:              "snoopy",
				DisplayName:     "Snoopy",
				Description:     "A tiny black-and-white beagle with a red collar for calm coding sessions.",
				SpritesheetPath: "spritesheet.webp",
				Category:        "mascot",
			},
		},
		{
			Manifest: PetJSON{
				ID:              "clippit",
				DisplayName:     "Clippy",
				Description:     "A classic paperclip assistant rebuilt from Microsoft Agent animation frames.",
				SpritesheetPath: "spritesheet.webp",
				Category:        "mascot",
			},
		},
		{
			Manifest: PetJSON{
				ID:              "tux",
				DisplayName:     "Tux",
				Description:     "A tiny pixel-adjacent Linux mascot for calm coding sessions.",
				SpritesheetPath: "spritesheet.webp",
				Category:        "mascot",
			},
		},
		{
			Manifest: PetJSON{
				ID:              "wall-e",
				DisplayName:     "Wall-E",
				Description:     "A tiny weathered trash-compactor robot companion with binocular eyes and treads.",
				SpritesheetPath: "spritesheet.webp",
				Category:        "mascot",
			},
		},
		{
			Manifest: PetJSON{
				ID:              "dobby",
				DisplayName:     "Dobby",
				Description:     "An earnest, genuinely helpful tiny house-elf companion.",
				SpritesheetPath: "spritesheet.webp",
				Category:        "mascot",
			},
		},
		{Manifest: PetJSON{ID: "aurago-neutral", DisplayName: "Silver", SpritesheetPath: "spritesheet.webp", Category: "persona"}},
		{Manifest: PetJSON{ID: "aurago-servant", DisplayName: "Butler", SpritesheetPath: "spritesheet.webp", Category: "persona"}},
		{Manifest: PetJSON{ID: "aurago-professional", DisplayName: "Manager", SpritesheetPath: "spritesheet.webp", Category: "persona"}},
		{Manifest: PetJSON{ID: "aurago-mistress", DisplayName: "Kommandantin", SpritesheetPath: "spritesheet.webp", Category: "persona"}},
		{Manifest: PetJSON{ID: "aurago-thinker", DisplayName: "Professor", SpritesheetPath: "spritesheet.webp", Category: "persona"}},
		{Manifest: PetJSON{ID: "aurago-evil", DisplayName: "Vampir", SpritesheetPath: "spritesheet.webp", Category: "persona"}},
		{Manifest: PetJSON{ID: "aurago-secretary", DisplayName: "Assistentin", SpritesheetPath: "spritesheet.webp", Category: "persona"}},
		{Manifest: PetJSON{ID: "aurago-psycho", DisplayName: "Green", SpritesheetPath: "spritesheet.webp", Category: "persona"}},
		{Manifest: PetJSON{ID: "aurago-punk", DisplayName: "Punk", SpritesheetPath: "spritesheet.webp", Category: "persona"}},
		{Manifest: PetJSON{ID: "aurago-friend", DisplayName: "Hoodie", SpritesheetPath: "spritesheet.webp", Category: "persona"}},
		{Manifest: PetJSON{ID: "aurago-mcp", DisplayName: "MCP", SpritesheetPath: "spritesheet.webp", Category: "persona"}},
		{Manifest: PetJSON{ID: "aurago-terminator", DisplayName: "Terminator", SpritesheetPath: "spritesheet.webp", Category: "persona"}},
	}
}

// bundledPetFiles are the files installBundledPet writes into Pets/<id>.
var bundledPetFiles = []string{"pet.json", "spritesheet.webp"}

// errPetEntryLinked marks a pet directory or file that exists as a link,
// junction or special file; bundled-pet installs never write through one.
var errPetEntryLinked = errors.New("pet entry is a link or special file")

// bundledPetEntryState inspects Pets/<id> and its bundledPetFiles with Lstat
// through root (the Pets directory), following nothing. It reports whether
// any of them is absent, or errPetEntryLinked when one that exists is not a
// real directory or regular file.
func bundledPetEntryState(root *os.Root, id string) (missing bool, err error) {
	info, err := root.Lstat(id)
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, errPetEntryLinked
	}
	for _, name := range bundledPetFiles {
		info, err := root.Lstat(id + "/" + name)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			missing = true
		case err != nil:
			return false, err
		case !info.Mode().IsRegular():
			return false, errPetEntryLinked
		}
	}
	return missing, nil
}

// bundledPetNeedsRepair reports whether the bundled pet id must be
// reinstalled: it does not load, and its directory or a bundled file is
// genuinely absent while every entry that exists is real. A pet that exists
// but is refused (a link anywhere in it, a special file, an invalid manifest)
// is left alone, logged when logSkip is set.
func bundledPetNeedsRepair(workspaceDir, id string, logSkip bool) bool {
	if _, err := getPetInDir(workspaceDir, id); err == nil {
		return false
	}
	root, err := os.OpenRoot(filepath.Join(workspaceDir, petsDirName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true
		}
		if logSkip {
			slog.Warn("desktop: bundled pet left unrepaired; pets directory is not readable", "pet", id, "error", err)
		}
		return false
	}
	defer root.Close()
	missing, err := bundledPetEntryState(root, id)
	if err == nil && missing {
		return true
	}
	if logSkip {
		reason := "existing files do not load (invalid manifest or spritesheet path)"
		if err != nil {
			reason = err.Error()
		}
		slog.Warn("desktop: bundled pet left unrepaired", "pet", id, "reason", reason)
	}
	return false
}

func ensureBundledDefaultPets(workspaceDir string) error {
	for _, pet := range bundledDefaultPets() {
		if !bundledPetNeedsRepair(workspaceDir, pet.Manifest.ID, true) {
			continue
		}
		if err := installBundledPet(workspaceDir, pet); err != nil {
			if errors.Is(err, webassets.ErrUnavailable) {
				return nil
			}
			if errors.Is(err, errPetEntryLinked) {
				// Replaced by a link since the check above; leave it.
				slog.Warn("desktop: bundled pet left unrepaired", "pet", pet.Manifest.ID, "reason", err.Error())
				continue
			}
			return err
		}
	}
	return nil
}

// InstallBundledDefaultPets installs all OpenPets pets bundled with AuraGo.
func InstallBundledDefaultPets(workspaceDir string) error {
	for _, pet := range bundledDefaultPets() {
		if err := installBundledPet(workspaceDir, pet); err != nil {
			return err
		}
	}
	return nil
}

// InstallBundledDefaultPet installs the built-in OpenPets default pet into the workspace.
func InstallBundledDefaultPet(workspaceDir string, spritesheet []byte) error {
	pet := bundledDefaultPets()[0]
	pet.Spritesheet = spritesheet
	return installBundledPet(workspaceDir, pet)
}

// bundledPetError logs the underlying error server-side and returns one that
// names only the pet and step, keeping webassets.ErrUnavailable detectable.
func bundledPetError(id, step string, err error) error {
	slog.Warn("desktop: bundled pet install failed", "pet", id, "step", step, "error", err)
	if errors.Is(err, webassets.ErrUnavailable) {
		return fmt.Errorf("bundled pet %q: %s: %w", id, step, webassets.ErrUnavailable)
	}
	return fmt.Errorf("bundled pet %q: %s failed", id, step)
}

// installBundledPet writes the bundled pet.json and spritesheet.webp into
// Pets/<id> through an os.Root at Pets. It refuses (errPetEntryLinked) when
// the pet directory or a target file exists as a link, junction or special
// file, so it never writes through a link.
func installBundledPet(workspaceDir string, pet bundledPet) error {
	id := pet.Manifest.ID
	if !petIDPattern.MatchString(id) {
		return fmt.Errorf("invalid bundled pet id %q", id)
	}
	if len(pet.Spritesheet) == 0 {
		data, err := petAssets.ReadFile("pets_assets/" + id + "/spritesheet.webp")
		if err != nil {
			return bundledPetError(id, "load bundled assets", err)
		}
		pet.Spritesheet = data
	}
	manifest, err := json.MarshalIndent(pet.Manifest, "", "  ")
	if err != nil {
		return bundledPetError(id, "marshal manifest", err)
	}
	petsDir := filepath.Join(workspaceDir, petsDirName)
	if err := os.MkdirAll(petsDir, 0o700); err != nil {
		return bundledPetError(id, "create pets directory", err)
	}
	root, err := os.OpenRoot(petsDir)
	if err != nil {
		return bundledPetError(id, "open pets directory", err)
	}
	defer root.Close()
	if _, err := bundledPetEntryState(root, id); err != nil {
		if errors.Is(err, errPetEntryLinked) {
			return fmt.Errorf("bundled pet %q: %w", id, errPetEntryLinked)
		}
		return bundledPetError(id, "inspect pet directory", err)
	}
	if err := root.Mkdir(id, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return bundledPetError(id, "create pet directory", err)
	}
	if err := root.WriteFile(id+"/pet.json", manifest, 0o600); err != nil {
		return bundledPetError(id, "write pet.json", err)
	}
	if err := root.WriteFile(id+"/spritesheet.webp", pet.Spritesheet, 0o600); err != nil {
		return bundledPetError(id, "write spritesheet", err)
	}
	return nil
}

package desktop

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
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

// ListPets returns all pets discovered in the workspace Pets directory.
func (s *Service) ListPets(ctx context.Context) ([]PetManifest, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	cfg := s.Config()
	return listPetsInDir(cfg.WorkspaceDir)
}

// GetPet returns a single pet by ID.
func (s *Service) GetPet(ctx context.Context, id string) (PetManifest, error) {
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
		return nil, fmt.Errorf("read pets directory: %w", err)
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

func getPetInDir(workspaceDir, id string) (PetManifest, error) {
	petDir := filepath.Join(workspaceDir, petsDirName, id)
	data, err := os.ReadFile(filepath.Join(petDir, "pet.json"))
	if err != nil {
		return PetManifest{}, fmt.Errorf("read pet %q: %w", id, err)
	}
	var pet PetJSON
	if err := json.Unmarshal(data, &pet); err != nil {
		return PetManifest{}, fmt.Errorf("parse pet %q: %w", id, err)
	}
	spritesheet := strings.TrimSpace(pet.SpritesheetPath)
	if spritesheet == "" {
		spritesheet = "spritesheet.webp"
	}
	spritesheetPath := filepath.Join(petDir, filepath.ToSlash(filepath.Clean(spritesheet)))
	if _, err := os.Stat(spritesheetPath); err != nil {
		return PetManifest{}, fmt.Errorf("pet %q spritesheet missing: %w", id, err)
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

// InstallBundledDefaultPet installs the built-in OpenPets default pet into the workspace.
func InstallBundledDefaultPet(workspaceDir string, spritesheet []byte) error {
	if len(spritesheet) == 0 {
		return fmt.Errorf("default pet spritesheet is empty")
	}
	petDir := filepath.Join(workspaceDir, petsDirName, "openpets-default")
	if err := os.MkdirAll(petDir, 0o700); err != nil {
		return fmt.Errorf("create default pet directory: %w", err)
	}
	manifest := PetJSON{
		ID:              "openpets-default",
		DisplayName:     "OpenPets Default",
		Description:     "The default OpenPets companion (MIT licensed).",
		SpritesheetPath: "spritesheet.webp",
		Category:        "mascot",
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal default pet manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(petDir, "pet.json"), data, 0o600); err != nil {
		return fmt.Errorf("write default pet.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(petDir, "spritesheet.webp"), spritesheet, 0o600); err != nil {
		return fmt.Errorf("write default pet spritesheet: %w", err)
	}
	return nil
}

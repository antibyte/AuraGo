package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GalleryCategory matches any keyword against an image's descriptive metadata.
type GalleryCategory struct {
	Name     string   `json:"name"`
	Keywords []string `json:"keywords"`
}

func parseGalleryCategories(raw string) ([]GalleryCategory, error) {
	categories := []GalleryCategory{}
	if raw == "" {
		return categories, nil
	}
	if len(raw) > 32768 {
		return nil, fmt.Errorf("gallery categories too large")
	}
	if err := json.Unmarshal([]byte(raw), &categories); err != nil {
		return nil, fmt.Errorf("invalid gallery categories: %w", err)
	}
	if len(categories) > 50 {
		return nil, fmt.Errorf("at most 50 gallery categories are allowed")
	}
	seen := map[string]bool{}
	for _, c := range categories {
		name := strings.TrimSpace(c.Name)
		key := strings.ToLower(name)
		if name == "" || len(name) > 100 || seen[key] || len(c.Keywords) == 0 || len(c.Keywords) > 30 {
			return nil, fmt.Errorf("invalid gallery category")
		}
		seen[key] = true
		for _, word := range c.Keywords {
			if strings.TrimSpace(word) == "" || len(word) > 100 {
				return nil, fmt.Errorf("invalid gallery category keyword")
			}
		}
	}
	if categories == nil {
		categories = []GalleryCategory{}
	}
	return categories, nil
}

func (s *Service) GalleryCategories(ctx context.Context) ([]GalleryCategory, string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, "", err
	}
	settings, err := s.listSettings(ctx)
	if err != nil {
		return nil, "", err
	}
	raw := settings["gallery.categories"]
	categories, err := parseGalleryCategories(raw)
	return categories, NoteVersion([]byte(raw)), err
}

// SaveGalleryCategories preserves edits made by another open gallery window.
func (s *Service) SaveGalleryCategories(ctx context.Context, categories []GalleryCategory, version string) (bool, error) {
	if err := s.ensureReady(ctx); err != nil {
		return false, err
	}
	settings, err := s.listSettings(ctx)
	if err != nil {
		return false, err
	}
	old := settings["gallery.categories"]
	if version != NoteVersion([]byte(old)) {
		return false, nil
	}
	data, err := json.Marshal(categories)
	if err != nil {
		return false, err
	}
	return s.compareAndSetSetting(ctx, "gallery.categories", old, string(data), SourceUser)
}

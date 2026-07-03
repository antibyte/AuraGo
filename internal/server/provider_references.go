package server

import (
	"aurago/internal/config"
	"sort"
	"strings"
)

type providerReferencePayload struct {
	Path string `json:"path"`
	Role string `json:"role,omitempty"`
}

func providerReferences(cfg *config.Config, providerID string) []providerReferencePayload {
	if cfg == nil || strings.TrimSpace(providerID) == "" {
		return nil
	}
	id := strings.TrimSpace(providerID)
	refs := []providerReferencePayload{}
	add := func(path, role, value string) {
		if strings.TrimSpace(value) == id {
			refs = append(refs, providerReferencePayload{Path: path, Role: role})
		}
	}
	add("llm.provider", "primary_llm", cfg.LLM.Provider)
	add("llm.helper_provider", "helper_llm", cfg.LLM.HelperProvider)
	add("vision.provider", "vision", cfg.Vision.Provider)
	add("whisper.provider", "speech_to_text", cfg.Whisper.Provider)
	add("embeddings.provider", "embeddings", cfg.Embeddings.Provider)
	add("llm_guardian.provider", "llm_guardian", cfg.LLMGuardian.Provider)
	add("mission_preparation.provider", "mission_preparation", cfg.MissionPreparation.Provider)
	add("image_generation.provider", "image_generation", cfg.ImageGeneration.Provider)
	add("music_generation.provider", "music_generation", cfg.MusicGeneration.Provider)
	add("video_generation.provider", "video_generation", cfg.VideoGeneration.Provider)
	add("a2a.llm.provider", "a2a_llm", cfg.A2A.LLM.Provider)
	sort.Slice(refs, func(i, j int) bool { return refs[i].Path < refs[j].Path })
	return refs
}

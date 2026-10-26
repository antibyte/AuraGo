package config

import (
	"sort"
	"strings"
)

// ProviderReference identifies a persisted provider selection, including dormant
// features. Runtime inheritance does not rewrite these operator-owned selections.
type ProviderReference struct {
	Path       string `json:"path"`
	Role       string `json:"role"`
	ProviderID string `json:"provider_id"`
}

func isBuiltinEmbeddingProvider(id string) bool {
	switch id {
	case "disabled", "internal", "external", "local-granite", "local-ollama-embeddings":
		return true
	default:
		return false
	}
}

// ProviderReferences is the shared inventory for deletion guards and diagnostics.
func (c *Config) ProviderReferences() []ProviderReference {
	if c == nil {
		return nil
	}
	refs := []ProviderReference{}
	add := func(path, role, id string) {
		if path == "music_generation.provider" && id == LocalMusicProviderID {
			return
		}
		if path == "embeddings.provider" && isBuiltinEmbeddingProvider(id) {
			return
		}
		if (path == "llm.provider" || path == "fallback_llm.provider") && strings.EqualFold(id, LocalLLMProviderID) {
			return
		}
		if strings.TrimSpace(id) != "" {
			refs = append(refs, ProviderReference{Path: path, Role: role, ProviderID: id})
		}
	}
	add("llm.provider", "primary_llm", c.LLM.Provider)
	add("llm.helper_provider", "helper_llm", c.LLM.HelperProvider)
	add("fallback_llm.provider", "fallback_llm", c.FallbackLLM.Provider)
	add("speech_lab.chat_llm_provider_id", "speech_lab", c.SpeechLab.ChatLLMProviderID)
	add("vision.provider", "vision", c.Vision.Provider)
	add("whisper.provider", "speech_to_text", c.Whisper.Provider)
	add("embeddings.provider", "embeddings", c.Embeddings.Provider)
	add("co_agents.llm.provider", "co_agents", c.CoAgents.LLM.Provider)
	for _, specialist := range []struct{ name, id string }{
		{"researcher", c.CoAgents.Specialists.Researcher.LLM.Provider},
		{"coder", c.CoAgents.Specialists.Coder.LLM.Provider},
		{"designer", c.CoAgents.Specialists.Designer.LLM.Provider},
		{"security", c.CoAgents.Specialists.Security.LLM.Provider},
		{"writer", c.CoAgents.Specialists.Writer.LLM.Provider},
	} {
		add("co_agents.specialists."+specialist.name+".llm.provider", "specialist", specialist.id)
	}
	for area, target := range c.LLMRouter.Areas {
		add("llm_router.areas."+area+".provider", "llm_router", target.Provider)
	}
	add("personality.v2_provider", "personality", c.Personality.V2Provider)
	add("memory_analysis.provider", "memory_analysis", c.MemoryAnalysis.Provider)
	add("tools.web_scraper.summary_provider", "summary", c.Tools.WebScraper.SummaryProvider)
	add("tools.wikipedia.summary_provider", "summary", c.Tools.Wikipedia.SummaryProvider)
	add("tools.ddg_search.summary_provider", "summary", c.Tools.DDGSearch.SummaryProvider)
	add("tools.pdf_extractor.summary_provider", "summary", c.Tools.PDFExtractor.SummaryProvider)
	add("llm_guardian.provider", "llm_guardian", c.LLMGuardian.Provider)
	add("mission_preparation.provider", "mission_preparation", c.MissionPreparation.Provider)
	add("flows.ai_provider", "flows", strings.TrimSpace(c.Flows.AIProvider))
	add("cyd_creator.model", "cyd_creator", c.CYDCreator.Model)
	add("image_generation.provider", "image_generation", c.ImageGeneration.Provider)
	add("music_generation.provider", "music_generation", c.MusicGeneration.Provider)
	add("video_generation.provider", "video_generation", c.VideoGeneration.Provider)
	add("a2a.llm.provider", "a2a_llm", c.A2A.LLM.Provider)
	add("sip.voice.agent_provider_id", "telephone_agent_llm", c.SIP.Voice.AgentProviderID)
	add("sip.voice.classic.asr_provider_id", "telephone_agent_asr", c.SIP.Voice.Classic.ASRProviderID)
	add("decisions.provider", "decisions_external_model", c.Decisions.Provider)
	add("yepapi.provider", "yepapi", c.YepAPI.Provider)
	add("virtual_computers.agent_provider", "virtual_computers", c.VirtualComputers.AgentProvider)
	sort.Slice(refs, func(i, j int) bool { return refs[i].Path < refs[j].Path })
	return refs
}

// MissingProviderReferences is passive; loading an old configuration remains
// possible so the operator can repair a dangling selection.
func (c *Config) MissingProviderReferences() []ProviderReference {
	var missing []ProviderReference
	known := map[string]bool{}
	if c != nil {
		for _, p := range c.Providers {
			known[p.ID] = true
		}
	}
	for _, ref := range c.ProviderReferences() {
		if !known[ref.ProviderID] {
			missing = append(missing, ref)
		}
	}
	return missing
}

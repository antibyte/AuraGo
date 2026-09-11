package config

import (
	"fmt"
	"math"
	"strings"
)

const LocalMusicProviderID = "aurago-acestep-local"

// LocalMusicConfig contains user choices, never runtime credentials or image URLs.
type LocalMusicConfig struct {
	Backend        string   `yaml:"backend" json:"backend"`
	Device         string   `yaml:"device" json:"device"`
	VRAMReserveGB  *float64 `yaml:"vram_reserve_gb,omitempty" json:"vram_reserve_gb,omitempty"`
	TimeoutSeconds int      `yaml:"timeout_seconds" json:"timeout_seconds"`
}

func (c LocalMusicConfig) Defaults() LocalMusicConfig {
	if c.Backend == "" {
		c.Backend = "auto"
	}
	if c.Device == "" {
		c.Device = "auto"
	}
	if c.VRAMReserveGB == nil {
		reserve := 1.0
		c.VRAMReserveGB = &reserve
	}
	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = 1800
	}
	return c
}

func (c LocalMusicConfig) Validate() error {
	c = c.Defaults()
	if !oneOf(c.Backend, "auto", "cuda", "rocm", "xpu", "vulkan", "cpu") {
		return fmt.Errorf("music_generation.local.backend must be auto, cuda, rocm, xpu, vulkan or cpu")
	}
	if len(c.Device) > 128 || strings.ContainsAny(c.Device, "\r\n\x00/\\") {
		return fmt.Errorf("invalid local music device")
	}
	if math.IsNaN(*c.VRAMReserveGB) || math.IsInf(*c.VRAMReserveGB, 0) || *c.VRAMReserveGB < 0 || *c.VRAMReserveGB > 256 {
		return fmt.Errorf("local music VRAM reserve must be between 0 and 256 GiB")
	}
	if c.TimeoutSeconds < 30 || c.TimeoutSeconds > 1800 {
		return fmt.Errorf("local music timeout must be between 30 and 1800 seconds")
	}
	return nil
}

func (c *Config) UsesLocalMusic() bool {
	return c != nil && c.MusicGeneration.Provider == LocalMusicProviderID
}

// MusicConfigured deliberately does not use a synthetic API key for local music.
func (c *Config) MusicConfigured() bool {
	return c != nil && c.MusicGeneration.Enabled && (c.UsesLocalMusic() || strings.TrimSpace(c.MusicGeneration.APIKey) != "")
}

func ValidateLocalMusicConfig(c *Config) error {
	if err := c.MusicGeneration.Local.Validate(); err != nil {
		return err
	}
	for _, p := range c.Providers {
		if strings.EqualFold(p.ID, LocalMusicProviderID) {
			return fmt.Errorf("provider ID %q is reserved for managed music", LocalMusicProviderID)
		}
	}
	refs := []string{c.LLM.Provider, c.LLM.HelperProvider, c.FallbackLLM.Provider,
		c.Vision.Provider, c.Whisper.Provider, c.Embeddings.Provider, c.CoAgents.LLM.Provider,
		c.A2A.LLM.Provider, c.Personality.V2Provider, c.MemoryAnalysis.Provider, c.LLMGuardian.Provider,
		c.MissionPreparation.Provider, c.ImageGeneration.Provider, c.VideoGeneration.Provider, c.YepAPI.Provider,
		c.Tools.WebScraper.SummaryProvider, c.Tools.Wikipedia.SummaryProvider, c.Tools.DDGSearch.SummaryProvider, c.Tools.PDFExtractor.SummaryProvider}
	for _, role := range []string{"researcher", "coder", "designer", "security", "writer"} {
		if spec := c.GetSpecialist(role); spec != nil {
			refs = append(refs, spec.LLM.Provider)
		}
	}
	for _, ref := range refs {
		if strings.EqualFold(strings.TrimSpace(ref), LocalMusicProviderID) {
			return fmt.Errorf("managed ACE-Step is available only for music generation")
		}
	}
	return nil
}

package server

import (
	"aurago/internal/config"
	"aurago/internal/newspaper"
)

type newspaperResearchTool struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	Reason    string `json:"reason,omitempty"`
	LastError string `json:"last_error,omitempty"`
}

// The runner and HTTP readiness endpoint deliberately share this resolver.
// "ready" means configured and permitted; only a request proves service health.
type newspaperResearchCapabilities struct {
	Ready  bool                    `json:"ready"`
	Reason string                  `json:"reason,omitempty"`
	Tools  []newspaperResearchTool `json:"tools"`
}

func resolveNewspaperCapabilities(cfg *config.Config, p newspaper.Profile, skillReady, modelReady bool) newspaperResearchCapabilities {
	c := newspaperResearchCapabilities{Tools: []newspaperResearchTool{
		{ID: "brave_search", State: "disabled", Reason: "disabled"},
		{ID: "ddg_search", State: "blocked", Reason: "network_disabled"},
		{ID: "rss", State: "needs_setup", Reason: "feeds_missing"},
		{ID: "web_scraper", State: "disabled", Reason: "scraper_disabled"},
	}}
	if cfg == nil {
		c.Reason = "disabled"
		return c
	}
	if cfg.BraveSearch.Enabled {
		c.Tools[0].State, c.Tools[0].Reason = "needs_setup", "key_missing"
		if cfg.BraveSearch.APIKey != "" {
			c.Tools[0].State, c.Tools[0].Reason = "ready", ""
		}
	}
	if cfg.Agent.AllowNetworkRequests {
		c.Tools[1].State, c.Tools[1].Reason = "ready", ""
	}
	if len(p.RSSFeeds) > 0 {
		c.Tools[2].State, c.Tools[2].Reason = "ready", ""
	}
	if cfg.Tools.WebScraper.Enabled {
		c.Tools[3].State, c.Tools[3].Reason = "ready", ""
	}
	switch {
	case !cfg.VirtualDesktop.Enabled || !cfg.Newspaper.Enabled:
		c.Reason = "disabled"
	case cfg.VirtualDesktop.ReadOnly || cfg.Newspaper.ReadOnly:
		c.Reason = "read_only"
	case !cfg.Agent.AllowNetworkRequests:
		c.Reason = "network_disabled"
	}
	if c.Reason != "" {
		for i := range c.Tools {
			c.Tools[i].State, c.Tools[i].Reason = "blocked", c.Reason
		}
		return c
	}
	switch {
	case !cfg.Tools.WebScraper.Enabled:
		c.Reason = "scraper_disabled"
	case !modelReady || cfg.LLM.Model == "":
		c.Reason = "model_missing"
	case !skillReady:
		c.Reason = "skill_missing"
	default:
		c.Ready = true
	}
	return c
}

func (c newspaperResearchCapabilities) allows(id string) bool {
	if !c.Ready {
		return false
	}
	for _, tool := range c.Tools {
		if tool.ID == id {
			return tool.State == "ready"
		}
	}
	return false
}

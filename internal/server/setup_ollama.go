package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/i18n"
)

const (
	setupOllamaProbeTimeout = 2 * time.Second
	setupOllamaCacheTTL     = 5 * time.Second
	setupOllamaMaxBody      = 1 << 20
	setupOllamaMaxModels    = 200
)

// setupOllamaTagsURL is the only address the setup detection ever contacts: the
// local Ollama of this installation. It takes no user input; tests replace it.
var setupOllamaTagsURL = func(isDocker bool) string {
	return strings.TrimSuffix(setupOllamaBaseURL(isDocker), "/v1") + "/api/tags"
}

type setupOllamaModel struct {
	Name          string `json:"name"`
	Size          int64  `json:"size"`
	ParameterSize string `json:"parameter_size,omitempty"`
	Family        string `json:"family,omitempty"`
}

type setupOllamaResult struct {
	Reachable bool               `json:"reachable"`
	BaseURL   string             `json:"base_url"`
	Models    []setupOllamaModel `json:"models"`
	Code      string             `json:"code,omitempty"`
}

// The probe is serialized and its result cached briefly, so a reloading wizard
// cannot turn the endpoint into a request amplifier against the local Ollama.
var setupOllamaProbeState struct {
	mu     sync.Mutex
	url    string
	at     time.Time
	result setupOllamaResult
}

func resetSetupOllamaProbeCache() {
	setupOllamaProbeState.mu.Lock()
	defer setupOllamaProbeState.mu.Unlock()
	setupOllamaProbeState.url = ""
	setupOllamaProbeState.at = time.Time{}
}

// handleSetupOllamaDetect reports whether the default local Ollama is running
// and which chat models it offers, so the wizard can offer a no-key plan.
func handleSetupOllamaDetect(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		show := needsSetup(s.Cfg)
		isDocker := s.Cfg.Runtime.IsDocker
		s.CfgMu.RUnlock()
		if !show {
			writeSetupGateError(w, http.StatusForbidden, "setup_already_completed", i18n.T(s.Cfg.Server.UILanguage, "backend.setup_already_completed"))
			return
		}
		if !authorizeSetupBootstrap(s, w, r) {
			return
		}
		// Not consumed: the wizard keeps using the same token for the final save.
		if !validateSetupCSRFToken(s, r.Header.Get("X-CSRF-Token"), false) {
			writeSetupGateError(w, http.StatusForbidden, "csrf_invalid", i18n.T(s.Cfg.Server.UILanguage, "backend.setup_invalid_csrf_token"))
			return
		}

		result := detectSetupOllama(r.Context(), isDocker)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(result)
	}
}

func detectSetupOllama(ctx context.Context, isDocker bool) setupOllamaResult {
	tagsURL := setupOllamaTagsURL(isDocker)

	setupOllamaProbeState.mu.Lock()
	defer setupOllamaProbeState.mu.Unlock()
	if setupOllamaProbeState.url == tagsURL && time.Since(setupOllamaProbeState.at) < setupOllamaCacheTTL {
		return setupOllamaProbeState.result
	}
	result := probeSetupOllama(ctx, tagsURL, setupOllamaBaseURL(isDocker))
	setupOllamaProbeState.url = tagsURL
	setupOllamaProbeState.at = time.Now()
	setupOllamaProbeState.result = result
	return result
}

func probeSetupOllama(ctx context.Context, tagsURL, baseURL string) setupOllamaResult {
	result := setupOllamaResult{BaseURL: baseURL, Models: []setupOllamaModel{}, Code: "unreachable"}
	parsed, err := url.Parse(tagsURL)
	if err != nil || !isAllowedOllamaSetupHost(strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))) {
		return result
	}

	client := &http.Client{
		Timeout:       setupOllamaProbeTimeout,
		Transport:     &http.Transport{Proxy: nil, DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsURL, nil)
	if err != nil {
		return result
	}
	resp, err := client.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		result.Code = "bad_response"
		return result
	}

	var payload struct {
		Models []struct {
			Name    string `json:"name"`
			Size    int64  `json:"size"`
			Details struct {
				Family        string `json:"family"`
				ParameterSize string `json:"parameter_size"`
			} `json:"details"`
		} `json:"models"`
	}
	limited := io.LimitReader(resp.Body, setupOllamaMaxBody)
	if err := json.NewDecoder(limited).Decode(&payload); err != nil {
		result.Code = "bad_response"
		return result
	}

	models := make([]setupOllamaModel, 0, len(payload.Models))
	for _, m := range payload.Models {
		name := strings.TrimSpace(m.Name)
		if name == "" {
			continue
		}
		// Embedding-only models cannot chat and would be a dead end as the main model.
		lowerName := strings.ToLower(name)
		lowerFamily := strings.ToLower(m.Details.Family)
		if strings.Contains(lowerName, "embed") || strings.Contains(lowerFamily, "bert") {
			continue
		}
		models = append(models, setupOllamaModel{Name: name, Size: m.Size, ParameterSize: m.Details.ParameterSize, Family: m.Details.Family})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
	if len(models) > setupOllamaMaxModels {
		models = models[:setupOllamaMaxModels]
	}
	result.Reachable = true
	result.Models = models
	result.Code = ""
	return result
}

package llm

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"

	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

type taskRouteEntry struct {
	client       ChatClient
	route        ModelRoute
	capabilities *ProviderCapabilityResult
}

func (e taskRouteEntry) supports(req openai.ChatCompletionRequest) bool {
	if e.capabilities != nil {
		return routeSupportsRequest(e.route, req, *e.capabilities)
	}
	return routeSupportsRequest(e.route, req)
}

type singleCompletionAttemptKey struct{}

// WithSingleCompletionAttempt also limits retries requested from managed API proxies.
func WithSingleCompletionAttempt(ctx context.Context) context.Context {
	return context.WithValue(ctx, singleCompletionAttemptKey{}, true)
}

// TaskRouteClient freezes route identities and clients for one run. It owns no probes.
type TaskRouteClient struct {
	mu      sync.Mutex
	entries []taskRouteEntry
	active  int
}

var taskProviderClients = struct {
	sync.Mutex
	entries map[[32]byte]*openai.Client
}{entries: make(map[[32]byte]*openai.Client)}

// TaskConfigFingerprint hashes routing and transport settings, including resolved
// credentials. JSON dereferences capability pointers, so equivalent clones match.
// Neither this material nor its credential fields are logged or persisted.
func TaskConfigFingerprint(cfg *config.Config) [32]byte {
	data, _ := json.Marshal([]any{cfg.LLM, cfg.LLMRouter, cfg.Providers, cfg.AIGateway, cfg.Manifest, cfg.CircuitBreaker, cfg.Agent.ContextWindow})
	secrets := []string{cfg.LLM.ProviderType, cfg.LLM.BaseURL, cfg.LLM.Model, cfg.LLM.APIKey, cfg.LLM.AccountID, cfg.LLM.HelperBaseURL, cfg.LLM.HelperAPIKey, cfg.LLM.HelperResolvedModel}
	for _, p := range cfg.Providers {
		secrets = append(secrets, p.ID, p.APIKey, p.OAuthClientSecret)
	}
	encoded, _ := json.Marshal(secrets)
	return sha256.Sum256(append(data, encoded...))
}

// NewTaskRouteClient captures the existing failover state without reconfiguring it.
func NewTaskRouteClient(cfg *config.Config, ordinary ChatClient, selected *config.ProviderEntry) *TaskRouteClient {
	client := &TaskRouteClient{}
	factory := NewClient
	if fm, ok := ordinary.(*FailoverManager); ok {
		fm.mu.RLock()
		factory = fm.clientFactory
		primary := taskRouteEntry{client: fm.primary, route: fm.primaryRoute}
		fallback := taskRouteEntry{client: fm.fallback, route: fm.fallbackRoute}
		if fm.isOnFallback && fm.fallback != nil {
			client.entries = append(client.entries, fallback)
		}
		client.entries = append(client.entries, primary)
		if !fm.isOnFallback && fm.fallback != nil {
			client.entries = append(client.entries, fallback)
		}
		fm.mu.RUnlock()
	} else {
		client.entries = append(client.entries, taskRouteEntry{client: ordinary, route: modelRouteFromConfig(cfg, false)})
	}
	if factory == nil {
		factory = NewClient
	}
	if selected != nil {
		view := TaskProviderConfig(cfg, *selected)
		// Hash the complete effective configuration, including credentials. Never log it.
		key := sha256.Sum256([]byte(fmt.Sprintf("%x|%p|%p", TaskConfigFingerprint(view), factory, ordinary)))
		taskProviderClients.Lock()
		targetClient := taskProviderClients.entries[key]
		if targetClient == nil {
			if len(taskProviderClients.entries) >= 64 {
				clear(taskProviderClients.entries)
			}
			targetClient = factory(view)
			taskProviderClients.entries[key] = targetClient
		}
		taskProviderClients.Unlock()
		caps := ResolveProviderCapabilities(*selected, CapabilityFallback{})
		client.entries = append([]taskRouteEntry{{client: targetClient, route: modelRouteFromConfig(view, false), capabilities: &caps}}, client.entries...)
	}
	seen := map[string]bool{}
	entries := client.entries[:0]
	for _, entry := range client.entries {
		key := entry.route.ProviderID + "|" + entry.route.BaseURL + "|" + entry.route.Model
		if entry.client != nil && !seen[key] {
			seen[key] = true
			entries = append(entries, entry)
		}
	}
	client.entries = entries
	return client
}

func TaskProviderConfig(cfg *config.Config, p config.ProviderEntry) *config.Config {
	view := cfg.Clone()
	view.LLM.Provider, view.LLM.ProviderType = p.ID, p.Type
	view.LLM.Model, view.LLM.BaseURL, view.LLM.APIKey, view.LLM.AccountID = p.Model, p.BaseURL, p.APIKey, p.AccountID
	// Exact model metadata must also be visible to existing capability resolution.
	for i := range view.Providers {
		if view.Providers[i].ID == p.ID {
			view.Providers[i] = p
		}
	}
	return view
}

func (c *TaskRouteClient) CandidateRoutes(req openai.ChatCompletionRequest) []ModelRoute {
	c.mu.Lock()
	defer c.mu.Unlock()
	routes := []ModelRoute{}
	for _, e := range c.entries[c.active:] {
		if e.supports(req) {
			routes = append(routes, e.route)
		}
	}
	return routes
}

func (c *TaskRouteClient) ActiveRoute() ModelRoute {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) == 0 {
		return ModelRoute{}
	}
	return c.entries[c.active].route
}

func (c *TaskRouteClient) ActiveProviderAndModel() (string, string) {
	r := c.ActiveRoute()
	return r.ProviderType, r.Model
}

// CanFallbackTaskRequest excludes cancellation and permission/configuration errors.
func CanFallbackTaskRequest(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var api *openai.APIError
	if errors.As(err, &api) {
		return api.HTTPStatusCode == 429 || api.HTTPStatusCode >= 500
	}
	var request *openai.RequestError
	if errors.As(err, &request) {
		return request.HTTPStatusCode == 429 || request.HTTPStatusCode >= 500
	}
	var network net.Error
	return errors.As(err, &network)
}

func (c *TaskRouteClient) next(req openai.ChatCompletionRequest, start int) (taskRouteEntry, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if start < c.active {
		start = c.active
	}
	for i := start; i < len(c.entries); i++ {
		if c.entries[i].supports(req) {
			c.active = i
			return c.entries[i], i, nil
		}
	}
	return taskRouteEntry{}, 0, fmt.Errorf("task route has no compatible provider")
}

func (c *TaskRouteClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	for index := 0; ; {
		if err := ctx.Err(); err != nil {
			return openai.ChatCompletionResponse{}, err
		}
		e, i, err := c.next(req, index)
		if err != nil {
			return openai.ChatCompletionResponse{}, err
		}
		copy := req
		copy.Model = e.route.Model
		resp, err := e.client.CreateChatCompletion(ctx, copy)
		if err == nil || !CanFallbackTaskRequest(err) || i+1 == len(c.entries) {
			return resp, err
		}
		index = i + 1
	}
}

func (c *TaskRouteClient) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	for index := 0; ; {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		e, i, err := c.next(req, index)
		if err != nil {
			return nil, err
		}
		copy := req
		copy.Model = e.route.Model
		stream, err := e.client.CreateChatCompletionStream(ctx, copy)
		if err == nil || !CanFallbackTaskRequest(err) || i+1 == len(c.entries) {
			return stream, err
		}
		index = i + 1
	}
}

// PruneTaskClientCache is called on config publication/shutdown, never per turn.
func PruneTaskClientCache() {
	taskProviderClients.Lock()
	clear(taskProviderClients.entries)
	taskProviderClients.Unlock()
}

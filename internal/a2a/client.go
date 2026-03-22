package a2a

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"aurago/internal/config"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
)

// RemoteAgentStatus represents the status of a remote A2A agent.
type RemoteAgentStatus struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	CardURL   string         `json:"card_url"`
	Enabled   bool           `json:"enabled"`
	Available bool           `json:"available"`
	LastCheck time.Time      `json:"last_check"`
	Error     string         `json:"error,omitempty"`
	Card      *a2a.AgentCard `json:"card,omitempty"`
}

// backoffState tracks exponential backoff for a failed remote agent.
type backoffState struct {
	failCount int
	nextRetry time.Time
}

// ClientManager manages A2A client connections to remote agents.
type ClientManager struct {
	cfg      *config.Config
	logger   *slog.Logger
	resolver *agentcard.Resolver

	mu      sync.RWMutex
	clients map[string]*a2aclient.Client // keyed by remote agent ID
	cards   map[string]*a2a.AgentCard    // cached agent cards
	status  map[string]*RemoteAgentStatus
	backoff map[string]*backoffState // exponential backoff per agent
}

// NewClientManager creates a new A2A client manager.
func NewClientManager(cfg *config.Config, logger *slog.Logger) *ClientManager {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	return &ClientManager{
		cfg:      cfg,
		logger:   logger.With("component", "a2a-client"),
		resolver: agentcard.NewResolver(httpClient),
		clients:  make(map[string]*a2aclient.Client),
		cards:    make(map[string]*a2a.AgentCard),
		status:   make(map[string]*RemoteAgentStatus),
		backoff:  make(map[string]*backoffState),
	}
}

// Initialize resolves agent cards and creates clients for all configured remote agents.
func (m *ClientManager) Initialize(ctx context.Context) {
	for _, ra := range m.cfg.A2A.Client.RemoteAgents {
		if !ra.Enabled {
			continue
		}
		m.resolveAndConnect(ctx, ra)
	}
}

// resolveAndConnect attempts to resolve the agent card and create a client for a remote agent.
func (m *ClientManager) resolveAndConnect(ctx context.Context, ra config.A2ARemoteAgent) {
	status := &RemoteAgentStatus{
		ID:      ra.ID,
		Name:    ra.Name,
		CardURL: ra.CardURL,
		Enabled: ra.Enabled,
	}

	// Resolve agent card
	resolveCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var opts []agentcard.ResolveOption
	if ra.APIKey != "" {
		opts = append(opts, agentcard.WithRequestHeader("X-API-Key", ra.APIKey))
	}
	if ra.BearerToken != "" {
		opts = append(opts, agentcard.WithRequestHeader("Authorization", "Bearer "+ra.BearerToken))
	}

	card, err := m.resolver.Resolve(resolveCtx, ra.CardURL, opts...)
	if err != nil {
		m.logger.Warn("Failed to resolve A2A agent card", "agent", ra.ID, "url", ra.CardURL, "error", err)
		status.Error = err.Error()
		status.LastCheck = time.Now()
		m.mu.Lock()
		m.status[ra.ID] = status
		m.updateBackoffLocked(ra.ID, false)
		m.mu.Unlock()
		return
	}

	// Create client from card
	var factoryOpts []a2aclient.FactoryOption
	client, err := a2aclient.NewFromCard(ctx, card, factoryOpts...)
	if err != nil {
		m.logger.Warn("Failed to create A2A client", "agent", ra.ID, "error", err)
		status.Error = err.Error()
		status.Card = card
		status.LastCheck = time.Now()
		m.mu.Lock()
		m.status[ra.ID] = status
		m.cards[ra.ID] = card
		m.updateBackoffLocked(ra.ID, false)
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	m.clients[ra.ID] = client
	m.cards[ra.ID] = card
	status.Available = true
	status.Card = card
	status.LastCheck = time.Now()
	m.status[ra.ID] = status
	m.updateBackoffLocked(ra.ID, true)
	m.mu.Unlock()

	m.logger.Info("A2A remote agent connected", "agent", ra.ID, "name", card.Name)
}

// updateBackoffLocked updates the backoff state for an agent.
// success=true resets the failure counter; success=false increments it.
// Must be called with m.mu held.
func (m *ClientManager) updateBackoffLocked(id string, success bool) {
	if success {
		delete(m.backoff, id)
		return
	}
	b := m.backoff[id]
	if b == nil {
		b = &backoffState{}
		m.backoff[id] = b
	}
	b.failCount++
	shift := b.failCount - 1 // first failure → 30s, second → 60s, ...
	if shift > 6 {
		shift = 6 // cap exponent: 30s * 2^6 = 32 min
	}
	delay := (30 * time.Second) << shift
	const maxBackoff = 30 * time.Minute
	if delay > maxBackoff {
		delay = maxBackoff
	}
	b.nextRetry = time.Now().Add(delay)
}

// SendMessage sends a message to a remote A2A agent.
func (m *ClientManager) SendMessage(ctx context.Context, agentID string, text string) (a2a.SendMessageResult, error) {
	m.mu.RLock()
	client, ok := m.clients[agentID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("remote agent %q not available", agentID)
	}

	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(text))
	req := &a2a.SendMessageRequest{
		Message: msg,
	}

	return client.SendMessage(ctx, req)
}

// GetTask retrieves a task from a remote agent.
func (m *ClientManager) GetTask(ctx context.Context, agentID string, taskID a2a.TaskID) (*a2a.Task, error) {
	m.mu.RLock()
	client, ok := m.clients[agentID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("remote agent %q not available", agentID)
	}

	return client.GetTask(ctx, &a2a.GetTaskRequest{ID: taskID})
}

// ListRemoteAgents returns the status of all configured remote agents.
func (m *ClientManager) ListRemoteAgents() []RemoteAgentStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]RemoteAgentStatus, 0, len(m.status))
	for _, s := range m.status {
		result = append(result, *s)
	}
	return result
}

// GetRemoteAgentStatus returns the status of a specific remote agent.
func (m *ClientManager) GetRemoteAgentStatus(agentID string) (*RemoteAgentStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.status[agentID]
	if !ok {
		return nil, false
	}
	cp := *s
	return &cp, true
}

// StartHealthCheck starts a background loop that periodically checks remote agent availability.
func (m *ClientManager) StartHealthCheck(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.checkHealth(ctx)
			}
		}
	}()
}

func (m *ClientManager) checkHealth(ctx context.Context) {
	// At most 3 concurrent reconnect attempts to avoid hammering the network.
	const maxConcurrent = 3
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, ra := range m.cfg.A2A.Client.RemoteAgents {
		if !ra.Enabled {
			continue
		}

		// Skip agents that are currently unavailable and still within their
		// exponential backoff window.
		m.mu.RLock()
		status := m.status[ra.ID]
		b := m.backoff[ra.ID]
		m.mu.RUnlock()
		if status != nil && !status.Available && b != nil && time.Now().Before(b.nextRetry) {
			continue
		}

		wg.Add(1)
		go func(ra config.A2ARemoteAgent) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			m.resolveAndConnect(ctx, ra)
		}(ra)
	}
	wg.Wait()
}

// IsAvailable returns whether a remote agent is available.
func (m *ClientManager) IsAvailable(agentID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.status[agentID]
	return ok && s.Available
}

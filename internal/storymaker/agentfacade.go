package storymaker

import (
	"context"
	"sync"
)

// AgentCreateRequest is a chat request for a new story.
type AgentCreateRequest struct {
	Topic, Audience, Aspect, Language string
	LengthSeconds                     int
	Voice                             *Voice
}

// AgentJobRef names a queued agent job.
type AgentJobRef struct {
	ProjectID, JobID string
	State            string
	ETAHintSeconds   int
}

// AgentStatus is what the chat tool reports about the latest agent job of a project.
type AgentStatus struct {
	State                   string
	Phase                   string
	ScenesDone, ScenesTotal int
	OpenChecks              []AgentCheck
	ErrorCode, NextAction   string
	PollAfterSeconds        int
	OutputPath              string
	ChangedScenes           []string
	UnchangedScenes         []string
	RemovedScenes           []string
}

// AgentProjectRow is one row of the chat tool's list.
type AgentProjectRow struct {
	ProjectID, Title, UpdatedAt, State string
	DurationSeconds                    float64
}

// AgentFacade is what the chat tool needs from the server. The server implements it, because the
// job runner lives there and package storymaker must not import it.
type AgentFacade interface {
	Available() bool
	Create(ctx context.Context, req AgentCreateRequest) (AgentJobRef, error)
	Status(ctx context.Context, projectID string) (AgentStatus, error)
	Revise(ctx context.Context, projectID, instruction string) (AgentJobRef, error)
	Render(ctx context.Context, projectID string) (renderJobID string, err error)
	// RenderSilent removes the speech like the make-silent route, then queues the export.
	RenderSilent(ctx context.Context, projectID string) (renderJobID string, scenesChanged int, err error)
	List(ctx context.Context) ([]AgentProjectRow, error)
}

// AgentError is a refusal the chat tool shows to the model: a stable code and the next action.
type AgentError struct {
	Code, Message, NextAction string
}

func (e *AgentError) Error() string { return e.Code + ": " + e.Message }

var (
	agentFacadeMu sync.RWMutex
	agentFacade   AgentFacade
)

// SetAgentFacade publishes the server's facade; nil withdraws it.
func SetAgentFacade(f AgentFacade) {
	agentFacadeMu.Lock()
	defer agentFacadeMu.Unlock()
	agentFacade = f
}

// CurrentAgentFacade returns the published facade, or nil.
func CurrentAgentFacade() AgentFacade {
	agentFacadeMu.RLock()
	defer agentFacadeMu.RUnlock()
	return agentFacade
}

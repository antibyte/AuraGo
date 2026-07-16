package virtualcomputers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultAgentTaskConcurrency = 2
	defaultAgentTaskTimeout     = 15 * time.Minute
	defaultAgentTaskRetention   = 30 * 24 * time.Hour
	maxAgentTaskEvents          = 500
	maxAgentTaskEventBytes      = 1024 * 1024
)

type TaskManagerOptions struct {
	MaxConcurrent int
	Timeout       time.Duration
	Retention     time.Duration
}

type activeAgentTask struct {
	cancel context.CancelFunc
	conn   *websocket.Conn
}

type TaskManager struct {
	ledger     *Ledger
	ownsLedger bool
	logger     *slog.Logger
	timeout    time.Duration
	retention  time.Duration
	sem        chan struct{}
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex
	active     map[string]*activeAgentTask
	wg         sync.WaitGroup
	closeOnce  sync.Once
}

var (
	defaultTaskManagerMu sync.RWMutex
	defaultTaskManager   *TaskManager
)

func SetDefaultTaskManager(manager *TaskManager) {
	defaultTaskManagerMu.Lock()
	defer defaultTaskManagerMu.Unlock()
	defaultTaskManager = manager
}

func DefaultTaskManager() *TaskManager {
	defaultTaskManagerMu.RLock()
	defer defaultTaskManagerMu.RUnlock()
	return defaultTaskManager
}

func OpenTaskManager(path string, logger *slog.Logger, opts TaskManagerOptions) (*TaskManager, error) {
	ledger, err := OpenLedger(path)
	if err != nil {
		return nil, err
	}
	manager, err := newTaskManager(ledger, logger, opts, true)
	if err != nil {
		_ = ledger.Close()
		return nil, err
	}
	return manager, nil
}

// NewTaskManager creates a task manager that borrows an existing ledger.
// Closing the manager does not close the shared ledger.
func NewTaskManager(ledger *Ledger, logger *slog.Logger, opts TaskManagerOptions) (*TaskManager, error) {
	return newTaskManager(ledger, logger, opts, false)
}

func newTaskManager(ledger *Ledger, logger *slog.Logger, opts TaskManagerOptions, ownsLedger bool) (*TaskManager, error) {
	if ledger == nil || ledger.db == nil {
		return nil, fmt.Errorf("virtual computers ledger is required")
	}
	if opts.MaxConcurrent <= 0 {
		opts.MaxConcurrent = defaultAgentTaskConcurrency
	}
	if opts.Timeout <= 0 {
		opts.Timeout = defaultAgentTaskTimeout
	}
	if opts.Retention <= 0 {
		opts.Retention = defaultAgentTaskRetention
	}
	ctx, cancel := context.WithCancel(context.Background())
	mgr := &TaskManager{
		ledger: ledger, ownsLedger: ownsLedger, logger: logger, timeout: opts.Timeout, retention: opts.Retention,
		sem: make(chan struct{}, opts.MaxConcurrent), ctx: ctx, cancel: cancel,
		active: make(map[string]*activeAgentTask),
	}
	if err := ledger.InterruptActiveAgentTasks(ctx); err != nil {
		cancel()
		return nil, err
	}
	if err := ledger.CleanupAgentTasks(ctx, time.Now().UTC().Add(-opts.Retention)); err != nil && logger != nil {
		logger.Warn("Failed to clean up virtual computer agent task history", "error", err)
	}
	return mgr, nil
}

func (m *TaskManager) Submit(client *Client, machineID, kind, instruction string) (AgentTask, error) {
	if m == nil || m.ledger == nil {
		return AgentTask{}, fmt.Errorf("virtual computer task manager is unavailable")
	}
	if client == nil {
		return AgentTask{}, fmt.Errorf("boringd client is required")
	}
	machineID = strings.TrimSpace(machineID)
	instruction = strings.TrimSpace(instruction)
	if machineID == "" {
		return AgentTask{}, fmt.Errorf("machine_id is required")
	}
	if instruction == "" {
		return AgentTask{}, fmt.Errorf("instruction is required")
	}
	if len(instruction) > 400 {
		return AgentTask{}, fmt.Errorf("instruction must not exceed 400 bytes")
	}
	if kind != AgentTaskKindShell && kind != AgentTaskKindDesktop {
		return AgentTask{}, fmt.Errorf("unsupported agent task kind %q", kind)
	}
	id, err := newAgentTaskID()
	if err != nil {
		return AgentTask{}, err
	}
	now := time.Now().UTC()
	task := AgentTask{ID: id, MachineID: machineID, Kind: kind, Instruction: instruction,
		Status: AgentTaskStatusQueued, CreatedAt: now, UpdatedAt: now}
	if err := m.ledger.InsertAgentTask(context.Background(), task); err != nil {
		return AgentTask{}, err
	}
	taskCtx, cancel := context.WithTimeout(m.ctx, m.timeout)
	m.mu.Lock()
	m.active[id] = &activeAgentTask{cancel: cancel}
	m.mu.Unlock()
	m.wg.Add(1)
	go m.run(taskCtx, client, task)
	return task, nil
}

func (m *TaskManager) run(ctx context.Context, client *Client, task AgentTask) {
	defer m.wg.Done()
	defer func() {
		m.mu.Lock()
		if active := m.active[task.ID]; active != nil {
			active.cancel()
		}
		delete(m.active, task.ID)
		m.mu.Unlock()
	}()
	select {
	case m.sem <- struct{}{}:
		defer func() { <-m.sem }()
	case <-ctx.Done():
		m.finish(task.ID, AgentTaskStatusFailed, contextErrorMessage(ctx.Err()))
		return
	}
	if ctx.Err() != nil {
		m.finish(task.ID, AgentTaskStatusFailed, contextErrorMessage(ctx.Err()))
		return
	}
	if err := m.ledger.UpdateAgentTaskRunning(context.Background(), task.ID); err != nil {
		m.finish(task.ID, AgentTaskStatusFailed, err.Error())
		return
	}
	channel := "shell-agent"
	if task.Kind == AgentTaskKindDesktop {
		channel = "agent"
	}
	wsURL, headers, err := client.WebSocketURL(task.MachineID, channel, task.Instruction)
	if err != nil {
		m.finish(task.ID, AgentTaskStatusFailed, err.Error())
		return
	}
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, headers)
	upstreamError := ""
	if resp != nil && resp.Body != nil {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		_ = resp.Body.Close()
		if readErr == nil {
			var payload struct {
				Error string `json:"error"`
			}
			if json.Unmarshal(body, &payload) == nil {
				upstreamError = strings.TrimSpace(payload.Error)
			}
		}
	}
	if err != nil {
		message := fmt.Sprintf("connect boringd agent websocket: %v", err)
		if upstreamError != "" {
			message += ": " + upstreamError
		}
		m.finish(task.ID, AgentTaskStatusFailed, message)
		return
	}
	defer conn.Close()
	m.mu.Lock()
	active := m.active[task.ID]
	if active != nil {
		active.conn = conn
	}
	canceled := active == nil || ctx.Err() != nil
	m.mu.Unlock()
	if canceled {
		_ = conn.Close()
		m.finish(task.ID, AgentTaskStatusFailed, contextErrorMessage(ctx.Err()))
		return
	}
	readDone := make(chan struct{})
	defer close(readDone)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-readDone:
		}
	}()
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				m.finish(task.ID, AgentTaskStatusFailed, contextErrorMessage(ctx.Err()))
			} else {
				m.finish(task.ID, AgentTaskStatusFailed, fmt.Sprintf("boringd agent websocket closed before completion: %v", err))
			}
			return
		}
		var event struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			m.finish(task.ID, AgentTaskStatusFailed, fmt.Sprintf("decode boringd agent event: %v", err))
			return
		}
		event.Type = strings.TrimSpace(event.Type)
		if event.Type == "" {
			event.Type = "unknown"
		}
		if err := m.ledger.AppendAgentTaskEvent(context.Background(), task.ID, event.Type, event.Text, maxAgentTaskEvents, maxAgentTaskEventBytes); err != nil {
			m.finish(task.ID, AgentTaskStatusFailed, err.Error())
			return
		}
		switch event.Type {
		case "done":
			m.finish(task.ID, AgentTaskStatusCompleted, "")
			return
		case "error":
			m.finish(task.ID, AgentTaskStatusFailed, strings.TrimSpace(event.Text))
			return
		}
	}
}

func (m *TaskManager) GetTask(id string) (AgentTask, bool) {
	if m == nil || m.ledger == nil {
		return AgentTask{}, false
	}
	task, ok, err := m.ledger.GetAgentTask(context.Background(), strings.TrimSpace(id))
	if err != nil {
		if m.logger != nil {
			m.logger.Warn("Failed to read virtual computer agent task", "task_id", id, "error", err)
		}
		return AgentTask{}, false
	}
	return task, ok
}

func (m *TaskManager) ListTasks(machineID string, limit int) ([]AgentTask, error) {
	if m == nil || m.ledger == nil {
		return nil, fmt.Errorf("virtual computer task manager is unavailable")
	}
	return m.ledger.ListAgentTasks(context.Background(), machineID, limit)
}

func (m *TaskManager) CancelTask(id string) bool {
	if m == nil || m.ledger == nil {
		return false
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	m.mu.Lock()
	active := m.active[id]
	m.mu.Unlock()
	if active == nil {
		task, ok := m.GetTask(id)
		if !ok || (task.Status != AgentTaskStatusQueued && task.Status != AgentTaskStatusRunning) {
			return false
		}
	}
	if err := m.ledger.FinishAgentTask(context.Background(), id, AgentTaskStatusCanceled, "task canceled"); err != nil {
		return false
	}
	if active == nil {
		return true
	}
	active.cancel()
	m.mu.Lock()
	conn := active.conn
	m.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
	return true
}

func (m *TaskManager) finish(id, status, errText string) {
	if err := m.ledger.FinishAgentTask(context.Background(), id, status, errText); err != nil {
		if m.logger != nil {
			m.logger.Warn("Failed to finish virtual computer agent task", "task_id", id, "status", status, "error", err)
		}
		return
	}
	if status == AgentTaskStatusFailed && m.logger != nil {
		machineID := ""
		if task, ok, err := m.ledger.GetAgentTask(context.Background(), id); err == nil && ok {
			machineID = task.MachineID
		}
		m.logger.Warn("Virtual computer agent task failed", "task_id", id, "machine_id", machineID, "error", errText)
	}
}

func (m *TaskManager) Close() error {
	if m == nil {
		return nil
	}
	var closeErr error
	m.closeOnce.Do(func() {
		_ = m.ledger.InterruptActiveAgentTasks(context.Background())
		m.cancel()
		m.mu.Lock()
		connections := make([]*websocket.Conn, 0, len(m.active))
		for _, active := range m.active {
			if active.conn != nil {
				connections = append(connections, active.conn)
			}
		}
		m.mu.Unlock()
		for _, conn := range connections {
			_ = conn.Close()
		}
		m.wg.Wait()
		if m.ownsLedger {
			closeErr = m.ledger.Close()
		}
	})
	return closeErr
}

func newAgentTaskID() (string, error) {
	var data [12]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("generate virtual computer agent task id: %w", err)
	}
	return "vct-" + hex.EncodeToString(data[:]), nil
}

func contextErrorMessage(err error) string {
	if err == context.DeadlineExceeded {
		return "agent task timed out"
	}
	return "agent task canceled"
}

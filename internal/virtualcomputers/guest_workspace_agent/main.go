//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/mdlayher/vsock"
)

const (
	protocolVersion          = "aurago.workspace.v1"
	defaultVSockPort         = 7331
	workspaceRoot            = "/workspace"
	runtimeRoot              = "/run/aurago"
	chromiumProfileDir       = runtimeRoot + "/chromium"
	maxMessageBytes          = 8 * 1024 * 1024
	maxFileChunkBytes        = 4 * 1024 * 1024
	maxOutputBytes           = 4 * 1024 * 1024
	maxConfiguredOutputBytes = 64 * 1024 * 1024
	maxJobs                  = 2
	maxLegacyFiles           = 50000
	maxLegacyBytes           = 512 * 1024 * 1024
)

type rpcRequest struct {
	JSONRPC  string          `json:"jsonrpc"`
	Protocol string          `json:"protocol"`
	ID       string          `json:"id"`
	Method   string          `json:"method"`
	Deadline string          `json:"deadline,omitempty"`
	Sequence uint64          `json:"sequence"`
	Params   json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC  string      `json:"jsonrpc"`
	Protocol string      `json:"protocol"`
	ID       string      `json:"id"`
	Sequence uint64      `json:"sequence"`
	Result   interface{} `json:"result,omitempty"`
	Error    *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Message
}

type capabilities struct {
	ProtocolVersion string   `json:"protocol_version"`
	MachineID       string   `json:"machine_id"`
	InstanceNonce   string   `json:"instance_nonce"`
	Capabilities    []string `json:"capabilities"`
	MaxMessageBytes int64    `json:"max_message_bytes"`
}

type legacyImportEntry struct {
	Path           string `json:"path"`
	Kind           string `json:"kind"`
	SizeBytes      int64  `json:"size_bytes"`
	FileCount      int    `json:"file_count"`
	DirectoryCount int    `json:"directory_count"`
}

type legacyScanResult struct {
	Paths      []string            `json:"paths"`
	Entries    []legacyImportEntry `json:"entries"`
	Digest     string              `json:"digest"`
	TotalBytes int64               `json:"total_bytes"`
	FileCount  int                 `json:"file_count"`
}

type publicJob struct {
	ID              string     `json:"id"`
	WorkspaceID     string     `json:"workspace_id,omitempty"`
	Mode            string     `json:"mode"`
	State           string     `json:"state"`
	ExitCode        *int       `json:"exit_code,omitempty"`
	PID             int        `json:"pid,omitempty"`
	ProcessGroup    int        `json:"process_group,omitempty"`
	OutputCursor    int64      `json:"output_cursor,omitempty"`
	OutputTruncated bool       `json:"output_truncated,omitempty"`
	Error           string     `json:"error,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

type startJobRequest struct {
	Command                string `json:"command"`
	WorkingDir             string `json:"working_dir,omitempty"`
	PTY                    bool   `json:"pty,omitempty"`
	Rows                   uint16 `json:"rows,omitempty"`
	Cols                   uint16 `json:"cols,omitempty"`
	TimeoutSeconds         int    `json:"timeout_seconds,omitempty"`
	WaitForCredentialGrant bool   `json:"wait_for_credential_grant,omitempty"`
	MaxOutputBytes         int64  `json:"max_output_bytes,omitempty"`
}

type execRequest struct {
	Command        string            `json:"command"`
	WorkingDir     string            `json:"working_dir,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	MaxOutputBytes int64             `json:"max_output_bytes,omitempty"`
}

type outputBuffer struct {
	mu        sync.Mutex
	data      []byte
	base      int64
	truncated bool
	maxBytes  int
}

func (b *outputBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	count := len(data)
	b.data = append(b.data, data...)
	limit := b.maxBytes
	if limit <= 0 {
		limit = maxOutputBytes
	}
	if len(b.data) > limit {
		drop := len(b.data) - limit
		copy(b.data, b.data[drop:])
		b.data = b.data[:limit]
		b.base += int64(drop)
		b.truncated = true
	}
	return count, nil
}

func (b *outputBuffer) page(cursor int64, limit int) (string, int64, bool, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if limit <= 0 || limit > 64*1024 {
		limit = 64 * 1024
	}
	truncated := b.truncated || cursor < b.base
	if cursor < b.base {
		cursor = b.base
	}
	endCursor := b.base + int64(len(b.data))
	if cursor > endCursor {
		cursor = endCursor
	}
	start := int(cursor - b.base)
	end := start + limit
	if end > len(b.data) {
		end = len(b.data)
	}
	chunk := string(append([]byte(nil), b.data[start:end]...))
	next := b.base + int64(end)
	return chunk, next, next >= endCursor, truncated
}

func (b *outputBuffer) all() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(append([]byte(nil), b.data...))
}

func (b *outputBuffer) cursor() (int64, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.base + int64(len(b.data)), b.truncated
}

type job struct {
	mu          sync.Mutex
	public      publicJob
	request     startJobRequest
	cmd         *exec.Cmd
	ptyFile     *os.File
	output      outputBuffer
	done        chan struct{}
	cancel      context.CancelFunc
	environment map[string]string
	started     bool
}

func (j *job) snapshot() publicJob {
	j.mu.Lock()
	defer j.mu.Unlock()
	copy := j.public
	copy.OutputCursor, copy.OutputTruncated = j.output.cursor()
	return copy
}

type browserSession struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id,omitempty"`
	State        string    `json:"state"`
	ActivePageID string    `json:"active_page_id,omitempty"`
	URLOrigin    string    `json:"url_origin,omitempty"`
	ControlOwner string    `json:"control_owner"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type browserRequest struct {
	Operation  string                 `json:"operation"`
	SessionID  string                 `json:"session_id,omitempty"`
	PageID     string                 `json:"page_id,omitempty"`
	URL        string                 `json:"url,omitempty"`
	ElementRef string                 `json:"element_ref,omitempty"`
	Selector   string                 `json:"selector,omitempty"`
	Text       string                 `json:"text,omitempty"`
	Value      string                 `json:"value,omitempty"`
	Key        string                 `json:"key,omitempty"`
	TimeoutMS  int                    `json:"timeout_ms,omitempty"`
	FullPage   bool                   `json:"full_page,omitempty"`
	X          float64                `json:"x,omitempty"`
	Y          float64                `json:"y,omitempty"`
	DeltaX     float64                `json:"delta_x,omitempty"`
	DeltaY     float64                `json:"delta_y,omitempty"`
	ToX        float64                `json:"to_x,omitempty"`
	ToY        float64                `json:"to_y,omitempty"`
	Path       string                 `json:"path,omitempty"`
	GrantID    string                 `json:"grant_id,omitempty"`
	Fields     map[string]string      `json:"fields,omitempty"`
	Options    map[string]interface{} `json:"options,omitempty"`
}

type browserResult struct {
	Session browserSession         `json:"session,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

type browserController struct {
	mu           sync.Mutex
	allocatorCtx context.Context
	allocatorEnd context.CancelFunc
	browserCtx   context.Context
	browserEnd   context.CancelFunc
	tabCtx       context.Context
	tabEnd       context.CancelFunc
	session      browserSession
}

type service struct {
	mu            sync.Mutex
	machineID     string
	instanceNonce string
	jobs          map[string]*job
	grants        map[string]map[string]string
	browser       *browserController
}

func newService() (*service, error) {
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace root: %w", err)
	}
	if err := os.MkdirAll(runtimeRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create runtime root: %w", err)
	}
	nonce, err := randomID()
	if err != nil {
		return nil, err
	}
	return &service{
		instanceNonce: nonce,
		jobs:          make(map[string]*job),
		grants:        make(map[string]map[string]string),
		browser:       &browserController{},
	}, nil
}

func main() {
	port := flag.Uint("vsock-port", defaultVSockPort, "guest vsock port")
	listenTCP := flag.String("listen-tcp", "", "development-only TCP listen address")
	flag.Parse()
	svc, err := newService()
	if err != nil {
		log.Fatal(err)
	}
	var listener net.Listener
	if strings.TrimSpace(*listenTCP) != "" {
		listener, err = net.Listen("tcp", strings.TrimSpace(*listenTCP))
	} else {
		listener, err = vsock.Listen(uint32(*port), nil)
	}
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", svc.handleHealth)
	mux.HandleFunc("/", svc.handleRPC)
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    16 * 1024,
	}
	log.Printf("aurago-workspace-agent listening on %s", listener.Addr())
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func (s *service) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "protocol_version": protocolVersion})
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
	CheckOrigin:     func(*http.Request) bool { return true },
}

func (s *service) handleRPC(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(maxMessageBytes)
	var lastSequence uint64
	for {
		var request rpcRequest
		if err := conn.ReadJSON(&request); err != nil {
			return
		}
		response := rpcResponse{JSONRPC: "2.0", Protocol: protocolVersion, ID: request.ID, Sequence: request.Sequence}
		if request.JSONRPC != "2.0" || request.Protocol != protocolVersion {
			response.Error = &rpcError{Code: "workspace_protocol_mismatch", Message: "unsupported workspace protocol"}
		} else if request.ID == "" || request.Method == "" {
			response.Error = &rpcError{Code: "invalid_request", Message: "request id and method are required"}
		} else if request.Sequence <= lastSequence {
			response.Error = &rpcError{Code: "invalid_sequence", Message: "request sequence must increase monotonically"}
		} else {
			lastSequence = request.Sequence
			ctx := r.Context()
			if request.Deadline != "" {
				deadline, err := time.Parse(time.RFC3339Nano, request.Deadline)
				if err != nil {
					response.Error = &rpcError{Code: "invalid_deadline", Message: "deadline must be RFC3339Nano"}
				} else {
					var cancel context.CancelFunc
					ctx, cancel = context.WithDeadline(ctx, deadline)
					response.Result, response.Error = s.dispatch(ctx, request.Method, request.Params)
					cancel()
				}
			} else {
				response.Result, response.Error = s.dispatch(ctx, request.Method, request.Params)
			}
		}
		if err := conn.WriteJSON(response); err != nil {
			return
		}
	}
}

func (s *service) dispatch(ctx context.Context, method string, raw json.RawMessage) (interface{}, *rpcError) {
	switch method {
	case "system.capabilities":
		var request struct {
			ProtocolVersion string `json:"protocol_version"`
			MachineID       string `json:"machine_id"`
			InstanceNonce   string `json:"instance_nonce"`
		}
		if err := decodeParams(raw, &request); err != nil {
			return nil, invalidParams(err)
		}
		if request.ProtocolVersion != protocolVersion || strings.TrimSpace(request.MachineID) == "" {
			return nil, &rpcError{Code: "workspace_protocol_mismatch", Message: "protocol version and machine id are required"}
		}
		s.mu.Lock()
		if s.machineID != "" && s.machineID != request.MachineID {
			s.mu.Unlock()
			s.resetRuntime("machine binding changed")
			s.mu.Lock()
		}
		s.machineID = request.MachineID
		result := capabilities{
			ProtocolVersion: protocolVersion, MachineID: s.machineID, InstanceNonce: s.instanceNonce,
			Capabilities:    []string{"shell.exec", "job.pty", "job.stream", "file.workspace", "browser.headful", "credential.runtime", "checkpoint.workspace_v2", "volume.legacy_import"},
			MaxMessageBytes: maxMessageBytes,
		}
		s.mu.Unlock()
		return result, nil
	case "workspace.close":
		s.resetRuntime("workspace closed")
		return map[string]bool{"closed": true}, nil
	case "workspace.prepare_checkpoint":
		return map[string]interface{}{"ready": true, "include": []string{workspaceRoot}}, nil
	case "volume.legacy_scan":
		var request struct {
			Paths []string `json:"paths"`
		}
		if err := decodeParams(raw, &request); err != nil {
			return nil, invalidParams(err)
		}
		result, err := scanLegacySelection(request.Paths)
		if err != nil {
			return nil, toRPCError(err)
		}
		return result, nil
	case "volume.legacy_import":
		var request struct {
			Paths          []string `json:"paths"`
			ExpectedDigest string   `json:"expected_digest"`
		}
		if err := decodeParams(raw, &request); err != nil {
			return nil, invalidParams(err)
		}
		result, err := importLegacySelection(request.Paths, request.ExpectedDigest)
		if err != nil {
			return nil, toRPCError(err)
		}
		return result, nil
	case "shell.exec":
		var request execRequest
		if err := decodeParams(raw, &request); err != nil {
			return nil, invalidParams(err)
		}
		return s.execSync(ctx, request)
	case "job.start":
		var request startJobRequest
		if err := decodeParams(raw, &request); err != nil {
			return nil, invalidParams(err)
		}
		created, err := s.createJob(request, nil)
		if err != nil {
			return nil, toRPCError(err)
		}
		return created.snapshot(), nil
	case "job.status":
		var request struct {
			JobID string `json:"job_id"`
		}
		if err := decodeParams(raw, &request); err != nil {
			return nil, invalidParams(err)
		}
		stored, err := s.getJob(request.JobID)
		if err != nil {
			return nil, toRPCError(err)
		}
		return stored.snapshot(), nil
	case "job.output":
		var request struct {
			JobID  string `json:"job_id"`
			Cursor int64  `json:"cursor"`
			Limit  int    `json:"limit"`
		}
		if err := decodeParams(raw, &request); err != nil {
			return nil, invalidParams(err)
		}
		stored, err := s.getJob(request.JobID)
		if err != nil {
			return nil, toRPCError(err)
		}
		data, next, eof, truncated := stored.output.page(request.Cursor, request.Limit)
		return map[string]interface{}{"job_id": stored.public.ID, "data": data, "cursor": request.Cursor, "next_cursor": next, "eof": eof, "truncated": truncated}, nil
	case "job.input":
		var request struct {
			JobID string `json:"job_id"`
			Input string `json:"input"`
			Rows  uint16 `json:"rows"`
			Cols  uint16 `json:"cols"`
		}
		if err := decodeParams(raw, &request); err != nil {
			return nil, invalidParams(err)
		}
		if err := s.inputJob(request.JobID, request.Input, request.Rows, request.Cols); err != nil {
			return nil, toRPCError(err)
		}
		return map[string]bool{"accepted": true}, nil
	case "job.cancel":
		var request struct {
			JobID string `json:"job_id"`
		}
		if err := decodeParams(raw, &request); err != nil {
			return nil, invalidParams(err)
		}
		if err := s.cancelJob(request.JobID); err != nil {
			return nil, toRPCError(err)
		}
		return map[string]bool{"canceled": true}, nil
	case "file.list":
		var request struct {
			Path string `json:"path"`
		}
		if err := decodeParams(raw, &request); err != nil {
			return nil, invalidParams(err)
		}
		entries, err := listWorkspaceFiles(request.Path)
		if err != nil {
			return nil, toRPCError(err)
		}
		return entries, nil
	case "file.read":
		var request struct {
			Path   string `json:"path"`
			Offset int64  `json:"offset"`
			Limit  int64  `json:"limit"`
		}
		if err := decodeParams(raw, &request); err != nil {
			return nil, invalidParams(err)
		}
		result, err := readWorkspaceFile(request.Path, request.Offset, request.Limit)
		if err != nil {
			return nil, toRPCError(err)
		}
		return result, nil
	case "file.write":
		var request struct {
			Path       string `json:"path"`
			DataBase64 string `json:"data_base64"`
			Append     bool   `json:"append"`
		}
		if err := decodeParams(raw, &request); err != nil {
			return nil, invalidParams(err)
		}
		if err := writeWorkspaceFile(request.Path, request.DataBase64, request.Append); err != nil {
			return nil, toRPCError(err)
		}
		return map[string]bool{"written": true}, nil
	case "credential.activate":
		var request struct {
			GrantID        string            `json:"grant_id"`
			CredentialName string            `json:"credential_name"`
			UsageType      string            `json:"usage_type"`
			JobID          string            `json:"job_id"`
			Values         map[string]string `json:"values"`
			ExpiresAt      time.Time         `json:"expires_at"`
		}
		if err := decodeParams(raw, &request); err != nil {
			return nil, invalidParams(err)
		}
		if err := s.activateCredential(request.GrantID, request.CredentialName, request.UsageType, request.JobID, request.Values, request.ExpiresAt); err != nil {
			return nil, toRPCError(err)
		}
		return map[string]bool{"active": true}, nil
	case "credential.revoke":
		var request struct {
			GrantID string `json:"grant_id"`
		}
		if err := decodeParams(raw, &request); err != nil {
			return nil, invalidParams(err)
		}
		s.revokeCredential(request.GrantID)
		return map[string]bool{"revoked": true}, nil
	default:
		if strings.HasPrefix(method, "browser.") {
			var request browserRequest
			if err := decodeParams(raw, &request); err != nil {
				return nil, invalidParams(err)
			}
			request.Operation = strings.TrimPrefix(method, "browser.")
			result, err := s.browser.perform(ctx, request)
			clearMap(request.Fields)
			if err != nil {
				return nil, toRPCError(err)
			}
			return result, nil
		}
		return nil, &rpcError{Code: "method_not_found", Message: "unsupported workspace method"}
	}
}

func (s *service) execSync(ctx context.Context, request execRequest) (interface{}, *rpcError) {
	if request.TimeoutSeconds <= 0 || request.TimeoutSeconds > 3600 {
		request.TimeoutSeconds = 3600
	}
	jobRequest := startJobRequest{Command: request.Command, WorkingDir: request.WorkingDir, TimeoutSeconds: request.TimeoutSeconds, MaxOutputBytes: request.MaxOutputBytes}
	created, err := s.createJob(jobRequest, request.Environment)
	if err != nil {
		return nil, toRPCError(err)
	}
	select {
	case <-created.done:
		return map[string]interface{}{"job": created.snapshot(), "output": created.output.all()}, nil
	case <-ctx.Done():
		_ = s.cancelJob(created.public.ID)
		return nil, &rpcError{Code: "deadline_exceeded", Message: ctx.Err().Error()}
	}
}

func (s *service) createJob(request startJobRequest, environment map[string]string) (*job, error) {
	if strings.TrimSpace(request.Command) == "" {
		return nil, &rpcError{Code: "invalid_argument", Message: "command is required"}
	}
	if request.TimeoutSeconds <= 0 || request.TimeoutSeconds > 3600 {
		request.TimeoutSeconds = 3600
	}
	if request.Rows == 0 {
		request.Rows = 24
	}
	if request.Cols == 0 {
		request.Cols = 80
	}
	if request.MaxOutputBytes <= 0 || request.MaxOutputBytes > maxConfiguredOutputBytes {
		request.MaxOutputBytes = maxOutputBytes
	}
	s.mu.Lock()
	running := 0
	for _, existing := range s.jobs {
		state := existing.snapshot().State
		if state == "queued" || state == "running" {
			running++
		}
	}
	if running >= maxJobs {
		s.mu.Unlock()
		return nil, &rpcError{Code: "job_limit_reached", Message: "maximum parallel jobs reached"}
	}
	id, err := randomID()
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	now := time.Now().UTC()
	created := &job{
		public:  publicJob{ID: id, Mode: "sync", State: "queued", CreatedAt: now, UpdatedAt: now},
		request: request, done: make(chan struct{}), environment: cloneMap(environment), output: outputBuffer{maxBytes: int(request.MaxOutputBytes)},
	}
	if request.PTY {
		created.public.Mode = "pty"
	}
	s.jobs[id] = created
	s.mu.Unlock()
	if !request.WaitForCredentialGrant {
		if err := s.launchJob(created); err != nil {
			return nil, err
		}
	}
	return created, nil
}

func (s *service) launchJob(stored *job) error {
	stored.mu.Lock()
	if stored.started {
		stored.mu.Unlock()
		return nil
	}
	stored.started = true
	request := stored.request
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(request.TimeoutSeconds)*time.Second)
	stored.cancel = cancel
	command := exec.CommandContext(ctx, "/bin/bash", "--noprofile", "--norc", "-lc", request.Command)
	workingDir := strings.TrimSpace(request.WorkingDir)
	if workingDir == "" {
		workingDir = workspaceRoot
	}
	command.Dir = workingDir
	command.Env = append(os.Environ(), "HISTFILE=/dev/null", "HISTCONTROL=ignorespace", "AURAGO_WORKSPACE=1")
	for key, value := range stored.environment {
		command.Env = append(command.Env, key+"="+value)
	}
	stored.cmd = command
	stored.mu.Unlock()

	var startErr error
	if request.PTY {
		stored.ptyFile, startErr = pty.StartWithSize(command, &pty.Winsize{Rows: request.Rows, Cols: request.Cols})
		if startErr == nil {
			go func() { _, _ = io.Copy(&stored.output, stored.ptyFile) }()
		}
	} else {
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		command.Stdout = &stored.output
		command.Stderr = &stored.output
		startErr = command.Start()
	}
	if startErr != nil {
		cancel()
		stored.mu.Lock()
		clearMap(stored.environment)
		stored.environment = nil
		clearStrings(command.Env)
		command.Env = nil
		stored.public.State = "failed"
		stored.public.Error = startErr.Error()
		now := time.Now().UTC()
		stored.public.UpdatedAt = now
		stored.public.CompletedAt = &now
		stored.mu.Unlock()
		close(stored.done)
		return fmt.Errorf("start job: %w", startErr)
	}
	now := time.Now().UTC()
	stored.mu.Lock()
	stored.public.State = "running"
	stored.public.PID = command.Process.Pid
	stored.public.ProcessGroup = command.Process.Pid
	stored.public.StartedAt = &now
	stored.public.UpdatedAt = now
	clearMap(stored.environment)
	stored.environment = nil
	clearStrings(command.Env)
	command.Env = nil
	stored.mu.Unlock()
	go s.waitJob(ctx, stored)
	return nil
}

func (s *service) waitJob(ctx context.Context, stored *job) {
	err := stored.cmd.Wait()
	if stored.ptyFile != nil {
		_ = stored.ptyFile.Close()
	}
	now := time.Now().UTC()
	stored.mu.Lock()
	defer stored.mu.Unlock()
	stored.public.UpdatedAt = now
	stored.public.CompletedAt = &now
	if stored.public.State == "canceled" {
		close(stored.done)
		return
	}
	if err == nil {
		code := 0
		stored.public.ExitCode = &code
		stored.public.State = "completed"
	} else {
		code := -1
		if exitErr := new(exec.ExitError); errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		}
		stored.public.ExitCode = &code
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			stored.public.State = "failed"
			stored.public.Error = "job deadline exceeded"
		} else {
			stored.public.State = "failed"
			stored.public.Error = err.Error()
		}
	}
	close(stored.done)
}

func (s *service) getJob(id string) (*job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.jobs[strings.TrimSpace(id)]
	if stored == nil {
		return nil, &rpcError{Code: "not_found", Message: "job was not found"}
	}
	return stored, nil
}

func (s *service) inputJob(id, input string, rows, cols uint16) error {
	stored, err := s.getJob(id)
	if err != nil {
		return err
	}
	stored.mu.Lock()
	defer stored.mu.Unlock()
	if stored.public.State != "running" || stored.ptyFile == nil {
		return &rpcError{Code: "job_not_interactive", Message: "job is not a running PTY"}
	}
	if rows > 0 && cols > 0 {
		if err := pty.Setsize(stored.ptyFile, &pty.Winsize{Rows: rows, Cols: cols}); err != nil {
			return fmt.Errorf("resize PTY: %w", err)
		}
	}
	if input != "" {
		if _, err := io.WriteString(stored.ptyFile, input); err != nil {
			return fmt.Errorf("write PTY input: %w", err)
		}
	}
	return nil
}

func (s *service) cancelJob(id string) error {
	stored, err := s.getJob(id)
	if err != nil {
		return err
	}
	stored.mu.Lock()
	if stored.public.State == "completed" || stored.public.State == "failed" || stored.public.State == "canceled" {
		stored.mu.Unlock()
		return nil
	}
	stored.public.State = "canceled"
	stored.public.UpdatedAt = time.Now().UTC()
	process := stored.cmd
	cancel := stored.cancel
	started := stored.started
	stored.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if started && process != nil && process.Process != nil {
		_ = syscall.Kill(-process.Process.Pid, syscall.SIGTERM)
		time.AfterFunc(2*time.Second, func() {
			select {
			case <-stored.done:
				return
			default:
				_ = syscall.Kill(-process.Process.Pid, syscall.SIGKILL)
			}
		})
	} else {
		now := time.Now().UTC()
		stored.mu.Lock()
		stored.public.CompletedAt = &now
		stored.mu.Unlock()
		close(stored.done)
	}
	return nil
}

func (s *service) activateCredential(grantID, credentialName, usageType, jobID string, values map[string]string, expiresAt time.Time) error {
	if strings.TrimSpace(grantID) == "" || !expiresAt.After(time.Now()) {
		return &rpcError{Code: "invalid_credential_grant", Message: "grant id and future expiry are required"}
	}
	if usageType != "shell" {
		return &rpcError{Code: "invalid_credential_grant", Message: "only shell grants are activated as job environment"}
	}
	stored, err := s.getJob(jobID)
	if err != nil {
		return err
	}
	stored.mu.Lock()
	if stored.started || stored.public.State != "queued" {
		stored.mu.Unlock()
		return &rpcError{Code: "job_already_started", Message: "credential-bound job must be queued with wait_for_credential_grant"}
	}
	prefix := "AURAGO_CRED_" + safeEnvironmentName(credentialName)
	for field, value := range values {
		stored.environment[prefix+"_"+safeEnvironmentName(field)] = value
	}
	stored.mu.Unlock()
	s.mu.Lock()
	s.grants[grantID] = map[string]string{}
	s.mu.Unlock()
	clearMap(values)
	return s.launchJob(stored)
}

func (s *service) revokeCredential(grantID string) {
	s.mu.Lock()
	values := s.grants[strings.TrimSpace(grantID)]
	delete(s.grants, strings.TrimSpace(grantID))
	s.mu.Unlock()
	clearMap(values)
}

func (s *service) resetRuntime(reason string) {
	s.mu.Lock()
	jobs := make([]*job, 0, len(s.jobs))
	for _, stored := range s.jobs {
		jobs = append(jobs, stored)
	}
	grants := s.grants
	s.jobs = make(map[string]*job)
	s.grants = make(map[string]map[string]string)
	s.machineID = ""
	nonce, _ := randomID()
	s.instanceNonce = nonce
	s.mu.Unlock()
	for _, stored := range jobs {
		_ = s.cancelJobDetached(stored, reason)
	}
	for _, values := range grants {
		clearMap(values)
	}
	s.browser.close()
	_ = os.RemoveAll(chromiumProfileDir)
}

func (s *service) cancelJobDetached(stored *job, reason string) error {
	stored.mu.Lock()
	if stored.cancel != nil {
		stored.cancel()
	}
	if stored.cmd != nil && stored.cmd.Process != nil {
		_ = syscall.Kill(-stored.cmd.Process.Pid, syscall.SIGKILL)
	}
	stored.public.State = "interrupted"
	stored.public.Error = reason
	stored.public.UpdatedAt = time.Now().UTC()
	stored.mu.Unlock()
	return nil
}

func scanLegacySelection(requested []string) (legacyScanResult, error) {
	paths, err := normalizeLegacyPaths(requested)
	if err != nil {
		return legacyScanResult{}, err
	}
	result := legacyScanResult{Paths: paths, Entries: make([]legacyImportEntry, 0, len(paths))}
	digest := sha256.New()
	for _, relative := range paths {
		root := filepath.Join("/root", filepath.FromSlash(relative))
		rootInfo, err := os.Lstat(root)
		if err != nil {
			if os.IsNotExist(err) {
				return legacyScanResult{}, &rpcError{Code: "legacy_path_not_found", Message: "selected legacy path was not found: " + relative}
			}
			return legacyScanResult{}, fmt.Errorf("inspect selected legacy path %q: %w", relative, err)
		}
		entry := legacyImportEntry{Path: relative, Kind: "file"}
		if rootInfo.IsDir() {
			entry.Kind = "directory"
		}
		err = filepath.Walk(root, func(current string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			pathRelative, err := filepath.Rel("/root", current)
			if err != nil {
				return err
			}
			pathRelative = filepath.ToSlash(pathRelative)
			if isSensitiveLegacyPath(pathRelative) {
				return &rpcError{Code: "legacy_secret_material", Message: "selected path contains credential material that cannot be imported: " + pathRelative}
			}
			mode := info.Mode()
			if mode&os.ModeSymlink != 0 || (!mode.IsRegular() && !mode.IsDir()) {
				return &rpcError{Code: "legacy_unsupported_file", Message: "selected path contains a link or special file: " + pathRelative}
			}
			_, _ = fmt.Fprintf(digest, "%s\x00%d\x00%d\x00%d\x00", pathRelative, uint32(mode.Perm()), info.Size(), info.ModTime().UnixNano())
			if mode.IsDir() {
				entry.DirectoryCount++
				return nil
			}
			entry.FileCount++
			entry.SizeBytes += info.Size()
			result.FileCount++
			result.TotalBytes += info.Size()
			if result.FileCount > maxLegacyFiles || result.TotalBytes > maxLegacyBytes {
				return &rpcError{Code: "legacy_import_too_large", Message: "legacy import exceeds the 50000 file or 512 MiB safety limit"}
			}
			return hashLegacyFile(digest, current, pathRelative)
		})
		if err != nil {
			return legacyScanResult{}, err
		}
		result.Entries = append(result.Entries, entry)
	}
	result.Digest = hex.EncodeToString(digest.Sum(nil))
	return result, nil
}

func normalizeLegacyPaths(requested []string) ([]string, error) {
	if len(requested) == 0 || len(requested) > 32 {
		return nil, &rpcError{Code: "invalid_legacy_paths", Message: "select between 1 and 32 legacy paths"}
	}
	seen := make(map[string]struct{}, len(requested))
	paths := make([]string, 0, len(requested))
	for _, raw := range requested {
		raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
		for _, component := range strings.Split(raw, "/") {
			if component == ".." {
				return nil, &rpcError{Code: "invalid_legacy_path", Message: "legacy paths must not contain parent traversal"}
			}
		}
		cleaned := filepath.ToSlash(filepath.Clean("/" + raw))
		cleaned = strings.TrimPrefix(cleaned, "/")
		if raw == "" || cleaned == "." || cleaned == "" || strings.HasPrefix(raw, "/") || strings.Contains(raw, "\x00") {
			return nil, &rpcError{Code: "invalid_legacy_path", Message: "legacy paths must be relative to /root"}
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		paths = append(paths, cleaned)
	}
	sort.Strings(paths)
	for index, candidate := range paths {
		for _, parent := range paths[:index] {
			if strings.HasPrefix(candidate, parent+"/") {
				return nil, &rpcError{Code: "overlapping_legacy_paths", Message: "legacy path selections must not overlap"}
			}
		}
	}
	return paths, nil
}

func isSensitiveLegacyPath(relative string) bool {
	lower := strings.ToLower("/" + filepath.ToSlash(relative))
	for _, component := range []string{"/.ssh/", "/.gnupg/", "/.aws/", "/.azure/", "/.kube/", "/.docker/"} {
		if strings.Contains(lower+"/", component) {
			return true
		}
	}
	base := strings.ToLower(filepath.Base(relative))
	if base == ".env" || strings.HasPrefix(base, ".env.") || base == ".netrc" || base == ".npmrc" || base == ".pypirc" || base == ".git-credentials" || base == "credentials.json" {
		return true
	}
	return strings.HasPrefix(base, "id_rsa") || strings.HasPrefix(base, "id_ed25519") || strings.HasSuffix(base, ".key") || strings.HasSuffix(base, ".p12") || strings.HasSuffix(base, ".pfx")
}

func hashLegacyFile(digest io.Writer, absolute, relative string) error {
	file, err := os.Open(absolute)
	if err != nil {
		return fmt.Errorf("open legacy file %q: %w", relative, err)
	}
	defer file.Close()
	buffer := make([]byte, 64*1024)
	previous := make([]byte, 0, 64)
	text := true
	for {
		count, readErr := file.Read(buffer)
		if count > 0 {
			chunk := buffer[:count]
			if _, err := digest.Write(chunk); err != nil {
				return err
			}
			if text {
				probe := append(append([]byte(nil), previous...), chunk...)
				if bytes.IndexByte(probe, 0) >= 0 || !utf8.Valid(probe) {
					text = false
				} else if containsCredentialMarker(probe) {
					return &rpcError{Code: "legacy_secret_material", Message: "selected file appears to contain credential material and cannot be imported: " + relative}
				}
				if len(probe) > 64 {
					previous = append(previous[:0], probe[len(probe)-64:]...)
				} else {
					previous = append(previous[:0], probe...)
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("read legacy file %q: %w", relative, readErr)
		}
	}
}

func containsCredentialMarker(data []byte) bool {
	lower := bytes.ToLower(data)
	for _, marker := range [][]byte{
		[]byte("-----begin private key-----"), []byte("-----begin rsa private key-----"),
		[]byte("-----begin openssh private key-----"), []byte("password="), []byte("password:"),
		[]byte("api_key="), []byte("api-key:"), []byte("access_token="), []byte("secret_key="),
	} {
		if bytes.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func importLegacySelection(paths []string, expectedDigest string) (legacyScanResult, error) {
	result, err := scanLegacySelection(paths)
	if err != nil {
		return legacyScanResult{}, err
	}
	if expectedDigest == "" || !strings.EqualFold(result.Digest, strings.TrimSpace(expectedDigest)) {
		return legacyScanResult{}, &rpcError{Code: "legacy_import_changed", Message: "legacy files changed after preview; scan and approve again"}
	}
	if err := os.RemoveAll(workspaceRoot); err != nil {
		return legacyScanResult{}, fmt.Errorf("reset workspace import target: %w", err)
	}
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		return legacyScanResult{}, fmt.Errorf("create workspace import target: %w", err)
	}
	for _, relative := range result.Paths {
		if err := copyLegacyPath(filepath.Join("/root", filepath.FromSlash(relative)), filepath.Join(workspaceRoot, filepath.FromSlash(relative))); err != nil {
			_ = os.RemoveAll(workspaceRoot)
			_ = os.MkdirAll(workspaceRoot, 0o755)
			return legacyScanResult{}, err
		}
	}
	return result, nil
}

func copyLegacyPath(source, destination string) error {
	return filepath.Walk(source, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, current)
		if err != nil {
			return err
		}
		target := destination
		if relative != "." {
			target = filepath.Join(destination, relative)
		}
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		input, err := os.Open(current)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			_ = input.Close()
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputCloseErr := input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputCloseErr != nil {
			return inputCloseErr
		}
		return closeErr
	})
}

func listWorkspaceFiles(relative string) ([]map[string]interface{}, error) {
	resolved, err := secureWorkspacePath(relative, true)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, fmt.Errorf("list workspace path: %w", err)
	}
	result := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		entryPath, _ := filepath.Rel(workspaceRoot, filepath.Join(resolved, entry.Name()))
		result = append(result, map[string]interface{}{
			"path": filepath.ToSlash(entryPath), "name": entry.Name(), "directory": entry.IsDir(),
			"size": info.Size(), "modified_at": info.ModTime().UTC(),
		})
	}
	sort.Slice(result, func(i, j int) bool { return fmt.Sprint(result[i]["name"]) < fmt.Sprint(result[j]["name"]) })
	return result, nil
}

func readWorkspaceFile(relative string, offset, limit int64) (map[string]interface{}, error) {
	resolved, err := secureWorkspacePath(relative, true)
	if err != nil {
		return nil, err
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > maxFileChunkBytes {
		limit = maxFileChunkBytes
	}
	file, err := os.Open(resolved)
	if err != nil {
		return nil, fmt.Errorf("open workspace file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat workspace file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, &rpcError{Code: "invalid_file", Message: "path is not a regular file"}
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek workspace file: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		return nil, fmt.Errorf("read workspace file: %w", err)
	}
	return map[string]interface{}{"data_base64": base64.StdEncoding.EncodeToString(data), "eof": offset+int64(len(data)) >= info.Size()}, nil
}

func writeWorkspaceFile(relative, encoded string, appendMode bool) error {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return &rpcError{Code: "invalid_base64", Message: "file data is not valid base64"}
	}
	if len(data) > maxFileChunkBytes {
		return &rpcError{Code: "message_too_large", Message: "file write exceeds 4 MiB"}
	}
	resolved, err := secureWorkspacePath(relative, false)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return fmt.Errorf("create workspace directory: %w", err)
	}
	flags := os.O_CREATE | os.O_WRONLY
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(resolved, flags, 0o600)
	if err != nil {
		return fmt.Errorf("open workspace file for write: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write workspace file: %w", err)
	}
	return file.Sync()
}

func secureWorkspacePath(relative string, mustExist bool) (string, error) {
	relative = filepath.Clean(strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(relative), "\\", "/"), "/"))
	if relative == "." {
		relative = ""
	}
	candidate := filepath.Join(workspaceRoot, relative)
	rootWithSeparator := workspaceRoot + string(os.PathSeparator)
	if candidate != workspaceRoot && !strings.HasPrefix(candidate, rootWithSeparator) {
		return "", &rpcError{Code: "path_outside_workspace", Message: "file operations are restricted to /workspace"}
	}
	probe := candidate
	if !mustExist {
		probe = filepath.Dir(candidate)
	}
	for {
		resolved, err := filepath.EvalSymlinks(probe)
		if err == nil {
			if resolved != workspaceRoot && !strings.HasPrefix(resolved, rootWithSeparator) {
				return "", &rpcError{Code: "path_outside_workspace", Message: "symlink escapes /workspace"}
			}
			break
		}
		if mustExist || !os.IsNotExist(err) {
			return "", fmt.Errorf("resolve workspace path: %w", err)
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return "", fmt.Errorf("resolve workspace path: %w", err)
		}
		probe = parent
	}
	return candidate, nil
}

func (b *browserController) perform(ctx context.Context, request browserRequest) (browserResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if request.TimeoutMS <= 0 || request.TimeoutMS > 120000 {
		request.TimeoutMS = 30000
	}
	if request.Operation == "open" {
		if b.session.State == "open" {
			return browserResult{Session: b.session}, nil
		}
		if err := b.openLocked(ctx, request); err != nil {
			return browserResult{}, err
		}
		return browserResult{Session: b.session}, nil
	}
	if b.session.State != "open" || b.tabCtx == nil {
		return browserResult{}, &rpcError{Code: "browser_not_open", Message: "browser session is not open"}
	}
	if request.SessionID != "" && request.SessionID != b.session.ID {
		return browserResult{}, &rpcError{Code: "not_found", Message: "browser session was not found"}
	}
	actionCtx, cancel := context.WithTimeout(b.tabCtx, time.Duration(request.TimeoutMS)*time.Millisecond)
	defer cancel()
	data := make(map[string]interface{})
	selector := browserSelector(request)
	switch request.Operation {
	case "list_tabs":
		tabs, err := b.tabsLocked(actionCtx)
		if err != nil {
			return browserResult{}, err
		}
		data["tabs"] = tabs
	case "switch_tab":
		if strings.TrimSpace(request.PageID) == "" {
			return browserResult{}, &rpcError{Code: "invalid_argument", Message: "page_id is required"}
		}
		if b.tabEnd != nil {
			b.tabEnd()
		}
		b.tabCtx, b.tabEnd = chromedp.NewContext(b.browserCtx, chromedp.WithTargetID(target.ID(request.PageID)))
		if err := chromedp.Run(b.tabCtx); err != nil {
			return browserResult{}, fmt.Errorf("switch browser tab: %w", err)
		}
		b.session.ActivePageID = request.PageID
	case "navigate":
		if err := validateBrowserURL(request.URL); err != nil {
			return browserResult{}, err
		}
		if err := chromedp.Run(actionCtx, chromedp.Navigate(request.URL)); err != nil {
			return browserResult{}, fmt.Errorf("navigate browser: %w", err)
		}
		b.session.URLOrigin = origin(request.URL)
		data["url"] = request.URL
	case "inspect":
		var inspected map[string]interface{}
		if err := chromedp.Run(actionCtx, chromedp.Evaluate(inspectScript, &inspected)); err != nil {
			return browserResult{}, fmt.Errorf("inspect browser page: %w", err)
		}
		data = inspected
	case "click":
		if selector == "" {
			return browserResult{}, &rpcError{Code: "invalid_argument", Message: "element_ref or selector is required"}
		}
		if err := chromedp.Run(actionCtx, chromedp.Click(selector, chromedp.ByQuery)); err != nil {
			return browserResult{}, fmt.Errorf("click browser element: %w", err)
		}
	case "type":
		if selector == "" {
			return browserResult{}, &rpcError{Code: "invalid_argument", Message: "element_ref or selector is required"}
		}
		if err := chromedp.Run(actionCtx, chromedp.Focus(selector, chromedp.ByQuery), chromedp.SetValue(selector, "", chromedp.ByQuery), chromedp.SendKeys(selector, request.Text, chromedp.ByQuery)); err != nil {
			return browserResult{}, fmt.Errorf("type browser text: %w", err)
		}
	case "select":
		if selector == "" {
			return browserResult{}, &rpcError{Code: "invalid_argument", Message: "element_ref or selector is required"}
		}
		if err := chromedp.Run(actionCtx, chromedp.SetValue(selector, request.Value, chromedp.ByQuery)); err != nil {
			return browserResult{}, fmt.Errorf("select browser option: %w", err)
		}
	case "press":
		if request.Key == "" {
			return browserResult{}, &rpcError{Code: "invalid_argument", Message: "key is required"}
		}
		if err := chromedp.Run(actionCtx, chromedp.KeyEvent(request.Key)); err != nil {
			return browserResult{}, fmt.Errorf("press browser key: %w", err)
		}
	case "wait":
		if selector == "" {
			if err := chromedp.Run(actionCtx, chromedp.Sleep(250*time.Millisecond)); err != nil {
				return browserResult{}, err
			}
		} else if err := chromedp.Run(actionCtx, chromedp.WaitVisible(selector, chromedp.ByQuery)); err != nil {
			return browserResult{}, fmt.Errorf("wait for browser element: %w", err)
		}
	case "click_xy":
		if err := chromedp.Run(actionCtx, chromedp.Evaluate(fmt.Sprintf(`document.elementFromPoint(%f,%f)?.click()`, request.X, request.Y), nil)); err != nil {
			return browserResult{}, fmt.Errorf("click browser coordinates: %w", err)
		}
	case "scroll":
		if err := chromedp.Run(actionCtx, chromedp.Evaluate(fmt.Sprintf(`window.scrollBy(%f,%f)`, request.DeltaX, request.DeltaY), nil)); err != nil {
			return browserResult{}, fmt.Errorf("scroll browser page: %w", err)
		}
	case "drag":
		script := fmt.Sprintf(`(()=>{const e=document.elementFromPoint(%f,%f);if(!e)return false;const fire=(t,x,y)=>e.dispatchEvent(new MouseEvent(t,{bubbles:true,clientX:x,clientY:y,buttons:1}));fire('mousedown',%f,%f);fire('mousemove',%f,%f);fire('mouseup',%f,%f);return true})()`, request.X, request.Y, request.X, request.Y, request.ToX, request.ToY, request.ToX, request.ToY)
		var dragged bool
		if err := chromedp.Run(actionCtx, chromedp.Evaluate(script, &dragged)); err != nil {
			return browserResult{}, fmt.Errorf("drag browser element: %w", err)
		}
		if !dragged {
			return browserResult{}, &rpcError{Code: "element_not_found", Message: "no draggable element was found at the requested coordinates"}
		}
	case "screenshot":
		var image []byte
		var action chromedp.Action
		if request.FullPage {
			action = chromedp.FullScreenshot(&image, 90)
		} else {
			action = chromedp.CaptureScreenshot(&image)
		}
		if err := chromedp.Run(actionCtx, action); err != nil {
			return browserResult{}, fmt.Errorf("capture browser screenshot: %w", err)
		}
		if request.Path != "" {
			resolved, err := secureWorkspacePath(request.Path, false)
			if err != nil {
				return browserResult{}, err
			}
			if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
				return browserResult{}, err
			}
			if err := os.WriteFile(resolved, image, 0o600); err != nil {
				return browserResult{}, fmt.Errorf("write browser screenshot: %w", err)
			}
			data["path"] = request.Path
		} else {
			data["data_base64"] = base64.StdEncoding.EncodeToString(image)
		}
		data["mime_type"] = "image/png"
	case "upload_file":
		if selector == "" || request.Path == "" {
			return browserResult{}, &rpcError{Code: "invalid_argument", Message: "selector and path are required"}
		}
		resolved, err := secureWorkspacePath(request.Path, true)
		if err != nil {
			return browserResult{}, err
		}
		if err := chromedp.Run(actionCtx, chromedp.SetUploadFiles(selector, []string{resolved}, chromedp.ByQuery)); err != nil {
			return browserResult{}, fmt.Errorf("upload browser file: %w", err)
		}
	case "list_downloads":
		downloads, err := listDownloads()
		if err != nil {
			return browserResult{}, err
		}
		data["downloads"] = downloads
	case "credential_fill":
		var liveOrigin string
		if err := chromedp.Run(actionCtx, chromedp.Evaluate(`window.location.origin`, &liveOrigin)); err != nil {
			return browserResult{}, fmt.Errorf("read current browser origin: %w", err)
		}
		if expected := origin(request.URL); expected == "" || expected != liveOrigin {
			return browserResult{}, &rpcError{Code: "credential_origin_mismatch", Message: "current page origin does not match the grant"}
		}
		if err := fillCredential(actionCtx, request.Fields, request.Options); err != nil {
			return browserResult{}, err
		}
		data["filled_fields"] = sortedKeys(request.Fields)
	case "close":
		b.closeLocked()
		return browserResult{Session: b.session}, nil
	default:
		return browserResult{}, &rpcError{Code: "method_not_found", Message: "unsupported browser operation"}
	}
	var liveURL string
	locationCtx, locationCancel := context.WithTimeout(b.tabCtx, 5*time.Second)
	if err := chromedp.Run(locationCtx, chromedp.Location(&liveURL)); err == nil {
		b.session.URLOrigin = origin(liveURL)
		data["url"] = liveURL
	}
	locationCancel()
	b.session.UpdatedAt = time.Now().UTC()
	return browserResult{Session: b.session, Data: data}, nil
}

func (b *browserController) openLocked(ctx context.Context, request browserRequest) error {
	if err := os.RemoveAll(chromiumProfileDir); err != nil {
		return fmt.Errorf("reset Chromium profile: %w", err)
	}
	if err := os.MkdirAll(chromiumProfileDir, 0o700); err != nil {
		return fmt.Errorf("create Chromium profile: %w", err)
	}
	executable, err := findChromium()
	if err != nil {
		return err
	}
	options := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(executable),
		chromedp.Flag("headless", false),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.UserDataDir(chromiumProfileDir),
	)
	b.allocatorCtx, b.allocatorEnd = chromedp.NewExecAllocator(context.Background(), options...)
	b.browserCtx, b.browserEnd = chromedp.NewContext(b.allocatorCtx)
	b.tabCtx, b.tabEnd = chromedp.NewContext(b.browserCtx)
	startCtx, cancel := context.WithTimeout(b.tabCtx, 30*time.Second)
	defer cancel()
	initialURL := "about:blank"
	if strings.TrimSpace(request.URL) != "" {
		if err := validateBrowserURL(request.URL); err != nil {
			b.closeLocked()
			return err
		}
		initialURL = request.URL
	}
	downloadDir := filepath.Join(workspaceRoot, "Downloads")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		b.closeLocked()
		return err
	}
	if err := chromedp.Run(startCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllow).WithDownloadPath(downloadDir).WithEventsEnabled(true).Do(ctx)
		}),
		chromedp.Navigate(initialURL),
	); err != nil {
		b.closeLocked()
		return fmt.Errorf("start Chromium: %w", err)
	}
	tabs, err := b.tabsLocked(startCtx)
	if err != nil {
		b.closeLocked()
		return err
	}
	pageID := ""
	if len(tabs) > 0 {
		pageID = fmt.Sprint(tabs[0]["id"])
	}
	id, err := randomID()
	if err != nil {
		b.closeLocked()
		return err
	}
	now := time.Now().UTC()
	b.session = browserSession{ID: id, State: "open", ActivePageID: pageID, URLOrigin: origin(initialURL), ControlOwner: "agent", CreatedAt: now, UpdatedAt: now}
	return nil
}

func (b *browserController) tabsLocked(ctx context.Context) ([]map[string]interface{}, error) {
	targets, err := chromedp.Targets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list browser tabs: %w", err)
	}
	result := make([]map[string]interface{}, 0)
	for _, info := range targets {
		if info.Type != "page" {
			continue
		}
		result = append(result, map[string]interface{}{"id": string(info.TargetID), "title": info.Title, "url": info.URL, "active": string(info.TargetID) == b.session.ActivePageID})
	}
	return result, nil
}

func (b *browserController) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closeLocked()
}

func (b *browserController) closeLocked() {
	if b.tabEnd != nil {
		b.tabEnd()
	}
	if b.browserEnd != nil {
		b.browserEnd()
	}
	if b.allocatorEnd != nil {
		b.allocatorEnd()
	}
	b.tabCtx, b.browserCtx, b.allocatorCtx = nil, nil, nil
	b.tabEnd, b.browserEnd, b.allocatorEnd = nil, nil, nil
	if b.session.ID != "" {
		b.session.State = "closed"
		b.session.UpdatedAt = time.Now().UTC()
	}
	_ = os.RemoveAll(chromiumProfileDir)
}

func fillCredential(ctx context.Context, fields map[string]string, options map[string]interface{}) error {
	encoded, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	submit, _ := options["submit"].(bool)
	script := fmt.Sprintf(`(()=>{const f=%s;const find=(names)=>{for(const n of names){const e=document.querySelector(n);if(e)return e}return null};const set=(e,v)=>{if(!e)return false;e.focus();const p=Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value');p.set.call(e,v);e.dispatchEvent(new Event('input',{bubbles:true}));e.dispatchEvent(new Event('change',{bubbles:true}));return true};const done=[];if(f.username&&set(find(['input[autocomplete="username"]','input[name*="user" i]','input[type="email"]']),f.username))done.push('username');if(f.password&&set(find(['input[autocomplete="current-password"]','input[type="password"]']),f.password))done.push('password');if(f.token&&set(find(['input[name*="token" i]','input[autocomplete="one-time-code"]']),f.token))done.push('token');if(%t){const b=find(['button[type="submit"]','input[type="submit"]']);if(b)b.click()}return done})()`, string(encoded), submit)
	var filled []string
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, &filled)); err != nil {
		return fmt.Errorf("fill browser credential: %w", err)
	}
	if len(filled) == 0 {
		return &rpcError{Code: "credential_fields_not_found", Message: "no matching credential fields were found on the page"}
	}
	return nil
}

const inspectScript = `(()=>{let n=0;const els=[...document.querySelectorAll('a[href],button,input,select,textarea,[role="button"],[tabindex]')].filter(e=>{const r=e.getBoundingClientRect();const s=getComputedStyle(e);return r.width>0&&r.height>0&&s.visibility!=='hidden'&&s.display!=='none'}).slice(0,200);const items=els.map(e=>{let ref=e.getAttribute('data-aurago-ref');if(!ref){ref='e'+(++n)+'-'+Math.random().toString(36).slice(2,8);e.setAttribute('data-aurago-ref',ref)}const r=e.getBoundingClientRect();return{ref,tag:e.tagName.toLowerCase(),role:e.getAttribute('role')||'',name:(e.getAttribute('aria-label')||e.innerText||e.getAttribute('placeholder')||e.getAttribute('name')||'').trim().slice(0,200),type:e.getAttribute('type')||'',disabled:!!e.disabled,x:Math.round(r.x),y:Math.round(r.y),width:Math.round(r.width),height:Math.round(r.height)}});return{url:location.href,title:document.title,text:(document.body?.innerText||'').slice(0,12000),elements:items}})()`

func browserSelector(request browserRequest) string {
	if strings.TrimSpace(request.ElementRef) != "" {
		return `[data-aurago-ref="` + strings.ReplaceAll(request.ElementRef, `"`, `\"`) + `"]`
	}
	return strings.TrimSpace(request.Selector)
}

func validateBrowserURL(raw string) error {
	if raw == "about:blank" {
		return nil
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return &rpcError{Code: "invalid_url", Message: "browser URL must use http or https"}
	}
	return nil
}

func origin(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
}

func listDownloads() ([]map[string]interface{}, error) {
	downloadDir := filepath.Join(workspaceRoot, "Downloads")
	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		return nil, fmt.Errorf("list browser downloads: %w", err)
	}
	result := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		result = append(result, map[string]interface{}{"name": entry.Name(), "path": filepath.ToSlash(filepath.Join("Downloads", entry.Name())), "size": info.Size(), "modified_at": info.ModTime().UTC()})
	}
	return result, nil
}

func findChromium() (string, error) {
	if info, err := os.Stat("/usr/lib/chromium/chromium"); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
		return "/usr/lib/chromium/chromium", nil
	}
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable"} {
		if found, err := exec.LookPath(name); err == nil {
			return found, nil
		}
	}
	return "", &rpcError{Code: "browser_unavailable", Message: "Chromium executable was not found"}
}

func decodeParams(raw json.RawMessage, target interface{}) error {
	if len(raw) == 0 || string(raw) == "null" {
		raw = []byte("{}")
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func invalidParams(err error) *rpcError {
	return &rpcError{Code: "invalid_params", Message: err.Error()}
}

func toRPCError(err error) *rpcError {
	var typed *rpcError
	if errors.As(err, &typed) {
		return typed
	}
	return &rpcError{Code: "internal_error", Message: err.Error()}
}

func randomID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(data), nil
}

func safeEnvironmentName(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	var result strings.Builder
	for _, char := range value {
		if (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			result.WriteRune(char)
		} else {
			result.WriteByte('_')
		}
	}
	cleaned := strings.Trim(result.String(), "_")
	if cleaned == "" {
		return "CREDENTIAL"
	}
	return cleaned
}

func cloneMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func clearMap(values map[string]string) {
	for key := range values {
		values[key] = ""
		delete(values, key)
	}
}

func clearStrings(values []string) {
	for index := range values {
		values[index] = ""
	}
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

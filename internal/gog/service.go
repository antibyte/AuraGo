// Package gog owns AuraGo's optional, single-attempt Google Workspace CLI service.
package gog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

const MaxOutput = 4 << 20
const MaxFile = 64 << 20

type Runner func(context.Context, string, []string, string, []string) (output string, exitCode int, started bool, err error)

type Service struct {
	Root         string
	Vault        *security.Vault
	Config       func() *config.Config
	RunProcess   Runner
	CheckRuntime func(context.Context) error
	tokenClient  *http.Client
}

type Request struct {
	AccountID string         `json:"account_id"`
	Service   string         `json:"service"`
	Operation string         `json:"operation"`
	Args      map[string]any `json:"args,omitempty"`
}

type Result struct {
	Status  string          `json:"status"`
	Code    string          `json:"code,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	Message string          `json:"message,omitempty"`
}

func Failure(code string) Result { return Result{Status: "error", Code: code} }

type accountState struct {
	refresh  sync.Mutex
	mu       sync.Mutex
	revision uint64
	active   map[uint64]context.CancelFunc
	next     uint64
}

var states sync.Map

func (s *Service) state(id string) *accountState {
	key := struct {
		vault *security.Vault
		id    string
	}{s.Vault, id}
	v, _ := states.LoadOrStore(key, &accountState{active: map[uint64]context.CancelFunc{}})
	return v.(*accountState)
}

func (s *Service) changeAccount(id string, change func() error) error {
	a := s.state(id)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.revision++
	for _, cancel := range a.active {
		cancel()
	}
	return change()
}

func Authorize(cfg *config.Config, source string, req Request, grant string) (config.GogAccount, error) {
	if cfg == nil || !cfg.Gog.Enabled {
		return config.GogAccount{}, fmt.Errorf("disabled")
	}
	a, ok := cfg.Gog.Account(req.AccountID)
	if !ok || !a.Enabled {
		return a, fmt.Errorf("account_disabled")
	}
	if source == "agent" && !a.AgentAccess || source == "desktop" && (!a.DesktopAccess || !cfg.VirtualDesktop.Enabled) {
		return a, fmt.Errorf("access_denied")
	}
	if source != "agent" && source != "desktop" && source != "test" {
		return a, fmt.Errorf("invalid_source")
	}
	if !cfg.Agent.AllowShell {
		return a, fmt.Errorf("shell_disabled")
	}
	if !cfg.Agent.AllowNetworkRequests {
		return a, fmt.Errorf("network_disabled")
	}
	if grant != "read" && (cfg.Gog.ReadOnly || source == "test" || source == "desktop" && cfg.VirtualDesktop.ReadOnly) {
		return a, fmt.Errorf("readonly")
	}
	g := a.Services[req.Service]
	allowed := map[string]bool{"read": g.Read, "create": g.Create, "update": g.Update, "delete": g.Delete, "send": g.Send}[grant]
	if !allowed {
		return a, fmt.Errorf("permission_denied")
	}
	return a, nil
}

// Execute admits only catalogued operations. Files are staged by the caller using
// its own rooted file permission boundary; CLI paths never come from Args.
func (s *Service) Execute(ctx context.Context, source string, req Request, files *Files) Result {
	op, ok := Lookup(req.Service, req.Operation)
	if !ok {
		return Failure("unknown_operation")
	}
	initial := s.Config()
	account, err := authorizeOperation(initial, source, req, op.Grant)
	if err != nil {
		return Failure(err.Error())
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	state := s.state(req.AccountID)
	state.mu.Lock()
	state.next++
	runID := state.next
	state.active[runID] = cancel
	revision := state.revision
	state.mu.Unlock()
	defer func() { state.mu.Lock(); delete(state.active, runID); state.mu.Unlock() }()
	policyOK := func() bool {
		cfg := s.Config()
		current, e := authorizeOperation(cfg, source, req, op.Grant)
		state.mu.Lock()
		same := state.revision == revision
		state.mu.Unlock()
		return same && e == nil && reflect.DeepEqual(account, current)
	}
	if !policyOK() {
		return Failure("authorization_changed")
	}
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !policyOK() {
					cancel()
					return
				}
			}
		}
	}()
	if s.RunProcess == nil {
		return Failure("runtime_unavailable")
	}
	if s.CheckRuntime != nil {
		if err = s.CheckRuntime(ctx); err != nil {
			return Result{Status: "error", Code: "execution_denied", Message: security.Scrub(err.Error())}
		}
	}
	bin, err := s.VerifiedBinary()
	if err != nil {
		return Failure("not_installed")
	}
	if err = os.MkdirAll(filepath.Join(s.Root, "work"), 0700); err != nil {
		return Failure("staging_unavailable")
	}
	dir, err := os.MkdirTemp(filepath.Join(s.Root, "work"), "run-")
	if err != nil {
		return Failure("staging_unavailable")
	}
	defer os.RemoveAll(dir)
	args, err := Arguments(req, op, dir, files)
	if err != nil {
		return Result{Status: "error", Code: "invalid_arguments", Message: err.Error()}
	}
	token, err := s.accessToken(ctx, account, op)
	if err != nil {
		return Result{Status: "error", Code: "authentication_required", Message: security.Scrub(err.Error())}
	}
	for service, grant := range additionalGrants(req) {
		if _, err = s.accessToken(ctx, account, Operation{Service: service, Grant: grant}); err != nil {
			return Result{Status: "error", Code: "authentication_required", Message: security.Scrub(err.Error())}
		}
	}
	if !policyOK() || ctx.Err() != nil {
		return Failure("authorization_changed")
	}
	args = append([]string{"--json", "--no-input", "--color=never", "--enable-commands-exact=" + op.Command}, args...)
	if op.Grant == "read" {
		args = append([]string{"--readonly"}, args...)
	}
	if op.Grant != "send" {
		args = append([]string{"--gmail-no-send"}, args...)
	}
	output, code, started, runErr := s.RunProcess(ctx, bin, args, dir, []string{"GOG_ACCESS_TOKEN=" + token, "GOG_HOME=" + filepath.Join(dir, "home")})
	if !started && runErr != nil {
		return Result{Status: "error", Code: "execution_denied", Message: security.Scrub(runErr.Error())}
	}
	valid := len(output) <= MaxOutput && json.Valid([]byte(output))
	var envelope struct {
		Error json.RawMessage `json:"error"`
	}
	if valid {
		_ = json.Unmarshal([]byte(output), &envelope)
	}
	if runErr != nil || code != 0 || !valid || len(envelope.Error) > 0 && string(envelope.Error) != "null" || !policyOK() {
		if started && op.Grant != "read" {
			return Result{Status: "external_status_unknown", Code: "write_outcome_unknown", Message: "The operation may have completed. Inspect the selected account before attempting another write."}
		}
		codes := map[int]string{2: "invalid_arguments", 4: "authentication_required", 5: "not_found", 6: "permission_denied", 7: "rate_limited", 8: "provider_unavailable", 10: "needs_setup"}
		failure := codes[code]
		if failure == "" {
			failure = "invalid_response"
		}
		if ctx.Err() != nil {
			failure = "cancelled"
		}
		return Failure(failure)
	}
	if op.Output && files != nil {
		if err = files.collect(dir); err != nil {
			return Failure("file_output_invalid")
		}
	}
	// A private process path is never a caller-visible download location.
	if op.Output {
		output = `{"downloaded":true}`
	}
	data := security.Scrub(output)
	if !json.Valid([]byte(data)) {
		return Failure("invalid_response")
	}
	return Result{Status: "ok", Data: json.RawMessage(data)}
}

package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"gopkg.in/yaml.v3"
)

type decisionsSetupOperation struct {
	id       string
	cancel   context.CancelFunc
	settings config.DecisionsConfig
	active   bool
	code     string
}

func (s *Server) handleDecisionsSetup(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Deployment         string `json:"deployment"`
		CPUFallbackAllowed *bool  `json:"cpu_fallback_allowed"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeDecisionsError(w, decisions.CodeInvalidRequest, http.StatusBadRequest)
		return
	}
	if decoder.Decode(new(any)) != io.EOF || request.CPUFallbackAllowed == nil {
		writeDecisionsError(w, decisions.CodeInvalidRequest, http.StatusBadRequest)
		return
	}
	service := s.Decisions
	if routed, ok := service.(*routedDecisions); ok {
		service = routed.local
	}
	setup, ok := service.(interface{ Setup(context.Context) error })
	if !ok {
		writeDecisionsError(w, decisions.CodeRuntimeUnavailable, http.StatusServiceUnavailable)
		return
	}
	if !s.decisionsHTTPAdmission.TryLock() {
		writeDecisionsError(w, decisions.CodeLifecycleBusy, http.StatusConflict)
		return
	}
	admitted := false
	defer func() {
		if !admitted {
			s.decisionsHTTPAdmission.Unlock()
		}
	}()
	s.CfgSaveMu.Lock()
	current := s.ConfigSnapshot()
	if current == nil || current.ConfigPath == "" {
		s.CfgSaveMu.Unlock()
		writeDecisionsError(w, "configuration_save_failed", http.StatusInternalServerError)
		return
	}
	settings := current.Decisions
	settings.Enabled, settings.Backend = true, config.DecisionsBackendLocal
	settings.Deployment, settings.CPUFallbackAllowed = request.Deployment, *request.CPUFallbackAllowed
	settings = settings.Normalize()
	if err := settings.Validate(); err != nil {
		s.CfgSaveMu.Unlock()
		writeDecisionsError(w, decisions.CodeInvalidRequest, http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(localLLMLifecycleContext(s), 6*time.Hour)
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		cancel()
		s.CfgSaveMu.Unlock()
		writeDecisionsError(w, decisions.CodeRuntimeUnavailable, http.StatusInternalServerError)
		return
	}
	op := &decisionsSetupOperation{id: hex.EncodeToString(id[:]), cancel: cancel, settings: settings, active: true}
	s.decisionsSetupMu.Lock()
	s.decisionsSetup = op
	s.decisionsSetupMu.Unlock()
	err := s.saveDecisionsSetup(ctx, current, settings)
	s.CfgSaveMu.Unlock()
	if err != nil {
		s.finishDecisionsSetup(op, "configuration_save_failed")
		writeDecisionsError(w, "configuration_save_failed", http.StatusInternalServerError)
		return
	}
	admitted = true
	go func() {
		defer s.decisionsHTTPAdmission.Unlock()
		err := setup.Setup(ctx)
		s.finishDecisionsSetup(op, decisions.ErrorCode(err))
	}()
	writeDecisionsJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "setup_id": op.id, "decisions": settings})
}

// The config save lock belongs to the caller. Only this section is replaced.
func (s *Server) saveDecisionsSetup(ctx context.Context, current *config.Config, settings config.DecisionsConfig) error {
	raw, err := os.ReadFile(current.ConfigPath)
	if err != nil {
		return fmt.Errorf("read decisions config: %w", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return fmt.Errorf("parse decisions config: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("invalid config document")
	}
	var value yaml.Node
	if err := value.Encode(settings); err != nil {
		return err
	}
	root := document.Content[0]
	found := false
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "decisions" {
			if found {
				return fmt.Errorf("duplicate decisions section")
			}
			root.Content[i+1] = &value
			found = true
		}
	}
	if !found {
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "decisions"}, &value)
	}
	raw, err = yaml.Marshal(&document)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := config.WriteFileAtomic(current.ConfigPath, raw, 0o600); err != nil {
		return err
	}
	next := current.Clone()
	next.Decisions = settings
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(next)
	s.CfgMu.Unlock()
	return nil
}

func (s *Server) finishDecisionsSetup(op *decisionsSetupOperation, code string) {
	op.cancel()
	s.decisionsSetupMu.Lock()
	defer s.decisionsSetupMu.Unlock()
	if s.decisionsSetup == op {
		op.active, op.code = false, code
	}
}

func (s *Server) cancelDecisionsSetup() {
	s.decisionsSetupMu.Lock()
	defer s.decisionsSetupMu.Unlock()
	if op := s.decisionsSetup; op != nil {
		op.code = ""
		if op.active {
			op.cancel()
		}
	}
}

func (s *Server) decisionsSetupConfigChanged(previous, next *config.Config) bool {
	s.decisionsSetupMu.Lock()
	defer s.decisionsSetupMu.Unlock()
	op := s.decisionsSetup
	if op != nil && op.settings != next.Decisions.Normalize() {
		op.code = ""
	}
	if op == nil || !op.active {
		return false
	}
	if op.settings != next.Decisions.Normalize() || previous != nil &&
		(previous.Docker.Enabled != next.Docker.Enabled || previous.Docker.ReadOnly != next.Docker.ReadOnly || previous.Docker.Host != next.Docker.Host) {
		op.cancel()
	}
	return true
}

func (s *Server) decisionsSetupStatus() decisions.Status {
	status := s.Decisions.Status()
	s.decisionsSetupMu.Lock()
	defer s.decisionsSetupMu.Unlock()
	if op := s.decisionsSetup; op != nil {
		status.SetupID, status.SetupActive = op.id, op.active
		if op.active && !status.Ready && (status.State == "disabled" || status.State == "not_installed" || status.State == "installed" || status.State == "stopped") {
			status.State, status.Code = "setting_up", ""
		}
		if !op.active && op.code != "" && op.code != decisions.CodeCancelled && !status.Ready {
			status.State, status.Code = "error", op.code
		}
		if !op.active && op.code == decisions.CodeCancelled && !status.Ready &&
			(status.State == "installing" || status.State == "starting" || status.State == "discovering_devices") {
			status.State, status.Code = "stopped", decisions.CodeCancelled
		}
	}
	return status
}

package server

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"aurago/internal/flows"
	"aurago/internal/security"
)

// flowCredentialMoveMu serialises the moves: the existence check, the vault write and the draft
// save of one move run before those of the next, so two requests for one secret name cannot both
// pass the check. Moves are rare.
var flowCredentialMoveMu sync.Mutex

// flowCredentialMoveBeforeSave is a test seam between the vault write and the draft save.
var flowCredentialMoveBeforeSave = func(flowID string) {}

// flowCredentialMove moves one typed-in credential of a flow's draft into the vault (POST
// /api/desktop/flows/{id}/credentials/move). The rewrite is flows.MoveCredential; this only
// checks the request, writes the vault entry and saves the draft at the revision the client
// saw. The value goes from the draft to the vault and nowhere else: not into the answer, the
// audit entry or a log line.
func (s *Server) flowCredentialMove(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		flowsMethodNotAllowed(w)
		return
	}
	if s.Vault == nil {
		flowsError(w, http.StatusServiceUnavailable, "FLOW_VAULT_UNAVAILABLE", "the vault is not available")
		return
	}
	var body struct {
		Revision   int    `json:"revision"`
		NodeID     string `json:"node_id"`
		Location   string `json:"location"`
		Path       string `json:"path"`
		SecretName string `json:"secret_name"`
		Replace    bool   `json:"replace"`
	}
	if !flowsDecode(w, r, &body, flowsSmallBodyLimit, false) {
		return
	}
	if !flowSecretNamePattern.MatchString(body.SecretName) {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "secret names use a-z, 0-9 and _ (at most 40 characters)")
		return
	}
	cfg := s.ConfigSnapshot()
	if !s.flowSecretRate.allow(ClientIP(r, cfg != nil && cfg.Server.HTTPS.BehindProxy), time.Now(), flowSecretWritesPerWindow, flowSecretWriteWindow) {
		w.Header().Set("Retry-After", "60")
		flowsError(w, http.StatusTooManyRequests, "FLOW_RATE_LIMITED", "too many flow secret changes; try again in a minute")
		return
	}
	flowCredentialMoveMu.Lock()
	defer flowCredentialMoveMu.Unlock()
	ctx := r.Context()
	rec, err := s.Flows.GetFlow(ctx, id)
	if err != nil {
		s.flowsErrorFrom(w, r, err)
		return
	}
	if rec.DraftRevision != body.Revision {
		s.flowsErrorFrom(w, r, flows.ErrRevisionConflict)
		return
	}
	doc, err := rec.Draft.Clone()
	if err != nil {
		s.flowsErrorFrom(w, r, err)
		return
	}
	moved, notMovable, value, err := flows.MoveCredential(doc, body.NodeID, body.Location, body.Path, body.SecretName)
	switch {
	case errors.Is(err, flows.ErrCredentialGone):
		flowsError(w, http.StatusConflict, "FLOW_REVISION_CONFLICT", "the value is no longer in the draft")
		return
	case errors.Is(err, flows.ErrCredentialNotMovable):
		flowsError(w, http.StatusUnprocessableEntity, "FLOW_CREDENTIAL_NOT_MOVABLE", "this value cannot be moved to the vault")
		return
	case err != nil:
		s.flowsErrorFrom(w, r, err)
		return
	}
	// Validate before the vault is touched, so a refusal leaves nothing behind.
	if issues := s.Flows.Validate(doc, flows.ModeDraft); flows.HasErrors(issues) {
		s.flowsErrorFrom(w, r, &flows.ValidationError{Issues: issues})
		return
	}
	key := flowSecretPrefix + body.SecretName
	keys, err := s.Vault.ListKeys()
	if err != nil {
		s.flowsErrorFrom(w, r, err)
		return
	}
	existed := slices.Contains(keys, key)
	if existed && !body.Replace {
		flowsError(w, http.StatusConflict, "FLOW_SECRET_EXISTS", "a secret with this name already exists")
		return
	}
	// The value a replace overwrites is kept in memory until the draft is saved, so a failed save
	// puts it back and the move never loses a vault secret.
	previous := ""
	if existed {
		if previous, err = s.Vault.ReadSecret(key); err != nil {
			s.flowsErrorFrom(w, r, err)
			return
		}
	}
	if err := s.Vault.WriteUserSecretContext(ctx, key, value, true); err != nil {
		s.flowsErrorFrom(w, r, err)
		return
	}
	flowCredentialMoveBeforeSave(id)
	// A concurrent save between the read and this CAS leaves the vault entry without a user;
	// the vault is put back and the client reloads (409) and tries again.
	rev, issues, err := s.Flows.SaveDraft(ctx, id, doc, body.Revision)
	if err != nil {
		s.restoreMovedSecret(key, existed, previous)
		s.flowsErrorFrom(w, r, err)
		return
	}
	s.recordFlowAudit("flow_credential_moved", id, rec.Name, "Flow "+rec.Name+": credential moved to secret "+body.SecretName+" in node(s) "+strings.Join(moved, ", "))
	s.broadcastFlowsChanged(id, "saved")
	if notMovable == nil {
		notMovable = []flows.CredentialFinding{}
	}
	flowsJSON(w, http.StatusOK, map[string]any{"draft_revision": rev, "doc": doc, "moved": moved, "not_movable": notMovable, "issues": nonNilIssues(issues)})
}

// restoreMovedSecret undoes the vault write of a move whose draft save failed: the entry a replace
// overwrote gets its old value back, an entry the move created is deleted. It runs on its own
// context (the request may be the reason the save failed) and never logs a value.
func (s *Server) restoreMovedSecret(key string, existed bool, previous string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), 10*time.Second)
	defer cancel()
	var err error
	if existed {
		err = s.Vault.WriteUserSecretContext(ctx, key, previous, true)
	} else if err = s.Vault.DeleteSecret(key); errors.Is(err, security.ErrSecretNotFound) {
		err = nil
	}
	if err != nil && s.Logger != nil {
		s.Logger.Error("flow credential move: could not restore the vault entry after a failed save", "secret", key, "error", err)
	}
}

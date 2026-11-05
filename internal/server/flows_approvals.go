package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/security"
)

const (
	flowApprovalBodyLimit         = 4 << 10
	flowApprovalCursorLimit       = 256
	flowApprovalPageLimit         = 50
	flowApprovalListResponseLimit = 128 << 10
	flowApprovalDetailLimit       = 64 << 10
)

var (
	flowApprovalGateIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	flowApprovalDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// flowApprovalAuthAvailable is shared by the node catalog, durable authority and
// decision HTTP guard. Login must be enabled and have both a configured password
// and a signing secret; a dangling enabled flag is not an owner login.
func flowApprovalAuthAvailable(cfg *config.Config) bool {
	return cfg != nil && cfg.Auth.Enabled && strings.TrimSpace(cfg.Auth.PasswordHash) != "" &&
		strings.TrimSpace(cfg.Auth.SessionSecret) != ""
}

func isFlowApprovalDecision(method string, parts []string) bool {
	return method == http.MethodPost && len(parts) == 3 && parts[0] == "approvals" && parts[2] == "decision"
}

// requireFlowApprovalDecisionSession refuses the ordinary bearer and auth-disabled
// shortcuts. Approval decisions are actions by the single owner browser session, with
// a required exact Origin; no client value is used as the actor identity.
func (s *Server) requireFlowApprovalDecisionSession(w http.ResponseWriter, r *http.Request) bool {
	if _, isBearer := bearerCredential(r.Header.Get("Authorization")); isBearer {
		flowsError(w, http.StatusForbidden, "FLOW_PERMISSION_DENIED", "a signed-in owner browser session is required")
		return false
	}
	cfg := s.ConfigSnapshot()
	if !flowApprovalAuthAvailable(cfg) {
		flowsError(w, http.StatusServiceUnavailable, "FLOW_APPROVAL_AUTH_UNAVAILABLE", "approval_auth_unavailable")
		return false
	}
	if !IsAuthenticated(r, cfg.Auth.SessionSecret) {
		flowsError(w, http.StatusUnauthorized, "FLOW_UNAUTHORIZED", "a signed-in owner browser session is required")
		return false
	}
	if !checkCSRFOriginWithPolicy(r, true) {
		flowsError(w, http.StatusForbidden, "FLOW_PERMISSION_DENIED", "same-origin request required")
		return false
	}
	return true
}

func (s *Server) flowsApprovals(w http.ResponseWriter, r *http.Request, rest []string) {
	switch {
	case len(rest) == 0:
		if r.Method != http.MethodGet {
			flowsMethodNotAllowed(w)
			return
		}
		s.flowApprovalList(w, r)
	case len(rest) == 1:
		if !flowApprovalGateIDPattern.MatchString(rest[0]) {
			flowApprovalNotFound(w)
			return
		}
		if r.Method != http.MethodGet {
			flowsMethodNotAllowed(w)
			return
		}
		s.flowApprovalDetail(w, r, rest[0])
	case len(rest) == 2 && rest[1] == "decision":
		if !flowApprovalGateIDPattern.MatchString(rest[0]) {
			flowApprovalNotFound(w)
			return
		}
		if r.Method != http.MethodPost {
			flowsMethodNotAllowed(w)
			return
		}
		s.flowApprovalDecision(w, r, rest[0])
	default:
		flowApprovalNotFound(w)
	}
}

func (s *Server) flowApprovalList(w http.ResponseWriter, r *http.Request) {
	cursor, ok := flowApprovalCursor(r)
	if !ok {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "the approval cursor is invalid")
		return
	}
	page, err := s.Flows.ListDurableApprovals(r.Context(), cursor)
	if err != nil {
		s.flowApprovalError(w, r, err)
		return
	}
	if page.Items == nil {
		page.Items = []flows.DurableApprovalItem{}
	}
	if len(page.Items) > flowApprovalPageLimit || len(page.NextCursor) > flowApprovalCursorLimit {
		s.flowApprovalUnavailable(w, r, errors.New("approval page exceeded its response bounds"))
		return
	}
	for i := range page.Items {
		if !validFlowApprovalItem(page.Items[i]) {
			s.flowApprovalUnavailable(w, r, errors.New("approval page contained invalid metadata"))
			return
		}
		s.scrubFlowApprovalDisplay(&page.Items[i])
		s.overlayFlowApprovalCaller(&page.Items[i], r)
	}
	s.flowApprovalWriteJSON(w, http.StatusOK, page, flowApprovalListResponseLimit)
}

func flowApprovalCursor(r *http.Request) (string, bool) {
	query := r.URL.Query()
	if len(query) == 0 {
		return "", true
	}
	if len(query) != 1 {
		return "", false
	}
	values, ok := query["cursor"]
	if !ok || len(values) != 1 || len(values[0]) > flowApprovalCursorLimit || !utf8.ValidString(values[0]) {
		return "", false
	}
	return values[0], true
}

func (s *Server) flowApprovalDetail(w http.ResponseWriter, r *http.Request, gateID string) {
	detail, err := s.Flows.GetDurableApproval(r.Context(), gateID)
	if err != nil {
		s.flowApprovalError(w, r, err)
		return
	}
	if !validFlowApprovalItem(detail.DurableApprovalItem) {
		s.flowApprovalUnavailable(w, r, errors.New("approval detail contained invalid metadata"))
		return
	}
	if detail.State == flows.DurableApprovalPending {
		if detail.ReviewAvailable != (detail.Review != nil) || (detail.CanDecide && detail.Review == nil) ||
			(detail.Review == nil && detail.BlockedReason != flows.DurableApprovalBlockKeyUnavailable &&
				detail.BlockedReason != flows.DurableApprovalBlockCheckpoint && detail.BlockedReason != flows.DurableApprovalBlockStale) ||
			(detail.Review != nil && (!utf8.ValidString(detail.Review.Summary) || len(detail.Review.Summary) == 0 ||
				len(detail.Review.Summary) > 2<<10 || len(detail.Review.ReviewData) == 0 || len(detail.Review.ReviewData) > 16<<10 ||
				!json.Valid(detail.Review.ReviewData))) {
			s.flowApprovalUnavailable(w, r, errors.New("pending approval review was incomplete or out of bounds"))
			return
		}
	} else if detail.ReviewAvailable || detail.Review != nil {
		// Terminal gates retain bounded metadata only. Never return a review payload
		// after the encrypted checkpoint has been removed.
		s.flowApprovalUnavailable(w, r, errors.New("terminal approval unexpectedly contained review data"))
		return
	}
	s.scrubFlowApprovalDisplay(&detail.DurableApprovalItem)
	s.overlayFlowApprovalCaller(&detail.DurableApprovalItem, r)
	s.flowApprovalWriteJSON(w, http.StatusOK, detail, flowApprovalDetailLimit)
}

type flowApprovalDecisionBody struct {
	GateGeneration *int64  `json:"gate_generation"`
	ContentDigest  *string `json:"content_digest"`
	Decision       *string `json:"decision"`
}

func (s *Server) flowApprovalDecision(w http.ResponseWriter, r *http.Request, gateID string) {
	var body flowApprovalDecisionBody
	if !s.decodeFlowApprovalDecision(w, r, &body) {
		return
	}
	if body.GateGeneration == nil || *body.GateGeneration <= 0 || body.ContentDigest == nil ||
		!flowApprovalDigestPattern.MatchString(*body.ContentDigest) || body.Decision == nil ||
		(*body.Decision != flows.DurableApprovalDecisionApprove && *body.Decision != flows.DurableApprovalDecisionReject) {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "the approval decision is invalid")
		return
	}
	result, err := s.Flows.DecideDurableApproval(r.Context(), gateID, *body.GateGeneration, *body.ContentDigest, *body.Decision, "local_admin")
	if err != nil {
		s.flowApprovalError(w, r, err)
		return
	}
	wantState := flows.DurableApprovalApproved
	if *body.Decision == flows.DurableApprovalDecisionReject {
		wantState = flows.DurableApprovalRejected
	}
	if !validFlowApprovalItem(result.DurableApprovalItem) || result.Actor != "local_admin" || result.Decision != *body.Decision ||
		result.State != wantState || result.ReviewAvailable || result.RunState == "" {
		s.flowApprovalUnavailable(w, r, errors.New("approval decision returned invalid metadata"))
		return
	}
	s.scrubFlowApprovalDisplay(&result.DurableApprovalItem)
	s.flowApprovalWriteJSON(w, http.StatusOK, result, flowApprovalDetailLimit)
}

func (s *Server) decodeFlowApprovalDecision(w http.ResponseWriter, r *http.Request, dst any) bool {
	mediaType, params, mediaErr := mime.ParseMediaType(strings.TrimSpace(r.Header.Get("Content-Type")))
	charset, hasCharset := params["charset"]
	if mediaErr != nil || mediaType != "application/json" || len(params) > 1 || (len(params) == 1 && !hasCharset) ||
		(hasCharset && !strings.EqualFold(charset, "utf-8")) {
		flowsError(w, http.StatusUnsupportedMediaType, "FLOW_BAD_REQUEST", "application/json is required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, flowApprovalBodyLimit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			flowsError(w, http.StatusRequestEntityTooLarge, "FLOW_TOO_LARGE", "the approval request is too large")
		} else {
			flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "the approval request is not valid JSON")
		}
		return false
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "the approval request must contain one JSON object")
		return false
	}
	return true
}

func (s *Server) overlayFlowApprovalCaller(item *flows.DurableApprovalItem, r *http.Request) {
	if item.State != flows.DurableApprovalPending {
		item.CanDecide = false
		if item.State == flows.DurableApprovalStale {
			item.BlockedReason = flows.DurableApprovalBlockStale
		} else if item.State == flows.DurableApprovalUnknown {
			item.BlockedReason = flows.DurableApprovalBlockCheckpoint
		} else {
			item.BlockedReason = flows.DurableApprovalBlockDecided
		}
		return
	}
	cfg := s.ConfigSnapshot()
	if !flowApprovalAuthAvailable(cfg) {
		item.CanDecide = false
		item.BlockedReason = flows.DurableApprovalBlockAuthUnavailable
		return
	}
	if _, isBearer := bearerCredential(r.Header.Get("Authorization")); isBearer || !IsAuthenticated(r, cfg.Auth.SessionSecret) {
		item.CanDecide = false
		item.BlockedReason = flows.DurableApprovalBlockSessionRequired
		return
	}
	if cfg.VirtualDesktop.ReadOnly || cfg.Tools.Missions.ReadOnly {
		item.CanDecide = false
		item.BlockedReason = flows.DurableApprovalBlockReadOnly
		return
	}
	if !cfg.Flows.DurableEnabled {
		item.CanDecide = false
		item.BlockedReason = flows.DurableApprovalBlockDurableDisabled
		return
	}
	if item.CanDecide && item.BlockedReason != "" {
		item.CanDecide = false
	}
	if !item.CanDecide && (item.BlockedReason == "" || !validFlowApprovalBlockReason(item.BlockedReason)) {
		item.BlockedReason = flows.DurableApprovalBlockCheckpoint
	}
}

// scrubFlowApprovalDisplay keeps metadata bounds small and masks registered or
// credential-shaped text. It never changes the reviewed payload or its digest.
func (s *Server) scrubFlowApprovalDisplay(item *flows.DurableApprovalItem) {
	if item == nil {
		return
	}
	item.FlowName = flowApprovalDisplayText(item.FlowName)
	item.NodeLabel = flowApprovalDisplayText(item.NodeLabel)
}

func flowApprovalDisplayText(value string) string {
	return flowCapRunes(security.RedactSensitiveInfo(scrubFlowText(value)), 120)
}

func validFlowApprovalItem(item flows.DurableApprovalItem) bool {
	if !flowApprovalGateIDPattern.MatchString(item.GateID) || !flowRunIDPattern.MatchString(item.RunID) || !flowIDPattern.MatchString(item.FlowID) ||
		!flows.ValidNodeID(item.NodeID) || !flows.ValidKey(item.NodeKey) || len(item.FlowName) > flows.MaxDocumentBytes ||
		len(item.NodeLabel) > flows.MaxDocumentBytes || len(item.AdmissionOwner) == 0 || len(item.AdmissionOwner) > 128 ||
		item.Revision <= 0 || item.GateGeneration <= 0 ||
		!flowApprovalDigestPattern.MatchString(item.ContentDigest) || !validFlowApprovalState(item.State) ||
		!validFlowApprovalBlockReason(item.BlockedReason) || !utf8.ValidString(item.FlowName) || !utf8.ValidString(item.NodeLabel) ||
		!utf8.ValidString(item.AdmissionOwner) {
		return false
	}
	if item.Actor != "" && item.Actor != "local_admin" {
		return false
	}
	if item.Decision != "" && item.Decision != flows.DurableApprovalDecisionApprove && item.Decision != flows.DurableApprovalDecisionReject {
		return false
	}
	return true
}

func validFlowApprovalState(state string) bool {
	switch state {
	case flows.DurableApprovalPending, flows.DurableApprovalApproved, flows.DurableApprovalRejected,
		flows.DurableApprovalExpired, flows.DurableApprovalCancelled, flows.DurableApprovalStale, flows.DurableApprovalUnknown:
		return true
	default:
		return false
	}
}

func validFlowApprovalBlockReason(reason string) bool {
	switch reason {
	case "", flows.DurableApprovalBlockAuthUnavailable, flows.DurableApprovalBlockSessionRequired,
		flows.DurableApprovalBlockReadOnly, flows.DurableApprovalBlockDurableDisabled, flows.DurableApprovalBlockStale,
		flows.DurableApprovalBlockKeyUnavailable, flows.DurableApprovalBlockCheckpoint,
		flows.DurableApprovalBlockDecided, flows.DurableApprovalBlockDeadline:
		return true
	default:
		return false
	}
}

func (s *Server) flowApprovalWriteJSON(w http.ResponseWriter, status int, value any, limit int) {
	data, err := json.Marshal(value)
	if err != nil {
		flowsWriteEncodeFailure(w)
		return
	}
	if len(data) > limit {
		s.flowApprovalUnavailable(w, nil, errors.New("approval response exceeded its response bounds"))
		return
	}
	flowsWriteBody(w, status, data)
}

func (s *Server) flowApprovalError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, flows.ErrDurableDisabled):
		flowsError(w, http.StatusServiceUnavailable, "FLOW_DURABLE_DISABLED", "durable processing is disabled")
	case errors.Is(err, flows.ErrDurableApprovalAuthUnavailable):
		flowsError(w, http.StatusServiceUnavailable, "FLOW_APPROVAL_AUTH_UNAVAILABLE", "approval_auth_unavailable")
	case errors.Is(err, flows.ErrDurableApprovalConflict):
		flowsError(w, http.StatusConflict, "FLOW_DURABLE_APPROVAL_CONFLICT", "the approval changed; refresh its status")
	case errors.Is(err, flows.ErrDurableApprovalNotFound):
		flowsError(w, http.StatusNotFound, "FLOW_DURABLE_APPROVAL_NOT_FOUND", "the approval was not found")
	case errors.Is(err, flows.ErrDurableApprovalUnavailable):
		s.flowApprovalUnavailable(w, r, err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		flowsError(w, http.StatusServiceUnavailable, "FLOW_REQUEST_CANCELLED", "the approval request was cancelled")
	default:
		// Approval status is authoritative only when the service confirms it. Any
		// unclassified storage or checkpoint error remains unknown to the client.
		s.flowApprovalUnavailable(w, r, err)
	}
}

func (s *Server) flowApprovalUnavailable(w http.ResponseWriter, r *http.Request, err error) {
	if s != nil && s.Logger != nil && err != nil {
		s.Logger.Warn("EasyDrag durable approval request could not be confirmed",
			"route", flowBoundRunes(func() string {
				if r == nil {
					return "response"
				}
				return r.URL.Path
			}(), flowsLogPathRunes),
			"error", flowsErrorText(err))
	}
	flowsError(w, http.StatusServiceUnavailable, "FLOW_DURABLE_APPROVAL_UNAVAILABLE", "the approval status is unavailable")
}

func flowApprovalNotFound(w http.ResponseWriter) {
	flowsError(w, http.StatusNotFound, "FLOW_DURABLE_APPROVAL_NOT_FOUND", "the approval was not found")
}

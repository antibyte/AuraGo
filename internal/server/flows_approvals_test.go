package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/flows"
)

const durableApprovalFlowJSON = `{"schema":1,"name":"Review gate","nodes":[
 {"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"label":"Start","position":{"x":0,"y":0},"params":{}},
 {"id":"n_bbbbbbbb","key":"approval","type":"logic.durable_approval","type_version":1,"label":"Review request","position":{"x":300,"y":0},"params":{"summary":"Review this request","review_data":{"item":"request-17"}}}],
 "edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}]}`

type c15ApprovalLogs struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *c15ApprovalLogs) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *c15ApprovalLogs) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func c15StartedApprovalServer(t *testing.T) (*Server, string, string, *c15ApprovalLogs) {
	t.Helper()
	s := c16Server(t, true, nil)
	logs := &c15ApprovalLogs{}
	s.Logger = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s.Cfg.Flows.DurableEnabled = true
	s.initFlows()
	if s.Flows == nil {
		t.Fatal("flows were not initialized")
	}
	if err := s.MissionManagerV2.Start(); err != nil {
		t.Fatalf("start Mission Control: %v", err)
	}
	s.startFlows(context.Background())
	t.Cleanup(func() { s.shutdownFlows(context.Background()) })
	adminToken, _, err := s.TokenManager.Create("C15 approval admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	readToken, _, err := s.TokenManager.Create("C15 approval reader", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s, adminToken, readToken, logs
}

func c15ApprovalHandler(s *Server) http.Handler {
	return s.trackHTTP(authMiddleware(s, http.HandlerFunc(s.handleFlows)))
}

func c15ApprovalRouteHandler(s *Server) http.Handler {
	// Exercise the route's stable error envelope where the shared auth middleware
	// intentionally returns its own server-wide lock/login/CSRF response first.
	return s.trackHTTP(http.HandlerFunc(s.handleFlows))
}

func c15ApprovalRequestWithHandler(handler http.Handler, s *Server, method, path, body, token string, session bool, origin, contentType string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1"+path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if session {
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func c15ApprovalRequest(s *Server, method, path, body, token string, session bool, origin, contentType string) *httptest.ResponseRecorder {
	return c15ApprovalRequestWithHandler(c15ApprovalHandler(s), s, method, path, body, token, session, origin, contentType)
}

func c15ApprovalRouteRequest(s *Server, method, path, body, token string, session bool, origin, contentType string) *httptest.ResponseRecorder {
	return c15ApprovalRequestWithHandler(c15ApprovalRouteHandler(s), s, method, path, body, token, session, origin, contentType)
}

func c15PublishConfig(t *testing.T, s *Server, mutate func(*config.Config)) {
	t.Helper()
	cfg := s.ConfigSnapshot().Clone()
	mutate(cfg)
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(cfg)
	s.CfgMu.Unlock()
}

func TestDurableApprovalHTTPGuards(t *testing.T) {
	s, adminToken, _, _ := c15StartedApprovalServer(t)
	path := "/api/desktop/flows/approvals/gate_missing/decision"
	validBody := `{"gate_generation":1,"content_digest":"` + strings.Repeat("a", 64) + `","decision":"approve"}`
	ownerOrigin := "http://127.0.0.1"
	tests := []struct {
		name        string
		mutate      func(*config.Config)
		token       string
		session     bool
		origin      string
		contentType string
		body        string
		wantStatus  int
		wantCode    string
	}{
		{name: "auth disabled", mutate: func(cfg *config.Config) { cfg.Auth.Enabled = false }, origin: ownerOrigin, contentType: "application/json", body: validBody, wantStatus: http.StatusServiceUnavailable, wantCode: "FLOW_APPROVAL_AUTH_UNAVAILABLE"},
		{name: "password not configured", mutate: func(cfg *config.Config) { cfg.Auth.PasswordHash = "" }, origin: ownerOrigin, contentType: "application/json", body: validBody, wantStatus: http.StatusServiceUnavailable, wantCode: "FLOW_APPROVAL_AUTH_UNAVAILABLE"},
		{name: "session signing secret not configured", mutate: func(cfg *config.Config) { cfg.Auth.SessionSecret = "" }, origin: ownerOrigin, contentType: "application/json", body: validBody, wantStatus: http.StatusServiceUnavailable, wantCode: "FLOW_APPROVAL_AUTH_UNAVAILABLE"},
		{name: "missing owner session", origin: ownerOrigin, contentType: "application/json", body: validBody, wantStatus: http.StatusUnauthorized, wantCode: "FLOW_UNAUTHORIZED"},
		{name: "bearer with owner cookie", token: adminToken, session: true, origin: ownerOrigin, contentType: "application/json", body: validBody, wantStatus: http.StatusForbidden, wantCode: "FLOW_PERMISSION_DENIED"},
		{name: "foreign origin", session: true, origin: "https://attacker.example", contentType: "application/json", body: validBody, wantStatus: http.StatusForbidden, wantCode: "FLOW_PERMISSION_DENIED"},
		{name: "missing origin", session: true, contentType: "application/json", body: validBody, wantStatus: http.StatusForbidden, wantCode: "FLOW_PERMISSION_DENIED"},
		{name: "readonly policy", mutate: func(cfg *config.Config) { cfg.VirtualDesktop.ReadOnly = true }, session: true, origin: ownerOrigin, contentType: "application/json", body: validBody, wantStatus: http.StatusForbidden, wantCode: "FLOW_PERMISSION_DENIED"},
		{name: "mission readonly policy", mutate: func(cfg *config.Config) { cfg.Tools.Missions.ReadOnly = true }, session: true, origin: ownerOrigin, contentType: "application/json", body: validBody, wantStatus: http.StatusForbidden, wantCode: "FLOW_PERMISSION_DENIED"},
		{name: "client actor", session: true, origin: ownerOrigin, contentType: "application/json", body: `{"gate_generation":1,"content_digest":"` + strings.Repeat("a", 64) + `","decision":"approve","actor":"attacker"}`, wantStatus: http.StatusBadRequest, wantCode: "FLOW_BAD_REQUEST"},
		{name: "trailing json", session: true, origin: ownerOrigin, contentType: "application/json", body: validBody + ` {}`, wantStatus: http.StatusBadRequest, wantCode: "FLOW_BAD_REQUEST"},
		{name: "oversized body", session: true, origin: ownerOrigin, contentType: "application/json", body: `{"gate_generation":1,"content_digest":"` + strings.Repeat("a", 5000) + `","decision":"approve"}`, wantStatus: http.StatusRequestEntityTooLarge, wantCode: "FLOW_TOO_LARGE"},
		{name: "unaccepted media type", session: true, origin: ownerOrigin, contentType: "application/json-garbage", body: validBody, wantStatus: http.StatusUnsupportedMediaType, wantCode: "FLOW_BAD_REQUEST"},
		{name: "UTF-8 JSON media type", session: true, origin: ownerOrigin, contentType: "application/json; charset=utf-8", body: validBody, wantStatus: http.StatusNotFound, wantCode: "FLOW_DURABLE_APPROVAL_NOT_FOUND"},
	}
	base := s.ConfigSnapshot().Clone()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base.Clone()
			if tc.mutate != nil {
				tc.mutate(cfg)
			}
			s.CfgMu.Lock()
			s.replaceConfigSnapshot(cfg)
			s.CfgMu.Unlock()
			w := c15ApprovalRouteRequest(s, http.MethodPost, path, tc.body, tc.token, tc.session, tc.origin, tc.contentType)
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.wantStatus, w.Body.String())
			}
			if tc.wantCode != "" && flowsBody(t, w)["code"] != tc.wantCode {
				t.Fatalf("code = %v, want %s: %s", flowsBody(t, w)["code"], tc.wantCode, w.Body.String())
			}
		})
	}
}

func TestDurableApprovalHTTPDecisionLostReplyReadback(t *testing.T) {
	s, _, readToken, logs := c15StartedApprovalServer(t)
	doc, err := flows.ParseFlow([]byte(durableApprovalFlowJSON))
	if err != nil {
		t.Fatal(err)
	}
	doc.Name = strings.Repeat("界", 150)
	rec, err := s.Flows.CreateFlow(context.Background(), flows.CreateRequest{Import: doc})
	if err != nil {
		t.Fatalf("create flow: %v", err)
	}
	published, _, err := s.Flows.Publish(context.Background(), rec.ID, rec.DraftRevision)
	if err != nil {
		t.Fatalf("publish flow: %v", err)
	}
	if err := s.Flows.SetEnabled(context.Background(), published.ID, true); err != nil {
		t.Fatalf("enable flow: %v", err)
	}
	ownerOrigin := "http://127.0.0.1"
	c15PublishConfig(t, s, func(cfg *config.Config) { cfg.Auth.Enabled = false })
	blockedStart := c15ApprovalRequest(s, http.MethodPost, "/api/desktop/flows/"+published.ID+"/run", "", "", false, ownerOrigin, "")
	if blockedStart.Code != http.StatusServiceUnavailable || flowsBody(t, blockedStart)["code"] != "FLOW_APPROVAL_AUTH_UNAVAILABLE" {
		t.Fatalf("start without owner login = %d %s", blockedStart.Code, blockedStart.Body.String())
	}
	emptyPage, listErr := s.Flows.ListDurableApprovals(context.Background(), "")
	if listErr != nil || len(emptyPage.Items) != 0 {
		t.Fatalf("auth-disabled admission left approval gates: %+v, %v", emptyPage.Items, listErr)
	}
	c15PublishConfig(t, s, func(cfg *config.Config) { cfg.Auth.Enabled = true })
	start := c15ApprovalRequest(s, http.MethodPost, "/api/desktop/flows/"+published.ID+"/run", "", "", true, ownerOrigin, "")
	if start.Code != http.StatusAccepted {
		t.Fatalf("start flow = %d %s", start.Code, start.Body.String())
	}
	var started struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(start.Body.Bytes(), &started); err != nil || started.RunID == "" {
		t.Fatalf("start response = %s, %v", start.Body.String(), err)
	}

	deadline := time.Now().Add(10 * time.Second)
	var item flows.DurableApprovalItem
	var lastListErr error
	for time.Now().Before(deadline) {
		page, listErr := s.Flows.ListDurableApprovals(context.Background(), "")
		lastListErr = listErr
		if listErr == nil && len(page.Items) == 1 && page.Items[0].RunID == started.RunID && page.Items[0].State == flows.DurableApprovalPending {
			item = page.Items[0]
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if item.GateID == "" || item.GateGeneration <= 0 || item.ContentDigest == "" {
		runState, runCode, runMessage := "unavailable", "", ""
		steps := make([]string, 0)
		runDetail, runErr := s.Flows.Run(context.Background(), started.RunID, false)
		if runDetail != nil && runDetail.Run != nil {
			runState = string(runDetail.Run.Status)
			runCode = runDetail.Run.ErrorCode
			runMessage = flowBoundRunes(runDetail.Run.ErrorMessage, 120)
			for _, step := range runDetail.Steps {
				steps = append(steps, step.NodeID+":"+string(step.Status)+":"+step.ErrorCode)
			}
		}
		runReadError := ""
		if runErr != nil {
			runReadError = flowBoundRunes(runErr.Error(), 120)
		}
		t.Fatalf("pending gate not found for run %s: item=%+v run_status=%s run_error_code=%s run_error=%q step_history=%v run_read_error=%q gate_list_error=%v flow_logs=%q",
			started.RunID, item, runState, runCode, runMessage, steps, runReadError, lastListErr, flowBoundRunes(logs.String(), 4000))
	}

	detailPath := "/api/desktop/flows/approvals/" + item.GateID
	c15PublishConfig(t, s, func(cfg *config.Config) { cfg.Auth.Enabled = false })
	blockedReview := c15ApprovalRequest(s, http.MethodGet, detailPath, "", "", false, "", "")
	if blockedReview.Code != http.StatusOK || blockedReview.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("auth-disabled review read = %d cache %q: %s", blockedReview.Code, blockedReview.Header().Get("Cache-Control"), blockedReview.Body.String())
	}
	var blockedDetail flows.DurableApprovalDetail
	if err := json.Unmarshal(blockedReview.Body.Bytes(), &blockedDetail); err != nil || blockedDetail.CanDecide ||
		blockedDetail.BlockedReason != flows.DurableApprovalBlockAuthUnavailable || blockedDetail.Review == nil || !blockedDetail.ReviewAvailable {
		t.Fatalf("auth-disabled pending review = %s, %v", blockedReview.Body.String(), err)
	}
	c15PublishConfig(t, s, func(cfg *config.Config) { cfg.Auth.Enabled = true })

	list := c15ApprovalRequest(s, http.MethodGet, "/api/desktop/flows/approvals", "", readToken, false, "", "")
	if list.Code != http.StatusOK || list.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("bearer inbox = %d cache %q: %s", list.Code, list.Header().Get("Cache-Control"), list.Body.String())
	}
	var page flows.DurableApprovalPage
	if err := json.Unmarshal(list.Body.Bytes(), &page); err != nil || len(page.Items) != 1 {
		t.Fatalf("bearer inbox = %s, %v", list.Body.String(), err)
	}
	visible := page.Items[0]
	if visible.CanDecide || visible.BlockedReason != flows.DurableApprovalBlockSessionRequired ||
		utf8.RuneCountInString(visible.FlowName) != 120 || !strings.HasSuffix(visible.FlowName, "…") {
		t.Fatalf("read bearer decision/display projection = %+v", visible)
	}

	detailReply := c15ApprovalRequest(s, http.MethodGet, detailPath, "", "", true, "", "")
	if detailReply.Code != http.StatusOK || detailReply.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("review detail = %d cache %q: %s", detailReply.Code, detailReply.Header().Get("Cache-Control"), detailReply.Body.String())
	}
	var detail flows.DurableApprovalDetail
	if err := json.Unmarshal(detailReply.Body.Bytes(), &detail); err != nil || detail.Review == nil || !detail.CanDecide || !detail.ReviewAvailable {
		t.Fatalf("pending review detail = %s, %v", detailReply.Body.String(), err)
	}
	var reviewData map[string]any
	if err := json.Unmarshal(detail.Review.ReviewData, &reviewData); err != nil || reviewData["item"] != "request-17" {
		t.Fatalf("review_data = %s, %v", detail.Review.ReviewData, err)
	}

	decisionBody, err := json.Marshal(struct {
		GateGeneration int64  `json:"gate_generation"`
		ContentDigest  string `json:"content_digest"`
		Decision       string `json:"decision"`
	}{item.GateGeneration, item.ContentDigest, flows.DurableApprovalDecisionApprove})
	if err != nil {
		t.Fatal(err)
	}
	// Deliberately discard the POST response. Recovery is a GET readback; the
	// client does not replay an uncertain decision mutation.
	_ = c15ApprovalRequest(s, http.MethodPost, detailPath+"/decision", string(decisionBody), "", true, ownerOrigin, "application/json")

	readback := c15ApprovalRequest(s, http.MethodGet, detailPath, "", "", true, "", "")
	if readback.Code != http.StatusOK || readback.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("decision readback = %d cache %q: %s", readback.Code, readback.Header().Get("Cache-Control"), readback.Body.String())
	}
	var terminal flows.DurableApprovalDetail
	if err := json.Unmarshal(readback.Body.Bytes(), &terminal); err != nil {
		t.Fatal(err)
	}
	if terminal.State != flows.DurableApprovalApproved || terminal.Decision != flows.DurableApprovalDecisionApprove ||
		terminal.Actor != "local_admin" || terminal.ReviewAvailable || terminal.Review != nil || terminal.GateID != item.GateID || terminal.RunID != started.RunID {
		t.Fatalf("terminal gate after readback = %+v", terminal)
	}

	deadline = time.Now().Add(10 * time.Second)
	var run *flows.RunDetail
	for time.Now().Before(deadline) {
		run, err = s.Flows.Run(context.Background(), started.RunID, false)
		if err == nil && run != nil && run.Run != nil && run.Run.Status == flows.RunSuccess {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil || run == nil || run.Run == nil || run.Run.ID != started.RunID || run.Run.Status != flows.RunSuccess {
		t.Fatalf("same run did not resume successfully: %+v, %v", run, err)
	}
	approvalStepStatuses := make([]flows.StepStatus, 0, 2)
	approvalSuccesses := 0
	for _, step := range run.Steps {
		if step.NodeID == "n_bbbbbbbb" {
			approvalStepStatuses = append(approvalStepStatuses, step.Status)
			if step.Status == flows.StepSuccess {
				approvalSuccesses++
			}
		}
	}
	if len(approvalStepStatuses) == 0 || approvalStepStatuses[len(approvalStepStatuses)-1] != flows.StepSuccess || approvalSuccesses != 1 {
		t.Fatalf("approval node did not resume to exactly one success: statuses=%v", approvalStepStatuses)
	}
	if !bytes.Contains(readback.Body.Bytes(), []byte(`"review_available":false`)) || bytes.Contains(readback.Body.Bytes(), []byte(`"review":`)) {
		t.Fatalf("terminal readback retained review payload: %s", readback.Body.String())
	}
}

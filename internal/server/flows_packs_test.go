package server

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/flows"
)

func localPackTestBody(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func createLocalPackGreetingBlock(t *testing.T, s *Server, name, expression string) *flows.FlowRecord {
	t.Helper()
	ctx := t.Context()
	block, err := s.Flows.CreateBlock(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := block.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	doc.Nodes[0].Params["fields"] = []any{map[string]any{"name": "name", "type": "string", "required": true}}
	doc.Nodes[1].Params["fields"] = []any{map[string]any{"name": "greeting", "type": "string", "required": true}}
	doc.Nodes[1].Params["values"] = map[string]any{"greeting": expression}
	revision, issues, err := s.Flows.SaveBlockDraft(ctx, block.ID, doc, block.DraftRevision)
	if err != nil {
		t.Fatalf("save block: %v %+v", err, issues)
	}
	block, issues, err = s.Flows.PublishBlock(ctx, block.ID, revision)
	if err != nil {
		t.Fatalf("publish block: %v %+v", err, issues)
	}
	return block
}

func createLocalPackSource(t *testing.T, s *Server) (*flows.FlowRecord, []string, []*flows.FlowRecord) {
	t.Helper()
	ctx := t.Context()
	leaf := createLocalPackGreetingBlock(t, s, "Leaf", "leaf {{block_input.data.name}}")
	middle := createLocalPackGreetingBlock(t, s, "Middle", "middle-v1 {{block_input.data.name}}")
	doc, err := middle.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	inputID, outputID := doc.Nodes[0].ID, doc.Nodes[1].ID
	call := flows.Node{ID: flows.NewNodeID(), Key: "nested", Type: "component.call", TypeVersion: 1,
		Params: map[string]any{"flow_id": leaf.ID, "revision": float64(leaf.LiveRevision), "inputs": map[string]any{"name": "{{block_input.data.name}}"}}}
	doc.Nodes = append(doc.Nodes, call)
	doc.Nodes[1].Params["values"] = map[string]any{"greeting": "{{nested.greeting}}"}
	doc.Edges = []flows.Edge{
		{ID: flows.NewEdgeID(), Source: flows.PortRef{Node: inputID, Port: flows.PortOut}, Target: flows.PortRef{Node: call.ID, Port: flows.PortIn}},
		{ID: flows.NewEdgeID(), Source: flows.PortRef{Node: call.ID, Port: flows.PortOut}, Target: flows.PortRef{Node: outputID, Port: flows.PortIn}},
	}
	revision, issues, err := s.Flows.SaveBlockDraft(ctx, middle.ID, doc, middle.DraftRevision)
	if err != nil {
		t.Fatalf("save nested block: %v %+v", err, issues)
	}
	middle, issues, err = s.Flows.PublishBlock(ctx, middle.ID, revision)
	if err != nil {
		t.Fatalf("publish nested block: %v %+v", err, issues)
	}
	startID, firstID, secondID := flows.NewNodeID(), flows.NewNodeID(), flows.NewNodeID()
	parentDoc := &flows.Flow{Schema: flows.SchemaVersion, Kind: flows.KindFlow, Name: "Pack source", Nodes: []flows.Node{
		{ID: startID, Key: "start", Type: flows.TypeTriggerManual, TypeVersion: 1, Params: map[string]any{"data": map[string]any{"name": "sample"}}},
		{ID: firstID, Key: "first", Type: "component.call", TypeVersion: 1, Params: map[string]any{"flow_id": middle.ID, "revision": float64(1), "inputs": map[string]any{"name": "Ada"}}},
		{ID: secondID, Key: "second", Type: "component.call", TypeVersion: 1, Params: map[string]any{"flow_id": middle.ID, "revision": float64(middle.LiveRevision), "inputs": map[string]any{"name": "Grace"}}},
	}, Edges: []flows.Edge{
		{ID: flows.NewEdgeID(), Source: flows.PortRef{Node: startID, Port: flows.PortOut}, Target: flows.PortRef{Node: firstID, Port: flows.PortIn}},
		{ID: flows.NewEdgeID(), Source: flows.PortRef{Node: firstID, Port: flows.PortOut}, Target: flows.PortRef{Node: secondID, Port: flows.PortIn}},
	}}
	parentDoc.Normalize()
	parent, err := s.Flows.CreateFlow(ctx, flows.CreateRequest{Import: parentDoc})
	if err != nil {
		t.Fatal(err)
	}
	return parent, []string{firstID, secondID}, []*flows.FlowRecord{leaf, middle}
}

func exportLocalPack(t *testing.T, s *Server, token string, flowID string) flows.LocalPack {
	t.Helper()
	response := flowsHTTPCall(t, flowsHTTPMux(s), http.MethodGet, "/api/desktop/flows/"+flowID+"/pack", token, "")
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Header().Get("Content-Disposition"), `filename="pack-source.easydrag-pack.json"`) {
		t.Fatalf("pack export = %d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	var pack flows.LocalPack
	if err := json.Unmarshal(response.Body.Bytes(), &pack); err != nil {
		t.Fatal(err)
	}
	if pack.Format != flows.LocalPackFormat || pack.Version != flows.LocalPackVersion || pack.Parent.Flow == nil || len(pack.Blocks) != 3 {
		t.Fatalf("exported pack = format %q version %d blocks %d", pack.Format, pack.Version, len(pack.Blocks))
	}
	if data, _ := pack.Parent.Flow.Nodes[0].Params["data"].(map[string]any); len(data) != 0 {
		t.Fatalf("manual sample data was exported: %+v", data)
	}
	return pack
}

type flowPackLostReplyWriter struct {
	header http.Header
	status int
}

func (w *flowPackLostReplyWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *flowPackLostReplyWriter) WriteHeader(status int) { w.status = status }
func (w *flowPackLostReplyWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func flowPackMuxCall(t *testing.T, mux *http.ServeMux, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	return response
}

func TestFlowPackHTTPExportPreviewImportSimulationAndLostReply(t *testing.T) {
	source, sourceAdmin := newFlowsTestServer(t)
	parent, callIDs, _ := createLocalPackSource(t, source)
	pack := exportLocalPack(t, source, sourceAdmin, parent.ID)
	if pack.Blocks[0].SourceID == pack.Blocks[1].SourceID && pack.Blocks[0].SourceRevision == pack.Blocks[1].SourceRevision {
		t.Fatal("distinct block source versions were collapsed")
	}

	dest, adminToken := newFlowsTestServer(t)
	mux := flowsHTTPMux(dest)
	missionsBefore := dest.MissionManagerV2.List()
	readerToken, _, err := dest.TokenManager.Create("pack reader", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	previewBody := localPackTestBody(t, map[string]any{"pack": pack})
	dest.Cfg.Tools.Missions.ReadOnly = true
	dest.Cfg.VirtualDesktop.ReadOnly = true
	previewResponse := flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/preview", readerToken, previewBody)
	if previewResponse.Code != http.StatusOK || previewResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("read-only preview = %d %s", previewResponse.Code, previewResponse.Body.String())
	}
	var preview flows.LocalPackPreview
	if err := json.Unmarshal(previewResponse.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Review.LocalOnly || !preview.Review.SampleDataCleared || len(preview.Proposal.Blocks) != 3 {
		t.Fatalf("preview review = %+v proposal=%+v", preview.Review, preview.Proposal)
	}
	reconcileBody := localPackTestBody(t, flows.LocalPackReconcileRequest{Proposal: preview.Proposal})
	reconcile := flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/reconcile", readerToken, reconcileBody)
	if reconcile.Code != http.StatusOK || flowsBody(t, reconcile)["status"] != "unknown" {
		t.Fatalf("read-only reconcile = %d %s", reconcile.Code, reconcile.Body.String())
	}
	blocked := flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/import", adminToken, localPackTestBody(t, flows.LocalPackImportRequest{Pack: pack, Proposal: preview.Proposal}))
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("read-only import = %d %s", blocked.Code, blocked.Body.String())
	}
	flowsBefore, err := dest.Flows.ListFlows(t.Context())
	if err != nil || len(flowsBefore) != 0 {
		t.Fatalf("preview/reconcile created flows: %+v %v", flowsBefore, err)
	}
	blocksBefore, err := dest.Flows.ListBlocks(t.Context(), "", 20)
	if err != nil || len(blocksBefore.Items) != 0 {
		t.Fatalf("preview/reconcile created blocks: %+v %v", blocksBefore, err)
	}
	if !reflect.DeepEqual(missionsBefore, dest.MissionManagerV2.List()) {
		t.Fatal("preview or reconciliation changed Mission Control state")
	}
	dest.Cfg.Tools.Missions.ReadOnly = false
	dest.Cfg.VirtualDesktop.ReadOnly = false

	importBody := localPackTestBody(t, flows.LocalPackImportRequest{Pack: pack, Proposal: preview.Proposal})
	imported := flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/import", adminToken, importBody)
	if imported.Code != http.StatusOK {
		t.Fatalf("import = %d %s", imported.Code, imported.Body.String())
	}
	var result flows.LocalPackImportResult
	if err := json.Unmarshal(imported.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !validLocalPackImportResult(result, preview.Proposal) || result.Flow.ID == parent.ID || len(result.Blocks) != 3 {
		t.Fatalf("import result = %+v", result)
	}
	mission, ok := dest.MissionManagerV2.Get(result.Flow.MissionID)
	if !ok || mission.Enabled {
		t.Fatalf("imported mission = %+v exists=%v; expected disabled", mission, ok)
	}
	for _, target := range preview.Proposal.Blocks {
		if target.LocalID == target.SourceID {
			t.Fatalf("import reused source block ID %q", target.SourceID)
		}
		if _, err := dest.Flows.GetBlock(t.Context(), target.LocalID); err != nil {
			t.Fatalf("imported block %s not found: %v", target.LocalID, err)
		}
	}
	flow, err := dest.Flows.GetFlow(t.Context(), result.Flow.ID)
	if err != nil || flow.LiveRevision != 0 || flow.DraftRevision != 1 {
		t.Fatalf("imported parent is not an unpublished draft: %+v %v", flow, err)
	}
	simulated := flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/"+result.Flow.ID+"/simulate", adminToken, `{"base_revision":1}`)
	if simulated.Code != http.StatusOK {
		t.Fatalf("import simulation = %d %s", simulated.Code, simulated.Body.String())
	}
	steps := flowsBody(t, simulated)["steps"].([]any)
	outputs := map[string]map[string]any{}
	for _, item := range steps {
		step := item.(map[string]any)
		if output, ok := step["output"].(map[string]any); ok {
			outputs[step["node_id"].(string)] = output
		}
	}
	if outputs[callIDs[0]]["greeting"] != "middle-v1 Ada" || outputs[callIDs[1]]["greeting"] != "leaf Grace" {
		t.Fatalf("simulation outputs = %+v", outputs)
	}
	db, err := sql.Open("sqlite", dest.flowsDBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var runs, pins int
	if err := db.QueryRow(`SELECT count(*) FROM flow_runs WHERE flow_id=?`, result.Flow.ID).Scan(&runs); err != nil || runs != 0 {
		t.Fatalf("import/simulation created durable runs: %d %v", runs, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM flow_block_version_pins`).Scan(&pins); err != nil || pins != 1 {
		t.Fatalf("imported nested version pins = %d %v, want 1", pins, err)
	}

	// A lost response is reconciled read-only. The test deliberately does not retry Import.
	secondPreview := flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/preview", readerToken, previewBody)
	var second flows.LocalPackPreview
	if secondPreview.Code != http.StatusOK || json.Unmarshal(secondPreview.Body.Bytes(), &second) != nil {
		t.Fatalf("second preview = %d %s", secondPreview.Code, secondPreview.Body.String())
	}
	request := httptest.NewRequest(http.MethodPost, "/api/desktop/flows/packs/import", strings.NewReader(localPackTestBody(t, flows.LocalPackImportRequest{Pack: pack, Proposal: second.Proposal})))
	request.Header.Set("Authorization", "Bearer "+adminToken)
	drop := &flowPackLostReplyWriter{}
	mux.ServeHTTP(drop, request)
	if drop.status != http.StatusOK {
		t.Fatalf("lost-reply import status = %d", drop.status)
	}
	check := flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/reconcile", readerToken, localPackTestBody(t, flows.LocalPackReconcileRequest{Proposal: second.Proposal}))
	var reconciled flows.LocalPackReconcileResult
	if check.Code != http.StatusOK || json.Unmarshal(check.Body.Bytes(), &reconciled) != nil || reconciled.Status != "applied_state_confirmed" || !validLocalPackRecords(reconciled.Flow, reconciled.Blocks, second.Proposal) {
		t.Fatalf("lost-reply reconciliation = %d %s", check.Code, check.Body.String())
	}
}

func TestFlowPackHTTPRejectsTamperingBoundsAndUnsafeDocuments(t *testing.T) {
	source, sourceAdmin := newFlowsTestServer(t)
	parent, _, _ := createLocalPackSource(t, source)
	pack := exportLocalPack(t, source, sourceAdmin, parent.ID)
	dest, token := newFlowsTestServer(t)
	mux := flowsHTTPMux(dest)
	previewBody := localPackTestBody(t, map[string]any{"pack": pack})
	previewResponse := flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/preview", token, previewBody)
	var preview flows.LocalPackPreview
	if previewResponse.Code != http.StatusOK || json.Unmarshal(previewResponse.Body.Bytes(), &preview) != nil {
		t.Fatalf("preview = %d %s", previewResponse.Code, previewResponse.Body.String())
	}
	if _, err := dest.MissionManagerV2.CreateFlowMission(preview.Proposal.ParentID, "Reserved pack owner"); err != nil {
		t.Fatalf("reserve proposed mission owner: %v", err)
	}
	ownerConflict := flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/import", token, localPackTestBody(t, flows.LocalPackImportRequest{Pack: pack, Proposal: preview.Proposal}))
	if ownerConflict.Code != http.StatusConflict || flowsBody(t, ownerConflict)["code"] != "FLOW_PACK_CONFLICT" {
		t.Fatalf("mission owner conflict = %d %s", ownerConflict.Code, ownerConflict.Body.String())
	}

	tamperedContent := pack
	contentCopy, err := pack.Parent.Flow.Clone()
	if err != nil {
		t.Fatal(err)
	}
	contentCopy.Name = "changed after preview"
	tamperedContent.Parent.Flow = contentCopy
	response := flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/preview", token, localPackTestBody(t, map[string]any{"pack": tamperedContent}))
	if response.Code != http.StatusConflict || flowsBody(t, response)["code"] != "FLOW_PACK_CONFLICT" {
		t.Fatalf("content tamper = %d %s", response.Code, response.Body.String())
	}

	badDigest := preview.Proposal
	badDigest.Digest = strings.Repeat("0", 64)
	response = flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/import", token, localPackTestBody(t, flows.LocalPackImportRequest{Pack: pack, Proposal: badDigest}))
	if response.Code != http.StatusBadRequest || flowsBody(t, response)["code"] != "FLOW_PACK_INVALID" {
		t.Fatalf("proposal digest tamper = %d %s", response.Code, response.Body.String())
	}

	for _, tc := range []struct {
		name string
		edit func(*flows.LocalPackProposal)
	}{
		{"parent source ID reuse", func(p *flows.LocalPackProposal) { p.ParentID = parent.ID }},
		{"parent ID overlaps block source", func(p *flows.LocalPackProposal) { p.ParentID = pack.Blocks[0].SourceID }},
		{"block source ID reuse", func(p *flows.LocalPackProposal) { p.Blocks[0].LocalID = p.Blocks[0].SourceID }},
		{"block target overlaps parent source", func(p *flows.LocalPackProposal) { p.Blocks[0].LocalID = pack.Parent.Flow.ID }},
	} {
		proposal := preview.Proposal
		proposal.Blocks = append([]flows.LocalPackBlockTarget(nil), preview.Proposal.Blocks...)
		tc.edit(&proposal)
		response = flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/import", token, localPackTestBody(t, flows.LocalPackImportRequest{Pack: pack, Proposal: proposal}))
		if response.Code != http.StatusBadRequest || flowsBody(t, response)["code"] != "FLOW_PACK_INVALID" {
			t.Fatalf("%s = %d %s", tc.name, response.Code, response.Body.String())
		}
	}
	crossSourceID := ""
	for _, target := range preview.Proposal.Blocks {
		for _, other := range preview.Proposal.Blocks {
			if other.SourceID != target.SourceID {
				crossSourceID = other.SourceID
				break
			}
		}
		if crossSourceID != "" {
			break
		}
	}
	if crossSourceID == "" {
		t.Fatal("fixture does not contain distinct source block IDs")
	}
	crossSource := preview.Proposal
	crossSource.Blocks = append([]flows.LocalPackBlockTarget(nil), preview.Proposal.Blocks...)
	crossSource.Blocks[0].LocalID = crossSourceID
	response = flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/import", token, localPackTestBody(t, flows.LocalPackImportRequest{Pack: pack, Proposal: crossSource}))
	if response.Code != http.StatusBadRequest || flowsBody(t, response)["code"] != "FLOW_PACK_INVALID" {
		t.Fatalf("block target overlaps another source ID = %d %s", response.Code, response.Body.String())
	}

	unsupported := pack
	unsupportedFlow, err := pack.Parent.Flow.Clone()
	if err != nil {
		t.Fatal(err)
	}
	unsupportedFlow.Nodes = append(unsupportedFlow.Nodes, flows.Node{ID: flows.NewNodeID(), Key: "disabled_unsafe", Type: "tools.send_notification", TypeVersion: 1, Settings: flows.NodeSettings{Disabled: true}, Params: map[string]any{}})
	unsupportedFlow.Normalize()
	unsupported.Parent.Flow = unsupportedFlow
	canonical, err := json.Marshal(unsupportedFlow)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(canonical)
	unsupported.Parent.Hash = hex.EncodeToString(hash[:])
	response = flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/preview", token, localPackTestBody(t, map[string]any{"pack": unsupported}))
	if response.Code != http.StatusUnprocessableEntity || flowsBody(t, response)["code"] != "FLOW_PACK_UNSUPPORTED" {
		t.Fatalf("unsupported node = %d %s", response.Code, response.Body.String())
	}

	for name, body := range map[string]string{
		"malformed": `{"pack":{}} {}`,
		"unknown field": `{"pack":{},"unexpected":true}`,
	} {
		response = flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/preview", token, body)
		if response.Code != http.StatusBadRequest || flowsBody(t, response)["code"] != "FLOW_PACK_INVALID" {
			t.Fatalf("%s body = %d %s", name, response.Code, response.Body.String())
		}
	}
	tooLarge := flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/preview", token, strings.Repeat(" ", flowPackBodyLimit+1))
	if tooLarge.Code != http.StatusRequestEntityTooLarge || flowsBody(t, tooLarge)["code"] != "FLOW_PACK_TOO_LARGE" {
		t.Fatalf("oversized body = %d %s", tooLarge.Code, tooLarge.Body.String())
	}
	if records, err := dest.Flows.ListFlows(t.Context()); err != nil || len(records) != 0 {
		t.Fatalf("rejected requests created flows: %+v %v", records, err)
	}
}

func TestFlowPackHTTPReadScopeAndSessionOrigin(t *testing.T) {
	s, adminToken := newFlowsTestServer(t)
	parent, _, _ := createLocalPackSource(t, s)
	pack := exportLocalPack(t, s, adminToken, parent.ID)
	body := localPackTestBody(t, map[string]any{"pack": pack})
	mux := flowsHTTPMux(s)
	readToken, _, err := s.TokenManager.Create("pack read only", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response := flowPackMuxCall(t, mux, http.MethodGet, "/api/desktop/flows/"+parent.ID+"/pack", readToken, ""); response.Code != http.StatusOK {
		t.Fatalf("read-scope export = %d %s", response.Code, response.Body.String())
	}
	if response := flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/preview", readToken, body); response.Code != http.StatusOK {
		t.Fatalf("read-scope preview = %d %s", response.Code, response.Body.String())
	}
	if response := flowPackMuxCall(t, mux, http.MethodPost, "/api/desktop/flows/packs/import", readToken, `{"pack":{},"proposal":{}}`); response.Code != http.StatusForbidden {
		t.Fatalf("read token imported a pack: %d %s", response.Code, response.Body.String())
	}
	if response := flowPackMuxCall(t, mux, http.MethodGet, "/api/desktop/flows/"+parent.ID+"/pack", "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous export = %d", response.Code)
	}
	callSession := func(origin string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/desktop/flows/packs/preview", strings.NewReader(body))
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		return response
	}
	if response := callSession(""); response.Code != http.StatusForbidden || flowsBody(t, response)["code"] != "FLOW_PERMISSION_DENIED" {
		t.Fatalf("session pack preview without Origin = %d %s", response.Code, response.Body.String())
	}
	if response := callSession("http://foreign.example"); response.Code != http.StatusForbidden || flowsBody(t, response)["code"] != "FLOW_PERMISSION_DENIED" {
		t.Fatalf("session pack preview with foreign Origin = %d %s", response.Code, response.Body.String())
	}
	if response := callSession("http://example.com"); response.Code != http.StatusOK {
		t.Fatalf("same-origin session pack preview = %d %s", response.Code, response.Body.String())
	}
}

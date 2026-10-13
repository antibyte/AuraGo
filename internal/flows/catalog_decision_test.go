package flows

import (
	"context"
	"math"
	"reflect"
	"testing"
	"time"

	"aurago/internal/decisions"
)

type expiredDecisionEvaluator struct{}

func (expiredDecisionEvaluator) Evaluate(ctx context.Context, _ DecisionRequest) (decisions.Response, error) {
	<-ctx.Done()
	return decisions.Response{}, NewNodeError("FLOW_AI_UNAVAILABLE", "external outcome unknown")
}

func TestAIDecisionPreservesProviderTimeoutAndProvenance(t *testing.T) {
	def := decisionDef(t)
	params := routeDecisionParams()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := def.Execute(ctx, ExecInput{Node: &Node{Params: params}, Params: params, Services: &Services{Decisions: expiredDecisionEvaluator{}}})
	if !hopeless(err) {
		t.Fatalf("provider timeout became retryable: %v", err)
	}
	response := choiceResponse("intent", "approve", .8)
	response.Execution = &decisions.ExternalExecution{Provider: "remote", ProviderID: "saved"}
	result, err := execDef(def, params, &Services{Decisions: &fakeDecisionEvaluator{response: response}})
	if err != nil || result.Output["execution"] != response.Execution {
		t.Fatalf("provenance missing: %+v %v", result, err)
	}
	response.Answers = nil
	_, err = execDef(def, params, &Services{Decisions: &fakeDecisionEvaluator{response: response}})
	if !hopeless(err) {
		t.Fatalf("malformed billed result became retryable: %v", err)
	}
}

type fakeDecisionEvaluator struct {
	request  DecisionRequest
	response decisions.Response
	err      error
}

func (f *fakeDecisionEvaluator) Evaluate(_ context.Context, request DecisionRequest) (decisions.Response, error) {
	f.request = request
	return f.response, f.err
}

func decisionDef(t *testing.T) *NodeDef {
	t.Helper()
	reg := NewRegistry()
	if err := RegisterAINodes(reg); err != nil {
		t.Fatal(err)
	}
	return lookupDef(t, reg, TypeAIDecision)
}

func routeDecisionParams() map[string]any {
	return map[string]any{
		"mode":  "route",
		"state": map[string]any{"subject": "{{trigger.data.subject}}"},
		"questions": []any{map[string]any{
			"id": "intent", "type": "choice", "instructions": "Choose the best route for {{trigger.data.subject}}.",
			"options": []any{map[string]any{"id": "approve", "label": "Approve"}, map[string]any{"id": "reject", "label": "Reject"}},
		}},
		"images":               []any{"{{trigger.data.image}}"},
		"confidence_threshold": 0.7,
	}
}

func choiceResponse(qid, selected string, confidence float64) decisions.Response {
	probabilities := map[string]float64{selected: confidence}
	other := "approve"
	if selected == other {
		other = "reject"
	}
	probabilities[other] = 1 - confidence
	return decisions.Response{Model: "jeff-2b", ModelRevision: "sha256:abc", Calibration: "uncalibrated",
		Answers: map[string]decisions.Answer{qid: {Type: "choice", Decision: selected, Choice: map[string]string{"approve": "Approve", "reject": "Reject"}[selected], Probabilities: probabilities, Confidence: confidence}},
	}
}

func TestAIDecisionCatalogAndAvailability(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterCatalog(reg, decisionEnv{}); err != nil {
		t.Fatal(err)
	}
	def := lookupDef(t, reg, TypeAIDecision)
	if def.Execute == nil || def.Validate == nil || !def.UntrustedOutput || !reflect.DeepEqual(def.ErrorModes, []ErrorMode{ErrorStop, ErrorPort}) {
		t.Fatalf("definition contract = %+v", def)
	}
	availability := def.Availability()
	if availability.State != BlockedState || availability.Code != "DECISIONS_RUNTIME_UNAVAILABLE" || availability.ConfigSection != "decisions" {
		t.Fatalf("availability = %+v", availability)
	}
	info := DescribeNodeTypes(reg, nil)
	for _, item := range info {
		if item.Type == TypeAIDecision {
			if item.DynamicOutputs != "decision_choices" || item.DynamicFields != "mode" || !reflect.DeepEqual(item.ErrorModes, []ErrorMode{ErrorStop, ErrorPort}) || item.Availability.Code != availability.Code {
				t.Fatalf("catalog info = %+v", item)
			}
			for _, param := range item.Params {
				if param.Name == "confidence_threshold" && (param.Min == nil || *param.Min != 0 || param.Max == nil || *param.Max != 1 || param.Step == nil || *param.Step != .01) {
					t.Fatalf("threshold metadata = %+v", param)
				}
			}
			return
		}
	}
	t.Fatal("decision node missing from catalog")
}

type decisionEnv struct{}

func (decisionEnv) ToolAvailability(string) Availability { return Availability{State: NeedsSetupState} }
func (decisionEnv) DecisionAvailability() Availability {
	return Availability{State: BlockedState, Code: "DECISIONS_RUNTIME_UNAVAILABLE", ConfigSection: "decisions"}
}

func TestAIDecisionPortsAreStableAndModeAware(t *testing.T) {
	def := decisionDef(t)
	params := routeDecisionParams()
	node := &Node{Params: params}
	fieldNames := func(fields []FieldSpec) []string {
		names := make([]string, len(fields))
		for index, field := range fields {
			names[index] = field.Name
		}
		return names
	}
	if got := fieldNames(def.FieldsOf(node)); !reflect.DeepEqual(got, []string{"answers", "model", "model_revision", "calibration", "runtime", "usage", "execution", "decision", "confidence"}) {
		t.Fatalf("route output fields = %v", got)
	}
	want := []string{"choice_approve", "choice_reject", "uncertain"}
	if got := def.OutputPorts(node); !reflect.DeepEqual(got, want) {
		t.Fatalf("route ports = %v", got)
	}
	questions := params["questions"].([]any)
	question := questions[0].(map[string]any)
	question["options"] = []any{question["options"].([]any)[1], question["options"].([]any)[0]}
	if got := def.OutputPorts(node); !reflect.DeepEqual(got, []string{"choice_reject", "choice_approve", "uncertain"}) {
		t.Fatalf("reordered ports = %v", got)
	}
	params["mode"] = "evaluate"
	if got := def.OutputPorts(node); !reflect.DeepEqual(got, []string{PortOut, "uncertain"}) {
		t.Fatalf("evaluate ports = %v", got)
	}
	if got := fieldNames(def.FieldsOf(node)); !reflect.DeepEqual(got, []string{"answers", "model", "model_revision", "calibration", "runtime", "usage", "execution"}) {
		t.Fatalf("evaluate output fields = %v", got)
	}
	node.Settings.OnError = ErrorPort
	if got := def.OutputPorts(node); !reflect.DeepEqual(got, []string{PortOut, "uncertain", PortError}) {
		t.Fatalf("evaluate error port = %v", got)
	}
}

func TestAIDecisionRoutesAndPassesOnlyMediaReferences(t *testing.T) {
	def := decisionDef(t)
	params := routeDecisionParams()
	media := &fakeDecisionEvaluator{response: choiceResponse("intent", "approve", .82)}
	res, err := execDef(def, params, &Services{Decisions: media})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !reflect.DeepEqual(res.Ports, []string{"choice_approve"}) || res.Output["decision"] != "approve" || res.Output["confidence"] != .82 {
		t.Fatalf("result = %+v", res)
	}
	if media.request.State == nil || !reflect.DeepEqual(media.request.Images, []string{"{{trigger.data.image}}"}) || len(media.request.Questions) != 1 || media.request.Questions[0].ID != "intent" {
		t.Fatalf("service request = %+v", media.request)
	}
	uncertain := &fakeDecisionEvaluator{response: choiceResponse("intent", "approve", .69)}
	res, err = execDef(def, params, &Services{Decisions: uncertain})
	if err != nil || !reflect.DeepEqual(res.Ports, []string{"uncertain"}) {
		t.Fatalf("uncertain result = %+v, %v", res, err)
	}
}

func TestAIDecisionEvaluatesTypedQuestionsTogether(t *testing.T) {
	def := decisionDef(t)
	params := map[string]any{
		"mode": "evaluate", "state": map[string]any{"value": 9},
		"questions": []any{
			map[string]any{"id": "intent", "type": "choice", "instructions": "Choose", "options": []any{map[string]any{"id": "a", "label": "A"}, map[string]any{"id": "b", "label": "B"}}},
			map[string]any{"id": "is_ready", "type": "noul", "instructions": "Is it ready?"},
			map[string]any{"id": "urgency", "type": "score", "instructions": "Score urgency", "options": []any{map[string]any{"id": "low", "label": "Low", "value": 0.0}, map[string]any{"id": "high", "label": "High", "value": 10.0}}},
		},
	}
	service := &fakeDecisionEvaluator{response: decisions.Response{Answers: map[string]decisions.Answer{
		"intent":   {Type: "choice", Decision: "a", Choice: "A", Confidence: .8, Probabilities: map[string]float64{"a": .8, "b": .2}},
		"is_ready": {Type: "noul", Decision: "yes", Noul: floatPtr(.8), Confidence: .8, Probabilities: map[string]float64{"no": .2, "yes": .8}},
		"urgency":  {Type: "score", Decision: "high", Score: floatPtr(8), Confidence: .8, Probabilities: map[string]float64{"low": .2, "high": .8}},
	}}}
	res, err := execDef(def, params, &Services{Decisions: service})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !reflect.DeepEqual(res.Ports, []string{PortOut}) || res.ItemCount != 3 || len(res.Output["answers"].(map[string]any)) != 3 {
		t.Fatalf("result = %+v", res)
	}
	if got := service.request.Questions[1].Options; len(got) != 2 || got[0].ID != "no" || got[1].ID != "yes" {
		t.Fatalf("noul options = %+v", got)
	}
}

func TestAIDecisionOutputsResolveInDownstreamTemplates(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(decisionDef(t))
	b := newFlow("Decision outputs")
	trigger := b.node("start", "test.trigger", nil)
	decision := b.node("decision", TypeAIDecision, map[string]any{
		"mode": "evaluate", "questions": []any{
			map[string]any{"id": "is_ready", "type": "noul", "instructions": "Is it ready?"},
			map[string]any{"id": "intent", "type": "choice", "instructions": "Choose", "options": []any{
				map[string]any{"id": "accept", "label": "Accept"}, map[string]any{"id": "reject", "label": "Reject"},
			}},
		},
	})
	after := b.node("after", "test.echo", map[string]any{
		"value": "{{decision.answers.is_ready.noul}}/{{decision.answers.intent.probabilities.accept}}",
	})
	b.edge(trigger, PortOut, decision)
	b.edge(decision, PortOut, after)
	service := &fakeDecisionEvaluator{response: decisions.Response{Answers: map[string]decisions.Answer{
		"is_ready": {Type: "noul", Decision: "yes", Noul: floatPtr(.75), Confidence: .75, Probabilities: map[string]float64{"no": .25, "yes": .75}},
		"intent":   {Type: "choice", Decision: "accept", Choice: "Accept", Confidence: .8, Probabilities: map[string]float64{"accept": .8, "reject": .2}},
	}}}
	result, _ := runWith(context.Background(), newTestEngine(reg, &Services{Decisions: service}, 4), b.build(), RunRequest{TriggerNode: trigger})
	if result.Status != RunSuccess || result.Outputs["after"]["value"] != "0.75/0.8" {
		t.Fatalf("decision fields were not template-resolvable: status=%s outputs=%#v", result.Status, result.Outputs)
	}
}

func TestAIDecisionRejectsMalformedCollections(t *testing.T) {
	def := decisionDef(t)
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "non-object question row", mutate: func(params map[string]any) {
			params["questions"] = append(params["questions"].([]any), 12)
		}},
		{name: "non-object option row", mutate: func(params map[string]any) {
			question := params["questions"].([]any)[0].(map[string]any)
			question["options"] = append(question["options"].([]any), false)
		}},
		{name: "non-list options", mutate: func(params map[string]any) {
			question := params["questions"].([]any)[0].(map[string]any)
			question["options"] = "approve,reject"
		}},
		{name: "non-canonical noul values", mutate: func(params map[string]any) {
			question := params["questions"].([]any)[0].(map[string]any)
			question["type"] = "noul"
			question["options"] = []any{
				map[string]any{"id": "no", "label": "No", "value": 0.0},
				map[string]any{"id": "yes", "label": "Yes", "value": 2.0},
			}
		}},
		{name: "non-text image row", mutate: func(params map[string]any) {
			params["images"] = []any{"media://image/a", 7}
		}},
		{name: "non-list images", mutate: func(params map[string]any) {
			params["images"] = "media://image/a"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := routeDecisionParams()
			test.mutate(params)
			if issues := def.Validate(&Node{Params: params}, ValidateContext{Mode: ModePublish}); len(issues) == 0 {
				t.Fatal("malformed collection was silently accepted")
			}
		})
	}
}

func TestAIDecisionRejectsInconsistentExpectedScore(t *testing.T) {
	def := decisionDef(t)
	params := map[string]any{"mode": "evaluate", "questions": []any{map[string]any{
		"id": "priority", "type": "score", "instructions": "Estimate priority", "options": []any{
			map[string]any{"id": "low", "label": "Low", "value": 0.0}, map[string]any{"id": "high", "label": "High", "value": 10.0},
		},
	}}}
	service := &fakeDecisionEvaluator{response: decisions.Response{Answers: map[string]decisions.Answer{
		"priority": {Type: "score", Decision: "high", Score: floatPtr(7.9), Confidence: .8, Probabilities: map[string]float64{"low": .2, "high": .8}},
	}}}
	if _, err := execDef(def, params, &Services{Decisions: service}); asNodeError(err).Code != "FLOW_AI_OUTPUT_INVALID" {
		t.Fatalf("inconsistent expected score accepted: %v", err)
	}
}

func TestAIDecisionValidationAndErrors(t *testing.T) {
	def := decisionDef(t)
	params := routeDecisionParams()
	questions := params["questions"].([]any)
	questions[0].(map[string]any)["type"] = "score"
	if issues := def.Validate(&Node{Params: params}, ValidateContext{Mode: ModePublish}); len(issues) == 0 {
		t.Fatal("route accepted a score question")
	}
	if _, err := execDef(def, routeDecisionParams(), &Services{}); asNodeError(err).Code != "FLOW_AI_UNAVAILABLE" {
		t.Fatalf("missing service = %v", err)
	}
	serviceErr := &fakeDecisionEvaluator{err: NewNodeError("DECISIONS_RUNTIME_UNAVAILABLE", "service stopped")}
	if _, err := execDef(def, routeDecisionParams(), &Services{Decisions: serviceErr}); asNodeError(err).Code != "DECISIONS_RUNTIME_UNAVAILABLE" {
		t.Fatalf("service error = %v", err)
	}
	technicalErr := &fakeDecisionEvaluator{err: NewNodeError("FLOW_NODE_FAILED", "inference failed")}
	if _, err := execDef(def, routeDecisionParams(), &Services{Decisions: technicalErr}); asNodeError(err).Code != "FLOW_NODE_FAILED" {
		t.Fatalf("technical service error was reclassified: %v", err)
	}
	timeoutErr := &fakeDecisionEvaluator{err: context.DeadlineExceeded}
	if _, err := execDef(def, routeDecisionParams(), &Services{Decisions: timeoutErr}); asNodeError(err).Code != "FLOW_NODE_TIMEOUT" {
		t.Fatalf("service timeout classification = %v", err)
	}
	bad := &fakeDecisionEvaluator{response: choiceResponse("intent", "not_a_candidate", .8)}
	if _, err := execDef(def, routeDecisionParams(), &Services{Decisions: bad}); asNodeError(err).Code != "FLOW_AI_OUTPUT_INVALID" {
		t.Fatalf("invalid answer = %v", err)
	}
	for _, confidence := range []float64{math.NaN(), math.Inf(1), -0.1, 1.1} {
		answer := choiceResponse("intent", "approve", .8)
		a := answer.Answers["intent"]
		a.Confidence = confidence
		answer.Answers["intent"] = a
		service := &fakeDecisionEvaluator{response: answer}
		if _, err := execDef(def, routeDecisionParams(), &Services{Decisions: service}); asNodeError(err).Code != "FLOW_AI_OUTPUT_INVALID" {
			t.Fatalf("confidence %v accepted: %v", confidence, err)
		}
	}
}

func TestAIDecisionRejectsContinueErrorPolicy(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterCatalog(reg, decisionEnv{}); err != nil {
		t.Fatal(err)
	}
	builder, _, _ := validFlow()
	flow := builder.build()
	node := flow.Nodes[1]
	node.Type = TypeAIDecision
	node.Settings.OnError = ErrorContinue
	node.Params = routeDecisionParams()
	flow.Nodes[1] = node
	issues := Validate(flow, reg, ValidateContext{Mode: ModePublish})
	found := false
	for _, issue := range issues {
		if issue.NodeID == node.ID && issue.Param == "on_error" && issue.Code == IssueParamInvalid {
			found = true
		}
	}
	if !found {
		t.Fatalf("continue error policy was not rejected: %+v", issues)
	}
}

func TestAIDecisionImportedContinuePolicyStillStopsAtRuntime(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(decisionDef(t))
	b := newFlow("Decision imported continue")
	trigger := b.node("start", "test.trigger", nil)
	decision := b.node("decision", TypeAIDecision, routeDecisionParams())
	after := b.node("after", "test.echo", map[string]any{"value": "should not run"})
	b.edge(trigger, PortOut, decision)
	b.edge(decision, decisionPortPrefix+"approve", after)
	flow := b.build()
	flow.NodeByID(decision).Settings.OnError = ErrorContinue
	service := &fakeDecisionEvaluator{err: NewNodeError("DECISIONS_RUNTIME_UNAVAILABLE", "runtime stopped")}
	engine := newTestEngine(reg, &Services{Decisions: service}, 4)
	result, _ := runWith(context.Background(), engine, flow, RunRequest{TriggerNode: trigger, Mode: ModeTest, TriggerData: map[string]any{
		"subject": "sample", "image": "media://image/sample",
	}})
	if result.Status != RunError || result.ErrorCode != "DECISIONS_RUNTIME_UNAVAILABLE" || result.ErrorNodeID != decision {
		t.Fatalf("result = %+v", result)
	}
	if step := stepOf(result, after); step == nil || step.Status != StepSkipped {
		t.Fatalf("downstream decision branch ran after invalid imported continue setting: %+v", step)
	}
}

func floatPtr(value float64) *float64 { return &value }

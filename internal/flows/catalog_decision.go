package flows

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"aurago/internal/decisions"
)

// TypeAIDecision is the managed multimodal decision node.
const TypeAIDecision = "ai.decision"

const (
	decisionModeRoute        = "route"
	decisionModeEvaluate     = "evaluate"
	decisionDefaultThreshold = 0.70
	maxDecisionQuestions     = 16
	maxDecisionOptions       = 62
	maxDecisionImages        = 8
	decisionPortPrefix       = "choice_"
)

var decisionIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

func aiDecisionDef(availability func() Availability) *NodeDef {
	if availability == nil {
		availability = func() Availability {
			return Availability{State: NeedsSetupState, Code: "DECISIONS_NOT_CONFIGURED", ConfigSection: "decisions"}
		}
	}
	threshold := decisionDefaultThreshold
	min, max, step := 0.0, 1.0, 0.01
	questionsDefault := []any{map[string]any{
		"id": "decision", "type": "choice", "instructions": "",
		"options": []any{
			map[string]any{"id": "option_a", "label": "Option A"},
			map[string]any{"id": "option_b", "label": "Option B"},
		},
	}}
	def := &NodeDef{
		Type: TypeAIDecision, Version: 1, Category: "ai", Icon: "git-branch", Color: "ai",
		LabelKey: "easydrag.node.ai_decision.label", DescriptionKey: "easydrag.node.ai_decision.description",
		SummaryKey: "easydrag.node.ai_decision.summary", UntrustedOutput: true, DefaultTimeout: 300 * time.Second,
		PrimaryInput: "state", ErrorModes: []ErrorMode{ErrorStop, ErrorPort},
		AvailabilityFunc: availability,
		Params: []ParamSpec{
			{Name: "mode", Kind: ParamSegmented, LabelKey: "easydrag.param.decision_mode", Default: decisionModeRoute,
				Options: []Option{option(decisionModeRoute, "decision_mode_route"), option(decisionModeEvaluate, "decision_mode_evaluate")}},
			{Name: "state", Kind: ParamJSON, LabelKey: "easydrag.param.decision_state", Templatable: true},
			{Name: "questions", Kind: ParamDecisionQuestions, LabelKey: "easydrag.param.decision_questions", Required: true, Default: questionsDefault, SensitiveSink: true},
			{Name: "images", Kind: ParamDecisionImages, LabelKey: "easydrag.param.decision_images", Templatable: true, SensitiveSink: true, Default: []any{}},
			{Name: "confidence_threshold", Kind: ParamNumber, LabelKey: "easydrag.param.decision_threshold", Default: threshold,
				Min: &min, Max: &max, Step: &step},
		},
	}
	def.Outputs = []string{PortOut, "uncertain"}
	def.OutputsFunc = decisionOutputPorts
	def.OutputFieldsFunc = func(node *Node) []FieldSpec {
		fields := []FieldSpec{
			{Name: "answers", Type: "object", Primary: true},
			{Name: "model", Type: "text"}, {Name: "model_revision", Type: "text"},
			{Name: "calibration", Type: "text"}, {Name: "runtime", Type: "object"}, {Name: "usage", Type: "object"},
			{Name: "execution", Type: "object"},
		}
		if decisionMode(node.Params) == decisionModeRoute {
			fields = append(fields, FieldSpec{Name: "decision", Type: "text"}, FieldSpec{Name: "confidence", Type: "number"})
		}
		return fields
	}
	def.Validate = validateDecisionNode
	def.Execute = executeDecisionNode
	return def
}

func decisionMode(params map[string]any) string {
	mode, _ := params["mode"].(string)
	if mode == decisionModeEvaluate {
		return mode
	}
	return decisionModeRoute
}

func decisionQuestions(params map[string]any) []map[string]any {
	list, _ := decisionRows(params["questions"])
	out := make([]map[string]any, 0, len(list))
	for _, raw := range list {
		row, ok := raw.(map[string]any)
		if ok {
			out = append(out, row)
		}
	}
	return out
}

func decisionOptions(question map[string]any) []map[string]any {
	list, _ := decisionRows(question["options"])
	out := make([]map[string]any, 0, len(list))
	for _, raw := range list {
		row, ok := raw.(map[string]any)
		if ok {
			out = append(out, row)
		}
	}
	return out
}

func decisionRows(raw any) ([]any, bool) {
	switch list := raw.(type) {
	case []any:
		return list, true
	case []map[string]any:
		rows := make([]any, len(list))
		for index := range list {
			rows[index] = list[index]
		}
		return rows, true
	default:
		return nil, false
	}
}

func decisionOutputPorts(n *Node) []string {
	if n == nil || n.Params == nil {
		return []string{decisionPortPrefix + "option_a", decisionPortPrefix + "option_b", "uncertain"}
	}
	if decisionMode(n.Params) == decisionModeEvaluate {
		return []string{PortOut, "uncertain"}
	}
	questions := decisionQuestions(n.Params)
	if len(questions) == 0 {
		return []string{decisionPortPrefix + "option_a", decisionPortPrefix + "option_b", "uncertain"}
	}
	question := questions[0]
	if question["type"] == "noul" {
		return []string{decisionPortPrefix + "no", decisionPortPrefix + "yes", "uncertain"}
	}
	ports := make([]string, 0, len(decisionOptions(question))+1)
	seen := map[string]bool{}
	for _, option := range decisionOptions(question) {
		id, _ := option["id"].(string)
		port := decisionPortPrefix + id
		if decisionIDPattern.MatchString(id) && !seen[port] {
			ports = append(ports, port)
			seen[port] = true
		}
	}
	return append(ports, "uncertain")
}

func validateDecisionNode(n *Node, _ ValidateContext) []Issue {
	if n == nil {
		return []Issue{paramIssue(nil, IssueParamInvalid, SeverityError, "questions", "decision node is missing")}
	}
	var issues []Issue
	add := func(param, message string) {
		issues = append(issues, paramIssue(n, IssueParamInvalid, SeverityError, param, message))
	}
	mode := decisionMode(n.Params)
	if v, exists := n.Params["mode"]; exists && v != decisionModeRoute && v != decisionModeEvaluate {
		add("mode", "mode must be route or evaluate")
	}
	questionsRaw, questionsList := decisionRows(n.Params["questions"])
	if !questionsList {
		add("questions", "questions must be an ordered list")
	}
	questions := make([]map[string]any, 0, len(questionsRaw))
	for index, raw := range questionsRaw {
		question, ok := raw.(map[string]any)
		if !ok {
			add(fmt.Sprintf("questions[%d]", index), "question must be an object")
			continue
		}
		questions = append(questions, question)
	}
	if len(questions) == 0 || len(questions) > maxDecisionQuestions {
		add("questions", "define between 1 and 16 questions")
	}
	if mode == decisionModeRoute && len(questions) != 1 {
		add("questions", "route mode requires exactly one question")
	}
	seenQuestions := map[string]bool{}
	for qi, q := range questions {
		param := fmt.Sprintf("questions[%d]", qi)
		id, _ := q["id"].(string)
		if !decisionIDPattern.MatchString(id) {
			add(param+".id", "question ids must use 1-32 lowercase letters, digits, or underscores, starting with a letter")
		}
		if seenQuestions[id] {
			add(param+".id", "question ids must be unique")
		}
		seenQuestions[id] = true
		typ, _ := q["type"].(string)
		if typ != "choice" && typ != "noul" && typ != "score" {
			add(param+".type", "question type must be choice, noul, or score")
		}
		instructions, _ := q["instructions"].(string)
		if strings.TrimSpace(instructions) == "" {
			add(param+".instructions", "instructions are required")
		}
		rawOptions, optionsPresent := q["options"]
		optionRows, optionsList := decisionRows(rawOptions)
		if typ != "noul" || optionsPresent {
			if !optionsList {
				add(param+".options", "options must be an ordered list")
			}
		}
		options := make([]map[string]any, 0, len(optionRows))
		for oi, rawOption := range optionRows {
			option, ok := rawOption.(map[string]any)
			if !ok {
				add(fmt.Sprintf("%s.options[%d]", param, oi), "option must be an object")
				continue
			}
			options = append(options, option)
		}
		if typ == "noul" {
			if len(options) != 0 && len(options) != 2 {
				add(param+".options", "noul options must be omitted or contain no and yes")
			}
			if len(options) == 2 {
				ids := map[string]bool{}
				for _, option := range options {
					oid, _ := option["id"].(string)
					ids[oid] = true
				}
				if !ids["no"] || !ids["yes"] {
					add(param+".options", "noul options must use the fixed ids no and yes")
				}
			}
		} else if len(options) < 2 || len(options) > maxDecisionOptions {
			add(param+".options", "define between 2 and 62 options")
		}
		if mode == decisionModeRoute && typ == "score" {
			add(param+".type", "route mode supports choice and noul questions only")
		}
		seenOptions := map[string]bool{}
		seenValues := map[float64]bool{}
		for oi, option := range options {
			optionParam := fmt.Sprintf("%s.options[%d]", param, oi)
			oid, _ := option["id"].(string)
			if !decisionIDPattern.MatchString(oid) {
				add(optionParam+".id", "option ids must use 1-32 lowercase letters, digits, or underscores, starting with a letter")
			}
			if seenOptions[oid] {
				add(optionParam+".id", "option ids must be unique")
			}
			seenOptions[oid] = true
			label, _ := option["label"].(string)
			if strings.TrimSpace(label) == "" {
				add(optionParam+".label", "option labels are required")
			}
			if typ == "choice" && option["value"] != nil {
				add(optionParam+".value", "choice options do not use numeric values")
			}
			if typ == "noul" && option["value"] != nil {
				value, ok := finiteNumber(option["value"])
				want := map[string]float64{"no": 0, "yes": 1}[oid]
				if !ok || (oid != "no" && oid != "yes") || value != want {
					add(optionParam+".value", "noul option values are fixed: no is 0 and yes is 1")
				}
			}
			if typ == "score" {
				value, hasValue := finiteNumber(option["value"])
				if option["value"] != nil && !hasValue {
					add(optionParam+".value", "score values must be finite numbers")
				}
				if !hasValue {
					value = float64(oi)
				}
				if seenValues[value] {
					add(optionParam+".value", "score values must be unique")
				}
				seenValues[value] = true
			}
		}
	}
	imageValue, imagesPresent := n.Params["images"]
	imageRows, imagesList := imageValue.([]any)
	if typedImages, ok := imageValue.([]string); ok {
		imageRows = make([]any, len(typedImages))
		for i := range typedImages {
			imageRows[i] = typedImages[i]
		}
		imagesList = true
	}
	if imagesPresent && imageValue != nil && !imagesList {
		add("images", "image references must be an ordered list")
	}
	images := make([]string, 0, len(imageRows))
	for index, raw := range imageRows {
		image, ok := raw.(string)
		if !ok {
			add(fmt.Sprintf("images[%d]", index), "image reference must be text")
			continue
		}
		images = append(images, image)
	}
	if len(images) > maxDecisionImages {
		add("images", "at most 8 image references are allowed")
	}
	for index, image := range images {
		if strings.TrimSpace(image) == "" {
			add(fmt.Sprintf("images[%d]", index), "image reference is required")
		}
	}
	if raw, exists := n.Params["confidence_threshold"]; exists && raw != nil {
		threshold, ok := finiteNumber(raw)
		if !ok || threshold < 0 || threshold > 1 {
			add("confidence_threshold", "confidence threshold must be a number from 0 to 1")
		}
	}
	return issues
}

func finiteNumber(v any) (float64, bool) {
	var value float64
	switch n := v.(type) {
	case float64:
		value = n
	case float32:
		value = float64(n)
	case int:
		value = float64(n)
	case int64:
		value = float64(n)
	default:
		return 0, false
	}
	return value, !math.IsNaN(value) && !math.IsInf(value, 0)
}

func decisionImageRefs(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		if stringsList, isStrings := raw.([]string); isStrings {
			return append([]string(nil), stringsList...)
		}
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func executeDecisionNode(ctx context.Context, in ExecInput) (ExecResult, error) {
	if in.Services == nil || in.Services.Decisions == nil {
		return ExecResult{}, NewNodeError("FLOW_AI_UNAVAILABLE", "the Decisions service is not available")
	}
	if err := ctx.Err(); err != nil {
		return ExecResult{}, invokeError(err)
	}
	mode := decisionMode(in.Params)
	if mode != decisionModeRoute && mode != decisionModeEvaluate {
		return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "mode must be route or evaluate")
	}
	resolvedNode := *in.Node
	resolvedNode.Params = in.Params
	if problems := validateDecisionNode(&resolvedNode, ValidateContext{Mode: ModePublish}); len(problems) > 0 {
		return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "%s", problems[0].Message)
	}
	questions, err := decodeDecisionQuestions(in.Params["questions"])
	if err != nil {
		return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "%s", err.Error())
	}
	images := decisionImageRefs(in.Params["images"])
	state := in.Params["state"]
	if state == nil {
		state = map[string]any{}
	}
	response, err := in.Services.Decisions.Evaluate(ctx, DecisionRequest{State: state, Questions: questions, Images: images})
	if err != nil {
		if hopeless(err) {
			return ExecResult{}, invokeError(err)
		}
		if ctx.Err() != nil {
			return ExecResult{}, invokeError(ctx.Err())
		}
		return ExecResult{}, invokeError(err)
	}
	if err := validateDecisionResponse(response, questions); err != nil {
		if response.Execution != nil {
			return ExecResult{}, NewNodeError("FLOW_AI_UNAVAILABLE", "Invalid external Decisions result: %s", err.Error())
		}
		return ExecResult{}, NewNodeError("FLOW_AI_OUTPUT_INVALID", "%s", err.Error())
	}
	threshold := decisionDefaultThreshold
	if value, ok := finiteNumber(in.Params["confidence_threshold"]); ok {
		threshold = value
	}
	answers := make(map[string]any, len(response.Answers))
	uncertain := false
	for _, question := range questions {
		answer := response.Answers[question.ID]
		answers[question.ID] = answer
		if answer.Confidence < threshold {
			uncertain = true
		}
	}
	output := map[string]any{
		"answers": answers, "model": response.Model, "model_revision": response.ModelRevision,
		"calibration": response.Calibration, "runtime": response.Runtime, "usage": response.Usage,
	}
	if response.Execution != nil {
		output["execution"] = response.Execution
	}
	ports := []string{PortOut}
	if mode == decisionModeRoute {
		answer := response.Answers[questions[0].ID]
		output["decision"] = answer.Decision
		output["confidence"] = answer.Confidence
		if uncertain {
			ports = []string{"uncertain"}
		} else {
			ports = []string{decisionPortPrefix + answer.Decision}
		}
	} else if uncertain {
		ports = []string{"uncertain"}
	}
	return ExecResult{Output: output, Ports: ports, ItemCount: len(questions)}, nil
}

func decodeDecisionQuestions(raw any) ([]decisions.Question, error) {
	rows, ok := decisionRows(raw)
	if !ok {
		return nil, errors.New("questions must be an ordered list")
	}
	out := make([]decisions.Question, 0, len(rows))
	for index, item := range rows {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("questions[%d] must be an object", index)
		}
		question := decisions.Question{ID: stringifyParam(row["id"]), Type: stringifyParam(row["type"]), Instructions: stringifyParam(row["instructions"])}
		for optionIndex, rawOption := range decisionOptions(row) {
			option := decisions.Option{ID: stringifyParam(rawOption["id"]), Label: stringifyParam(rawOption["label"])}
			if rawOption["value"] != nil {
				value, ok := finiteNumber(rawOption["value"])
				if !ok {
					return nil, fmt.Errorf("questions[%d].options[%d].value must be finite", index, optionIndex)
				}
				option.Value = &value
			}
			question.Options = append(question.Options, option)
		}
		out = append(out, question)
	}
	for index := range out {
		if out[index].Type != "noul" {
			continue
		}
		if len(out[index].Options) == 0 {
			out[index].Options = []decisions.Option{{ID: "no", Label: "No"}, {ID: "yes", Label: "Yes"}}
		} else if len(out[index].Options) == 2 && out[index].Options[0].ID == "yes" {
			out[index].Options[0], out[index].Options[1] = out[index].Options[1], out[index].Options[0]
		}
	}
	return out, nil
}

func stringifyParam(raw any) string { s, _ := raw.(string); return s }

func validateDecisionResponse(response decisions.Response, questions []decisions.Question) error {
	if len(response.Answers) != len(questions) {
		return errors.New("the Decisions service returned a different answer count")
	}
	for _, question := range questions {
		answer, ok := response.Answers[question.ID]
		if !ok || answer.Type != question.Type {
			return fmt.Errorf("the Decisions service returned an invalid answer for %s", question.ID)
		}
		if !finiteUnit(answer.Confidence) {
			return fmt.Errorf("the Decisions service returned an invalid confidence for %s", question.ID)
		}
		optionIDs, labels := map[string]bool{}, map[string]string{}
		for _, option := range question.Options {
			optionIDs[option.ID] = true
			labels[option.ID] = option.Label
		}
		if !optionIDs[answer.Decision] {
			return fmt.Errorf("the Decisions service selected an unknown option for %s", question.ID)
		}
		if len(answer.Probabilities) != len(question.Options) {
			return fmt.Errorf("the Decisions service returned incomplete probabilities for %s", question.ID)
		}
		maxProbability := -1.0
		var total float64
		for id := range optionIDs {
			probability, exists := answer.Probabilities[id]
			if !exists || !finiteUnit(probability) {
				return fmt.Errorf("the Decisions service returned invalid probabilities for %s", question.ID)
			}
			if probability > maxProbability {
				maxProbability = probability
			}
			total += probability
		}
		if math.Abs(total-1) > 1e-5 || math.Abs(maxProbability-answer.Confidence) > 1e-6 || answer.Probabilities[answer.Decision]+1e-9 < maxProbability {
			return fmt.Errorf("the Decisions service returned inconsistent probabilities for %s", question.ID)
		}
		switch question.Type {
		case "choice":
			if answer.Choice != labels[answer.Decision] || answer.Noul != nil || answer.Score != nil {
				return fmt.Errorf("the Decisions service returned invalid choice fields for %s", question.ID)
			}
		case "noul":
			yesProbability, exists := answer.Probabilities["yes"]
			if !exists || answer.Noul == nil || !finiteUnit(*answer.Noul) || math.Abs(*answer.Noul-yesProbability) > 1e-6 || answer.Score != nil || answer.Choice != "" {
				return fmt.Errorf("the Decisions service returned invalid noul fields for %s", question.ID)
			}
		case "score":
			if answer.Score == nil || math.IsNaN(*answer.Score) || math.IsInf(*answer.Score, 0) || answer.Noul != nil || answer.Choice != "" {
				return fmt.Errorf("the Decisions service returned invalid score fields for %s", question.ID)
			}
			expected := 0.0
			for index, option := range question.Options {
				value := float64(index)
				if option.Value != nil {
					value = *option.Value
				}
				expected += answer.Probabilities[option.ID] * value
			}
			tolerance := 1e-6 * math.Max(1, math.Abs(expected))
			if math.Abs(*answer.Score-expected) > tolerance {
				return fmt.Errorf("the Decisions service returned an inconsistent score for %s", question.ID)
			}
		}
	}
	return nil
}

func finiteUnit(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

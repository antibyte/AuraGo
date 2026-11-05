package flows

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
)

func childTextSafe(s string, scrub Scrubber) bool {
	return scrub.Text == nil || scrub.Text(s) == s
}

func childKeysSafe(value any, scrub Scrubber) bool {
	switch v := value.(type) {
	case map[string]any:
		seen := make(map[string]bool, len(v))
		for key, item := range v {
			out := key
			if scrub.Text != nil {
				out = scrub.Text(key)
			}
			if seen[out] || !childKeysSafe(item, scrub) {
				return false
			}
			seen[out] = true
		}
	case []any:
		for _, item := range v {
			if !childKeysSafe(item, scrub) {
				return false
			}
		}
	}
	return true
}

func childResponseTrace(source ChildTrace, scrub Scrubber) (ChildTrace, error) {
	if source.Schema != 1 || !ValidChildParentSeq(source.ParentSeq) ||
		!childTextSafe(source.RunID, scrub) || !childTextSafe(strconv.Itoa(source.ParentSeq), scrub) || len(source.Frames) > maxChildHistoryFrames || source.History.ParentSeq != source.ParentSeq {
		return ChildTrace{}, ErrChildHistoryUnavailable
	}
	switch source.History.State {
	case ChildHistoryPending, ChildHistoryComplete, ChildHistoryTruncated, ChildHistoryIncomplete, ChildHistoryLegacyUnknown:
	default:
		return ChildTrace{}, ErrChildHistoryUnavailable
	}
	out := source
	out.Frames = make([]ChildFrame, 0, len(source.Frames))
	valid := map[string]bool{}
	steps := 0
	lost := false
	for _, original := range source.Frames {
		if !ValidChildInvocationID(original.InvocationID) || !nodeIDPattern.MatchString(original.CallNodeID) || original.CallAttempt < 1 || original.Revision < 1 || !blockFlowID.MatchString(original.BlockID) || len(original.Hash) != 64 {
			return ChildTrace{}, ErrChildHistoryUnavailable
		}
		if _, err := hex.DecodeString(original.Hash); err != nil {
			return ChildTrace{}, ErrChildHistoryUnavailable
		}
		if original.ItemIndex != nil && (*original.ItemIndex < 0 || *original.ItemIndex >= maxBatchItems) {
			return ChildTrace{}, ErrChildHistoryUnavailable
		}
		switch original.Status {
		case "", RunSuccess, RunError, RunCancelled:
		default:
			return ChildTrace{}, ErrChildHistoryUnavailable
		}
		if !ValidChildInvocationID(original.InvocationID) || !childTextSafe(original.InvocationID, scrub) ||
			!childTextSafe(original.ParentInvocationID, scrub) || !childTextSafe(original.CallNodeID, scrub) ||
			!childTextSafe(original.BlockID, scrub) || !childTextSafe(original.Hash, scrub) || !childTextSafe(strconv.Itoa(original.Revision), scrub) ||
			(original.ParentInvocationID != "" && !valid[original.ParentInvocationID]) || valid[original.InvocationID] {
			lost = true
			continue
		}
		frame := original
		frame.Steps = nil
		if !childTextSafe(frame.ErrorCode, scrub) {
			frame.ErrorCode = ""
			lost = true
		}
		for ordinal, entry := range original.Steps {
			if entry.Ordinal != ordinal+1 || entry.Ordinal > maxChildHistorySteps || steps >= maxChildHistorySteps || !nodeIDPattern.MatchString(entry.Step.NodeID) {
				return ChildTrace{}, ErrChildHistoryUnavailable
			}
			if !childTextSafe(entry.Step.NodeID, scrub) || !childTextSafe(entry.Step.NodeKey, scrub) {
				lost = true
				continue
			}
			step := entry.Step
			for _, port := range step.Ports {
				if !childTextSafe(port, scrub) {
					step.Ports = nil
					lost = true
					break
				}
			}
			if !childKeysSafe(step.Params, scrub) {
				step.Params, step.ParamsTruncated = childOmitted(), true
			} else {
				step.Params, step.ParamsTruncated = childBoundValue(step.Params, step.ParamsTruncated, scrub)
			}
			if !childKeysSafe(step.Output, scrub) {
				step.Output, step.OutputTruncated = childOmitted(), true
			} else {
				step.Output, step.OutputTruncated = childBoundValue(step.Output, step.OutputTruncated, scrub)
			}
			step.ErrorMessage = ""
			if !childTextSafe(step.ErrorCode, scrub) {
				step.ErrorCode = ""
				lost = true
			}
			entry.Step = step
			frame.Steps = append(frame.Steps, entry)
			steps++
		}
		if frame.Steps == nil {
			frame.Steps = []ChildStep{}
		}
		valid[frame.InvocationID] = true
		out.Frames = append(out.Frames, frame)
	}
	out.History.RetainedInvocations = len(out.Frames)
	out.History.RetainedSteps = steps
	if lost {
		out.History.State = ChildHistoryIncomplete
		out.History.TotalInvocations = nil
		out.History.TotalSteps = nil
	}
	// Redaction can expand text. Drop complete records, not value prefixes.
	for {
		data, err := json.Marshal(out)
		if err != nil {
			return ChildTrace{}, err
		}
		if len(data) <= MaxChildTraceBytes {
			break
		}
		out.History.Truncated = true
		if out.History.State != ChildHistoryIncomplete {
			out.History.State = ChildHistoryTruncated
		}
		out.History.TotalInvocations = nil
		out.History.TotalSteps = nil
		if len(out.Frames) == 0 {
			return ChildTrace{}, ErrChildHistoryUnavailable
		}
		last := len(out.Frames) - 1
		if count := len(out.Frames[last].Steps); count > 0 {
			out.Frames[last].Steps = out.Frames[last].Steps[:count-1]
			out.History.RetainedSteps--
		} else {
			out.Frames = out.Frames[:last]
			out.History.RetainedInvocations--
		}
	}
	return out, nil
}

func childResponseDocument(source ChildTraceDocument, scrub Scrubber) ChildTraceDocument {
	if !childTextSafe(source.BlockID, scrub) || !childTextSafe(source.Hash, scrub) {
		return ChildTraceDocument{DocMissing: ChildDocumentRedacted}
	}
	if source.Doc == nil {
		return source
	}
	data, err := json.Marshal(source.Doc)
	if err != nil {
		source.Doc = nil
		source.DocMissing = ChildDocumentUnavailable
		return source
	}
	if scrub.JSON != nil {
		data, err = scrub.JSON(data)
	}
	if err != nil || len(data) > MaxDocumentBytes {
		source.Doc = nil
		source.DocMissing = ChildDocumentRedacted
		return source
	}
	doc, err := ParseFlow(data)
	if err != nil || doc.ID != source.Doc.ID || doc.Kind != source.Doc.Kind || len(doc.Nodes) != len(source.Doc.Nodes) || len(doc.Edges) != len(source.Doc.Edges) {
		source.Doc = nil
		source.DocMissing = ChildDocumentRedacted
		return source
	}
	for i := range doc.Nodes {
		if doc.Nodes[i].ID != source.Doc.Nodes[i].ID || doc.Nodes[i].Key != source.Doc.Nodes[i].Key || doc.Nodes[i].Type != source.Doc.Nodes[i].Type {
			source.Doc = nil
			source.DocMissing = ChildDocumentRedacted
			return source
		}
	}
	for i := range doc.Edges {
		if doc.Edges[i] != source.Doc.Edges[i] {
			source.Doc = nil
			source.DocMissing = ChildDocumentRedacted
			return source
		}
	}
	source.Doc = doc
	return source
}

func scrubOrdinaryResponse(value any, scrub Scrubber, out any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if scrub.JSON != nil {
		data, err = scrub.JSON(data)
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func sanitizeChildSimulation(source SimulationResult, scrub Scrubber) (SimulationResult, error) {
	base := source
	base.ChildTraces = nil
	base.ChildDocuments = nil
	var out SimulationResult
	if err := scrubOrdinaryResponse(base, scrub, &out); err != nil {
		return out, err
	}
	// Reserve the exact metadata-only envelope for every trace first, so later
	// redaction growth cannot consume a later parent's diagnostic marker.
	markers := make([]ChildTrace, 0, len(source.ChildTraces))
	traceBytes := 2
	for _, trace := range source.ChildTraces {
		if trace.RunID != "" || !ValidChildParentSeq(trace.ParentSeq) || !childTextSafe(strconv.Itoa(trace.ParentSeq), scrub) {
			return out, ErrChildHistoryUnavailable
		}
		marker := ChildTrace{Schema: 1, ParentSeq: trace.ParentSeq, Frames: []ChildFrame{}, History: ChildHistorySummary{ParentSeq: trace.ParentSeq, State: ChildHistoryTruncated, Truncated: true}}
		if trace.History.State == ChildHistoryIncomplete {
			marker.History.State = ChildHistoryIncomplete
		}
		data, err := json.Marshal(marker)
		if err != nil {
			return out, err
		}
		traceBytes += len(data) + 1
		if traceBytes > maxChildHistoryBytes {
			return out, ErrChildHistoryUnavailable
		}
		markers = append(markers, marker)
	}
	for i, trace := range source.ChildTraces {
		clean, err := childResponseTrace(trace, scrub)
		if err != nil {
			return out, err
		}
		encoded, err := json.Marshal(clean)
		if err != nil {
			return out, err
		}
		reserved, err := json.Marshal(markers[i])
		if err != nil {
			return out, err
		}
		delta := len(encoded) - len(reserved)
		if delta > maxChildHistoryBytes-traceBytes {
			clean = markers[i]
		} else {
			traceBytes += delta
		}
		out.ChildTraces = append(out.ChildTraces, clean)
	}
	// Summary identity is the explicit engine sequence, never a slice index or
	// node ID. Redaction/budget cuts may have changed the retained counts.
	summaries := make(map[int]ChildHistorySummary, len(out.ChildTraces))
	for _, trace := range out.ChildTraces {
		summaries[trace.ParentSeq] = trace.History
	}
	for i := range out.Steps {
		if out.Steps[i].ChildHistory == nil {
			continue
		}
		if summary, ok := summaries[out.Steps[i].ChildHistory.ParentSeq]; ok {
			copySummary := summary
			out.Steps[i].ChildHistory = &copySummary
		} else {
			out.Steps[i].ChildHistory = nil
		}
	}
	documents := make([]ChildTraceDocument, 0, len(source.ChildDocuments))
	docMarkers := make([]ChildTraceDocument, 0, len(source.ChildDocuments))
	docBytes := 2
	for _, doc := range source.ChildDocuments {
		clean := childResponseDocument(doc, scrub)
		marker := clean
		marker.Doc = nil
		if marker.DocMissing == "" {
			marker.DocMissing = ChildDocumentBudget
		}
		data, err := json.Marshal(marker)
		if err != nil {
			return out, err
		}
		docBytes += len(data) + 1
		if docBytes > MaxChildSimulationDocumentBytes {
			return out, ErrChildHistoryUnavailable
		}
		documents = append(documents, clean)
		docMarkers = append(docMarkers, marker)
	}
	for i, clean := range documents {
		data, err := json.Marshal(clean)
		if err != nil {
			return out, err
		}
		marker, err := json.Marshal(docMarkers[i])
		if err != nil {
			return out, err
		}
		delta := len(data) - len(marker)
		if delta > MaxChildSimulationDocumentBytes-docBytes {
			clean = docMarkers[i]
		} else {
			docBytes += delta
		}
		out.ChildDocuments = append(out.ChildDocuments, clean)
	}
	return out, nil
}

// EncodeChildHistoryResponse performs the final typed scrub. The returned JSON
// must be sent unchanged; a later untyped scrub could corrupt identities.
func EncodeChildHistoryResponse(value any, scrub Scrubber) ([]byte, error) {
	var result any
	switch v := value.(type) {
	case *ChildTrace:
		if v == nil {
			return nil, ErrChildHistoryUnavailable
		}
		clean, err := childResponseTrace(*v, scrub)
		if err != nil {
			return nil, err
		}
		result = clean
	case *ChildTraceDocument:
		if v == nil {
			return nil, ErrChildHistoryUnavailable
		}
		result = childResponseDocument(*v, scrub)
	case *SimulationResult:
		if v == nil {
			return nil, ErrChildHistoryUnavailable
		}
		clean, err := sanitizeChildSimulation(*v, scrub)
		if err != nil {
			return nil, err
		}
		result = clean
	case *BlockSimulationResult:
		if v == nil {
			return nil, ErrChildHistoryUnavailable
		}
		base := *v
		base.SimulationResult.ChildTraces = nil
		base.SimulationResult.ChildDocuments = nil
		var clean BlockSimulationResult
		if err := scrubOrdinaryResponse(base, scrub, &clean); err != nil {
			return nil, err
		}
		sim, err := sanitizeChildSimulation(v.SimulationResult, scrub)
		if err != nil {
			return nil, err
		}
		clean.SimulationResult = sim
		result = clean
	case *ScenarioSimulationResult:
		if v == nil || v.SimulationResult == nil {
			return nil, ErrChildHistoryUnavailable
		}
		base := *v
		plain := *v.SimulationResult
		plain.ChildTraces = nil
		plain.ChildDocuments = nil
		base.SimulationResult = &plain
		var clean ScenarioSimulationResult
		if err := scrubOrdinaryResponse(base, scrub, &clean); err != nil {
			return nil, err
		}
		sim, err := sanitizeChildSimulation(*v.SimulationResult, scrub)
		if err != nil {
			return nil, err
		}
		clean.SimulationResult = &sim
		result = clean
	default:
		return nil, errors.New("unsupported child history response")
	}
	return json.Marshal(result)
}

package flows

import "time"

// Severity of a validation issue.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Issue is one validation finding. The UI translates Code; Message is English for logs and agents.
type Issue struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	NodeID   string   `json:"node_id,omitempty"`
	EdgeID   string   `json:"edge_id,omitempty"`
	Param    string   `json:"param,omitempty"`
	Message  string   `json:"message"`
}

// ValidationMode selects draft (save) or publish rules.
type ValidationMode int

const (
	ModeDraft ValidationMode = iota
	ModePublish
)

// ValidateContext is passed to Validate and to node-specific validators.
type ValidateContext struct {
	Mode     ValidationMode
	Now      time.Time
	Location *time.Location
	Flow     *Flow
}

// Issue codes.
const (
	IssueSchema                  = "FLOW_SCHEMA"
	IssueNameRequired            = "FLOW_NAME_REQUIRED"
	IssueTooManyNodes            = "FLOW_TOO_MANY_NODES"
	IssueNoTrigger               = "FLOW_NO_TRIGGER"
	IssueCycle                   = "FLOW_CYCLE"
	IssueNodeIDInvalid           = "NODE_ID_INVALID"
	IssueNodeIDDuplicate         = "NODE_ID_DUPLICATE"
	IssueNodeKeyInvalid          = "NODE_KEY_INVALID"
	IssueNodeKeyReserved         = "NODE_KEY_RESERVED"
	IssueNodeKeyDuplicate        = "NODE_KEY_DUPLICATE"
	IssueNodeTypeUnknown         = "NODE_TYPE_UNKNOWN"
	IssueNodeUnavailable         = "NODE_UNAVAILABLE"
	IssueNodeUnreachable         = "NODE_UNREACHABLE"
	IssueParamRequired           = "PARAM_REQUIRED"
	IssueParamInvalid            = "PARAM_INVALID"
	IssueEdgeIDDuplicate         = "EDGE_ID_DUPLICATE"
	IssueEdgeNodeMissing         = "EDGE_NODE_MISSING"
	IssueEdgePortInvalid         = "EDGE_PORT_INVALID"
	IssueEdgeDuplicate           = "EDGE_DUPLICATE"
	IssueEdgeSelf                = "EDGE_SELF"
	IssueTemplateSyntax          = "TEMPLATE_SYNTAX"
	IssueTemplateUnknownRoot     = "TEMPLATE_UNKNOWN_ROOT"
	IssueTemplateNotUpstream     = "TEMPLATE_NOT_UPSTREAM"
	IssueTemplateRootUnavailable = "TEMPLATE_ROOT_UNAVAILABLE"
	IssueTemplateUnknownField    = "TEMPLATE_UNKNOWN_FIELD"
	IssueUntrustedData           = "UNTRUSTED_DATA_TO_SINK"
)

// HasErrors reports whether any issue has error severity.
func HasErrors(issues []Issue) bool {
	for _, is := range issues {
		if is.Severity == SeverityError {
			return true
		}
	}
	return false
}

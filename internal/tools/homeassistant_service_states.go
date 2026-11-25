package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"aurago/internal/security"
)

// HAChangedState is one entity state Home Assistant reports as changed by a service call.
type HAChangedState struct {
	EntityID    string         `json:"entity_id"`
	State       string         `json:"state"`
	Attributes  map[string]any `json:"attributes"`
	LastChanged time.Time      `json:"last_changed"`
	LastUpdated time.Time      `json:"last_updated"`
}

// HAServiceErrorKind classifies a failed HACallServiceStates call.
type HAServiceErrorKind string

const (
	HAServiceInvalid     HAServiceErrorKind = "invalid"     // bad domain, service, entity id or data; nothing was sent
	HAServicePolicy      HAServiceErrorKind = "policy"      // read-only mode, service or target policy refused; nothing was sent
	HAServiceUnreachable HAServiceErrorKind = "unreachable" // no answer: transport error or cancellation (a slow answer ends at the hub's command timeout first: 502)
	HAServiceAuth        HAServiceErrorKind = "auth"        // HTTP 401 or 403
	HAServiceRejected    HAServiceErrorKind = "rejected"    // another 4xx: Home Assistant refused the call
	HAServiceUpstream    HAServiceErrorKind = "upstream"    // HTTP 5xx
	HAServiceBadAnswer   HAServiceErrorKind = "bad_answer"  // 2xx whose body is not a state list (the call ran)
)

// HAServiceError reports a failed typed service call. Error() never contains Home Assistant's answer.
// Detail is a fixed English text (for policy refusals the shared refusal, which names only the
// service and config keys). LogDetail is a bounded, scrubbed excerpt of the answer for the server log
// only; never show it to a client. Err is the transport or decode error (log only).
type HAServiceError struct {
	Kind      HAServiceErrorKind
	Status    int
	Detail    string
	LogDetail string
	Err       error
}

func (e *HAServiceError) Error() string {
	msg := "home assistant service call failed (" + string(e.Kind) + ")"
	if e.Status != 0 {
		msg += fmt.Sprintf(" HTTP %d", e.Status)
	}
	if e.Kind == HAServicePolicy || e.Kind == HAServiceInvalid {
		msg += ": " + e.Detail
	}
	return msg
}

func (e *HAServiceError) Unwrap() error { return e.Err }

const (
	haServiceAnswerLimit int64 = 2 << 20
	haServiceMaxStates         = 512
	haLogExcerptBytes          = 256
)

var haServiceNameRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// haTargetKeys select targets or override the call; the typed call targets exactly one entity.
var haTargetKeys = []string{"entity_id", "target", "area_id", "device_id", "label_id", "floor_id", "domain", "service"}

// HACallServiceStates calls one Home Assistant service for one concrete entity under the same gates
// as HACallServiceContext (home_assistant.readonly, allowed/blocked services, target refusal) and
// returns the states Home Assistant reports as changed by the call. It is HomeDash's command path;
// HACallServiceContext (agent tool, HA Switchboard) is unchanged.
func HACallServiceStates(ctx context.Context, cfg HAConfig, domain, service, entityID string, data map[string]any) ([]HAChangedState, error) {
	// No lowercasing: the typed call is stricter than HACallServiceContext and rejects "Light".
	domain, service = strings.TrimSpace(domain), strings.TrimSpace(service)
	if !haServiceNameRe.MatchString(domain) || !haServiceNameRe.MatchString(service) {
		return nil, &HAServiceError{Kind: HAServiceInvalid, Detail: "invalid domain or service"}
	}
	if !haConcreteEntity.MatchString(entityID) || strings.HasPrefix(entityID, "group.") {
		return nil, &HAServiceError{Kind: HAServiceInvalid, Detail: "the call needs one concrete entity id"}
	}
	payload := map[string]any{"entity_id": entityID}
	for k, v := range data {
		if slices.Contains(haTargetKeys, k) {
			return nil, &HAServiceError{Kind: HAServiceInvalid, Detail: "service data must not select targets"}
		}
		payload[k] = v
	}
	if cfg.ReadOnly {
		return nil, &HAServiceError{Kind: HAServicePolicy, Detail: "Home Assistant is in read-only mode (home_assistant.readonly)"}
	}
	if msg := HAServiceTargetRefusal(domain, service, payload, cfg.AllowedServices, cfg.BlockedServices); msg != "" {
		return nil, &HAServiceError{Kind: HAServicePolicy, Detail: msg}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &HAServiceError{Kind: HAServiceInvalid, Detail: "service data is not JSON", Err: err}
	}
	raw, code, _, err := haDo(ctx, cfg, http.MethodPost, haServiceEndpoint(domain, service), string(body), haServiceAnswerLimit)
	excerpt := haLogExcerpt(raw)
	switch {
	case err != nil && code == 0:
		return nil, &HAServiceError{Kind: HAServiceUnreachable, Detail: "Home Assistant did not answer", Err: err}
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return nil, &HAServiceError{Kind: HAServiceAuth, Status: code, Detail: "Home Assistant refused the access token", LogDetail: excerpt}
	case code >= 500:
		return nil, &HAServiceError{Kind: HAServiceUpstream, Status: code, Detail: "Home Assistant failed", LogDetail: excerpt}
	case code < 200 || code > 299:
		return nil, &HAServiceError{Kind: HAServiceRejected, Status: code, Detail: "Home Assistant rejected the call", LogDetail: excerpt}
	case err != nil:
		return nil, &HAServiceError{Kind: HAServiceBadAnswer, Status: code, Detail: "the answer could not be read", LogDetail: excerpt, Err: err}
	}
	states, ok := decodeHAChangedStates(raw)
	if !ok {
		return nil, &HAServiceError{Kind: HAServiceBadAnswer, Status: code, Detail: "the answer is not a state list", LogDetail: excerpt}
	}
	return states, nil
}

// decodeHAChangedStates reads a plain state list or {"changed_states": [...]}; an empty body is an
// empty list. Entries with invalid ids are dropped; at most haServiceMaxStates are kept.
func decodeHAChangedStates(raw []byte) ([]HAChangedState, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return []HAChangedState{}, true
	}
	var list []HAChangedState
	if trimmed[0] == '[' {
		if json.Unmarshal(trimmed, &list) != nil {
			return nil, false
		}
	} else {
		var wrapped struct {
			ChangedStates []HAChangedState `json:"changed_states"`
		}
		if json.Unmarshal(trimmed, &wrapped) != nil || wrapped.ChangedStates == nil {
			return nil, false
		}
		list = wrapped.ChangedStates
	}
	out := make([]HAChangedState, 0, min(len(list), haServiceMaxStates))
	for _, st := range list {
		if len(out) >= haServiceMaxStates {
			break
		}
		if haConcreteEntity.MatchString(st.EntityID) {
			out = append(out, st)
		}
	}
	return out, true
}

// haLogExcerpt bounds and scrubs an answer body for the server log. It is never shown to clients.
func haLogExcerpt(raw []byte) string {
	if len(raw) > haLogExcerptBytes {
		raw = raw[:haLogExcerptBytes]
	}
	s := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, strings.ToValidUTF8(string(raw), ""))
	return security.Scrub(strings.TrimSpace(s))
}

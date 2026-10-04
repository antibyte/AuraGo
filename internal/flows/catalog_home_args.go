package flows

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Parameter readers and limits of the smart home and planner nodes
// (catalog_actions_home.go). Every reader returns the value the tool gets or a
// FLOW_PARAM_INVALID whose message never echoes the value, and the node validators run
// the same readers on literal parameters, so what Validate rejects Execute rejects too.

const (
	// maxHAEntityBytes is Home Assistant's own limit for an entity id.
	maxHAEntityBytes = 255
	// maxHAServiceBytes bounds "domain.service". Real names are far shorter.
	maxHAServiceBytes = 128
	// maxHAServiceDataBytes, maxHAServiceDataDepth and maxHAServiceDataValues bound
	// service_data: the encoded size, the nesting of lists and objects, and the number
	// of values. Service data is a handful of settings (brightness, colour, a message);
	// the limits keep a template that points at a large record from being posted to
	// Home Assistant, and keep the walk below cheap whatever the value is.
	maxHAServiceDataBytes  = 64 << 10
	maxHAServiceDataDepth  = 20
	maxHAServiceDataValues = 1 << 16

	// maxMQTTTopicBytes bounds a topic. MQTT allows 65535 bytes, real topics are a
	// few dozen, and the topic is echoed in the output.
	maxMQTTTopicBytes = 1024
	// maxMQTTPayloadBytes is the default limit of the AuraGo MQTT client
	// (mqtt.buffer.max_payload_bytes, 256 KiB). A lower configured limit makes the
	// tool refuse a payload that this check let through.
	maxMQTTPayloadBytes = 256 << 10

	// maxPlannerTitleRunes bounds a planner title. The titles of open todos and
	// upcoming appointments are listed in the agent's prompt, so a title is one short
	// line.
	maxPlannerTitleRunes = 500
	// maxPlannerDescriptionBytes bounds the description of an appointment or a todo.
	maxPlannerDescriptionBytes = 64 << 10
	// maxPlannerTimeBytes bounds the text of a date and time (the longest layout is
	// about 35 bytes).
	maxPlannerTimeBytes = 64
	// maxRemindMinutes is the longest reminder lead time, one year.
	maxRemindMinutes = 366 * 24 * 60
)

// Choices of the enumerated parameters; the definitions say which one is the default.
var (
	haOperations   = []string{"call_service", "get_state"}
	todoPriorities = []string{"low", "medium", "high"}
)

// isHAIdent reports whether s is a Home Assistant identifier: one or more lower case
// letters, digits or underscores.
func isHAIdent(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

// haEntityID reads an entity id: "domain.object_id" in lower case letters, digits and
// underscores, as Home Assistant writes them. That leaves out what the tool would pass
// on as it is, "all", comma separated lists and anything that is not an id. The text
// is not echoed.
func haEntityID(v any) (string, error) {
	s, err := scalarTextParam(v, "the entity")
	if err != nil {
		return "", err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", NewNodeError("FLOW_PARAM_INVALID", "choose an entity")
	}
	domain, object, ok := strings.Cut(s, ".")
	if len(s) > maxHAEntityBytes || !ok || !isHAIdent(domain) || !isHAIdent(object) {
		return "", NewNodeError("FLOW_PARAM_INVALID", "the entity must be an entity id such as light.living_room (lower case letters, digits and underscores)")
	}
	return s, nil
}

// haServiceName reads the service: "service" or "domain.service", each part lower case
// letters, digits and underscores. The domain is "" when the text has none. The text is
// not echoed.
func haServiceName(v any) (domain, service string, err error) {
	s, err := scalarTextParam(v, "the service")
	if err != nil {
		return "", "", err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", NewNodeError("FLOW_PARAM_INVALID", "enter a service such as turn_on")
	}
	service = s
	hasDomain := false
	if d, rest, ok := strings.Cut(s, "."); ok {
		domain, service, hasDomain = d, rest, true
	}
	if len(s) > maxHAServiceBytes || !isHAIdent(service) || hasDomain && !isHAIdent(domain) {
		return "", "", NewNodeError("FLOW_PARAM_INVALID", "the service must be written like turn_on or light.turn_on (lower case letters, digits and underscores)")
	}
	return domain, service, nil
}

// errHAServiceTemplate is the error for a service that is not written out: what the
// node calls on Home Assistant is decided by the document's author, never by data that
// flows through the run.
func errHAServiceTemplate() error {
	return NewNodeError("FLOW_PARAM_INVALID", "the service must be written out; it cannot be taken from another node")
}

// haServiceParam reads the service of a running node. The parameter is not
// templatable: a template in the node's own (raw) parameter fails, even though the run
// has resolved it to a valid name, so data from a webhook or a mail cannot choose the
// service. A caller without the raw node only has the resolved value to judge.
func haServiceParam(in ExecInput) (domain, service string, err error) {
	if in.Node != nil && isTemplateText(in.Node.Params["service"]) {
		return "", "", errHAServiceTemplate()
	}
	return haServiceName(in.Params["service"])
}

// valueFits reports whether v nests at most depth levels of lists and objects and
// holds at most *budget values altogether (every value counts, containers included).
// It only follows the JSON shapes lists and objects; a cycle runs into the depth limit.
func valueFits(v any, depth int, budget *int) bool {
	*budget--
	if *budget < 0 {
		return false
	}
	switch x := v.(type) {
	case map[string]any:
		if depth < 1 {
			return false
		}
		for _, item := range x {
			if !valueFits(item, depth-1, budget) {
				return false
			}
		}
	case []any:
		if depth < 1 {
			return false
		}
		for _, item := range x {
			if !valueFits(item, depth-1, budget) {
				return false
			}
		}
	}
	return true
}

// haServiceData reads the optional service_data: nothing, null, blank text and an empty
// object mean no data; anything else must be an object of at most
// maxHAServiceDataBytes encoded bytes, maxHAServiceDataDepth levels and
// maxHAServiceDataValues values that can be encoded as JSON. A text that holds JSON is
// not parsed: it is not an object. The result is the object itself (read-only), nil
// for none.
//
// The Home Assistant tool drops the keys entity_id, domain and service from the data
// without a word (they come from the other parameters); this node passes them on and
// leaves that to the tool.
func haServiceData(v any) (map[string]any, error) {
	switch x := v.(type) {
	case nil:
		return nil, nil
	case string:
		if strings.TrimSpace(x) == "" {
			return nil, nil
		}
	case map[string]any:
		if len(x) == 0 {
			return nil, nil
		}
		budget := maxHAServiceDataValues
		if !valueFits(x, maxHAServiceDataDepth, &budget) {
			return nil, NewNodeError("FLOW_PARAM_INVALID", "service_data is nested more than %d levels deep or holds more than %d values", maxHAServiceDataDepth, maxHAServiceDataValues)
		}
		s, err := marshalCompact(x)
		if err != nil {
			return nil, NewNodeError("FLOW_PARAM_INVALID", "service_data holds a value that is not valid JSON")
		}
		if len(s) > maxHAServiceDataBytes {
			return nil, tooLong("service data", len(s), maxHAServiceDataBytes, "bytes")
		}
		return x, nil
	}
	return nil, NewNodeError("FLOW_PARAM_INVALID", "service_data must be an object")
}

// mqttTopic reads the topic to publish to: text, trimmed, 1 to maxMQTTTopicBytes bytes,
// valid UTF-8 without control characters, and without the wildcards + and # (the
// client refuses those on a publish, and a NUL byte). The text is not echoed.
func mqttTopic(v any) (string, error) {
	s, err := scalarTextParam(v, "the topic")
	if err != nil {
		return "", err
	}
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", NewNodeError("FLOW_PARAM_INVALID", "enter a topic")
	case len(s) > maxMQTTTopicBytes:
		return "", tooLong("topic", len(s), maxMQTTTopicBytes, "bytes")
	case !validHeaderValue(s) || strings.ContainsRune(s, '\t'):
		return "", NewNodeError("FLOW_PARAM_INVALID", "the topic contains characters that are not allowed")
	case strings.ContainsAny(s, "+#"):
		return "", NewNodeError("FLOW_PARAM_INVALID", "a topic to publish to cannot contain the wildcards + or #")
	}
	return s, nil
}

// mqttPayload reads the payload: text is sent as it is, a list or object as compact
// JSON (a value that cannot be encoded fails the node), other scalars in their natural
// form, nothing as the empty payload (with retain on, that clears the retained message
// of the topic, which is what MQTT defines). At most maxMQTTPayloadBytes of valid UTF-8:
// a payload with broken text is refused instead of being repaired, as a changed byte
// can change the meaning of a command.
func mqttPayload(v any) (string, error) {
	s, err := documentText("the payload", v)
	if err != nil {
		return "", err
	}
	switch {
	case len(s) > maxMQTTPayloadBytes:
		return "", tooLong("payload", len(s), maxMQTTPayloadBytes, "bytes")
	case !utf8.ValidString(s):
		return "", NewNodeError("FLOW_PARAM_INVALID", "the payload is not valid text")
	}
	return s, nil
}

// mqttQoS reads the quality of service: 0, 1 or 2 as a number or as text; nothing and
// blank text are 0. Anything else fails, where the tool would silently use the
// configured default for a value outside 0 to 2.
func mqttQoS(v any) (int, error) {
	switch x := v.(type) {
	case nil:
		return 0, nil
	case string:
		switch strings.TrimSpace(x) {
		case "", "0":
			return 0, nil
		case "1":
			return 1, nil
		case "2":
			return 2, nil
		}
	case float64, float32, int, int64:
		if f, ok := toNumber(x); ok && (f == 0 || f == 1 || f == 2) {
			return int(f), nil
		}
	}
	return 0, NewNodeError("FLOW_PARAM_INVALID", "qos must be 0, 1 or 2")
}

// mqttRetain reads the retain flag. Nothing and blank text are off; anything else must
// be a flag flagValue reads. A retained message stays on the broker and goes to every
// later subscriber, so a value that cannot be read fails the node instead of being
// guessed.
func mqttRetain(v any) (bool, error) {
	switch x := v.(type) {
	case nil:
		return false, nil
	case string:
		if strings.TrimSpace(x) == "" {
			return false, nil
		}
	}
	on, known := flagValue(v)
	if !known {
		return false, NewNodeError("FLOW_PARAM_INVALID", "retain must be true or false")
	}
	return on, nil
}

// plannerTitle reads the required title of an appointment or a todo: one line of text,
// trimmed, at most maxPlannerTitleRunes characters.
func plannerTitle(v any) (string, error) {
	s, err := scalarTextParam(v, "the title")
	if err != nil {
		return "", err
	}
	s = strings.TrimSpace(s)
	switch n := utf8.RuneCountInString(s); {
	case s == "":
		return "", NewNodeError("FLOW_PARAM_INVALID", "enter a title")
	case n > maxPlannerTitleRunes:
		return "", tooLong("title", n, maxPlannerTitleRunes, "characters")
	case !validHeaderValue(s) || strings.ContainsRune(s, '\t'):
		return "", NewNodeError("FLOW_PARAM_INVALID", "the title contains characters that are not allowed")
	}
	return s, nil
}

// plannerDescription reads the optional description: text, or a list or object as
// compact JSON, trimmed, at most maxPlannerDescriptionBytes, broken UTF-8 replaced.
// Nothing, blank text and an empty list or object mean none.
func plannerDescription(v any) (string, error) {
	if isEmptyValue(v) {
		return "", nil
	}
	s, err := documentText("the description", v)
	if err != nil {
		return "", err
	}
	if s = strings.TrimSpace(s); len(s) > maxPlannerDescriptionBytes {
		return "", tooLong("description", len(s), maxPlannerDescriptionBytes, "bytes")
	}
	return validUTF8(s), nil
}

// plannerTime reads an optional date and time as text (the layouts of toTime; one
// without a zone is read in loc). ok is false for nothing or blank text. The text is
// not echoed.
func plannerTime(v any, loc *time.Location, what string) (time.Time, bool, error) {
	s, err := scalarTextParam(v, what)
	if err != nil {
		return time.Time{}, false, err
	}
	if s = strings.TrimSpace(s); s == "" {
		return time.Time{}, false, nil
	}
	if len(s) <= maxPlannerTimeBytes {
		if at, ok := toTime(s, loc); ok {
			// The nodes send the time in UTC and the planner reads RFC 3339, which has
			// four digit years: a time whose UTC year is outside 0 to 9999 (a date near
			// the edge in a zone with an offset) cannot be sent.
			if y := at.UTC().Year(); y < 0 || y > 9999 {
				return time.Time{}, false, NewNodeError("FLOW_PARAM_INVALID", "%s is out of range (the planner takes years 0 to 9999 in UTC)", what)
			}
			return at, true, nil
		}
	}
	return time.Time{}, false, NewNodeError("FLOW_PARAM_INVALID", "%s is not a date and time (write 2026-10-05 09:00)", what)
}

// remindMinutes reads the reminder lead time in minutes: a number from 0 to
// maxRemindMinutes, as a number or as text; nothing and blank text are 0, no reminder.
func remindMinutes(v any) (float64, error) {
	switch x := v.(type) {
	case nil:
		return 0, nil
	case string:
		if strings.TrimSpace(x) == "" {
			return 0, nil
		}
	}
	m, ok := toNumber(v)
	if !ok || m < 0 || m > maxRemindMinutes {
		return 0, NewNodeError("FLOW_PARAM_INVALID", "remind_minutes must be a number from 0 to %d", maxRemindMinutes)
	}
	return m, nil
}

// readerCheck adapts a reader that returns something other than text to the
// signature literalIssue wants.
func readerCheck[T any](read func(any) (T, error)) func(any) (string, error) {
	return func(v any) (string, error) {
		_, err := read(v)
		return "", err
	}
}

// timeCheck is the check literalIssue runs for a date and time parameter.
func timeCheck(loc *time.Location, what string) func(any) (string, error) {
	return func(v any) (string, error) {
		_, _, err := plannerTime(v, loc, what)
		return "", err
	}
}

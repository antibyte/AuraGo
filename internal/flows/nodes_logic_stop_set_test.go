package flows

import (
	"reflect"
	"strings"
	"testing"
)

// A stop node whose status is neither success nor error must not end the run as a
// success: the status picks whether the run counts as failed, so an unreadable one is a
// parameter error (final at once), and the publish check reports it. Absent, null,
// blank text and a template are left alone, as for every enumerated parameter.
func TestStopNodeRefusesAnUnknownStatus(t *testing.T) {
	for _, status := range []any{"succes", "failed", "ok", "1", 1.0, true, []any{"error"}, map[string]any{}} {
		res, err := execLogic(t, TypeStop, map[string]any{"status": status, "message": "m"}, nil, nil)
		ne := asNodeError(err)
		if ne == nil || ne.Code != "FLOW_PARAM_INVALID" || !hopeless(err) || res.Stop != nil {
			t.Errorf("status %#v: result %+v, error %v; want a non-retryable FLOW_PARAM_INVALID and no stop", status, res, err)
		}
		wantOneIssue(t, "status", validateLogic(t, TypeStop, map[string]any{"status": status}), IssueParamInvalid, "status")
	}
	for status, want := range map[string]RunStatus{"success": RunSuccess, "error": RunError, " Error ": RunError, "SUCCESS": RunSuccess, "": RunSuccess} {
		res, err := execLogic(t, TypeStop, map[string]any{"status": status}, nil, nil)
		if err != nil || res.Stop == nil || res.Stop.Status != want {
			t.Errorf("status %q: result %+v, error %v; want a stop with %s", status, res, err, want)
		}
	}
	for _, status := range []any{nil, "", "success", "error", "{{trigger.data.s}}"} {
		if issues := validateLogic(t, TypeStop, map[string]any{"status": status}); len(issues) != 0 {
			t.Errorf("status %#v: unexpected issues %+v", status, issues)
		}
	}
	if issues := validateLogic(t, TypeStop, nil); len(issues) != 0 {
		t.Errorf("no params: %+v", issues)
	}
}

// A literal value that its declared type cannot take fails the run every time, so
// publishing refuses it. A template, a value that is not there yet (null) and the
// types that take anything are not judged; a type that is itself unknown keeps its own
// single issue.
func TestSetValidatorRefusesLiteralsThatDoNotConvert(t *testing.T) {
	validate := func(typ string, value any) []Issue {
		return validateLogic(t, TypeSet, map[string]any{"fields": []any{map[string]any{"name": "x", "type": typ, "value": value}}})
	}
	for _, c := range []struct {
		typ   string
		value any
	}{
		{"number", "abc"}, {"number", ""}, {"number", true}, {"number", []any{1.0}}, {"number", "1e999"},
		{"bool", "maybe"}, {"bool", "enabled"},
		{"list", "nope"}, {"list", 5.0}, {"list", ""}, {"object", "[1]"}, {"object", "x"}, {"object", []any{}},
	} {
		wantOneIssue(t, c.typ+" "+Stringify(c.value), validate(c.typ, c.value), IssueParamInvalid, "fields[0].value")
	}
	for _, c := range []struct {
		typ   string
		value any
	}{
		{"number", "42"}, {"number", 4.5}, {"number", " 7 "}, {"number", nil}, {"number", "{{trigger.data.n}}"}, {"number", "1{{a.b}}"},
		{"bool", "ja"}, {"bool", "Nein"}, {"bool", true}, {"bool", 0.0}, {"bool", nil}, {"bool", ""}, {"bool", "{{trigger.data.b}}"},
		{"list", `[1]`}, {"list", []any{}}, {"list", "{{trigger.data.l}}"}, {"object", `{"a":1}`}, {"object", map[string]any{}},
		{"text", []any{"x"}}, {"text", nil}, {"auto", map[string]any{"a": "b"}}, {"", "anything"},
	} {
		if issues := validate(c.typ, c.value); len(issues) != 0 {
			t.Errorf("%s %#v: unexpected issues %+v", c.typ, c.value, issues)
		}
	}
	// An unknown type is reported once, by the type check.
	wantOneIssue(t, "unknown type", validate("bogus", "abc"), IssueParamInvalid, "fields[0].type")
	// A field without a name is skipped at run time, so its value is not judged.
	issues := validateLogic(t, TypeSet, map[string]any{"fields": []any{map[string]any{"name": " ", "type": "number", "value": "abc"}}})
	wantOneIssue(t, "unnamed field", issues, IssueParamRequired, "fields[0].name")
	// The message names the field and stays short whatever the value is.
	long := validate("number", strings.Repeat("ü", 10000))
	if len(long) != 1 || !strings.Contains(long[0].Message, `"x"`) || len(long[0].Message) > 300 {
		t.Errorf("long value: %+v", long)
	}
}

// A bool takes the words flagValue reads (true, yes, 1, ja, on / false, no, 0, nein, off),
// numbers, containers and nothing; any other text used to become false silently and now
// fails with FLOW_VALUE_TYPE, so a typo cannot switch a flag off.
func TestSetBoolRefusesUnknownText(t *testing.T) {
	for _, c := range []struct {
		value any
		want  bool
	}{
		{"true", true}, {" Yes ", true}, {"ON", true}, {"1", true}, {"ja", true}, {true, true}, {2.0, true}, {[]any{1.0}, true}, {map[string]any{"a": 1.0}, true},
		{"false", false}, {"No", false}, {" off\n", false}, {"0", false}, {"NEIN", false}, {false, false}, {0.0, false}, {nil, false}, {"", false}, {"  ", false},
		{[]any{}, false}, {map[string]any{}, false},
	} {
		got, err := convertValue(c.value, "bool")
		if err != nil || got != c.want {
			t.Errorf("bool %#v = %#v, %v; want %v", c.value, got, err, c.want)
		}
	}
	for _, value := range []any{"maybe", "enabled", "tru", "2", "y"} {
		_, err := execLogic(t, TypeSet, map[string]any{"fields": []any{map[string]any{"name": "b", "type": "bool", "value": value}}}, nil, nil)
		if ne := asNodeError(err); ne == nil || ne.Code != "FLOW_VALUE_TYPE" || !hopeless(err) {
			t.Errorf("bool %#v: error = %v", value, err)
		}
	}
	// The other types and auto keep passing the value through.
	res, err := execLogic(t, TypeSet, map[string]any{"fields": []any{map[string]any{"name": "a", "value": "maybe"}}}, nil, nil)
	if err != nil || !reflect.DeepEqual(res.Output, map[string]any{"a": "maybe"}) {
		t.Errorf("auto = %#v, %v", res.Output, err)
	}
}

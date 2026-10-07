package agent

import "testing"

// 1c-13 B9: the flow tool invoker passes mqtt_publish's qos as an int and retain as a bool
// (internal/server/flows_tool_invoker_args.go flowNormalizeMQTTArgs). The decoder must
// accept those, and a JSON number for qos. A retain in text is ignored by the decoder,
// which is why the invoker turns it into a bool.
func TestDecodeMQTTArgsAcceptsTheFlowInvokersTypes(t *testing.T) {
	cases := []struct {
		params     map[string]interface{}
		qos        int
		retain     bool
		retainNote string
	}{
		{map[string]interface{}{"topic": "a/b", "qos": 1, "retain": true}, 1, true, "bool"},
		{map[string]interface{}{"topic": "a/b", "qos": 2.0, "retain": false}, 2, false, "bool"},
		{map[string]interface{}{"topic": "a/b", "qos": int64(1)}, 1, false, "absent"},
		{map[string]interface{}{"topic": "a/b", "qos": 0, "retain": "true"}, 0, false, "text is ignored"},
	}
	for _, c := range cases {
		req := decodeMQTTArgs(ToolCall{Action: "mqtt_publish", Params: c.params})
		if req.Topic != "a/b" || req.QoS != c.qos || req.Retain != c.retain {
			t.Errorf("%v (retain %s): decoded %+v", c.params, c.retainNote, req)
		}
	}
}

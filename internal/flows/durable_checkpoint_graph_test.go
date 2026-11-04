package flows

import "testing"

func TestDurableGraphAllowedIsFailClosed(t *testing.T) {
	f := simpleFlow("Local")
	f.Nodes = append(f.Nodes, Node{ID: testNodeID(3), Type: durableWaitType, TypeVersion: 1})
	if !durableGraphAllowed(f) {
		t.Fatal("pure graph refused")
	}
	for _, typ := range []string{TypeHTTPRequest, TypeFileWrite, TypeAIStep, TypeWait, "tool.unknown", TypeTriggerWebhook} {
		bad, err := f.Clone()
		if err != nil {
			t.Fatal(err)
		}
		bad.Nodes = append(bad.Nodes, Node{ID: testNodeID(4), Type: typ, TypeVersion: 1, Settings: NodeSettings{Disabled: true}})
		if durableGraphAllowed(bad) {
			t.Fatalf("allowed disabled %s", typ)
		}
	}
	bad, _ := f.Clone()
	bad.Nodes[1].TypeVersion = 2
	if durableGraphAllowed(bad) {
		t.Fatal("allowed unknown node version")
	}
}

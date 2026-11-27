package storymaker

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func referenceAnswer(t *testing.T, s *Studio, topic string) (StudioResult, map[string]any) {
	t.Helper()
	args := map[string]any{}
	if topic != "" {
		args["topic"] = topic
	}
	res := s.Call(context.Background(), StudioCall{Tool: ToolCatalog, Operation: "reference", Args: args})
	return res, answerOf(t, res)
}

func TestFreeReferenceCodeTopicHasNoCatalogCalls(t *testing.T) {
	s, _, _ := newFreeStudioRig(t)
	s.SetPhase(PhaseFill, "s1")
	res, a := referenceAnswer(t, s, "code")
	if res.Status != "ok" {
		t.Fatalf("free reference code: %s", res.JSON)
	}
	guide, _ := a["guide"].(string)
	for _, banned := range []string{"api.add(", "char.", "prop."} {
		if strings.Contains(guide, banned) {
			t.Errorf("free code reference contains %q", banned)
		}
	}
	for _, want := range []string{"no catalog assets", "Never call api.add", "g.path", "onFrame"} {
		if !strings.Contains(guide, want) {
			t.Errorf("free code reference lacks %q", want)
		}
	}
	if !strings.Contains(guide, freeMotionExample) {
		t.Error("free code reference lacks the free motion example")
	}
	if a["version"] != "storymaker-reference/2-free" {
		t.Errorf("version = %v", a["version"])
	}
	if want := codeReferenceFree + "\n\n## Motion example\n\n```javascript\n" + freeMotionExample + "\n```\n"; guide != want {
		t.Error("the free code guide is not the free reference plus the free motion example")
	}
	if len(codeReferenceFree) > len(codeReference) {
		t.Errorf("code-free.md (%d bytes) is longer than code.md (%d bytes)", len(codeReferenceFree), len(codeReference))
	}
}

func TestFreeReferenceRefusesTheAnimationTopic(t *testing.T) {
	s, _, _ := newFreeStudioRig(t)
	s.SetPhase(PhaseFill, "s1")
	for _, topic := range []string{"animation", ""} {
		res, a := referenceAnswer(t, s, topic)
		if res.Status != "refused" || a["code"] != "invalid_data" {
			t.Fatalf("free reference topic %q = %s", topic, res.JSON)
		}
		if msg, _ := a["message"].(string); msg != "A free project has only the code reference." {
			t.Errorf("topic %q message = %q", topic, msg)
		}
		if next, _ := a["next_action"].(string); next != "Use topic code." {
			t.Errorf("topic %q next_action = %q", topic, next)
		}
	}
	if _, a := referenceAnswer(t, s, "bogus"); a["code"] != "invalid_data" {
		t.Errorf("free reference topic bogus = %v", a)
	}
}

func TestCatalogReferenceAnswersStayAsTheyWere(t *testing.T) {
	s, _, _ := newStudioWriteRig(t, nil)
	s.SetPhase(PhaseFill, "s1")
	wantCode := codeReference + "\n\n## Motion example\n\n```javascript\n" + codeReferenceExample + "\n```\n" +
		"\n## SVG asset example\n\nA custom charge controller between catalog solar-panel and battery props. Off-screen narration; adapt labels and reveal times to the story.\n\n```javascript\n" + svgReferenceExample + "\n```\n"
	for topic, want := range map[string]string{"animation": animationReference, "": animationReference, "code": wantCode} {
		res, a := referenceAnswer(t, s, topic)
		if res.Status != "ok" || a["guide"] != want || a["version"] != "storymaker-reference/2" {
			t.Errorf("catalog reference %q changed: status %s, version %v, guide equal %v", topic, res.Status, a["version"], a["guide"] == want)
		}
	}
	if _, a := referenceAnswer(t, s, "bogus"); a["code"] != "invalid_data" {
		t.Errorf("catalog reference topic bogus = %v", a)
	}
}

func TestFreePutCodeRefusesCatalogPieces(t *testing.T) {
	ctx := context.Background()
	s, ps, id := newFreeStudioRig(t)
	s.SetPhase(PhaseFill, "s1")

	read := codeCall(t, s, "get_code", "s1", nil, "")
	write := codeCall(t, s, "put_code", "s1", map[string]any{"source": freeMotionExample}, read["rev"].(string))
	if write["status"] != "ok" {
		t.Fatalf("put_code with the free motion example = %v", write)
	}
	saved, _, err := ps.LoadCodeSource(ctx, id, "s1")
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := ps.LoadCodeBundle(ctx, id, "s1")
	if err != nil {
		t.Fatal(err)
	}
	writes, issues := s.writes["s1"], s.lastIssueCount["s1"]

	for name, source := range map[string]string{
		"plain":   "export default function scene(api) { api.add('prop.lock', {}); }",
		"spaced":  "export default function scene(api) { const a = api . add ('prop.lock', {}); }",
		"newline": "export default function scene(api) {\n  api\n    .add(\n 'char.punk', {});\n}",
	} {
		got := codeCall(t, s, "put_code", "s1", map[string]any{"source": source}, write["rev"].(string))
		if got["status"] != "refused" || got["code"] != "free_mode_code_only" {
			t.Errorf("%s: put_code = %v", name, got)
			continue
		}
		if msg, _ := got["message"].(string); !strings.Contains(msg, "api.add places catalog people and props") {
			t.Errorf("%s: message = %q", name, msg)
		}
		if next, _ := got["next_action"].(string); !strings.Contains(next, "g.path") {
			t.Errorf("%s: next_action = %q", name, next)
		}
		current, _, _ := ps.LoadCodeSource(ctx, id, "s1")
		currentBundle, _ := ps.LoadCodeBundle(ctx, id, "s1")
		if !bytes.Equal(saved, current) || !bytes.Equal(bundle, currentBundle) {
			t.Errorf("%s: a refused source replaced the saved files", name)
		}
	}
	if s.writes["s1"] != writes || s.lastIssueCount["s1"] != issues || s.charged["s1"] != 0 {
		t.Errorf("a refused source used the budget: writes %d->%d, issues %d->%d, charged %d", writes, s.writes["s1"], issues, s.lastIssueCount["s1"], s.charged["s1"])
	}

	// The refusal comes after the revision check: a stale rev is still reported as stale.
	if got := codeCall(t, s, "put_code", "s1", map[string]any{"source": "export default function scene(api) { api.add('prop.lock', {}); }"}, MustNotExist); got["code"] != "rev_mismatch" {
		t.Errorf("stale rev with api.add = %v", got)
	}
	// A source that does not call api.add, but names it, is fine.
	named := "export default function scene(api) { api.duration(4); } // never call api.add"
	if got := codeCall(t, s, "put_code", "s1", map[string]any{"source": named}, write["rev"].(string)); got["status"] != "ok" {
		t.Errorf("a comment that names api.add without a call = %v", got)
	}
}

func TestCatalogPutCodeStillAcceptsApiAdd(t *testing.T) {
	s, _, _ := newStudioWriteRig(t, nil)
	s.SetPhase(PhaseFill, "s1")
	read := codeCall(t, s, "get_code", "s1", nil, "")
	got := codeCall(t, s, "put_code", "s1", map[string]any{"source": codeReferenceExample}, read["rev"].(string))
	if got["status"] != "ok" {
		t.Fatalf("catalog put_code with api.add = %v", got)
	}
	if next, _ := got["next_action"].(string); next != "Activate it with put_scene and data.kind code. Keep the narration; omit actors, props and beats." {
		t.Errorf("catalog next_action changed: %q", next)
	}
}

func TestFreeMotionExampleBundlesWithinTheAgentLimit(t *testing.T) {
	if len(freeMotionExample) >= StudioCodeBytes {
		t.Fatalf("free-motion.js is %d bytes, the agent limit is %d", len(freeMotionExample), StudioCodeBytes)
	}
	if res := BundleCodeScene([]byte(freeMotionExample), "s1", StudioCodeBytes); len(res.Errors) != 0 {
		t.Fatalf("free-motion.js does not bundle: %+v", res.Errors)
	}
	for _, banned := range []string{"api.add", "char.", "prop."} {
		if strings.Contains(freeMotionExample, banned) {
			t.Errorf("free-motion.js contains %q", banned)
		}
	}
}

func TestCodeReferencesNameTheSafeArea(t *testing.T) {
	for name, text := range map[string]string{"code.md": codeReference, "code-free.md": codeReferenceFree} {
		if strings.Contains(text, "710..1210") {
			t.Errorf("%s still names the fixed range x=710..1210", name)
		}
		if !strings.Contains(text, "api.safe()") {
			t.Errorf("%s does not name api.safe()", name)
		}
	}
}

func TestFreeReferenceTeachesTheBlocks(t *testing.T) {
	for name, text := range map[string]string{"code.md": codeReference, "code-free.md": codeReferenceFree} {
		if !strings.Contains(text, "api.diagram") {
			t.Errorf("%s does not teach api.diagram", name)
		}
		if strings.Contains(text, "placeholders") {
			t.Errorf("%s still calls the blocks placeholders", name)
		}
		for _, want := range []string{"## Explainer blocks", "api.icon(", "api.chart(", "api.headline(", "api.list(", "highlight(nodeId, timing)"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s lacks %q", name, want)
			}
		}
	}
	if !strings.Contains(freeMotionExample, "api.diagram(") || !strings.Contains(freeMotionExample, "api.headline(") || !strings.Contains(freeMotionExample, "api.safe()") {
		t.Error("free-motion.js does not use api.headline, api.diagram and api.safe()")
	}
	if lines := strings.Count(freeMotionExample, "\n"); lines >= 40 {
		t.Errorf("free-motion.js has %d lines, want fewer than 40", lines)
	}
	if strings.Count(freeMotionExample, "api.onFrame(") != 1 {
		t.Error("free-motion.js has not exactly one onFrame accent")
	}
}

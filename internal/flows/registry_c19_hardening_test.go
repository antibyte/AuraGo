package flows

import "testing"

// 1c-19: Registry.Generation keys the server's cache of the encoded palette, so every
// change of the registered set must move it and nothing else may.
func TestRegistryGenerationCountsEveryChange(t *testing.T) {
	reg := NewRegistry()
	gen := reg.Generation()
	step := func(what string, changed bool) {
		t.Helper()
		next := reg.Generation()
		if changed && next <= gen {
			t.Fatalf("%s: generation %d did not move from %d", what, next, gen)
		}
		if !changed && next != gen {
			t.Fatalf("%s: generation moved from %d to %d", what, gen, next)
		}
		gen = next
	}
	step("a new registry", false)

	if err := reg.Register(&NodeDef{Type: "c19.a"}); err != nil {
		t.Fatal(err)
	}
	step("Register", true)
	if err := reg.Register(&NodeDef{Type: "c19.a"}); err == nil {
		t.Fatal("a duplicate type must fail")
	}
	step("a failed Register", false)
	if err := reg.Register(nil); err == nil {
		t.Fatal("a nil definition must fail")
	}
	step("a nil Register", false)

	reg.MustRegister(&NodeDef{Type: "c19.b"})
	step("MustRegister", true)
	reg.Replace(&NodeDef{Type: "c19.a", Version: 2})
	step("Replace", true)

	reg.Lookup("c19.a")
	reg.All()
	step("Lookup and All", false)

	if n := reg.RemoveWhere(func(d *NodeDef) bool { return d.Type == "c19.none" }); n != 0 {
		t.Fatalf("RemoveWhere removed %d", n)
	}
	step("a RemoveWhere that removed nothing", false)
	if n := reg.RemoveWhere(func(d *NodeDef) bool { return d.Type == "c19.b" }); n != 1 {
		t.Fatalf("RemoveWhere removed %d", n)
	}
	step("RemoveWhere", true)

	reg.ReplaceWhere(nil, nil)
	step("a ReplaceWhere without a predicate and definitions", false)
	reg.ReplaceWhere(nil, []*NodeDef{{Type: "c19.c"}})
	step("a ReplaceWhere that adds", true)
	reg.ReplaceWhere(func(d *NodeDef) bool { return d.Type == "c19.c" }, nil)
	step("a ReplaceWhere that removes", true)

	func() {
		defer func() { _ = recover() }()
		reg.ReplaceWhere(func(*NodeDef) bool { panic("c19") }, []*NodeDef{{Type: "c19.d"}})
	}()
	step("a ReplaceWhere whose predicate panicked", false)
	if _, ok := reg.Lookup("c19.d"); ok {
		t.Fatal("a panicking ReplaceWhere must leave the registry unchanged")
	}
}

// RefreshGenericTools goes through ReplaceWhere, so a tool refresh invalidates a cached
// palette even when it installs the same tool names again (the definitions are new).
func TestRegistryGenerationFollowsAGenericToolRefresh(t *testing.T) {
	reg := NewRegistry()
	tools := []GenericTool{{Name: "c19_tool", Category: "other", Schema: map[string]any{"type": "object",
		"properties": map[string]any{"text": map[string]any{"type": "string"}}}}}
	RefreshGenericTools(reg, tools, StaticEnv{})
	first := reg.Generation()
	if _, ok := reg.Lookup(GenericTypePrefix + "c19_tool"); !ok || first == 0 {
		t.Fatalf("the generic node was not registered (generation %d)", first)
	}
	RefreshGenericTools(reg, tools, StaticEnv{})
	if reg.Generation() == first {
		t.Fatal("a second refresh must move the generation")
	}
}

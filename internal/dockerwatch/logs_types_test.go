package dockerwatch

import (
	"testing"

	"aurago/internal/dockerhost"
)

func TestProjectSelectorMatches(t *testing.T) {
	in := func(project, workingDir string) dockerhost.ContainerSummary {
		return dockerhost.ContainerSummary{ComposeProject: project, ComposeWorkingDir: workingDir}
	}
	stack := in("shop", "/opt/stacks/shop")
	trailing := in("shop", "/opt/stacks/shop/")
	external := in("shop", "/home/me/shop")
	nolabel := in("shop", "")
	other := in("blog", "/opt/stacks/shop")
	for _, tc := range []struct {
		name string
		sel  ProjectSelector
		want [5]bool // stack, trailing, external, nolabel, other
	}{
		{"project only", ProjectSelector{Project: "shop"}, [5]bool{true, true, true, true, false}},
		{"managed stack", ProjectSelector{Project: "shop", WorkingDir: "/opt/stacks/shop"}, [5]bool{true, true, false, false, false}},
		{"external project sharing the stack's name", ProjectSelector{Project: "shop", WorkingDir: "/opt/stacks/shop", Exclude: true}, [5]bool{false, false, true, true, false}},
		{"no project", ProjectSelector{}, [5]bool{}},
	} {
		got := [5]bool{tc.sel.Matches(stack), tc.sel.Matches(trailing), tc.sel.Matches(external), tc.sel.Matches(nolabel), tc.sel.Matches(other)}
		if got != tc.want {
			t.Errorf("%s: Matches = %v, want %v", tc.name, got, tc.want)
		}
	}
}

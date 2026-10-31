package tools

import (
	"aurago/internal/dockerutil"
	"testing"
)

func TestKaggleManagedKernelProtectedByNameAndLabel(t *testing.T) {
	for _, tc := range []struct {
		names  []string
		labels map[string]string
	}{
		{names: []string{"/aurago-kaggle-session"}},
		{names: []string{"AURAGO-KAGGLE-SESSION"}},
		{names: []string{"renamed"}, labels: map[string]string{"aurago.managed": dockerutil.KaggleNotebookOwner}},
	} {
		if !dockerManagedResourceExcluded(tc.labels, tc.names, false, []string{dockerutil.KaggleNotebookOwner}) {
			t.Fatal("kernel visible to generic list", tc.names)
		}
		owners := DockerContainerOwnersFromMetadata(tc.names, tc.labels, dockerutil.KaggleNotebookOwner)
		if !owners[dockerutil.KaggleNotebookOwner] {
			t.Fatal("kernel ownership lost", tc.names)
		}
	}
	if dockerManagedResourceExcluded(nil, []string{"jupyter-user"}, false, []string{dockerutil.KaggleNotebookOwner}) {
		t.Fatal("unrelated container protected")
	}
}

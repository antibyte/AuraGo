package ui

import (
	"strings"
	"testing"
)

func TestGrafanaConfigShowsPartialErrors(t *testing.T) {
	t.Parallel()

	js := readEmbeddedText(t, "cfg/grafana.js")
	for _, want := range []string{
		"res.data.partial_errors",
		"grafanaRenderSummary(res.data.summary || {}, res.data.partial_errors || [])",
		"config.grafana.partial_warning",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("grafana config UI must surface partial status errors; missing marker %q", want)
		}
	}
}

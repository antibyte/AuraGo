package agent

import (
	"slices"
	"testing"
)

func TestKaggleAdaptiveDiscoveryGermanAndEnglish(t *testing.T) {
	for _, tc := range []struct{ query, tool string }{
		{"Suche Kaggle Datensätze", "kaggle_datasets"}, {"Find Kaggle datasets", "kaggle_datasets"},
		{"Zeige Kaggle Modelle", "kaggle_models"}, {"Find Kaggle models", "kaggle_models"},
		{"Starte mein Kaggle Notebook", "kaggle_notebooks"}, {"Run this Kaggle notebook", "kaggle_notebooks"},
		{"Kaggle Wettbewerb Einreichung", "kaggle_competitions"}, {"Kaggle competition submission", "kaggle_competitions"},
		{"Zeige Kaggle Benchmarks", "kaggle_benchmarks"}, {"Show Kaggle benchmarks", "kaggle_benchmarks"},
		{"Lies Kaggle Foren", "kaggle_forums"}, {"Read Kaggle forums", "kaggle_forums"},
		{"Starte Jupyter lokal", "kaggle_runtime"}, {"Start local Jupyter", "kaggle_runtime"},
	} {
		if !slices.Contains(adaptiveSeedsForQuery(tc.query, nil), tc.tool) {
			t.Errorf("%s missing %s", tc.query, tc.tool)
		}
		if slices.Contains(adaptiveSeedsForQuery(tc.query, func(name string) bool { return name != tc.tool }), tc.tool) {
			t.Errorf("disabled %s offered", tc.tool)
		}
	}
}

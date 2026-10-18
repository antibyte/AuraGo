package ui

import "testing"

// Parameter values containing $&, $1 or $$ must reach the rendered text unchanged.
func TestSharedTranslateInsertsParamValuesLiterally(t *testing.T) {
	t.Parallel()
	runDashboardNodeCheck(t, "testdata/shared-translate.cjs")
}

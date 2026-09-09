package agent

import (
	"os"
	"testing"

	"aurago/internal/testutil"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.RunWithGameMakerAssets(m.Run))
}

package tools

import "testing"

func TestDockerBodyMessageExtractsEngineError(t *testing.T) {
	got := dockerBodyMessage(500, []byte(`{"message":"driver failed programming external connectivity"}`))
	if got != "driver failed programming external connectivity" {
		t.Fatalf("dockerBodyMessage = %q", got)
	}
}

package chibisensei

import (
	"testing"

	"aurago/internal/chibisensei/trainer"
	"aurago/internal/llamacpp"
)

// The converter the trainer pins and the runtime that serves the GGUF must come from the same llama.cpp build.
func TestRuntimePinMatchesTrainerLock(t *testing.T) {
	lock, err := trainer.LoadLock()
	if err != nil {
		t.Fatal(err)
	}
	if lock.LlamaCppTag != llamacpp.Build {
		t.Fatalf("trainer lock llama_cpp_tag %q != llamacpp.Build %q", lock.LlamaCppTag, llamacpp.Build)
	}
}

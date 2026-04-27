package uid

import (
	"errors"
	"strings"
	"testing"
)

func TestNewPanicsWhenRandomReadFails(t *testing.T) {
	oldRandRead := randRead
	randRead = func([]byte) (int, error) {
		return 0, errors.New("entropy unavailable")
	}
	defer func() { randRead = oldRandRead }()

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("New() did not panic when random read failed")
		}
		if !strings.Contains(recovered.(error).Error(), "entropy unavailable") {
			t.Fatalf("panic = %v, want entropy error", recovered)
		}
	}()

	_ = New()
}

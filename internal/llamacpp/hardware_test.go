package llamacpp

import (
	"context"
	"testing"
)

func TestDetectHardwareWithAugment(t *testing.T) {
	ctx := context.Background()
	base := DetectHardware(ctx)
	if base.Fingerprint == "" {
		t.Fatal("empty fingerprint")
	}
	if again := DetectHardware(ctx); again.Fingerprint != base.Fingerprint {
		t.Fatal("fingerprint is not stable")
	}
	withLine := DetectHardwareWith(ctx, func(_ context.Context, info *HardwareInfo) string {
		info.NVIDIA = true
		return "docker=x"
	})
	if !withLine.NVIDIA || withLine.Fingerprint == base.Fingerprint {
		t.Fatalf("augment line did not change the result: %+v", withLine)
	}
	empty := DetectHardwareWith(ctx, func(context.Context, *HardwareInfo) string { return "" })
	if empty.Fingerprint != base.Fingerprint {
		t.Fatal("an empty evidence line changed the fingerprint")
	}
}

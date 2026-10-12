package decisions

import (
	"reflect"
	"testing"
)

func TestOrderedDevicesPrefersGPUAndDiscreteBeforeIntegratedThenCPU(t *testing.T) {
	integrated := true
	discrete := false
	devices := []runtimeDevice{
		{ID: "cpu", Backend: "cpu", Type: "CPU"},
		{ID: "igpu", Backend: "cuda", Type: "GPU", Integrated: &integrated, MemoryFree: 8},
		{ID: "vulkan", Backend: "vulkan", Type: "GPU", Integrated: &discrete, MemoryFree: 4},
		{ID: "cuda", Backend: "cuda", Type: "GPU", Integrated: &discrete, MemoryFree: 2},
	}
	ordered, err := orderedDevices(devices, "windows", true)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, device := range ordered {
		got = append(got, device.ID)
	}
	want := []string{"cuda", "vulkan", "igpu", "cpu"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ordered device ids = %v, want %v", got, want)
	}
	withoutCPUConsent, err := orderedDevices(devices, "windows", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, device := range withoutCPUConsent {
		if device.Type == "CPU" {
			t.Fatal("CPU was exposed as a start candidate without persistent consent")
		}
	}
}

func TestOrderedDevicesRequiresCPUConsentWhenNoAcceleratorExists(t *testing.T) {
	_, err := orderedDevices([]runtimeDevice{{ID: "cpu", Backend: "cpu", Type: "CPU"}}, "windows", false)
	if ErrorCode(err) != CodeCPUConsentRequired {
		t.Fatalf("ErrorCode(%v) = %q", err, ErrorCode(err))
	}
}

func TestHybridLayerAttemptsAreBoundedDescendingAndUnique(t *testing.T) {
	if got, want := hybridLayerAttempts(32), []int{24, 16, 8}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hybridLayerAttempts(32) = %v, want %v", got, want)
	}
	if got, want := hybridLayerAttempts(3), []int{2, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hybridLayerAttempts(3) = %v, want %v", got, want)
	}
	if got := hybridLayerAttempts(1); len(got) != 0 {
		t.Fatalf("hybridLayerAttempts(1) = %v, want none", got)
	}
}

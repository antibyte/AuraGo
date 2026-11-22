package onnxrt

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"unsafe"
)

const (
	fakeSessionOptions uintptr = 0x5E55
	fakeCUDAOptions    uintptr = 0xC0DA
	fakeErrorStatus    uintptr = 0xE7
)

// fakeProviderAPI stands in for the ONNX Runtime provider-registration
// entries, so the CUDA/generic branching is testable without a GPU or runtime.
type fakeProviderAPI struct {
	failCall string

	calls              []string
	genericProvider    string
	genericOptionCount uintptr
	cudaOptions        map[string]string
	updatedOptions     uintptr
	appendedOptions    uintptr
	appendedSession    uintptr
	releasedOptions    []uintptr
}

func (fake *fakeProviderAPI) result(call string) uintptr {
	fake.calls = append(fake.calls, call)
	if call == fake.failCall {
		return fakeErrorStatus
	}
	return 0
}

func (fake *fakeProviderAPI) runtime() *Runtime {
	message := append([]byte("fake "+fake.failCall+" failure"), 0)
	r := &Runtime{env: 1}
	r.functions.getErrorMessage = func(uintptr) *byte { return &message[0] }
	r.functions.releaseStatus = func(status uintptr) {
		if status == fakeErrorStatus {
			fake.calls = append(fake.calls, "releaseStatus")
		}
	}
	r.functions.appendExecutionProvider = func(options uintptr, name *byte, _, _ **byte, count uintptr) uintptr {
		fake.genericProvider = cString(name)
		fake.genericOptionCount = count
		fake.appendedSession = options
		return fake.result("append")
	}
	r.functions.createCUDAProviderOptions = func(out *uintptr) uintptr {
		status := fake.result("createCUDA")
		if status == 0 {
			*out = fakeCUDAOptions
		}
		return status
	}
	r.functions.updateCUDAProviderOptions = func(cudaOptions uintptr, keys, values **byte, count uintptr) uintptr {
		fake.updatedOptions = cudaOptions
		fake.cudaOptions = map[string]string{}
		keyPointers := unsafe.Slice(keys, count)
		valuePointers := unsafe.Slice(values, count)
		for i := range keyPointers {
			fake.cudaOptions[cString(keyPointers[i])] = cString(valuePointers[i])
		}
		return fake.result("updateCUDA")
	}
	r.functions.appendExecutionProviderCUDAV2 = func(options, cudaOptions uintptr) uintptr {
		fake.appendedSession = options
		fake.appendedOptions = cudaOptions
		return fake.result("appendCUDAV2")
	}
	r.functions.releaseCUDAProviderOptions = func(cudaOptions uintptr) {
		fake.calls = append(fake.calls, "releaseCUDA")
		fake.releasedOptions = append(fake.releasedOptions, cudaOptions)
	}
	return r
}

func TestRegisterProviderCUDAUsesDedicatedAPI(t *testing.T) {
	fake := &fakeProviderAPI{}
	if err := fake.runtime().registerProvider(fakeSessionOptions, ProviderCUDA, nil); err != nil {
		t.Fatalf("register CUDA: %v", err)
	}
	wantCalls := []string{"createCUDA", "updateCUDA", "appendCUDAV2", "releaseCUDA"}
	if !slices.Equal(fake.calls, wantCalls) {
		t.Fatalf("CUDA registration calls = %v, want %v (generic append rejects CUDA)", fake.calls, wantCalls)
	}
	if !reflect.DeepEqual(fake.cudaOptions, map[string]string{"device_id": "0"}) {
		t.Fatalf("CUDA provider options = %v", fake.cudaOptions)
	}
	if fake.updatedOptions != fakeCUDAOptions || fake.appendedOptions != fakeCUDAOptions {
		t.Fatalf("CUDA options updated %#x, appended %#x, want %#x", fake.updatedOptions, fake.appendedOptions, fakeCUDAOptions)
	}
	if fake.appendedSession != fakeSessionOptions {
		t.Fatalf("CUDA appended to %#x, want %#x", fake.appendedSession, fakeSessionOptions)
	}
	if !slices.Equal(fake.releasedOptions, []uintptr{fakeCUDAOptions}) {
		t.Fatalf("released CUDA options = %#x", fake.releasedOptions)
	}
}

func TestRegisterProviderCUDAMatchesNameCaseInsensitively(t *testing.T) {
	fake := &fakeProviderAPI{}
	if err := fake.runtime().registerProvider(fakeSessionOptions, "cudaexecutionprovider", nil); err != nil {
		t.Fatalf("register lower-case CUDA: %v", err)
	}
	wantCalls := []string{"createCUDA", "updateCUDA", "appendCUDAV2", "releaseCUDA"}
	if !slices.Equal(fake.calls, wantCalls) {
		t.Fatalf("lower-case CUDA registration calls = %v, want %v", fake.calls, wantCalls)
	}
}

func TestRegisterProviderCUDACallerOptionsOverrideDefaults(t *testing.T) {
	fake := &fakeProviderAPI{}
	opts := map[string]string{"device_id": "1", "arena_extend_strategy": "kSameAsRequested"}
	if err := fake.runtime().registerProvider(fakeSessionOptions, ProviderCUDA, opts); err != nil {
		t.Fatalf("register CUDA: %v", err)
	}
	if !reflect.DeepEqual(fake.cudaOptions, opts) {
		t.Fatalf("CUDA provider options = %v, want %v", fake.cudaOptions, opts)
	}
}

func TestRegisterProviderKeepsOtherProvidersOnGenericCall(t *testing.T) {
	for _, provider := range []string{ProviderCoreML, ProviderWebGPU} {
		fake := &fakeProviderAPI{}
		if err := fake.runtime().registerProvider(fakeSessionOptions, provider, nil); err != nil {
			t.Fatalf("register %s: %v", provider, err)
		}
		if !slices.Equal(fake.calls, []string{"append"}) {
			t.Fatalf("%s calls = %v, want only the generic append", provider, fake.calls)
		}
		if fake.genericProvider != appendProviderName(provider) || fake.genericOptionCount != 0 {
			t.Fatalf("%s generic append got %q with %d options", provider, fake.genericProvider, fake.genericOptionCount)
		}
		if fake.appendedSession != fakeSessionOptions {
			t.Fatalf("%s appended to %#x, want %#x", provider, fake.appendedSession, fakeSessionOptions)
		}
	}
}

func TestRegisterProviderCUDAFailuresReleaseOptions(t *testing.T) {
	tests := map[string]struct {
		failCall  string
		wantCalls []string
	}{
		"create options":  {"createCUDA", []string{"createCUDA", "releaseStatus"}},
		"update options":  {"updateCUDA", []string{"createCUDA", "updateCUDA", "releaseStatus", "releaseCUDA"}},
		"append provider": {"appendCUDAV2", []string{"createCUDA", "updateCUDA", "appendCUDAV2", "releaseStatus", "releaseCUDA"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fake := &fakeProviderAPI{failCall: tc.failCall}
			err := fake.runtime().registerProvider(fakeSessionOptions, ProviderCUDA, nil)
			if err == nil || !strings.Contains(err.Error(), "fake "+tc.failCall+" failure") || !strings.Contains(err.Error(), "enable "+ProviderCUDA) {
				t.Fatalf("error = %v, want the ONNX Runtime status message under the enable prefix", err)
			}
			if !slices.Equal(fake.calls, tc.wantCalls) {
				t.Fatalf("calls = %v, want %v", fake.calls, tc.wantCalls)
			}
		})
	}
}

func TestRegisterProviderCUDARejectsNULOptionsBeforeNativeCalls(t *testing.T) {
	fake := &fakeProviderAPI{}
	err := fake.runtime().registerProvider(fakeSessionOptions, ProviderCUDA, map[string]string{"device_id": "0\x001"})
	if err == nil {
		t.Fatal("NUL in a CUDA option value was accepted")
	}
	if len(fake.calls) != 0 {
		t.Fatalf("native calls before validation: %v", fake.calls)
	}
}

func TestBindFunctionsBindsEveryEntry(t *testing.T) {
	// Distinct non-zero fake addresses: purego only builds trampolines here and
	// never calls them, so a missing binding shows up as a nil function field.
	table := make([]uintptr, indexSessionOptionsAppendExecutionProvider+1)
	for i := range table {
		table[i] = 0x10000 + uintptr(i)*0x10
	}
	bound := bindFunctions(unsafe.Pointer(&table[0]))
	value := reflect.ValueOf(bound)
	for i := 0; i < value.NumField(); i++ {
		if value.Field(i).IsNil() {
			t.Errorf("functions.%s is not bound", value.Type().Field(i).Name)
		}
	}
}

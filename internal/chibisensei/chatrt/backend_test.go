package chatrt

import (
	"context"
	"errors"
	"testing"

	"aurago/internal/llamacpp"
)

func TestResolveBackend(t *testing.T) {
	nvidia := llamacpp.HardwareInfo{NVIDIA: true}
	vulkan := llamacpp.HardwareInfo{Vulkan: true}
	both := llamacpp.HardwareInfo{NVIDIA: true, Vulkan: true}
	cases := []struct {
		name        string
		goos, arch  string
		requested   string
		hw          llamacpp.HardwareInfo
		want        string
		unsupported bool
		hwCalls     int
	}{
		{"auto linux nvidia", "linux", "amd64", "auto", nvidia, "cuda", false, 1},
		{"auto linux both prefers cuda", "linux", "amd64", "auto", both, "cuda", false, 1},
		{"auto linux vulkan", "linux", "amd64", "auto", vulkan, "vulkan", false, 1},
		{"auto linux none", "linux", "amd64", "auto", llamacpp.HardwareInfo{}, "cpu", false, 1},
		{"auto windows nvidia", "windows", "amd64", "auto", nvidia, "cuda", false, 1},
		{"auto windows arm64 vulkan has no asset", "windows", "arm64", "auto", vulkan, "cpu", false, 1},
		{"auto darwin arm64", "darwin", "arm64", "auto", nvidia, "metal", false, 0},
		{"auto darwin amd64", "darwin", "amd64", "auto", nvidia, "cpu", false, 1},
		{"cpu ignores hardware", "linux", "amd64", "cpu", nvidia, "cpu", false, 0},
		{"cuda requested and present", "linux", "amd64", "cuda", llamacpp.HardwareInfo{}, "cuda", false, 0},
		{"vulkan requested without asset", "windows", "arm64", "vulkan", vulkan, "cpu", false, 0},
		{"cuda requested on darwin", "darwin", "amd64", "cuda", nvidia, "cpu", false, 0},
		{"no cpu archive", "freebsd", "amd64", "auto", nvidia, "", true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newFakeLauncher(true)
			l.goos, l.goarch, l.hw = tc.goos, tc.arch, tc.hw
			res := resolveBackend(context.Background(), l, tc.requested)
			if res.selected != tc.want || res.unsupported != tc.unsupported {
				t.Fatalf("got %q unsupported=%v", res.selected, res.unsupported)
			}
			if l.hardwareCalls != tc.hwCalls {
				t.Fatalf("hardware probed %d times, want %d", l.hardwareCalls, tc.hwCalls)
			}
		})
	}
}

func TestInstallSet(t *testing.T) {
	ids := func(set []llamacpp.AssetSpec) []string {
		out := []string{}
		for _, a := range set {
			out = append(out, a.ID)
		}
		return out
	}
	cases := []struct {
		goos, arch, backend string
		want                []string
	}{
		{"linux", "amd64", "cuda", []string{"llama-linux-amd64-cpu", "llama-linux-amd64-cuda"}},
		{"windows", "amd64", "cuda", []string{"llama-windows-amd64-cpu", "llama-windows-amd64-cuda", llamacpp.WindowsCUDARuntimeID}},
		{"darwin", "arm64", "metal", []string{"llama-darwin-arm64-metal"}},
		{"darwin", "arm64", "cpu", []string{"llama-darwin-arm64-metal"}},
		{"windows", "arm64", "vulkan", []string{"llama-windows-arm64-cpu"}},
		{"linux", "arm64", "cpu", []string{"llama-linux-arm64-cpu"}},
	}
	for _, tc := range cases {
		set, ok := installSet(tc.goos, tc.arch, tc.backend)
		if !ok || len(ids(set)) != len(tc.want) {
			t.Fatalf("%s/%s/%s: %v ok=%v", tc.goos, tc.arch, tc.backend, ids(set), ok)
		}
		for i := range tc.want {
			if ids(set)[i] != tc.want[i] {
				t.Fatalf("%s/%s/%s: %v, want %v", tc.goos, tc.arch, tc.backend, ids(set), tc.want)
			}
		}
	}
	if _, ok := installSet("freebsd", "amd64", "cpu"); ok {
		t.Fatal("freebsd has a CPU archive")
	}
}

func TestManagerGPUFallback(t *testing.T) {
	ctx := context.Background()
	newGPU := func(goos string) (*fixture, *fakeLauncher) {
		l := newFakeLauncher(true)
		l.goos, l.hw = goos, llamacpp.HardwareInfo{NVIDIA: true}
		fx := newFixture(t, l)
		fx.setConfig(func(c *Config) { c.Backend = "auto" })
		return fx, l
	}

	t.Run("no evidence falls back and sticks", func(t *testing.T) {
		fx, l := newGPU("linux")
		var procs []*fakeProc
		l.launchFn = func(_ context.Context, spec LaunchSpec) (Process, error) {
			p := &fakeProc{gpu: false, done: make(chan struct{})}
			procs = append(procs, p)
			return p, nil
		}
		lease := fx.acquire("a")
		if lease.Backend() != "cpu" {
			t.Fatalf("backend %q", lease.Backend())
		}
		specs := l.launchSpecs()
		if len(specs) != 2 || specs[0].Backend != "cuda" || specs[1].Backend != "cpu" {
			t.Fatalf("launch specs %+v", specs)
		}
		if !procs[0].isStopped() {
			t.Fatal("the GPU start without evidence is still running")
		}
		if st := fx.m.Status(ctx); !st.Fallback || st.Selected != "cuda" {
			t.Fatalf("status %+v", st)
		}
		lease.Release()
		fx.acquire("b").Release()
		specs = l.launchSpecs()
		if len(specs) != 3 || specs[2].Backend != "cpu" {
			t.Fatalf("second key did not start directly on cpu: %+v", specs)
		}
	})

	t.Run("launch error falls back", func(t *testing.T) {
		fx, l := newGPU("linux")
		l.launchFn = func(_ context.Context, spec LaunchSpec) (Process, error) {
			if spec.Backend == "cuda" {
				return nil, errors.New("no cuda device")
			}
			return &fakeProc{done: make(chan struct{})}, nil
		}
		lease := fx.acquire("a")
		defer lease.Release()
		if lease.Backend() != "cpu" || !fx.m.Status(ctx).Fallback {
			t.Fatalf("backend %q", lease.Backend())
		}
	})

	t.Run("verified GPU start is kept", func(t *testing.T) {
		fx, l := newGPU("linux")
		l.launchFn = func(context.Context, LaunchSpec) (Process, error) {
			return &fakeProc{gpu: true, done: make(chan struct{})}, nil
		}
		lease := fx.acquire("a")
		defer lease.Release()
		st := fx.m.Status(ctx)
		if lease.Backend() != "cuda" || st.Fallback || len(st.Instances) != 1 || !st.Instances[0].GPUVerified || st.Instances[0].Backend != "cuda" {
			t.Fatalf("backend %q status %+v", lease.Backend(), st)
		}
	})

	t.Run("install and a backend change clear the flag", func(t *testing.T) {
		fx, l := newGPU("linux")
		l.launchFn = func(_ context.Context, spec LaunchSpec) (Process, error) {
			if spec.Backend == "cuda" {
				return nil, errors.New("no cuda device")
			}
			return &fakeProc{done: make(chan struct{})}, nil
		}
		fx.acquire("a").Release()
		if !fx.m.Status(ctx).Fallback {
			t.Fatal("fallback not set")
		}
		l.mu.Lock()
		l.verified = false
		l.mu.Unlock()
		fx.m.mu.Lock()
		fx.m.installed = nil
		fx.m.mu.Unlock()
		if err := fx.m.Install(ctx); err != nil {
			t.Fatal(err)
		}
		if fx.m.Status(ctx).Fallback {
			t.Fatal("BeginInstall kept the fallback flag")
		}
		fx.acquire("b").Release() // falls back again
		if !fx.m.Status(ctx).Fallback {
			t.Fatal("fallback not set again")
		}
		fx.setConfig(func(c *Config) { c.Backend = "cpu" })
		if st := fx.m.Status(ctx); st.Fallback || st.Selected != "cpu" {
			t.Fatalf("a backend change kept the flag: %+v", st)
		}
	})

	t.Run("windows cuda launch carries the supplement", func(t *testing.T) {
		fx, l := newGPU("windows")
		l.launchFn = func(context.Context, LaunchSpec) (Process, error) {
			return &fakeProc{gpu: true, done: make(chan struct{})}, nil
		}
		fx.acquire("a").Release()
		specs := l.launchSpecs()
		if len(specs) != 1 || len(specs[0].Assets) != 2 || specs[0].Assets[0].ID != "llama-windows-amd64-cuda" ||
			specs[0].Assets[1].ID != llamacpp.WindowsCUDARuntimeID {
			t.Fatalf("specs %+v", specs)
		}
	})

	t.Run("cpu fallback uses only the cpu archive", func(t *testing.T) {
		fx, l := newGPU("windows")
		l.launchFn = func(_ context.Context, spec LaunchSpec) (Process, error) {
			if spec.Backend != "cpu" {
				return nil, errors.New("gpu failed")
			}
			return &fakeProc{done: make(chan struct{})}, nil
		}
		fx.acquire("a").Release()
		specs := l.launchSpecs()
		if len(specs) != 2 || len(specs[1].Assets) != 1 || specs[1].Assets[0].ID != "llama-windows-amd64-cpu" {
			t.Fatalf("specs %+v", specs)
		}
	})
}

package decisions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"aurago/internal/config"
)

type deadlineDiscoveryFactory struct{ testFactory }

func (*deadlineDiscoveryFactory) discover(ctx context.Context, _ string, _ RuntimeAccess, _ modelPaths, _ string) ([]runtimeDevice, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestDiscoveryTimeoutAllowsStopAndLaterStart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		factory := &deadlineDiscoveryFactory{}
		manager := startManagerForTest(t, config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative, CPUFallbackAllowed: true}, &factory.testFactory)
		manager.factory = factory
		if err := manager.Start(context.Background()); ErrorCode(err) != CodeRuntimeDiscoveryTimeout {
			t.Fatalf("start: %v", err)
		}
		if len(factory.attempts) != 0 || manager.Status().CPUFallbackActive {
			t.Fatal("timeout triggered CPU fallback")
		}
		if err := manager.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
		ready := &testFactory{devices: []runtimeDevice{{ID: "cuda0", Name: "GPU", Backend: "cuda", Type: "GPU", BinarySHA: strings.Repeat("a", 64)}}}
		manager.factory = ready
		if err := manager.Start(context.Background()); err != nil || !manager.Status().Ready {
			t.Fatalf("later start: %v", err)
		}
		if err := manager.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
}

func TestDiscoveryHasIndependentBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager := &Manager{factory: &deadlineDiscoveryFactory{}}
		start := time.Now()
		_, err := manager.discoverDevices(context.Background(), "native", RuntimeAccess{}, modelPaths{})
		if ErrorCode(err) != CodeRuntimeDiscoveryTimeout || !errors.Is(err, context.DeadlineExceeded) || time.Since(start) != 60*time.Second {
			t.Fatalf("discovery returned %v after %v", err, time.Since(start))
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = manager.discoverDevices(ctx, "native", RuntimeAccess{}, modelPaths{})
		if ErrorCode(err) != CodeCancelled || !errors.Is(err, context.Canceled) {
			t.Fatalf("parent cause lost: %v", err)
		}
	})
}

func TestNativeDiscoveryBoundsOutputAndReapsProbe(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"stdout", "stderr", "1", "tree"} {
		t.Run(mode, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "started")
			t.Setenv(nativeDiscoveryHelperEnv, mode)
			t.Setenv(nativeDiscoveryMarkerEnv, marker)
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			result := make(chan error, 1)
			go func() {
				_, err := runNativeDiscovery(ctx, executable, filepath.Dir(executable), nil)
				result <- err
			}()
			want := CodeRuntimeDiscoveryOutputLimit
			started := marker
			if mode == "tree" {
				started += ".child"
			}
			// Check cleanup after startup, including the descendant, regardless of host load.
			for {
				if _, err := os.Stat(started); err == nil {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal("helper never started")
				case <-time.After(10 * time.Millisecond):
				}
			}
			if mode == "1" || mode == "tree" {
				want = CodeCancelled
				cancel()
			}
			select {
			case err := <-result:
				if ErrorCode(err) != want {
					t.Fatalf("probe error = %v, want %s", err, want)
				}
			case <-time.After(6 * time.Second):
				t.Fatal("probe did not terminate")
			}
			if _, err := os.Stat(marker); err != nil {
				t.Fatal("helper never started")
			}
			if mode == "tree" {
				before, err := os.ReadFile(marker + ".child")
				if err != nil {
					t.Fatal("descendant never started", err)
				}
				time.Sleep(100 * time.Millisecond)
				after, err := os.ReadFile(marker + ".child")
				if err != nil || string(before) != string(after) {
					t.Fatal("discovery descendant survived termination")
				}
			}
			// Run waits for process exit and both output readers before returning.
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
		})
	}
}

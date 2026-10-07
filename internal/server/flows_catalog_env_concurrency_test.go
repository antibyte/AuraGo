package server

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"

	"aurago/internal/config"
	"aurago/internal/flows"
)

// Concurrency tests of flowCatalogEnv: refreshes that race a configuration change, and
// ToolAvailability (the palette's availability hooks) while a refresh runs. The tests
// over the agent's real tool schemas are in flows_catalog_env_hardening_test.go.

// c12DeadlockGuard bounds every wait in these tests, so a deadlock fails instead of hanging.
const c12DeadlockGuard = 10 * time.Second

// c12OrderProbe is how long TestC12RefreshMuOrdersInstalls gives a second refresh to
// overtake the first. It can only make a missing refreshMu go unnoticed, never fail a
// correct implementation.
const c12OrderProbe = 200 * time.Millisecond

// c12Within runs fn and fails the test when it does not return within c12DeadlockGuard.
func c12Within(t *testing.T, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(c12DeadlockGuard):
		t.Fatalf("%s did not return within %s: deadlock", what, c12DeadlockGuard)
	}
}

// c12Wait waits for ch and fails the test after c12DeadlockGuard.
func c12Wait(t *testing.T, what string, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(c12DeadlockGuard):
		t.Fatalf("timed out after %s waiting for %s", c12DeadlockGuard, what)
	}
}

// Requirement 1: two refreshes that race end with the tools of the current
// configuration, whatever order they finish their snapshots in.
func TestC12ConcurrentRefreshesInstallTheCurrentConfig(t *testing.T) {
	cfg1, cfg2 := &config.Config{}, &config.Config{}
	var cur atomic.Pointer[config.Config]
	cur.Store(cfg1)
	entered, release := make(chan struct{}), make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	var builds1, builds2 atomic.Int32
	env := &flowCatalogEnv{
		current: cur.Load,
		schemas: func(cfg *config.Config) []openai.Tool {
			if cfg == cfg1 {
				builds1.Add(1)
				enterOnce.Do(func() { close(entered) })
				<-release
				return []openai.Tool{fnTool("filesystem"), fnTool("c12_old_only")}
			}
			builds2.Add(1)
			return []openai.Tool{fnTool("filesystem"), fnTool("proxmox")}
		},
	}
	reg := flows.NewRegistry()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		env.refreshRegistry(reg, cfg1)
	}()
	c12Wait(t, "the refresh of the old configuration", entered)
	cur.Store(cfg2)
	go func() {
		defer wg.Done()
		env.refreshRegistry(reg, cfg2)
	}()
	releaseOnce.Do(func() { close(release) })
	c12Within(t, "the two refreshes", wg.Wait)

	if _, ok := reg.Lookup(flows.GenericTypePrefix + "proxmox"); !ok {
		t.Fatal("the registry lacks tool.proxmox of the current configuration")
	}
	if _, ok := reg.Lookup(flows.GenericTypePrefix + "c12_old_only"); ok {
		t.Fatal("the registry holds a tool of the replaced configuration")
	}
	if builds1.Load() != 1 || builds2.Load() != 1 {
		t.Fatalf("schemas built %d times for the old and %d times for the current configuration, want 1 and 1",
			builds1.Load(), builds2.Load())
	}
	if a := env.ToolAvailability("proxmox"); a.State != flows.AvailableState {
		t.Fatalf("proxmox = %+v, want available", a)
	}
	if a := env.ToolAvailability("c12_old_only"); a.State != flows.NeedsSetupState {
		t.Fatalf("c12_old_only = %+v, want needs_setup", a)
	}
}

// Requirement 1: a refresh for a configuration that is no longer current installs
// nothing and does not replace the cached snapshot of the current one.
func TestC12StaleRefreshInstallsNothing(t *testing.T) {
	cfg1, cfg2 := &config.Config{}, &config.Config{}
	var cur atomic.Pointer[config.Config]
	cur.Store(cfg2)
	var builds1, builds2 atomic.Int32
	env := &flowCatalogEnv{
		current: cur.Load,
		schemas: func(cfg *config.Config) []openai.Tool {
			if cfg == cfg1 {
				builds1.Add(1)
				return []openai.Tool{fnTool("filesystem"), fnTool("c12_old_only")}
			}
			builds2.Add(1)
			return []openai.Tool{fnTool("filesystem"), fnTool("proxmox")}
		},
	}
	reg := flows.NewRegistry()
	env.refreshRegistry(reg, cfg2)
	env.refreshRegistry(reg, cfg1)
	if _, ok := reg.Lookup(flows.GenericTypePrefix + "c12_old_only"); ok {
		t.Fatal("a stale refresh installed the tools of the replaced configuration")
	}
	if _, ok := reg.Lookup(flows.GenericTypePrefix + "proxmox"); !ok {
		t.Fatal("a stale refresh removed the tools of the current configuration")
	}
	// ToolAvailability reads the cache for cfg2: had the stale snapshot replaced it, the
	// schemas of cfg2 would have been built a second time here.
	if a := env.ToolAvailability("proxmox"); a.State != flows.AvailableState {
		t.Fatalf("proxmox = %+v, want available", a)
	}
	if builds2.Load() != 1 {
		t.Fatalf("the current configuration's schemas were built %d times; the stale snapshot replaced the cache", builds2.Load())
	}
	if builds1.Load() != 1 {
		t.Fatalf("the stale configuration's schemas were built %d times, want 1", builds1.Load())
	}
}

// Requirement 1 and 5: a slow schema build holds no lock that ToolAvailability needs.
func TestC12ToolAvailabilityDoesNotWaitForASchemaBuild(t *testing.T) {
	cfg1, cfg2 := &config.Config{}, &config.Config{}
	var cur atomic.Pointer[config.Config]
	cur.Store(cfg2)
	entered, release := make(chan struct{}), make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	env := &flowCatalogEnv{
		current: cur.Load,
		schemas: func(cfg *config.Config) []openai.Tool {
			if cfg == cfg1 {
				enterOnce.Do(func() { close(entered) })
				<-release
			}
			return []openai.Tool{fnTool("filesystem"), fnTool("proxmox")}
		},
	}
	reg := flows.NewRegistry()
	env.refreshRegistry(reg, cfg2)
	done := make(chan struct{})
	go func() {
		defer close(done)
		env.refreshRegistry(reg, cfg1)
	}()
	c12Wait(t, "the slow schema build", entered)
	c12Within(t, "ToolAvailability during a schema build", func() {
		if a := env.ToolAvailability("proxmox"); a.State != flows.AvailableState {
			t.Errorf("proxmox = %+v, want available", a)
		}
	})
	releaseOnce.Do(func() { close(release) })
	c12Wait(t, "the stale refresh", done)
}

// Requirement 5: describing the palette (whose hooks call ToolAvailability) while the
// configuration changes and the generic nodes are refreshed neither deadlocks nor ends
// with tools of a replaced configuration.
func TestC12DescribeWhileRefreshing(t *testing.T) {
	cfgs := []*config.Config{{}, {}}
	var cur atomic.Pointer[config.Config]
	cur.Store(cfgs[0])
	env := &flowCatalogEnv{
		current: cur.Load,
		schemas: func(cfg *config.Config) []openai.Tool {
			if cfg == cfgs[1] {
				return []openai.Tool{fnTool("filesystem"), fnTool("proxmox")}
			}
			return []openai.Tool{fnTool("filesystem"), fnTool("c12_other")}
		},
	}
	reg := flows.NewRegistry()
	if err := flows.RegisterCatalog(reg, env); err != nil {
		t.Fatal(err)
	}
	const rounds = 50
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			cfg := cfgs[(i+1)%2]
			cur.Store(cfg)
			env.refreshRegistry(reg, cfg)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			for _, info := range flows.DescribeNodeTypes(reg, nil) {
				if info.Availability.State == flows.BlockedState {
					t.Errorf("%s is blocked: %s", info.Type, info.Availability.Reason)
					return
				}
			}
		}
	}()
	c12Within(t, "describing while refreshing", wg.Wait)
	final := cur.Load()
	env.refreshRegistry(reg, final)
	_, hasProxmox := reg.Lookup(flows.GenericTypePrefix + "proxmox")
	_, hasOther := reg.Lookup(flows.GenericTypePrefix + "c12_other")
	if hasProxmox != (final == cfgs[1]) || hasOther != (final == cfgs[0]) {
		t.Fatalf("the registry holds proxmox=%v c12_other=%v, which is not the current configuration", hasProxmox, hasOther)
	}
}

// Requirement 1: a refresh that passed its current-configuration check installs before
// a refresh of the newer configuration starts: refreshMu orders them, so the newer tools
// win. Without the mutex the second refresh installs first and the first one overwrites
// it with the tools of the replaced configuration.
func TestC12RefreshMuOrdersInstalls(t *testing.T) {
	cfg1, cfg2 := &config.Config{}, &config.Config{}
	var cur atomic.Pointer[config.Config]
	cur.Store(cfg1)
	var calls atomic.Int32
	checked, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	env := &flowCatalogEnv{
		// The second current() call of the first refresh is its install check (the first
		// is the cache policy in snapshot): it reads cfg1, then holds the refresh between
		// the check and the install.
		current: func() *config.Config {
			v := cur.Load()
			if calls.Add(1) == 2 {
				close(checked)
				<-release
			}
			return v
		},
		schemas: func(cfg *config.Config) []openai.Tool {
			if cfg == cfg1 {
				return []openai.Tool{fnTool("filesystem"), fnTool("c12_old_only")}
			}
			return []openai.Tool{fnTool("filesystem"), fnTool("proxmox")}
		},
	}
	reg := flows.NewRegistry()
	firstDone, secondDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(firstDone)
		env.refreshRegistry(reg, cfg1)
	}()
	c12Wait(t, "the first refresh's install check", checked)
	cur.Store(cfg2)
	go func() {
		defer close(secondDone)
		env.refreshRegistry(reg, cfg2)
	}()
	// With refreshMu the second refresh cannot finish before the first; without it, it
	// finishes here.
	select {
	case <-secondDone:
	case <-time.After(c12OrderProbe):
	}
	releaseOnce.Do(func() { close(release) })
	c12Wait(t, "the first refresh", firstDone)
	c12Wait(t, "the second refresh", secondDone)
	if _, ok := reg.Lookup(flows.GenericTypePrefix + "c12_old_only"); ok {
		t.Fatal("the refresh of the replaced configuration installed last")
	}
	if _, ok := reg.Lookup(flows.GenericTypePrefix + "proxmox"); !ok {
		t.Fatal("the registry lacks the current configuration's tools")
	}
}

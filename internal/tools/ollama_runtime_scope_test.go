package tools

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

func newOllamaRuntimeForTest(t *testing.T) *ollamaRuntime {
	t.Helper()
	previous := ollamaRuntimeWork
	w := &ollamaRuntime{jobs: map[string]*ollamaWork{}, active: map[string]*ollamaWork{}}
	ollamaRuntimeWork = w
	t.Cleanup(func() { ShutdownOllamaRuntime(); ollamaRuntimeWork = previous })
	return w
}

func TestOllamaConfigChangesOnlyCancelAffectedRoles(t *testing.T) {
	for _, test := range []struct {
		name, container    string
		managed            bool
		unrelated, changed func(*config.Config)
	}{
		{"embeddings", ollamaEmbContainerName, true,
			func(c *config.Config) { c.Ollama.ManagedInstance.MemoryLimit = "4g" },
			func(c *config.Config) { c.Embeddings.LocalOllama.Model = "new-embedding" }},
		{"inference", ollamaManagedContainerName, true,
			func(c *config.Config) { c.Embeddings.LocalOllama.Model = "new-embedding" },
			func(c *config.Config) { c.Ollama.ManagedInstance.MemoryLimit = "4g" }},
		{"embedding-permissions", ollamaEmbContainerName, true,
			func(c *config.Config) { c.Ollama.ReadOnly = true },
			func(c *config.Config) { c.Docker.ReadOnly = true }},
		{"inference-permissions", ollamaManagedContainerName, true,
			func(c *config.Config) { c.Ollama.ReadOnly = true },
			func(c *config.Config) { c.Docker.ReadOnly = true }},
		{"managed-pull", "", true,
			func(c *config.Config) { c.Embeddings.LocalOllama.Model = "new-embedding" },
			func(c *config.Config) { c.Ollama.ManagedInstance.MemoryLimit = "4g" }},
		{"external-pull", "", false,
			func(c *config.Config) { c.Docker.Enabled = false; c.Docker.Host = "tcp://other:2375" },
			func(c *config.Config) { c.Ollama.ReadOnly = true }},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := newOllamaRuntimeForTest(t)
			var cfg config.Config
			cfg.Docker.Enabled = true
			cfg.Embeddings.LocalOllama.Enabled = test.managed
			cfg.Embeddings.LocalOllama.Model = "embedding"
			cfg.Ollama.Enabled = true
			cfg.Ollama.URL = "http://fixture.invalid"
			cfg.Ollama.ManagedInstance.Enabled = test.managed
			ConfigureOllamaRuntime(context.Background(), &cfg)
			oldRevision := w.revision
			op, err := w.start(oldRevision, "role", "", test.container, func(job *ollamaWork) error {
				<-job.ctx.Done()
				return job.ctx.Err()
			})
			if err != nil {
				t.Fatal(err)
			}
			w.mu.Lock()
			job := w.jobs[op.ID]
			w.mu.Unlock()
			test.unrelated(&cfg)
			ConfigureOllamaRuntime(context.Background(), &cfg)
			cfg.Docker.AllowHostAccess = true
			ConfigureOllamaRuntime(context.Background(), &cfg)
			if job.ctx.Err() != nil {
				t.Fatal("unrelated settings cancelled work")
			}
			if operations := OllamaOperations(test.container); len(operations) != 1 || operations[0].ID != op.ID {
				t.Fatal("unchanged role lost its visible receipt", operations)
			}
			if _, err := w.start(oldRevision, "stale", "", test.container, func(*ollamaWork) error { t.Error("stale snapshot ran"); return nil }); err == nil {
				t.Fatal("global admission fence accepted stale settings")
			}
			test.changed(&cfg)
			ConfigureOllamaRuntime(context.Background(), &cfg)
			if job.ctx.Err() == nil {
				t.Fatal("changed role retained obsolete work")
			}
		})
	}
}

func TestOllamaStartupWorkFollowsServerOwner(t *testing.T) {
	w := newOllamaRuntimeForTest(t)
	var cfg config.Config
	ConfigureOllamaRuntime(context.Background(), &cfg)
	op, err := w.start(w.revision, "startup", "", ollamaEmbContainerName, func(job *ollamaWork) error {
		<-job.ctx.Done()
		return job.ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	job := w.jobs[op.ID]
	w.mu.Unlock()
	owner, cancel := context.WithCancel(context.Background())
	defer cancel()
	ConfigureOllamaRuntime(owner, &cfg)
	if job.ctx.Err() != nil {
		t.Fatal("server handoff cancelled unchanged startup work")
	}
	cancel()
	select {
	case <-job.done:
	case <-time.After(3 * time.Second):
		t.Fatal("startup work outlived server ownership")
	}
}

func TestOllamaStopAdmissionSurvivesBusyWorkersAndFullHistory(t *testing.T) {
	for _, fullHistory := range []bool{false, true} {
		t.Run(fmt.Sprint(fullHistory), func(t *testing.T) {
			configureDockerSecurityTestPermissions(t, false)
			w := newOllamaRuntimeForTest(t)
			releaseWorkers := make(chan struct{})
			defer close(releaseWorkers)
			releaseStops := make(chan struct{})
			var stopRelease sync.Once
			defer stopRelease.Do(func() { close(releaseStops) })
			var cfg config.Config
			cfg.Docker.Enabled = true
			cfg.Docker.Host = fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("unexpected mutation: %s", r.Method)
				}
				<-releaseStops
				w.WriteHeader(http.StatusNotFound)
			})
			cfg.Ollama.ManagedInstance.Enabled = true
			cfg.Embeddings.LocalOllama.Enabled = true
			ConfigureOllamaRuntime(context.Background(), &cfg)
			if fullHistory {
				for i := 0; i < 128; i++ {
					done := make(chan struct{})
					close(done)
					id := fmt.Sprint(i)
					w.jobs[id] = &ollamaWork{OllamaOperation: OllamaOperation{ID: id, State: "unknown"}, key: id, done: done}
				}
			} else {
				for i := 0; i < 4; i++ {
					if _, err := w.start(w.revision, fmt.Sprint(i), "", "", func(job *ollamaWork) error {
						<-job.ctx.Done()
						<-releaseWorkers
						return job.ctx.Err()
					}); err != nil {
						t.Fatal(err)
					}
				}
			}
			cfg.Ollama.ManagedInstance.Enabled = false
			cfg.Embeddings.LocalOllama.Enabled = false
			ConfigureOllamaRuntime(context.Background(), &cfg)
			ops := []OllamaOperation{}
			for _, embeddings := range []bool{false, true} {
				op, err := StartOllamaStop(&cfg, embeddings)
				if err != nil {
					t.Fatal("disable lost its stop", err)
				}
				again, err := StartOllamaStop(&cfg, embeddings)
				if err != nil || again.ID != op.ID {
					t.Fatal("duplicate stop used another slot", op, again, err)
				}
				ops = append(ops, op)
			}
			if _, err := w.start(w.revision, "extra", "", "", func(*ollamaWork) error { t.Error("ordinary work exceeded admission limit"); return nil }); err == nil {
				t.Fatal("stop reservation admitted more ordinary work")
			}
			stopRelease.Do(func() { close(releaseStops) })
			for _, op := range ops {
				w.mu.Lock()
				job := w.jobs[op.ID]
				w.mu.Unlock()
				select {
				case <-job.done:
				case <-time.After(3 * time.Second):
					t.Fatal("stop never finished")
				}
				w.mu.Lock()
				state := job.State
				w.mu.Unlock()
				if state != "stopped" {
					t.Fatal(state)
				}
			}
			if OllamaStopsPending() {
				t.Fatal("confirmed stops remained pending")
			}
		})
	}
}

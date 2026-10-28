package injectionguard

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

func TestModelReferenceOffline(t *testing.T) {
	root := os.Getenv("AURAGO_INJECTION_GUARD_CACHE")
	if root == "" || os.Getenv("AURAGO_INJECTION_GUARD_INTEGRATION") != "1" {
		t.Skip("explicit prepared-cache/integration opt-in required")
	}
	// connection uses VerifyRuntime and the worker verifies local files. No Prepare/download path.
	m := NewManager(context.Background(), root, "")
	defer m.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	start := time.Now()
	w, err := m.connection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("offline cold startup/selftest=%s platform=%s/%s CPUs=%d", time.Since(start), runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	proc, err := process.NewProcess(int32(w.(*worker).cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	var peak atomic.Uint64
	stop := make(chan struct{})
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if info, e := proc.MemoryInfo(); e == nil {
					peak.Store(max(peak.Load(), info.RSS))
				}
			}
		}
	}()
	defer func() {
		close(stop)
		<-joined
		t.Logf("sampled peak worker RSS=%.1f MiB", float64(peak.Load())/(1024*1024))
	}()
	data, err := os.ReadFile("testdata/model_reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Runtime string
		Cases   []struct {
			Kind, Text      string
			Score           float64
			Tokens, Windows int
		}
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	counts, hits := map[string]int{}, map[string]int{}
	for i, tc := range fixture.Cases {
		started := time.Now()
		call, cancel := context.WithTimeout(context.Background(), ScanTimeout)
		score, err := w.Score(call, tc.Text)
		cancel()
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if math.Abs(score-tc.Score) > .002 {
			t.Errorf("case %d score %.8f reference %.8f", i, score, tc.Score)
		}
		counts[tc.Kind]++
		if score >= Threshold {
			hits[tc.Kind]++
		}
		t.Logf("case=%d kind=%s tokens=%d windows=%d score=%.8f warm=%s", i, tc.Kind, tc.Tokens, tc.Windows, score, time.Since(started))
	}
	t.Logf("reference ORT=%s counts=%v positive=%v", fixture.Runtime, counts, hits)
	// A dead native worker is withheld, then restarted on the next explicit evaluation.
	m.Configure(true, "quarantine")
	m.mu.Lock()
	m.status.Ready = true
	m.mu.Unlock()
	_, _, _, err = m.Evaluate(context.Background(), safeSample)
	if err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	dead := m.worker.(*worker)
	m.mu.Unlock()
	_ = dead.cmd.Process.Kill()
	<-dead.done
	if _, _, _, err = m.Evaluate(context.Background(), safeSample); err == nil {
		t.Fatal("crashed worker allowed content")
	}
	if _, _, _, err = m.Evaluate(context.Background(), safeSample); err != nil {
		t.Fatal("worker did not recover", err)
	}
}

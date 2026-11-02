package server

import (
	"sync"
	"testing"

	"aurago/internal/config"
)

func TestConfigSnapshotConcurrentFirstPublication(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 100; j++ {
				if s.ConfigSnapshot() == nil {
					t.Error("missing snapshot")
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		s.CfgMu.Lock()
		defer s.CfgMu.Unlock()
		s.replaceConfigSnapshot(&config.Config{})
	}()
	close(start)
	wg.Wait()
	if s.ConfigSnapshot() != s.Cfg {
		t.Fatal("publication lost")
	}
}

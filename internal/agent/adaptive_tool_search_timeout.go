package agent

import (
	"fmt"
	"log/slog"
	"time"
)

const adaptiveToolGuideSearchTimeout = 1500 * time.Millisecond

func searchToolGuidesWithTimeout(searcher toolGuideSearcher, query string, topK int, logger *slog.Logger) ([]string, error) {
	if searcher == nil || query == "" {
		return nil, nil
	}

	type result struct {
		paths []string
		err   error
	}

	resultCh := make(chan result, 1)
	go func() {
		paths, err := searcher.SearchToolGuides(query, topK)
		resultCh <- result{paths: paths, err: err}
	}()

	select {
	case res := <-resultCh:
		return res.paths, res.err
	case <-time.After(adaptiveToolGuideSearchTimeout):
		if logger != nil {
			logger.Warn("[AdaptiveTools] Semantic tool search timed out", "timeout_ms", adaptiveToolGuideSearchTimeout.Milliseconds())
		}
		return nil, fmt.Errorf("semantic tool search timed out after %s", adaptiveToolGuideSearchTimeout)
	}
}

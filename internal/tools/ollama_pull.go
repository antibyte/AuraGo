package tools

import (
	"context"
	"strings"
	"sync"
)

type ollamaPullFlight struct {
	done  chan struct{}
	owner context.Context
	err   error
}

var ollamaModelFlights = struct {
	sync.Mutex
	entries map[string]*ollamaPullFlight
}{entries: map[string]*ollamaPullFlight{}}

// Automatic and agent pulls share an endpoint/model flight. A waiter can cancel
// independently; an uncertain mutation is returned without automatic replay.
func ollamaPullShared(cfg OllamaConfig, model string) error {
	ctx := nativeHTTPContext(cfg.Context)
	if err := ctx.Err(); err != nil {
		return err
	}
	key := strings.TrimRight(cfg.URL, "/") + "\x00" + model
	ollamaModelFlights.Lock()
	if flight := ollamaModelFlights.entries[key]; flight != nil && flight.owner.Err() == nil {
		ollamaModelFlights.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-flight.done:
			return flight.err
		}
	}
	flight := &ollamaPullFlight{done: make(chan struct{}), owner: ctx}
	ollamaModelFlights.entries[key] = flight
	ollamaModelFlights.Unlock()
	flight.err = ollamaPullVerified(cfg, model)
	ollamaModelFlights.Lock()
	close(flight.done)
	if ollamaModelFlights.entries[key] == flight {
		delete(ollamaModelFlights.entries, key)
	}
	ollamaModelFlights.Unlock()
	return flight.err
}

package llm

import (
	"context"
	"sync"
)

type attemptRouteKey struct{}
type attemptRouteObservation struct {
	mu       sync.RWMutex
	route    ModelRoute
	assigned bool
}

func observeAttemptRoute(ctx context.Context) (context.Context, *attemptRouteObservation) {
	observation := &attemptRouteObservation{}
	return context.WithValue(ctx, attemptRouteKey{}, observation), observation
}
func recordAttemptRoute(ctx context.Context, route ModelRoute) {
	if observation, ok := ctx.Value(attemptRouteKey{}).(*attemptRouteObservation); ok {
		observation.mu.Lock()
		observation.route = route
		observation.assigned = true
		observation.mu.Unlock()
	}
}
func (o *attemptRouteObservation) snapshot(client ChatClient, model string) ModelRoute {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.assigned {
		return o.route
	}
	if owner, ok := client.(interface{ ActiveRoute() ModelRoute }); ok {
		return owner.ActiveRoute()
	}
	kind, resolved := activeProviderAndModel(client, model)
	return ModelRoute{ProviderType: kind, Model: resolved}
}

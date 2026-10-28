package dockerstacks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aurago/internal/dockerhost"
)

// serviceLabel names a container by its compose service, else by its name.
func serviceLabel(c dockerhost.ContainerSummary) string {
	switch {
	case c.ComposeService != "":
		return c.ComposeService
	case len(c.Names) > 0:
		return c.Names[0]
	}
	return c.ID
}

// assessHealth judges the containers of a project after up. It is settled when
// no container is still starting its health check or restarting, or when timedOut.
// The warning names what is wrong: unhealthy services, services that exited
// with an error, services that keep restarting, and, after the time ran out,
// the services that never became healthy.
func assessHealth(members []dockerhost.ContainerSummary, timedOut bool, wait time.Duration) (settled bool, warning string) {
	var starting, restarting, unhealthy, crashed []string
	for _, c := range members {
		name := serviceLabel(c)
		switch {
		case c.State == "restarting":
			restarting = append(restarting, name)
		case c.State == "dead":
			crashed = append(crashed, name+" is dead")
		case c.State == "exited":
			if code, ok := exitCodeOf(c); ok && code != 0 {
				crashed = append(crashed, fmt.Sprintf("%s exited with code %d", name, code))
			}
		case c.Health == "starting":
			starting = append(starting, name)
		case c.Health == "unhealthy":
			unhealthy = append(unhealthy, name)
		}
	}
	settled = timedOut || (len(starting) == 0 && len(restarting) == 0)
	if !settled {
		return false, ""
	}
	var parts []string
	if len(unhealthy) > 0 {
		parts = append(parts, strings.Join(unhealthy, ", ")+isOrAre(len(unhealthy), "is", "are")+" unhealthy")
	}
	parts = append(parts, crashed...)
	if len(restarting) > 0 {
		parts = append(parts, strings.Join(restarting, ", ")+" keeps restarting")
	}
	if len(starting) > 0 {
		parts = append(parts, fmt.Sprintf("%s did not become healthy within %s", strings.Join(starting, ", "), wait))
	}
	return true, strings.Join(parts, "; ")
}

func isOrAre(n int, one, many string) string {
	if n == 1 {
		return " " + one
	}
	return " " + many
}

// healthWait watches the containers of a project after up until none is still
// starting, for at most HealthWait. It sets the state health_wait when there is
// something to wait for. warning describes what is wrong when it ends; ports are
// the published ports of the project.
func (s *Service) healthWait(ctx context.Context, run *opRun, e engine, project string) (warning string, ports []PortLink, err error) {
	waitCtx, cancel := context.WithTimeout(ctx, s.opts.HealthWait)
	defer cancel()
	waiting := false
	for {
		containers, listErr := dockerhost.ListContainers(waitCtx, e.c)
		switch {
		case listErr != nil && ctx.Err() != nil:
			return "", nil, ctx.Err()
		case listErr != nil && waitCtx.Err() == nil:
			return "", nil, listErr
		case listErr == nil:
			members := projectGroups(containers)[project]
			if settled, text := assessHealth(members, false, s.opts.HealthWait); settled {
				return text, portLinks(members), nil
			}
			if !waiting {
				waiting = true
				run.setState(StateHealthWait)
			}
			select {
			case <-ctx.Done():
				return "", nil, ctx.Err()
			case <-time.After(s.opts.HealthPoll):
				continue
			case <-waitCtx.Done():
			}
		}
		// The time is up: judge what is there now.
		final, err := dockerhost.ListContainers(ctx, e.c)
		if err != nil {
			return "", nil, err
		}
		members := projectGroups(final)[project]
		_, text := assessHealth(members, true, s.opts.HealthWait)
		return text, portLinks(members), nil
	}
}

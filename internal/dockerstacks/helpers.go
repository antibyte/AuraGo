package dockerstacks

import (
	"context"
	"time"

	"aurago/internal/dockerhost"
)

// resolveRunnerImage finds the reference under which the host has the runner
// image: the pinned digest reference, or the local tag of an image that was
// copied from another engine (a saved digest reference loses its name on load).
func (e engine) resolveRunnerImage(ctx context.Context) (ref string, present bool, err error) {
	for _, candidate := range []string{RunnerImage, RunnerLocalTag} {
		_, found, err := e.inspectImage(ctx, candidate)
		if err != nil {
			return "", false, err
		}
		if found {
			return candidate, true, nil
		}
	}
	return "", false, nil
}

// IO container states.
const (
	ioOK       = "ok"
	ioMissing  = "missing"
	ioMismatch = "mismatch"
)

// ioStatus inspects the IO container of a host. "ok" means a container of that
// name with the IO role label that was created for this host's stacks
// directory; "mismatch" means a container of that name that is not (the stacks
// directory of the host changed, or the name is taken by something else). The
// container's inspect document is returned when there is one.
func (e engine) ioStatus(ctx context.Context, h dockerhost.Host) (string, containerState, error) {
	st, err := e.inspectContainer(ctx, IOContainerName)
	if isNotFound(err) {
		return ioMissing, containerState{}, nil
	}
	if err != nil {
		return "", containerState{}, err
	}
	if st.Labels[LabelRole] != RoleIO || st.Labels[LabelStacksDir] != h.StacksDir {
		return ioMismatch, st, nil
	}
	return ioOK, st, nil
}

// waitExited inspects a container until it is no longer running and returns
// its final state. The log stream of a container ends when it exits, but the
// state can lag by a moment.
func (e engine) waitExited(ctx context.Context, ref string) (containerState, error) {
	for {
		st, err := e.inspectContainer(ctx, ref)
		if err != nil {
			return containerState{}, err
		}
		if !st.Running && st.Status != "created" && st.Status != "restarting" {
			return st, nil
		}
		select {
		case <-ctx.Done():
			return containerState{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// apiTooOldError is the error of a host whose Engine API is below 1.41.
func apiTooOldError(apiVersion string) *Error {
	return newError(CodeAPITooOld,
		"Docker Engine API %q is too old: stacks need API 1.%d (Docker 20.10) or newer", apiVersion, MinAPIMinor).
		withFields(map[string]any{"api_version": apiVersion})
}

// notPreparedError is the error of a read on a host without the helpers.
func notPreparedError(st HelperStatus) *Error {
	return newError(CodeHostNotPrepared, "this Docker host is not prepared for stacks: the helper image %s is needed", RunnerImageTag).
		withFields(map[string]any{"helpers": st})
}

// helpers reports what the host has of the helpers and the image reference to
// create helper containers from. Nothing is created or pulled.
func (s *Service) helpers(ctx context.Context, e engine, h dockerhost.Host) (HelperStatus, string, error) {
	st := HelperStatus{RunnerImage: RunnerImageTag, IOContainer: ioMissing}
	v, err := e.version(ctx)
	if err != nil {
		return st, "", err
	}
	st.APIVersion = v.APIVersion
	st.APISupported = supportsStacks(v.APIVersion)
	if !st.APISupported {
		return st, "", nil
	}
	ref, present, err := e.resolveRunnerImage(ctx)
	if err != nil {
		return st, "", err
	}
	st.RunnerImagePresent = present
	io, _, err := e.ioStatus(ctx, h)
	if err != nil {
		return st, "", err
	}
	st.IOContainer = io
	st.Prepared = st.APISupported && st.RunnerImagePresent && st.IOContainer == ioOK
	return st, ref, nil
}

// Helpers reports what a host has of the helper image and the IO container.
// It only reads.
func (s *Service) Helpers(ctx context.Context, hostID string) (HelperStatus, error) {
	h, e, err := s.target(ctx, hostID)
	if err != nil {
		return HelperStatus{}, err
	}
	st, _, err := s.helpers(ctx, e, h)
	return st, err
}

// requirePrepared returns the helper image of a host that is ready for stacks.
// A host that is not gets CodeHostNotPrepared (with the helper status) or
// CodeAPITooOld; reads never pull or create anything.
func (s *Service) requirePrepared(ctx context.Context, e engine, h dockerhost.Host) (string, error) {
	if image, ok := s.readyImage(h.ID); ok {
		return image, nil
	}
	st, ref, err := s.helpers(ctx, e, h)
	if err != nil {
		return "", err
	}
	switch {
	case !st.APISupported:
		return "", apiTooOldError(st.APIVersion)
	case !st.Prepared:
		return "", notPreparedError(st)
	}
	s.markReady(h.ID, ref)
	return ref, nil
}

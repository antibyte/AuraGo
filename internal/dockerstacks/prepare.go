package dockerstacks

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"aurago/internal/dockerhost"
)

// pullCommand is the command that puts the runner image on a host by hand.
const pullCommand = "docker pull " + RunnerImageTag + "@" + RunnerImageDigest

// imageMissingError says that the runner image is neither on the host nor
// obtainable, and names the exact command that fixes it.
func imageMissingError(reason string) *Error {
	return newError(CodeRunnerImageMissing,
		"The helper image %s is not on this Docker host and could not be pulled or copied (%s). Run `%s` on the host, then prepare it again.",
		RunnerImageTag, reason, pullCommand).
		withFields(map[string]any{"command": pullCommand})
}

// ensureHost makes a host ready for stacks, step by step: the Engine API must be
// 1.41 or newer, the runner image must be on the host (pulled, or copied from the
// local engine when the pull fails), and the IO container must exist for the
// host's stacks directory. It returns the image reference to create helper
// containers from. Every step that does work is reported in the operation's
// output; verbose also reports the steps that find everything in place.
// Concurrent calls for one host are serialized.
func (s *Service) ensureHost(ctx context.Context, run *opRun, e engine, h dockerhost.Host, verbose bool) (string, error) {
	lock := s.hostLock(h.ID)
	lock.Lock()
	defer lock.Unlock()

	v, err := e.version(ctx)
	if err != nil {
		return "", err
	}
	if !supportsStacks(v.APIVersion) {
		return "", apiTooOldError(v.APIVersion)
	}
	if verbose {
		run.line(fmt.Sprintf("Docker Engine %s, API %s", v.Version, v.APIVersion))
	}

	ref, present, err := e.resolveRunnerImage(ctx)
	if err != nil {
		return "", err
	}
	if present {
		if verbose {
			run.line("Helper image " + RunnerImageTag + " is present")
		}
	} else {
		run.line("Pulling the helper image " + RunnerImageTag + " (about 64 MB)")
		if ref, err = s.acquireRunnerImage(ctx, run, e, h); err != nil {
			return "", err
		}
	}

	if err := s.ensureIOContainer(ctx, run, e, h, ref, verbose); err != nil {
		return "", err
	}
	s.markReady(h.ID, ref)
	return ref, nil
}

// acquireRunnerImage pulls the runner image. When the pull fails on a host
// other than the local one, the image is copied from the local engine, but only
// if both engines report the same OS and architecture. Otherwise the error is
// CodeRunnerImageMissing with the exact pull command.
func (s *Service) acquireRunnerImage(ctx context.Context, run *opRun, e engine, h dockerhost.Host) (string, error) {
	pullErr := e.pullImage(ctx, runnerImageRepo, runnerImagePullTag, run.pullMessage)
	if pullErr == nil {
		return RunnerImage, nil
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	reason := engineMessage(pullErr)
	if h.ID == s.opts.LocalHostID {
		return "", imageMissingError(reason)
	}
	run.line("The pull failed (" + reason + "); trying to copy the image from the local engine")
	return s.copyRunnerImage(ctx, run, e, reason)
}

// copyRunnerImage streams the runner image from the local engine to the target
// (GET /images/<ref>/get into POST /images/load) and tags it with RunnerLocalTag.
func (s *Service) copyRunnerImage(ctx context.Context, run *opRun, target engine, pullReason string) (string, error) {
	local, err := s.hosts.Client(ctx, s.opts.LocalHostID)
	if err != nil {
		return "", imageMissingError(pullReason + "; the local engine is not reachable")
	}
	source := newEngine(local)
	info, found, err := source.inspectImage(ctx, RunnerImage)
	if err != nil {
		return "", imageMissingError(pullReason + "; the local engine could not be asked: " + engineMessage(err))
	}
	if !found {
		return "", imageMissingError(pullReason + "; the local engine does not have the image either")
	}
	sv, err := source.version(ctx)
	if err != nil {
		return "", imageMissingError(pullReason + "; the local engine could not be asked: " + engineMessage(err))
	}
	tv, err := target.version(ctx)
	if err != nil {
		return "", err
	}
	if sv.OS != tv.OS || sv.Arch != tv.Arch {
		return "", imageMissingError(fmt.Sprintf("%s; the local engine is %s/%s but this host is %s/%s", pullReason, sv.OS, sv.Arch, tv.OS, tv.Arch))
	}

	run.line(fmt.Sprintf("Copying the helper image from the local engine (%s/%s)", sv.OS, sv.Arch))
	body, err := source.saveImage(ctx, RunnerImage)
	if err != nil {
		return "", err
	}
	defer body.Close()
	loaded, err := target.loadImage(ctx, body)
	if err != nil {
		return "", err
	}
	if ref, present, err := target.resolveRunnerImage(ctx); err != nil {
		return "", err
	} else if present {
		return ref, nil // a store that keeps the digest name needs no tag
	}
	if loaded == "" {
		loaded = info.ID
	}
	repo, tag, _ := strings.Cut(RunnerLocalTag, ":")
	if err := target.tagImage(ctx, loaded, repo, tag); err != nil {
		return "", err
	}
	return RunnerLocalTag, nil
}

// ensureIOContainer creates the IO container of the host's stacks directory,
// or recreates it when it was made for another directory. A container of that
// name without the IO role label is never touched.
func (s *Service) ensureIOContainer(ctx context.Context, run *opRun, e engine, h dockerhost.Host, image string, verbose bool) error {
	state, st, err := e.ioStatus(ctx, h)
	if err != nil {
		return err
	}
	switch state {
	case ioOK:
		if verbose {
			run.line("IO container " + IOContainerName + " is in place")
		}
		return nil
	case ioMismatch:
		if st.Labels[LabelRole] != RoleIO {
			return newError(CodeStacksDirInvalid,
				"a container named %s already exists and is not a Docker Manager helper; rename or remove it, then prepare the host again", IOContainerName)
		}
		run.line("Recreating the IO container for the stacks directory " + h.StacksDir)
		if err := e.removeContainer(ctx, IOContainerName); err != nil {
			return err
		}
	default:
		run.line("Creating the IO container " + IOContainerName)
	}
	if _, err := e.createContainer(ctx, IOContainerName, ioSpec(image, h)); err != nil {
		if dockerhost.StatusOf(err) == http.StatusConflict { // another preparation was faster
			if again, _, ioErr := e.ioStatus(ctx, h); ioErr == nil && again == ioOK {
				return nil
			}
		}
		return err
	}
	return nil
}

// stacksDirError explains that a helper container could not use the stacks
// directory (the Engine refused the bind mount, for example because Docker
// Desktop does not share the path).
func stacksDirError(h dockerhost.Host, cause error) *Error {
	e := newError(CodeStacksDirInvalid, "the Docker host cannot use the stacks directory %s: %s", h.StacksDir, engineMessage(cause))
	e.Err = cause
	return e
}

// limitedBuffer keeps at most max bytes and drops the rest.
type limitedBuffer struct {
	max int
	buf []byte
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.max - len(b.buf); room > 0 {
		if len(p) < room {
			room = len(p)
		}
		b.buf = append(b.buf, p[:room]...)
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string { return string(b.buf) }

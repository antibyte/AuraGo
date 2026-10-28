package dockerstacks

import (
	"context"
	"path"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

// Compose acts on every container that carries the project label, wherever it
// was started from. The stack's project name is the stack name, so a compose
// project of that name that runs from another folder is not this stack: an up
// with --remove-orphans, a down or a delete would remove or recreate its
// containers and volumes. The Docker Manager therefore refuses to start
// anything for a stack whose name another folder's project already uses.

// ownsContainer reports whether a project container belongs to the stack folder
// dir: it carries no working directory label (an older Compose, or a hand-made
// label set) or the label names that very folder.
func ownsContainer(c dockerhost.ContainerSummary, dir string) bool {
	return c.ComposeWorkingDir == "" || path.Clean(c.ComposeWorkingDir) == path.Clean(dir)
}

// splitProject separates the containers of a compose project into those that
// belong to the stack folder dir and those that were started from another one.
func splitProject(members []dockerhost.ContainerSummary, dir string) (own, foreign []dockerhost.ContainerSummary) {
	for _, c := range members {
		if ownsContainer(c, dir) {
			own = append(own, c)
		} else {
			foreign = append(foreign, c)
		}
	}
	return own, foreign
}

// foreignWorkingDir returns the working directory of the first container of the
// compose project that is not the stack folder dir's, "" when there is none.
func foreignWorkingDir(containers []dockerhost.ContainerSummary, project, dir string) string {
	_, foreign := splitProject(projectGroups(containers)[project], dir)
	if len(foreign) == 0 {
		return ""
	}
	return foreign[0].ComposeWorkingDir
}

// projectConflict is the error for a stack name that a compose project of
// another folder uses.
func projectConflict(name, workingDir string) *Error {
	return newError(CodeProjectConflict,
		"a compose project named %q already runs from %s, not from this stack's folder; compose would act on its containers and volumes. Rename the stack or remove that project first.",
		name, dockerutil.SanitizeOneLine(workingDir, 200)).
		withFields(map[string]any{"working_dir": workingDir})
}

// checkProjectConflict asks the Engine whether a compose project named like the
// stack runs from another folder, and answers CodeProjectConflict when it does.
func (s *Service) checkProjectConflict(ctx context.Context, e engine, h dockerhost.Host, name string) error {
	containers, err := dockerhost.ListContainers(ctx, e.c)
	if err != nil {
		return err
	}
	if other := foreignWorkingDir(containers, name, stackDir(h, name)); other != "" {
		return projectConflict(name, other)
	}
	return nil
}

// conflictPrecheckTimeout bounds the early check of Start, which has to answer
// quickly even when the host is slow.
const conflictPrecheckTimeout = 5 * time.Second

// refuseForeignProject is the early refusal of Start. It is best effort: a host
// that cannot be asked right now is no reason to refuse (Start has always
// answered at once and left unreachable hosts to the operation, which fails with
// the host's error), and the operation checks again before it creates its runner.
func (s *Service) refuseForeignProject(ctx context.Context, hostID, name string) error {
	ctx, cancel := context.WithTimeout(ctx, conflictPrecheckTimeout)
	defer cancel()
	h, e, err := s.target(ctx, hostID)
	if err != nil {
		return nil
	}
	if err := s.checkProjectConflict(ctx, e, h, name); CodeOf(err) == CodeProjectConflict {
		return err
	}
	return nil
}

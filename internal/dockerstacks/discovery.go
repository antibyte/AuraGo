package dockerstacks

import (
	"context"
	"slices"
	"sort"
	"strconv"
	"strings"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

const (
	listerStdoutMax = 4 << 20
	listerStderrMax = 4 << 10
)

// parseListerOutput reads the lines `find` printed (one path per line, two
// levels below the stacks directory dir) into the folders that hold a compose
// file or a .env. Of several compose files in one folder the first of
// ComposeFileNames wins. A folder whose name cannot be a stack name is ignored
// when it holds a compose file.
func parseListerOutput(dir, stdout string) (map[string]scanFolder, []IgnoredFolder) {
	folders := map[string]scanFolder{}
	invalid := map[string]bool{}
	for _, line := range strings.Split(stdout, "\n") {
		rel, ok := strings.CutPrefix(strings.TrimRight(line, "\r"), dir+"/")
		if !ok {
			continue
		}
		name, file, ok := strings.Cut(rel, "/")
		if !ok || file == "" || strings.Contains(file, "/") {
			continue
		}
		rank := composeFileRank(file)
		if file != EnvFileName && rank < 0 {
			continue
		}
		if ValidateName(name) != nil {
			if rank >= 0 {
				invalid[name] = true
			}
			continue
		}
		f := folders[name]
		switch {
		case file == EnvFileName:
			f.hasEnv = true
		case f.composeFile == "" || rank < composeFileRank(f.composeFile):
			f.composeFile = file
		}
		folders[name] = f
	}
	ignored := make([]IgnoredFolder, 0, len(invalid))
	for name := range invalid {
		ignored = append(ignored, IgnoredFolder{Name: name, Reason: "name_invalid"})
	}
	sort.Slice(ignored, func(i, j int) bool { return ignored[i].Name < ignored[j].Name })
	return folders, ignored
}

// runLister runs the short-lived lister container on the host and returns what
// `find` printed. The container is removed again whatever happens. find exits 1
// when it cannot read a data folder of some stack; that is no failure as long as
// it found something. An Engine that refuses the container (a stacks directory
// it cannot bind) or a find that found nothing and complained is
// CodeStacksDirInvalid. The run has a deadline of its own (listerTimeout) on top
// of the caller's: a find that hangs on a stalled mount is killed, and the
// answer is CodeStacksDirInvalid too.
func (s *Service) runLister(caller context.Context, e engine, h dockerhost.Host, image string) (out string, err error) {
	ctx, cancel := context.WithTimeout(caller, listerTimeout)
	defer cancel()
	defer func() {
		if err != nil && ctx.Err() != nil && caller.Err() == nil {
			out, err = "", newError(CodeStacksDirInvalid, "the stacks directory %s could not be listed within %s; a mount of it may be stalled", h.StacksDir, listerTimeout)
		}
	}()
	name := listerNamePrefix + newShortID()
	id, err := e.createContainer(ctx, name, listerSpec(image, h))
	if err != nil {
		return "", err
	}
	defer func() {
		if ctx.Err() != nil { // the deadline or a cancelled caller: stop the find before the container goes
			killCtx, cancelKill := context.WithTimeout(context.Background(), cleanupTimeout)
			_ = e.killContainer(killCtx, id)
			cancelKill()
		}
		s.removeQuietly(e, id)
	}()
	if err := e.startContainer(ctx, id); err != nil {
		if dockerhost.StatusOf(err) >= 400 {
			return "", stacksDirError(h, err)
		}
		return "", err
	}
	stdout := &limitedBuffer{max: listerStdoutMax}
	stderr := &limitedBuffer{max: listerStderrMax}
	if err := e.followLogs(ctx, id, 0, stdout, stderr); err != nil {
		return "", err
	}
	st, err := e.waitExited(ctx, id)
	if err != nil {
		return "", err
	}
	switch {
	case st.ExitCode == 0:
		return stdout.String(), nil
	case st.ExitCode == 1 && len(stdout.buf) > 0:
		return stdout.String(), nil
	}
	reason := firstNonBlankLine(stderr.String())
	if reason == "" {
		reason = "find exited with code " + strconv.Itoa(st.ExitCode)
	}
	return "", newError(CodeStacksDirInvalid, "the stacks directory %s could not be listed: %s", h.StacksDir, dockerutil.SanitizeOneLine(reason, 200))
}

// removeQuietly removes a helper container with a context of its own (the
// caller's may be cancelled) and logs a failure.
func (s *Service) removeQuietly(e engine, id string) {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	if err := e.removeContainer(ctx, id); err != nil {
		s.log.Warn("docker manager helper container could not be removed", "container", id, "error", err)
	}
}

// scanFolders lists the stacks directory with the lister and caches the result.
func (s *Service) scanFolders(ctx context.Context, e engine, h dockerhost.Host, image string) (*scanCache, error) {
	stdout, err := s.runLister(ctx, e, h, image)
	if err != nil {
		return nil, err
	}
	folders, ignored := parseListerOutput(h.StacksDir, stdout)
	c := &scanCache{at: s.opts.Now(), folders: folders, ignored: ignored}
	s.storeScan(h.ID, c)
	return c, nil
}

// discover returns the folder scan of a host: the cached one when it is younger
// than ListCacheTTL (and refresh is false), else a fresh lister run. A host
// that is not prepared is an error and nothing is created.
func (s *Service) discover(ctx context.Context, e engine, h dockerhost.Host, refresh bool) (*scanCache, bool, error) {
	if !refresh {
		if c, ok := s.cachedScan(h.ID); ok {
			return c, true, nil
		}
	}
	image, err := s.requirePrepared(ctx, e, h)
	if err != nil {
		return nil, false, err
	}
	c, err := s.scanFolders(ctx, e, h, image)
	return c, false, err
}

// operationRefs maps the stacks of a host that have an operation to it.
func (s *Service) operationRefs(hostID string) map[string]OperationRef {
	s.mu.Lock()
	runs := make([]*opRun, 0, len(s.runs))
	for _, run := range s.runs {
		runs = append(runs, run)
	}
	s.mu.Unlock()
	refs := map[string]OperationRef{}
	for _, run := range runs {
		run.mu.Lock()
		op := run.op
		run.mu.Unlock()
		if op.HostID == hostID && op.Stack != "" {
			refs[op.Stack] = OperationRef{ID: op.ID, Kind: op.Kind, State: op.State}
		}
	}
	return refs
}

// projectGroups groups the compose containers of a host by project. The
// Docker Manager's own helper containers never belong to a project.
func projectGroups(containers []dockerhost.ContainerSummary) map[string][]dockerhost.ContainerSummary {
	groups := map[string][]dockerhost.ContainerSummary{}
	for _, c := range containers {
		if c.ComposeProject == "" || isHelperContainer(c) {
			continue
		}
		groups[c.ComposeProject] = append(groups[c.ComposeProject], c)
	}
	return groups
}

// List returns the stacks of a host (the folders of the stacks directory that
// hold a compose file), the compose projects that are not stacks, and the
// folders that were ignored. The folder scan runs a lister container and is
// cached for ListCacheTTL; refresh bypasses the cache. Container status is read
// live on every call. A host without the helpers answers CodeHostNotPrepared.
func (s *Service) List(ctx context.Context, hostID string, refresh bool) (Listing, error) {
	h, e, err := s.target(ctx, hostID)
	if err != nil {
		return Listing{}, err
	}
	scan, cached, err := s.discover(ctx, e, h, refresh)
	if err != nil {
		return Listing{}, err
	}
	containers, err := dockerhost.ListContainers(ctx, e.c)
	if err != nil {
		return Listing{}, err
	}
	return s.buildListing(h, scan, cached, containers), nil
}

func (s *Service) buildListing(h dockerhost.Host, scan *scanCache, cached bool, containers []dockerhost.ContainerSummary) Listing {
	groups := projectGroups(containers)
	refs := s.operationRefs(h.ID)

	names := make([]string, 0, len(scan.folders))
	for name, f := range scan.folders {
		if f.composeFile != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	listing := Listing{
		HostID:    h.ID,
		StacksDir: h.StacksDir,
		Stacks:    make([]StackSummary, 0, len(names)),
		External:  []ProjectSummary{},
		Ignored:   append([]IgnoredFolder{}, scan.ignored...),
		ScannedAt: scan.at,
		Cached:    cached,
	}
	for _, name := range names {
		f := scan.folders[name]
		members := groups[name]
		status, count := DeriveStatus(members)
		summary := StackSummary{
			Name:         name,
			HostID:       h.ID,
			Dir:          stackDir(h, name),
			ComposeFile:  f.composeFile,
			HasEnv:       f.hasEnv,
			Status:       status,
			Services:     count,
			Ports:        portLinks(members),
			ContainerIDs: shortIDs(members),
			Created:      oldestCreated(members),
		}
		if ref, ok := refs[name]; ok {
			summary.Operation = &ref
		}
		listing.Stacks = append(listing.Stacks, summary)
	}

	projects := make([]string, 0, len(groups))
	for project := range groups {
		if f, isStack := scan.folders[project]; !isStack || f.composeFile == "" {
			projects = append(projects, project)
		}
	}
	sort.Strings(projects)
	for _, project := range projects {
		members := groups[project]
		status, count := DeriveStatus(members)
		listing.External = append(listing.External, ProjectSummary{
			Name:         project,
			WorkingDir:   workingDirOf(members),
			Status:       status,
			Services:     count,
			Ports:        portLinks(members),
			ContainerIDs: shortIDs(members),
		})
	}
	return listing
}

// workingDirOf is the compose working directory label of the first container that has one.
func workingDirOf(members []dockerhost.ContainerSummary) string {
	i := slices.IndexFunc(members, func(c dockerhost.ContainerSummary) bool { return c.ComposeWorkingDir != "" })
	if i < 0 {
		return ""
	}
	return members[i].ComposeWorkingDir
}

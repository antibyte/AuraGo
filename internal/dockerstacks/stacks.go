package dockerstacks

import (
	"context"
	"errors"
	"path"
	"sort"

	"gopkg.in/yaml.v3"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

// ioGone turns "the IO container does not exist" into the error a read on an
// unprepared host gets, with the current helper status.
func (s *Service) ioGone(ctx context.Context, e engine, h dockerhost.Host) error {
	s.mu.Lock()
	delete(s.ready, h.ID)
	s.mu.Unlock()
	st, _, err := s.helpers(ctx, e, h)
	if err != nil {
		return err
	}
	return notPreparedError(st)
}

// stackMissing is the error for a stack without a compose file. A HEAD request
// cannot tell a missing file from a missing IO container (both are a bodiless
// 404), so the helpers are checked again before the stack is declared gone.
func (s *Service) stackMissing(ctx context.Context, e engine, h dockerhost.Host, name string) error {
	st, _, err := s.helpers(ctx, e, h)
	if err != nil {
		return err
	}
	if !st.Prepared {
		s.mu.Lock()
		delete(s.ready, h.ID)
		s.mu.Unlock()
		return notPreparedError(st)
	}
	return newError(CodeStackNotFound, "there is no stack %q on this host", name)
}

// locateCompose finds the compose file of a stack: the first of
// ComposeFileNames that exists in its folder. "" means the folder has none. The
// name the last folder scan found is tried first.
func (s *Service) locateCompose(ctx context.Context, e engine, h dockerhost.Host, name string) (string, error) {
	dir := stackDir(h, name)
	var hint string
	if c, ok := s.cachedScan(h.ID); ok {
		hint = c.folders[name].composeFile
	}
	if hint != "" {
		found, err := e.pathExists(ctx, path.Join(dir, hint))
		if err != nil {
			return "", err
		}
		if found {
			return hint, nil
		}
	}
	for _, candidate := range ComposeFileNames {
		if candidate == hint {
			continue
		}
		found, err := e.pathExists(ctx, path.Join(dir, candidate))
		if err != nil {
			return "", err
		}
		if found {
			return candidate, nil
		}
	}
	return "", nil
}

// readStackFile reads a file of a stack as FileContent.
func (s *Service) readStackFile(ctx context.Context, e engine, h dockerhost.Host, name, file string) (FileContent, error) {
	data, exists, err := e.readFile(ctx, path.Join(stackDir(h, name), file))
	if errors.Is(err, errIOContainerGone) {
		return FileContent{}, s.ioGone(ctx, e, h)
	}
	if err != nil {
		return FileContent{}, err
	}
	if !exists {
		return FileContent{Name: file}, nil
	}
	return FileContent{Name: file, Content: string(data), Revision: Revision(data), Exists: true}, nil
}

// composeService is one service of a compose file.
type composeService struct {
	name  string
	image string
}

// parseComposeServices lists the services of a compose file in file order,
// with the image each one names. A file without a services mapping has none; a
// file that is not valid YAML, or whose top level is not a mapping, is an error.
func parseComposeServices(content []byte) ([]composeService, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("the compose file is not a YAML mapping")
	}
	var services []composeService
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "services" || root.Content[i+1].Kind != yaml.MappingNode {
			continue
		}
		list := root.Content[i+1].Content
		for j := 0; j+1 < len(list); j += 2 {
			svc := composeService{name: list[j].Value}
			if body := list[j+1]; body.Kind == yaml.MappingNode {
				for k := 0; k+1 < len(body.Content); k += 2 {
					if body.Content[k].Value == "image" && body.Content[k+1].Kind == yaml.ScalarNode {
						svc.image = body.Content[k+1].Value
					}
				}
			}
			services = append(services, svc)
		}
	}
	return services, nil
}

// serviceDetail describes one service from its containers: the running one
// when there is one, else the first.
func serviceDetail(name, fileImage string, containers []dockerhost.ContainerSummary) ServiceDetail {
	d := ServiceDetail{Name: name, Image: fileImage, State: "missing", Ports: []PortLink{}}
	if len(containers) == 0 {
		return d
	}
	best := containers[0]
	for _, c := range containers {
		if c.State == "running" {
			best = c
			break
		}
	}
	d.State, d.Health, d.ContainerID = best.State, best.Health, best.ID
	if d.Image == "" {
		d.Image = best.Image
	}
	if code, ok := exitCodeOf(best); ok && best.State == "exited" {
		d.ExitCode = &code
	}
	d.Ports = portLinks(containers)
	return d
}

// serviceList builds the services of a stack: the compose file's services in
// file order first, then the containers of the project that match none of them.
func serviceList(services []composeService, members []dockerhost.ContainerSummary) []ServiceDetail {
	byService := map[string][]dockerhost.ContainerSummary{}
	for _, c := range members {
		byService[c.ComposeService] = append(byService[c.ComposeService], c)
	}
	list := make([]ServiceDetail, 0, len(services)+len(byService))
	known := map[string]bool{}
	for _, svc := range services {
		known[svc.name] = true
		list = append(list, serviceDetail(svc.name, svc.image, byService[svc.name]))
	}
	var stray []string
	for name := range byService {
		if !known[name] {
			stray = append(stray, name)
		}
	}
	sort.Strings(stray)
	for _, name := range stray {
		label := name
		if label == "" {
			label = byService[name][0].ID
			if names := byService[name][0].Names; len(names) > 0 {
				label = names[0]
			}
		}
		list = append(list, serviceDetail(label, "", byService[name]))
	}
	return list
}

// Get returns a stack with its compose file, its .env, its services and its
// status. The files are read through the IO container, the status from the
// containers labelled with the project name.
func (s *Service) Get(ctx context.Context, hostID, name string) (StackDetail, error) {
	if err := ValidateName(name); err != nil {
		return StackDetail{}, err
	}
	h, e, err := s.target(ctx, hostID)
	if err != nil {
		return StackDetail{}, err
	}
	if _, err := s.requirePrepared(ctx, e, h); err != nil {
		return StackDetail{}, err
	}
	composeName, err := s.locateCompose(ctx, e, h, name)
	if errors.Is(err, errIOContainerGone) {
		return StackDetail{}, s.ioGone(ctx, e, h)
	}
	if err != nil {
		return StackDetail{}, err
	}
	if composeName == "" {
		return StackDetail{}, s.stackMissing(ctx, e, h, name)
	}
	compose, err := s.readStackFile(ctx, e, h, name, composeName)
	if err != nil {
		return StackDetail{}, err
	}
	env, err := s.readStackFile(ctx, e, h, name, EnvFileName)
	if err != nil {
		return StackDetail{}, err
	}
	containers, err := dockerhost.ListContainers(ctx, e.c)
	if err != nil {
		return StackDetail{}, err
	}
	members := projectGroups(containers)[name]

	status, count := DeriveStatus(members)
	detail := StackDetail{
		StackSummary: StackSummary{
			Name: name, HostID: h.ID, Dir: stackDir(h, name), ComposeFile: composeName, HasEnv: env.Exists,
			Status: status, Services: count, Ports: portLinks(members), ContainerIDs: shortIDs(members),
			Created: oldestCreated(members),
		},
		Compose: compose,
		Env:     env,
	}
	if ref, ok := s.operationRefs(h.ID)[name]; ok {
		detail.Operation = &ref
	}
	services, parseErr := parseComposeServices([]byte(compose.Content))
	if parseErr != nil {
		detail.ParseError = dockerutil.SanitizeOneLine(parseErr.Error(), 300)
		services = nil
	}
	detail.ServiceList = serviceList(services, members)
	return detail, nil
}

// checkSize refuses a file above MaxFileBytes.
func checkSize(file, content string) error {
	if len(content) > MaxFileBytes {
		return tooLarge(file)
	}
	return nil
}

// Create writes a new stack: its folder, compose.yaml and, when in.Env is set,
// .env. A folder that already has a compose file is CodeStackExists; a folder
// without one (leftover data) is used as it is, except that an .env in it is
// never replaced (CodeStackExists naming it when in.Env is set; kept when not).
// The stack's status is draft until it is deployed.
func (s *Service) Create(ctx context.Context, hostID string, in CreateInput) (StackDetail, error) {
	if err := ValidateName(in.Name); err != nil {
		return StackDetail{}, err
	}
	if err := checkSize("the compose file", in.Compose); err != nil {
		return StackDetail{}, err
	}
	if in.Env != nil {
		if err := checkSize("the .env file", *in.Env); err != nil {
			return StackDetail{}, err
		}
	}
	h, e, err := s.target(ctx, hostID)
	if err != nil {
		return StackDetail{}, err
	}
	if _, err := s.requirePrepared(ctx, e, h); err != nil {
		return StackDetail{}, err
	}
	if err := s.refuseWhileDeleting(h.ID, in.Name); err != nil {
		return StackDetail{}, err
	}
	lock := s.fileLock(h.ID, in.Name)
	lock.Lock()
	defer lock.Unlock()

	dir := stackDir(h, in.Name)
	dirExists, err := e.pathExists(ctx, dir)
	if err != nil {
		return StackDetail{}, err
	}
	if dirExists {
		existing, err := s.locateCompose(ctx, e, h, in.Name)
		if errors.Is(err, errIOContainerGone) {
			return StackDetail{}, s.ioGone(ctx, e, h)
		}
		if err != nil {
			return StackDetail{}, err
		}
		if existing != "" {
			exists := newError(CodeStackExists, "the stack %q already exists on this host", in.Name)
			exists.Err = ErrStackExists
			return StackDetail{}, exists
		}
		if in.Env != nil {
			// Leftover data may include a .env; it is never replaced behind the user's back.
			envExists, err := e.pathExists(ctx, path.Join(dir, EnvFileName))
			if err != nil {
				return StackDetail{}, err
			}
			if envExists {
				return StackDetail{}, newError(CodeStackExists,
					"the folder of stack %q already holds a %s that would be overwritten; remove it or create the stack without one", in.Name, EnvFileName)
			}
		}
	}

	files := []tarEntry{{name: ComposeFileNames[0], mode: composeFileMode, content: []byte(in.Compose)}}
	if in.Env != nil {
		files = append(files, tarEntry{name: EnvFileName, mode: envFileMode, content: []byte(*in.Env)})
	}
	extractIn := dir
	if !dirExists {
		// A new folder travels in the archive itself; an existing one keeps its owner and mode.
		extractIn = h.StacksDir
		for i := range files {
			files[i].name = in.Name + "/" + files[i].name
		}
		files = append([]tarEntry{{name: in.Name, dir: true, mode: stackDirMode}}, files...)
	}
	archive, err := buildTar(files, s.opts.Now())
	if err != nil {
		return StackDetail{}, err
	}
	err = e.putArchive(ctx, extractIn, archive)
	s.invalidateScan(h.ID)
	switch {
	case errors.Is(err, errIOContainerGone):
		return StackDetail{}, s.ioGone(ctx, e, h)
	case errors.Is(err, errArchivePathMissing):
		return StackDetail{}, newError(CodeStacksDirInvalid, "the stacks directory %s does not exist on the Docker host", h.StacksDir)
	case err != nil:
		return StackDetail{}, err
	}
	return s.Get(ctx, hostID, in.Name)
}

// WriteFiles stores the compose file and/or the .env of an existing stack.
// Every file carries the revision it was edited from; if the file on the host
// has another revision (it changed meanwhile), nothing is written and the
// answer is a *ConflictError with what is on the host now. All files are checked
// before any is written. Writes are refused (BusyError) while a delete runs.
func (s *Service) WriteFiles(ctx context.Context, hostID, name string, in FilesInput) (FileRevisions, error) {
	if err := ValidateName(name); err != nil {
		return FileRevisions{}, err
	}
	if in.Compose == nil && in.Env == nil {
		return FileRevisions{}, newError(CodeInvalidOperation, "the request names no file to write")
	}
	if in.Compose != nil {
		if err := checkSize("the compose file", in.Compose.Content); err != nil {
			return FileRevisions{}, err
		}
	}
	if in.Env != nil {
		if err := checkSize("the .env file", in.Env.Content); err != nil {
			return FileRevisions{}, err
		}
	}
	h, e, err := s.target(ctx, hostID)
	if err != nil {
		return FileRevisions{}, err
	}
	if _, err := s.requirePrepared(ctx, e, h); err != nil {
		return FileRevisions{}, err
	}
	if err := s.refuseWhileDeleting(h.ID, name); err != nil {
		return FileRevisions{}, err
	}
	lock := s.fileLock(h.ID, name)
	lock.Lock()
	defer lock.Unlock()

	composeName, err := s.locateCompose(ctx, e, h, name)
	if errors.Is(err, errIOContainerGone) {
		return FileRevisions{}, s.ioGone(ctx, e, h)
	}
	if err != nil {
		return FileRevisions{}, err
	}
	if composeName == "" {
		return FileRevisions{}, s.stackMissing(ctx, e, h, name)
	}

	type pending struct {
		label string
		file  string
		mode  int64
		write *FileWrite
	}
	var writes []pending
	if in.Compose != nil {
		writes = append(writes, pending{"compose", composeName, composeFileMode, in.Compose})
	}
	if in.Env != nil {
		writes = append(writes, pending{"env", EnvFileName, envFileMode, in.Env})
	}
	var entries []tarEntry
	for _, w := range writes {
		current, err := s.readStackFile(ctx, e, h, name, w.file)
		if err != nil {
			return FileRevisions{}, err
		}
		if current.Revision != w.write.Revision {
			return FileRevisions{}, &ConflictError{File: w.label, Current: current}
		}
		entries = append(entries, tarEntry{name: w.file, mode: w.mode, content: []byte(w.write.Content)})
	}
	archive, err := buildTar(entries, s.opts.Now())
	if err != nil {
		return FileRevisions{}, err
	}
	err = e.putArchive(ctx, stackDir(h, name), archive)
	s.invalidateScan(h.ID)
	switch {
	case errors.Is(err, errIOContainerGone):
		return FileRevisions{}, s.ioGone(ctx, e, h)
	case errors.Is(err, errArchivePathMissing):
		return FileRevisions{}, newError(CodeStackNotFound, "the folder of stack %q is gone", name)
	case err != nil:
		return FileRevisions{}, err
	}
	var revisions FileRevisions
	if in.Compose != nil {
		revisions.Compose = Revision([]byte(in.Compose.Content))
	}
	if in.Env != nil {
		revisions.Env = Revision([]byte(in.Env.Content))
	}
	return revisions, nil
}

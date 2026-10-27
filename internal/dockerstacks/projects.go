package dockerstacks

import (
	"context"
	"slices"
	"sort"
	"strings"
	"time"

	"aurago/internal/dockerhost"
)

// ProjectAction starts, stops or restarts every container of a compose project
// through the Engine's container API. It is meant for the external projects (the
// ones that are not stacks of the Docker Manager); a stack with an operation
// running is refused with a *BusyError. All containers are tried even when one
// fails; the first failure is returned.
func (s *Service) ProjectAction(ctx context.Context, hostID, project, action string) error {
	switch action {
	case "start", "stop", "restart":
	default:
		return newError(CodeInvalidOperation, "%q is not a project action (start, stop or restart)", action)
	}
	if project == "" {
		return newError(CodeStackNotFound, "no compose project was named")
	}
	h, e, err := s.target(ctx, hostID)
	if err != nil {
		return err
	}
	if run := s.activeRun(h.ID, project); run != nil {
		op := run.snapshot()
		return &BusyError{OperationID: op.ID, Kind: op.Kind}
	}
	containers, err := dockerhost.ListContainers(ctx, e.c)
	if err != nil {
		return err
	}
	members := projectGroups(containers)[project]
	if len(members) == 0 {
		return newError(CodeStackNotFound, "there is no compose project %q on this host", project)
	}
	sort.SliceStable(members, func(i, j int) bool { return serviceLabel(members[i]) < serviceLabel(members[j]) })
	var first error
	for _, c := range members {
		if err := dockerhost.ContainerAction(ctx, e.c, c.ID, action); err != nil && first == nil {
			first = err
		}
	}
	s.invalidateScan(h.ID)
	return first
}

// volumeSizeTimeout bounds the size query, which makes the Engine measure every volume.
const volumeSizeTimeout = 25 * time.Second

// ProjectVolumes lists the volumes that carry the compose project label
// com.docker.compose.project=<name> (what `down --volumes` would remove), by
// name. Sizes come from /system/df?type=volume on an Engine with API 1.42 or
// newer; where the Engine does not report one, or the measurement fails or takes
// too long, Size is -1.
func (s *Service) ProjectVolumes(ctx context.Context, hostID, name string) ([]ProjectVolume, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	_, e, err := s.target(ctx, hostID)
	if err != nil {
		return nil, err
	}
	volumes, err := e.projectVolumes(ctx, name)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(volumes, func(a, b ProjectVolume) int { return strings.Compare(a.Name, b.Name) })
	if len(volumes) == 0 {
		return volumes, nil
	}
	v, err := e.version(ctx)
	if err != nil {
		return volumes, nil
	}
	if minor, ok := apiMinor(v.APIVersion); !ok || minor < 42 {
		return volumes, nil
	}
	sizeCtx, cancel := context.WithTimeout(ctx, volumeSizeTimeout)
	defer cancel()
	if sizes, err := e.volumeSizes(sizeCtx); err == nil {
		for i := range volumes {
			if size, ok := sizes[volumes[i].Name]; ok {
				volumes[i].Size = size
			}
		}
	}
	return volumes, nil
}

// PublishedPorts lists the host ports that running containers of the host
// publish, for the port conflict check of the editor. The ports of
// excludeProject (the stack being edited, "" for none) are left out, and so are
// the Docker Manager's own helper containers. A port bound to all interfaces
// has HostIP "" (0.0.0.0 and :: are one binding here).
func (s *Service) PublishedPorts(ctx context.Context, hostID, excludeProject string) ([]PublishedPort, error) {
	_, e, err := s.target(ctx, hostID)
	if err != nil {
		return nil, err
	}
	containers, err := dockerhost.ListContainers(ctx, e.c)
	if err != nil {
		return nil, err
	}
	ports := []PublishedPort{}
	seen := map[PublishedPort]bool{}
	for _, c := range containers {
		if c.State != "running" || isHelperContainer(c) || (excludeProject != "" && c.ComposeProject == excludeProject) {
			continue
		}
		name := c.ID
		if len(c.Names) > 0 {
			name = c.Names[0]
		}
		for _, p := range c.Ports {
			if p.PublicPort <= 0 {
				continue
			}
			port := PublishedPort{HostIP: normalizeBindIP(p.IP), Port: p.PublicPort, Protocol: p.Type, Container: name, Project: c.ComposeProject}
			if !seen[port] {
				seen[port] = true
				ports = append(ports, port)
			}
		}
	}
	slices.SortFunc(ports, func(a, b PublishedPort) int {
		switch {
		case a.Port != b.Port:
			return a.Port - b.Port
		case a.Protocol != b.Protocol:
			return strings.Compare(a.Protocol, b.Protocol)
		case a.Container != b.Container:
			return strings.Compare(a.Container, b.Container)
		}
		return strings.Compare(a.HostIP, b.HostIP)
	})
	return ports, nil
}

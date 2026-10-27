package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

const (
	dmListTimeout   = 20 * time.Second
	dmReadTimeout   = 30 * time.Second
	dmActionTimeout = 60 * time.Second
)

var dmContainerActions = map[string]bool{
	"start": true, "stop": true, "restart": true, "pause": true, "unpause": true, "kill": true,
}

// dockerContainerView is the ContainerView of the API contract: the container
// plus the protection facts the UI needs before it offers a destructive action.
type dockerContainerView struct {
	dockerhost.ContainerSummary
	ProtectedOwner string `json:"protected_owner,omitempty"`
	Self           bool   `json:"self,omitempty"`
	DockerEndpoint bool   `json:"docker_endpoint,omitempty"`
	SharedNetwork  bool   `json:"shared_network,omitempty"`
}

// localProtectionFlags reuses the Containers page's own computation
// (adminContainerListJSON: self, shared network, Docker endpoint and owner) for
// the local engine, keyed by the 12-character container ID, so the two views
// can never disagree. A failed lookup returns nil; the owner then still comes
// from the container's names and labels.
func (dm *dmRequest) localProtectionFlags(ctx context.Context) map[string]adminContainerEntry {
	cfg, _, _ := containerDockerConfig(dm.s)
	var parsed struct {
		Status     string                `json:"status"`
		Containers []adminContainerEntry `json:"containers"`
	}
	if json.Unmarshal([]byte(adminContainerListJSON(ctx, dm.s, cfg)), &parsed) != nil || parsed.Status != "ok" {
		return nil
	}
	flags := make(map[string]adminContainerEntry, len(parsed.Containers))
	for _, entry := range parsed.Containers {
		flags[entry.ID] = entry
	}
	return flags
}

func (dm *dmRequest) containerViews(ctx context.Context, host dockerhost.Host, list []dockerhost.ContainerSummary) []dockerContainerView {
	var local map[string]adminContainerEntry
	if host.ID == dockerhost.LocalHostID {
		local = dm.localProtectionFlags(ctx)
	}
	views := make([]dockerContainerView, 0, len(list))
	for _, container := range list {
		view := dockerContainerView{
			ContainerSummary: container,
			ProtectedOwner:   firstContainerOwner(container.Names, container.Labels),
		}
		if flags, ok := local[container.ID]; ok {
			view.Self, view.DockerEndpoint, view.SharedNetwork = flags.Self, flags.DockerEndpoint, flags.SharedNetwork
			if flags.ProtectedOwner != "" {
				view.ProtectedOwner = flags.ProtectedOwner
			}
		}
		views = append(views, view)
	}
	return views
}

// containerList serves GET /hosts/{id}/containers.
func (dm *dmRequest) containerList(hostID string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	ctx, cancel := dm.context(dmListTimeout)
	defer cancel()
	host, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	list, err := dockerhost.ListContainers(ctx, client)
	if err != nil {
		dm.engineFailure(err, "")
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"containers": dm.containerViews(ctx, host, list)})
}

// containerSub serves /hosts/{id}/containers/{ref}/{inspect|logs|action}.
func (dm *dmRequest) containerSub(hostID, ref, sub string) {
	switch {
	case sub == "inspect":
		dm.containerInspect(hostID, ref)
	case sub == "logs":
		dm.containerLogs(hostID, ref)
	case dmContainerActions[sub]:
		dm.containerAction(hostID, ref, sub)
	default:
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeInvalidAction, "unknown container action: "+dockerutil.SanitizeOneLine(sub, 32), nil)
	}
}

// containerAction runs start, stop, restart, pause, unpause or kill. These
// never ask for a protection confirmation, like /api/containers: stopping a
// protected container is confirmed in the UI only.
func (dm *dmRequest) containerAction(hostID, ref, action string) {
	if !dm.allow(http.MethodPost) {
		return
	}
	if dm.readOnly {
		dm.readOnlyRefusal()
		return
	}
	ctx, cancel := dm.context(dmActionTimeout)
	defer cancel()
	_, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	if err := dockerhost.ContainerAction(ctx, client, ref, action); err != nil {
		dm.engineFailure(err, dmCodeContainerNotFound)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, nil)
}

// containerInspect serves GET .../inspect (secret environment values are
// redacted by the engine layer).
func (dm *dmRequest) containerInspect(hostID, ref string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	ctx, cancel := dm.context(dmReadTimeout)
	defer cancel()
	_, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	raw, err := dockerhost.InspectContainer(ctx, client, ref)
	if err != nil {
		dm.engineFailure(err, dmCodeContainerNotFound)
		return
	}
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"inspect": raw})
}

// containerLogs serves GET .../logs?tail=N; the engine layer clamps N.
func (dm *dmRequest) containerLogs(hostID, ref string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	ctx, cancel := dm.context(dmReadTimeout)
	defer cancel()
	_, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	tail, _ := strconv.Atoi(dm.r.URL.Query().Get("tail"))
	logs, err := dockerhost.ContainerLogs(ctx, client, ref, tail)
	if err != nil {
		dm.engineFailure(err, dmCodeContainerNotFound)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"logs": logs})
}

// containerRemove serves DELETE .../containers/{ref}?force=1&confirm=protected.
func (dm *dmRequest) containerRemove(hostID, ref string) {
	if !dm.allow(http.MethodDelete) {
		return
	}
	if dm.readOnly {
		dm.readOnlyRefusal()
		return
	}
	ctx, cancel := dm.context(dmActionTimeout)
	defer cancel()
	host, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	if !dm.removeAllowed(ctx, host, client, ref) {
		return
	}
	force := dmTruthy(dm.r.URL.Query().Get("force"))
	if err := dockerhost.RemoveContainer(ctx, client, ref, force); err != nil {
		dm.engineFailure(err, dmCodeContainerNotFound)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, nil)
}

func dmTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// removeAllowed applies the confirmation rule of DELETE /api/containers/{id}.
// The local host, and a remote host that is the Engine AuraGo itself uses (its
// address and port equal docker.host, e.g. a tcp socket proxy), run the full
// classification of containerActionAllowed (self, Docker endpoint, shared
// network, owner). Any other remote host has its own Engine and its own
// containers: the owner from names and labels, plus the self check on the
// container's full ID (an Engine reached by another route, such as an SSH or
// tcp host on AuraGo's own machine, can still hold the container AuraGo runs
// in). It writes the 409 and returns false when the request stops.
func (dm *dmRequest) removeAllowed(ctx context.Context, host dockerhost.Host, client *dockerutil.Client, ref string) bool {
	cfg, _, _ := containerDockerConfig(dm.s)
	if host.ID == dockerhost.LocalHostID || host.ReachesEngineAt(cfg.Host) {
		return containerActionAllowed(dm.s, cfg, ref, "remove", dm.w, dm.r)
	}
	p, err := dm.remoteContainerProtection(ctx, client, ref)
	if err != nil {
		dm.engineFailure(err, "")
		return false
	}
	if p.protected() && dm.r.URL.Query().Get(containerConfirmParam) != containerConfirmProtected {
		dmWriteError(dm.w, http.StatusConflict, containerCodeConfirmationRequired, p.confirmationMessage(), dmFields{"owner": p.label()})
		return false
	}
	return true
}

// remoteContainerProtection finds the container by name, full ID or ID prefix
// and returns the AuraGo owner its names and labels name, and whether AuraGo's
// own self signals name its full ID. An unknown reference keeps the
// reserved-name check, like classifyContainerForAction; the remove then reports
// the 404.
func (dm *dmRequest) remoteContainerProtection(ctx context.Context, client *dockerutil.Client, ref string) (containerProtection, error) {
	list, err := dockerhost.ListContainers(ctx, client)
	if err != nil {
		return containerProtection{}, err
	}
	needle := strings.ToLower(ref)
	for _, container := range list {
		if dmContainerMatches(container, needle) {
			return containerProtection{
				Owner: firstContainerOwner(container.Names, container.Labels),
				Self:  containerIsSelf(containerRuntimeIsDocker(dm.s), container.FullID),
			}, nil
		}
	}
	return containerProtection{Owner: firstContainerOwner([]string{ref}, nil)}, nil
}

func dmContainerMatches(container dockerhost.ContainerSummary, needle string) bool {
	for _, name := range container.Names {
		if strings.ToLower(strings.TrimPrefix(name, "/")) == needle {
			return true
		}
	}
	return len(needle) >= 4 && strings.HasPrefix(strings.ToLower(container.FullID), needle)
}

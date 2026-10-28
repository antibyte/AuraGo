package server

import (
	"net/http"

	"aurago/internal/dockerstacks"
	"aurago/internal/dockerutil"
)

// ── Docker Manager stacks API: helpers, stacks, files, operations, projects ──
// Everything below /api/docker-manager/hosts/{id}/ that belongs to the stacks
// service. docker.readonly refuses prepare, create, file writes, operations,
// delete and project actions with 403 read_only; the reads, lint and convert
// change nothing and stay available.

// dmProjectActions are the actions of POST /hosts/{id}/projects/{project}/{action}.
var dmProjectActions = map[string]bool{"start": true, "stop": true, "restart": true}

// stackRoute serves the stack routes and hands the others of the stacks API to
// operationRoute and lintRoute. It returns false when the path matches none of
// them.
func (dm *dmRequest) stackRoute(segs []string) bool {
	switch {
	case len(segs) == 3 && segs[0] == "hosts" && segs[2] == "helpers":
		dm.helperStatus(segs[1])
	case len(segs) == 3 && segs[0] == "hosts" && segs[2] == "prepare":
		dm.prepareHost(segs[1])
	case len(segs) == 3 && segs[0] == "hosts" && segs[2] == "stacks":
		dm.stackCollection(segs[1])
	case len(segs) == 4 && segs[0] == "hosts" && segs[2] == "stacks":
		dm.stackItem(segs[1], segs[3])
	case len(segs) == 5 && segs[0] == "hosts" && segs[2] == "stacks":
		dm.stackSub(segs[1], segs[3], segs[4])
	case len(segs) == 5 && segs[0] == "hosts" && segs[2] == "projects":
		dm.projectSub(segs[1], segs[3], segs[4])
	default:
		return dm.operationRoute(segs) || dm.lintRoute(segs)
	}
	return true
}

// helperStatus serves GET /hosts/{id}/helpers: what the host has of the runner
// image and the IO container. It only reads.
func (dm *dmRequest) helperStatus(hostID string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	svc, ok := dm.stackService()
	if !ok {
		return
	}
	ctx, cancel := dm.context(dmStackReadTimeout)
	defer cancel()
	helpers, err := svc.Helpers(ctx, hostID)
	if err != nil {
		dm.stackFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"helpers": helpers})
}

// prepareHost serves POST /hosts/{id}/prepare: it starts the prepare operation
// (runner image and IO container) and answers 202 with it.
func (dm *dmRequest) prepareHost(hostID string) {
	if !dm.allow(http.MethodPost) {
		return
	}
	if dm.readOnly {
		dm.readOnlyRefusal()
		return
	}
	svc, ok := dm.stackService()
	if !ok {
		return
	}
	ctx, cancel := dm.context(dmStackStartTimeout)
	defer cancel()
	op, err := svc.Prepare(ctx, hostID)
	if err != nil {
		dm.operationFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusAccepted, dmFields{"operation": op})
}

// stackCollection serves GET and POST /hosts/{id}/stacks.
func (dm *dmRequest) stackCollection(hostID string) {
	switch dm.r.Method {
	case http.MethodGet:
		dm.stackList(hostID)
	case http.MethodPost:
		dm.stackCreate(hostID)
	default:
		dm.allow(http.MethodGet, http.MethodPost)
	}
}

// stackList serves GET /hosts/{id}/stacks[?refresh=1]. The listing's members sit
// at the top level of the answer.
func (dm *dmRequest) stackList(hostID string) {
	svc, ok := dm.stackService()
	if !ok {
		return
	}
	ctx, cancel := dm.context(dmStackReadTimeout)
	defer cancel()
	listing, err := svc.List(ctx, hostID, dmTruthy(dm.r.URL.Query().Get("refresh")))
	if err != nil {
		dm.stackFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{
		"host_id":    listing.HostID,
		"stacks_dir": listing.StacksDir,
		"stacks":     dmSlice(listing.Stacks),
		"external":   dmSlice(listing.External),
		"ignored":    dmSlice(listing.Ignored),
		"scanned_at": listing.ScannedAt,
		"cached":     listing.Cached,
	})
}

// dmStackCreate is the body of POST /hosts/{id}/stacks.
type dmStackCreate struct {
	dockerstacks.CreateInput
	Deploy bool `json:"deploy"`
}

// stackCreate serves POST /hosts/{id}/stacks: it writes the stack's files and,
// with deploy, starts the up operation. When the files were written but the
// operation could not be started, the answer is still 201 with the stack and a
// deploy_error member, so the client does not retry a create that succeeded.
func (dm *dmRequest) stackCreate(hostID string) {
	if dm.readOnly {
		dm.readOnlyRefusal()
		return
	}
	svc, ok := dm.stackService()
	if !ok {
		return
	}
	var in dmStackCreate
	if !dm.decodeSized(&in, dmStackBodyLimit) {
		return
	}
	ctx, cancel := dm.context(dmStackCreateTimeout)
	defer cancel()
	detail, err := svc.Create(ctx, hostID, in.CreateInput)
	if err != nil {
		dm.stackFailure(err)
		return
	}
	fields := dmFields{"stack": detail}
	if in.Deploy {
		op, err := svc.Start(ctx, hostID, in.Name, dockerstacks.KindUp, dockerstacks.OpOptions{})
		if err != nil {
			failure, coded := stackFailureOf(err)
			if !coded {
				dm.s.logDockerManager("Docker stack deploy could not be started", err)
				failure = dmFailure{code: dmCodeInternal, message: "The deployment could not be started"}
			}
			fields["deploy_error"] = dmFields{"code": failure.code, "message": failure.message}
		} else {
			fields["operation"] = op
		}
	}
	dmWriteOK(dm.w, http.StatusCreated, fields)
}

// stackItem serves GET and DELETE /hosts/{id}/stacks/{name}.
func (dm *dmRequest) stackItem(hostID, name string) {
	switch dm.r.Method {
	case http.MethodGet:
		dm.stackGet(hostID, name)
	case http.MethodDelete:
		dm.stackDelete(hostID, name)
	default:
		dm.allow(http.MethodGet, http.MethodDelete)
	}
}

func (dm *dmRequest) stackGet(hostID, name string) {
	svc, ok := dm.stackService()
	if !ok {
		return
	}
	ctx, cancel := dm.context(dmStackReadTimeout)
	defer cancel()
	detail, err := svc.Get(ctx, hostID, name)
	if err != nil {
		dm.stackFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"stack": detail})
}

// stackDelete serves DELETE /hosts/{id}/stacks/{name}[?volumes=1]: it starts the
// delete operation (down, then the stack folder is removed) and answers 202.
func (dm *dmRequest) stackDelete(hostID, name string) {
	if dm.readOnly {
		dm.readOnlyRefusal()
		return
	}
	svc, ok := dm.stackService()
	if !ok {
		return
	}
	ctx, cancel := dm.context(dmStackStartTimeout)
	defer cancel()
	opts := dockerstacks.OpOptions{Volumes: dmTruthy(dm.r.URL.Query().Get("volumes"))}
	op, err := svc.Start(ctx, hostID, name, dockerstacks.KindDelete, opts)
	if err != nil {
		dm.operationFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusAccepted, dmFields{"operation": op})
}

// stackSub serves /hosts/{id}/stacks/{name}/{files|volumes|ops|logs}.
func (dm *dmRequest) stackSub(hostID, name, sub string) {
	switch sub {
	case "logs":
		dm.stackLogs(hostID, name)
	case "files":
		dm.stackFiles(hostID, name)
	case "volumes":
		dm.stackVolumes(hostID, name)
	case "ops":
		dm.stackOperation(hostID, name)
	default:
		dm.notFound()
	}
}

// stackFiles serves PUT .../files: the compose file and/or the .env, each with
// the revision it was edited from. A file that changed on the host meanwhile is
// a 409 revision_conflict that carries what is on the host now.
func (dm *dmRequest) stackFiles(hostID, name string) {
	if !dm.allow(http.MethodPut) {
		return
	}
	if dm.readOnly {
		dm.readOnlyRefusal()
		return
	}
	svc, ok := dm.stackService()
	if !ok {
		return
	}
	var in dockerstacks.FilesInput
	if !dm.decodeSized(&in, dmStackBodyLimit) {
		return
	}
	ctx, cancel := dm.context(dmStackFileTimeout)
	defer cancel()
	revisions, err := svc.WriteFiles(ctx, hostID, name, in)
	if err != nil {
		dm.stackFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"revisions": revisions})
}

// stackVolumes serves GET .../volumes: the volumes `down --volumes` would remove.
func (dm *dmRequest) stackVolumes(hostID, name string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	svc, ok := dm.stackService()
	if !ok {
		return
	}
	ctx, cancel := dm.context(dmStackReadTimeout)
	defer cancel()
	volumes, err := svc.ProjectVolumes(ctx, hostID, name)
	if err != nil {
		dm.stackFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"volumes": dmSlice(volumes)})
}

// dmOperationRequest is the body of POST .../ops.
type dmOperationRequest struct {
	Kind    string `json:"kind"`
	Volumes bool   `json:"volumes"`
}

// stackOperation serves POST .../ops: it starts config, up, down, stop, restart,
// pull or update and answers 202 with the queued operation. Delete has its own
// route (DELETE on the stack), so that removing a stack is never one typo away.
func (dm *dmRequest) stackOperation(hostID, name string) {
	if !dm.allow(http.MethodPost) {
		return
	}
	if dm.readOnly {
		dm.readOnlyRefusal()
		return
	}
	svc, ok := dm.stackService()
	if !ok {
		return
	}
	var in dmOperationRequest
	if !dm.decode(&in) {
		return
	}
	if in.Kind == dockerstacks.KindDelete {
		dmWriteError(dm.w, http.StatusBadRequest, dockerstacks.CodeInvalidOperation, "delete is started with DELETE on the stack, not through ops", nil)
		return
	}
	ctx, cancel := dm.context(dmStackStartTimeout)
	defer cancel()
	op, err := svc.Start(ctx, hostID, name, in.Kind, dockerstacks.OpOptions{Volumes: in.Volumes})
	if err != nil {
		dm.operationFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusAccepted, dmFields{"operation": op})
}

// projectAction serves POST /hosts/{id}/projects/{project}/{start|stop|restart}:
// the action on every container of an external compose project, through the
// Engine's container API.
func (dm *dmRequest) projectAction(hostID, project, action string) {
	if !dm.allow(http.MethodPost) {
		return
	}
	if !dmProjectActions[action] {
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeInvalidAction, "unknown project action: "+dockerutil.SanitizeOneLine(action, 32), nil)
		return
	}
	if dm.readOnly {
		dm.readOnlyRefusal()
		return
	}
	svc, ok := dm.stackService()
	if !ok {
		return
	}
	ctx, cancel := dm.context(dmActionTimeout)
	defer cancel()
	if err := svc.ProjectAction(ctx, hostID, project, action); err != nil {
		dm.stackFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, nil)
}

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerstacks"
	"aurago/internal/dockerutil"
)

// ── Docker Manager stacks API: shared pieces ────────────────────────────────
// The stack, operation, lint and convert routes (docker_manager_stacks.go,
// docker_manager_operations.go, docker_manager_lint.go) call the stacks
// service through the narrow interface below and answer its failures through
// the mapping in this file.

// Timeouts of the stack routes. The routes that only start an operation answer
// as soon as it is queued; the others may run a lister or read files on the host.
const (
	dmStackReadTimeout   = 45 * time.Second // helpers, list, get, volumes, lint
	dmStackFileTimeout   = 30 * time.Second // file writes
	dmStackCreateTimeout = 45 * time.Second
	dmStackStartTimeout  = 15 * time.Second // prepare, ops, delete
	// dmStackBodyLimit bounds a request that carries compose and .env text: two
	// files of dockerstacks.MaxFileBytes, JSON-escaped.
	dmStackBodyLimit = 8 << 20
)

// dmCodeStacksUnavailable answers a stack route while the stacks service is not
// open (Server.DockerStacks is nil).
const dmCodeStacksUnavailable = "stacks_unavailable"

// dmStackService is what the stack handlers use of *dockerstacks.Service, so the
// handler tests can substitute a small fake.
type dmStackService interface {
	Helpers(ctx context.Context, hostID string) (dockerstacks.HelperStatus, error)
	Prepare(ctx context.Context, hostID string) (dockerstacks.Operation, error)
	List(ctx context.Context, hostID string, refresh bool) (dockerstacks.Listing, error)
	Get(ctx context.Context, hostID, name string) (dockerstacks.StackDetail, error)
	Create(ctx context.Context, hostID string, in dockerstacks.CreateInput) (dockerstacks.StackDetail, error)
	WriteFiles(ctx context.Context, hostID, name string, in dockerstacks.FilesInput) (dockerstacks.FileRevisions, error)
	Start(ctx context.Context, hostID, name, kind string, opts dockerstacks.OpOptions) (dockerstacks.Operation, error)
	Operation(ctx context.Context, id string) (dockerstacks.Operation, error)
	Operations(ctx context.Context, f dockerstacks.OperationFilter) ([]dockerstacks.Operation, error)
	Subscribe(id string) (dockerstacks.Operation, <-chan dockerstacks.Event, func(), error)
	ProjectAction(ctx context.Context, hostID, project, action string) error
	ProjectVolumes(ctx context.Context, hostID, name string) ([]dockerstacks.ProjectVolume, error)
	PublishedPorts(ctx context.Context, hostID, excludeProject string) ([]dockerstacks.PublishedPort, error)
}

var _ dmStackService = (*dockerstacks.Service)(nil)

// stackService returns the stacks service, or answers 503 stacks_unavailable.
func (dm *dmRequest) stackService() (dmStackService, bool) {
	if dm.stacks == nil {
		dmWriteError(dm.w, http.StatusServiceUnavailable, dmCodeStacksUnavailable, "The Docker stacks service is not available", nil)
		return nil, false
	}
	return dm.stacks, true
}

// dmStackStatus is the HTTP status of every code of internal/dockerstacks.
var dmStackStatus = map[string]int{
	dockerstacks.CodeStackNameInvalid:   http.StatusBadRequest,
	dockerstacks.CodeStackNotFound:      http.StatusNotFound,
	dockerstacks.CodeStackExists:        http.StatusConflict,
	dockerstacks.CodeRevisionConflict:   http.StatusConflict,
	dockerstacks.CodeOperationBusy:      http.StatusConflict,
	dockerstacks.CodeOperationNotFound:  http.StatusNotFound,
	dockerstacks.CodeInvalidOperation:   http.StatusBadRequest,
	dockerstacks.CodeFileTooLarge:       http.StatusRequestEntityTooLarge,
	dockerstacks.CodeHostNotPrepared:    http.StatusConflict,
	dockerstacks.CodeRunnerImageMissing: http.StatusConflict,
	dockerstacks.CodeAPITooOld:          http.StatusConflict,
	dockerstacks.CodeStacksDirInvalid:   http.StatusConflict,
	dockerstacks.CodeConvertFailed:      http.StatusBadRequest,
}

// dmFailure is an error answer that is ready to be written.
type dmFailure struct {
	status  int
	code    string
	message string
	extra   dmFields
}

// stackFailureOf classifies the failures that internal/dockerstacks produces
// itself: *ConflictError (409 revision_conflict with the file and what is on the
// host), *BusyError (409 operation_in_progress with the running operation) and
// *Error (the status table above; its Fields join the envelope, but never replace
// status, code or message). ok is false for anything else: Engine, transport and
// registry errors.
func stackFailureOf(err error) (dmFailure, bool) {
	var conflict *dockerstacks.ConflictError
	var busy *dockerstacks.BusyError
	var coded *dockerstacks.Error
	switch {
	case errors.As(err, &conflict):
		return dmFailure{http.StatusConflict, dockerstacks.CodeRevisionConflict, conflict.Error(),
			dmFields{"file": conflict.File, "current": conflict.Current}}, true
	case errors.As(err, &busy):
		return dmFailure{http.StatusConflict, dockerstacks.CodeOperationBusy, busy.Error(),
			dmFields{"operation_id": busy.OperationID, "kind": busy.Kind}}, true
	case errors.As(err, &coded):
		status, known := dmStackStatus[coded.Code]
		if !known {
			status = http.StatusInternalServerError
		}
		extra := dmFields{}
		for key, value := range coded.Fields {
			if key != "status" && key != "code" && key != "message" {
				extra[key] = value
			}
		}
		return dmFailure{status, coded.Code, dockerutil.SanitizeOneLine(coded.Error(), 512), extra}, true
	}
	return dmFailure{}, false
}

func (dm *dmRequest) writeFailure(f dmFailure) {
	dmWriteError(dm.w, f.status, f.code, f.message, f.extra)
}

// stackFailure answers an error of a call that touched the host's Engine: the
// coded failures above, an unknown host as 404 host_not_found, and anything else
// through engineFailure (docker_error, or host_unreachable with the reason).
func (dm *dmRequest) stackFailure(err error) {
	if f, ok := stackFailureOf(err); ok {
		dm.writeFailure(f)
		return
	}
	if errors.Is(err, dockerhost.ErrHostNotFound) {
		dm.hostFailure(err)
		return
	}
	dm.engineFailure(err, "")
}

// operationFailure answers an error of a call that never reaches an Engine:
// starting an operation (it only records one), reading the journal, the pure
// converter. An unknown host is 404; anything else is a logged 500 and must not
// be reported as an unreachable host.
func (dm *dmRequest) operationFailure(err error) {
	if f, ok := stackFailureOf(err); ok {
		dm.writeFailure(f)
		return
	}
	dm.hostFailure(err)
}

// decodeSized reads a JSON body of at most limit bytes. A body over the limit
// answers 413 file_too_large; anything else that is not JSON answers 400.
func (dm *dmRequest) decodeSized(dst any, limit int64) bool {
	body := http.MaxBytesReader(dm.w, dm.r.Body, limit)
	if err := json.NewDecoder(body).Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			dmWriteError(dm.w, http.StatusRequestEntityTooLarge, dockerstacks.CodeFileTooLarge, "the request body is too large", nil)
			return false
		}
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeBadRequest, "the request body is not valid JSON", nil)
		return false
	}
	return true
}

// dmSlice turns a nil slice into an empty one, so a list always encodes as [].
func dmSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

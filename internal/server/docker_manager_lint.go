package server

import (
	"errors"
	"net/http"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerstacks"
)

// ── Docker Manager lint and docker run converter ────────────────────────────
// Both are pure functions of the request (dockerstacks.Lint and
// dockerstacks.ConvertDockerRun): they change nothing on any host, so they stay
// available in docker.readonly mode.

// dmConvertMaxBytes bounds the command of a convert request. The converter
// takes no context, so its work is bounded by its input instead of a timeout.
const dmConvertMaxBytes = 64 << 10

// lintRoute serves the lint and convert routes. It returns false when the path
// is neither.
func (dm *dmRequest) lintRoute(segs []string) bool {
	switch {
	case len(segs) == 3 && segs[0] == "hosts" && segs[2] == "lint":
		dm.lintStack(segs[1])
	case len(segs) == 2 && segs[0] == "convert" && segs[1] == "docker-run":
		dm.convertDockerRun()
	default:
		return false
	}
	return true
}

// lintStack serves POST /hosts/{id}/lint: diagnostics and a preview for a
// compose file and its .env. The ports other containers publish on the host are
// read for the conflict check; when the host cannot be read (or the stacks
// service is not open) the lint runs without them and the answer says
// ports_unchecked. An unknown host id is a 404.
func (dm *dmRequest) lintStack(hostID string) {
	if !dm.allow(http.MethodPost) {
		return
	}
	var in dockerstacks.LintInput
	if !dm.decodeSized(&in, dmStackBodyLimit) {
		return
	}
	var published []dockerstacks.PublishedPort
	unchecked := true
	if dm.stacks != nil {
		ctx, cancel := dm.context(dmStackReadTimeout)
		defer cancel()
		ports, err := dm.stacks.PublishedPorts(ctx, hostID, in.Stack)
		switch {
		case err == nil:
			published, unchecked = ports, false
		case errors.Is(err, dockerhost.ErrHostNotFound):
			dm.hostFailure(err)
			return
		}
	}
	result := dockerstacks.Lint(in, published)
	fields := dmFields{"diagnostics": dmSlice(result.Diagnostics), "preview": result.Preview}
	if unchecked {
		fields["ports_unchecked"] = true
	}
	dmWriteOK(dm.w, http.StatusOK, fields)
}

// dmConvertRequest is the body of POST /convert/docker-run.
type dmConvertRequest struct {
	Command string `json:"command"`
}

// convertDockerRun serves POST /convert/docker-run: one `docker run` command in,
// compose YAML and the warnings for flags that could not be converted out.
func (dm *dmRequest) convertDockerRun() {
	if !dm.allow(http.MethodPost) {
		return
	}
	var in dmConvertRequest
	if !dm.decode(&in) {
		return
	}
	if len(in.Command) > dmConvertMaxBytes {
		dmWriteError(dm.w, http.StatusBadRequest, dockerstacks.CodeConvertFailed, "The command is longer than 64 KiB.", nil)
		return
	}
	result, err := dockerstacks.ConvertDockerRun(in.Command)
	if err != nil {
		dm.operationFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{
		"compose":  result.Compose,
		"service":  result.Service,
		"warnings": dmSlice(result.Warnings),
	})
}

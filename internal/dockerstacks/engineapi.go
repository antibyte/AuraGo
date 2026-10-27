package dockerstacks

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

// engine wraps the Engine calls the stacks service makes on one host. Plain
// calls keep the client's 30 s timeout; streaming calls (logs follow, image
// pull, save and load) use a copy of the client with Timeout 0 and are bounded
// by their context instead. Every non-2xx answer is a *dockerhost.StatusError,
// so callers classify by dockerhost.StatusOf.
type engine struct{ c *dockerutil.Client }

func newEngine(c *dockerutil.Client) engine { return engine{c: c} }

// streamClient is the client without a total timeout.
func (e engine) streamClient() *http.Client {
	hc := *e.c.HTTPClient()
	hc.Timeout = 0
	return &hc
}

// statusError turns a non-2xx response into a *dockerhost.StatusError carrying
// the Engine's message (one printable line).
func statusError(resp *http.Response) error {
	message := dockerutil.SanitizeOneLine(dockerutil.EngineErrorMessage(dockerutil.ReadErrorBody(resp.Body)), 512)
	return &dockerhost.StatusError{
		Status: resp.StatusCode,
		Err:    fmt.Errorf("Docker API returned %d: %s", resp.StatusCode, message),
	}
}

// request sends one Engine request, with an optional body and content type,
// and returns the open response of a 2xx answer; the caller closes it.
func (e engine) request(ctx context.Context, method, path string, body io.Reader, contentType string, stream bool) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, dockerutil.Endpoint(path), body)
	if err != nil {
		return nil, fmt.Errorf("create Docker request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	client := e.c.HTTPClient()
	if stream {
		client = e.streamClient()
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Docker request: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, statusError(resp)
	}
	return resp, nil
}

// json is DoJSON with a *dockerhost.StatusError for a non-2xx answer.
func (e engine) json(ctx context.Context, method, path string, in, out any) error {
	status, err := e.c.DoJSON(ctx, method, path, in, out)
	if err != nil && status != 0 && (status < 200 || status >= 300) {
		return &dockerhost.StatusError{Status: status, Err: err}
	}
	return err
}

func isNotFound(err error) bool { return dockerhost.StatusOf(err) == http.StatusNotFound }

// engineVersion is the part of GET /version the stacks service reads.
type engineVersion struct {
	Version    string `json:"Version"`
	APIVersion string `json:"ApiVersion"`
	OS         string `json:"Os"`
	Arch       string `json:"Arch"`
}

// version reads GET /version.
func (e engine) version(ctx context.Context) (engineVersion, error) {
	var v engineVersion
	err := e.json(ctx, http.MethodGet, "version", nil, &v)
	return v, err
}

// apiMinor reads the minor number of an Engine API version such as "1.45".
func apiMinor(version string) (int, bool) {
	major, minor, found := strings.Cut(strings.TrimSpace(version), ".")
	if !found || major != "1" {
		return 0, false
	}
	n, err := strconv.Atoi(minor)
	return n, err == nil
}

// supportsStacks reports an Engine API of 1.41 (Docker 20.10) or newer.
func supportsStacks(apiVersion string) bool {
	minor, ok := apiMinor(apiVersion)
	return ok && minor >= MinAPIMinor
}

// containerState is the part of a container's inspect document the stacks
// service reads.
type containerState struct {
	ID        string
	Name      string
	Image     string // Config.Image
	Labels    map[string]string
	Status    string // created, running, paused, restarting, removing, exited, dead
	Running   bool
	ExitCode  int
	OOMKilled bool
	Error     string // State.Error
}

// inspectContainer reads GET /containers/<ref>/json. A missing container is a
// 404 *dockerhost.StatusError.
func (e engine) inspectContainer(ctx context.Context, ref string) (containerState, error) {
	var raw struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Config struct {
			Image  string            `json:"Image"`
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		State struct {
			Status    string `json:"Status"`
			Running   bool   `json:"Running"`
			ExitCode  int    `json:"ExitCode"`
			OOMKilled bool   `json:"OOMKilled"`
			Error     string `json:"Error"`
		} `json:"State"`
	}
	if err := e.json(ctx, http.MethodGet, "containers/"+url.PathEscape(ref)+"/json", nil, &raw); err != nil {
		return containerState{}, err
	}
	return containerState{
		ID:        raw.ID,
		Name:      strings.TrimPrefix(raw.Name, "/"),
		Image:     raw.Config.Image,
		Labels:    raw.Config.Labels,
		Status:    raw.State.Status,
		Running:   raw.State.Running,
		ExitCode:  raw.State.ExitCode,
		OOMKilled: raw.State.OOMKilled,
		Error:     raw.State.Error,
	}, nil
}

// createContainer sends POST /containers/create?name=<name> and returns the id.
func (e engine) createContainer(ctx context.Context, name string, spec containerSpec) (string, error) {
	var out struct {
		ID string `json:"Id"`
	}
	if err := e.json(ctx, http.MethodPost, "containers/create?name="+url.QueryEscape(name), spec, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// startContainer starts a container; the Engine's 304 (already started) is success.
func (e engine) startContainer(ctx context.Context, ref string) error {
	return dockerhost.ContainerAction(ctx, e.c, ref, "start")
}

// killContainer sends SIGKILL; the Engine's 409 (not running) is not an error here.
func (e engine) killContainer(ctx context.Context, ref string) error {
	err := dockerhost.ContainerAction(ctx, e.c, ref, "kill")
	if dockerhost.StatusOf(err) == http.StatusConflict || isNotFound(err) {
		return nil
	}
	return err
}

// removeContainer force-removes a container together with its anonymous
// volumes; a container that is already gone is success.
func (e engine) removeContainer(ctx context.Context, ref string) error {
	err := dockerhost.RemoveContainer(ctx, e.c, ref, true)
	if isNotFound(err) {
		return nil
	}
	return err
}

// followLogs streams the logs of a non-TTY container (stdout and stderr, no
// timestamps) into the writers until the stream ends, which for a running
// container is when it exits. since is a unix time in seconds (0 = from the
// start). Either writer may be nil.
func (e engine) followLogs(ctx context.Context, ref string, since int64, stdout, stderr io.Writer) error {
	path := "containers/" + url.PathEscape(ref) + "/logs?follow=true&stdout=true&stderr=true"
	if since > 0 {
		path += "&since=" + strconv.FormatInt(since, 10)
	}
	resp, err := e.request(ctx, http.MethodGet, path, nil, "", true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return dockerhost.DemuxStreams(resp.Body, stdout, stderr)
}

// imageInfo is the part of an image's inspect document the stacks service reads.
type imageInfo struct {
	ID   string `json:"Id"`
	OS   string `json:"Os"`
	Arch string `json:"Architecture"`
}

// inspectImage reads GET /images/<ref>/json; found is false for a 404.
func (e engine) inspectImage(ctx context.Context, ref string) (info imageInfo, found bool, err error) {
	err = e.json(ctx, http.MethodGet, "images/"+url.PathEscape(ref)+"/json", nil, &info)
	if isNotFound(err) {
		return imageInfo{}, false, nil
	}
	return info, err == nil, err
}

// jsonMessage is one line of the JSON message streams of image pull and load.
type jsonMessage struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Stream   string `json:"stream"`
	Progress struct {
		Current int64 `json:"current"`
		Total   int64 `json:"total"`
	} `json:"progressDetail"`
	Error       string `json:"error"`
	ErrorDetail struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
}

// readJSONMessages decodes a JSON message stream line by line and hands every
// message to fn (which may be nil). An error event ends the read with a
// *dockerutil.JSONMessageError. Lines that are not JSON are skipped.
func readJSONMessages(r io.Reader, fn func(jsonMessage)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), dockerutil.MaxJSONMessageLine)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var msg jsonMessage
		if json.Unmarshal(line, &msg) != nil {
			continue
		}
		if text := strings.TrimSpace(msg.ErrorDetail.Message); text != "" {
			return &dockerutil.JSONMessageError{Message: text}
		}
		if text := strings.TrimSpace(msg.Error); text != "" {
			return &dockerutil.JSONMessageError{Message: text}
		}
		if fn != nil {
			fn(msg)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read Docker message stream: %w", err)
	}
	return nil
}

// pullImage pulls fromImage:tag (tag may be a digest) and hands every message
// of the pull stream to fn.
func (e engine) pullImage(ctx context.Context, fromImage, tag string, fn func(jsonMessage)) error {
	path := "images/create?fromImage=" + url.QueryEscape(fromImage) + "&tag=" + url.QueryEscape(tag)
	resp, err := e.request(ctx, http.MethodPost, path, nil, "", true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return readJSONMessages(resp.Body, fn)
}

// saveImage opens GET /images/<ref>/get, the image as a tar stream.
func (e engine) saveImage(ctx context.Context, ref string) (io.ReadCloser, error) {
	resp, err := e.request(ctx, http.MethodGet, "images/"+url.PathEscape(ref)+"/get", nil, "", true)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// loadImage sends a tar stream to POST /images/load and returns what the
// Engine reported as loaded ("sha256:..." image id or a reference), "" when it
// said nothing recognizable.
func (e engine) loadImage(ctx context.Context, tarStream io.Reader) (string, error) {
	resp, err := e.request(ctx, http.MethodPost, "images/load?quiet=1", tarStream, "application/x-tar", true)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	loaded := ""
	err = readJSONMessages(resp.Body, func(m jsonMessage) {
		const idPrefix, refPrefix = "Loaded image ID: ", "Loaded image: "
		text := strings.TrimSpace(m.Stream)
		switch {
		case strings.HasPrefix(text, idPrefix):
			loaded = strings.TrimSpace(strings.TrimPrefix(text, idPrefix))
		case strings.HasPrefix(text, refPrefix) && loaded == "":
			loaded = strings.TrimSpace(strings.TrimPrefix(text, refPrefix))
		}
	})
	return loaded, err
}

// tagImage sends POST /images/<ref>/tag?repo=...&tag=....
func (e engine) tagImage(ctx context.Context, ref, repo, tag string) error {
	path := "images/" + url.PathEscape(ref) + "/tag?repo=" + url.QueryEscape(repo) + "&tag=" + url.QueryEscape(tag)
	return e.json(ctx, http.MethodPost, path, nil, nil)
}

// projectVolumes lists the volumes that carry the compose project label.
func (e engine) projectVolumes(ctx context.Context, project string) ([]ProjectVolume, error) {
	filters, _ := json.Marshal(map[string][]string{"label": {"com.docker.compose.project=" + project}})
	var out struct {
		Volumes []struct {
			Name   string `json:"Name"`
			Driver string `json:"Driver"`
		} `json:"Volumes"`
	}
	if err := e.json(ctx, http.MethodGet, "volumes?filters="+url.QueryEscape(string(filters)), nil, &out); err != nil {
		return nil, err
	}
	volumes := make([]ProjectVolume, 0, len(out.Volumes))
	for _, v := range out.Volumes {
		volumes = append(volumes, ProjectVolume{Name: v.Name, Driver: v.Driver, Size: -1})
	}
	return volumes, nil
}

// volumeSizes reads GET /system/df?type=volume (API 1.42 and newer): the size
// in bytes of every volume the Engine measured, by name. A volume the Engine
// could not measure (it reports -1) is left out.
func (e engine) volumeSizes(ctx context.Context) (map[string]int64, error) {
	var out struct {
		Volumes []struct {
			Name      string `json:"Name"`
			UsageData *struct {
				Size int64 `json:"Size"`
			} `json:"UsageData"`
		} `json:"Volumes"`
	}
	if err := e.json(ctx, http.MethodGet, "system/df?type=volume", nil, &out); err != nil {
		return nil, err
	}
	sizes := make(map[string]int64, len(out.Volumes))
	for _, v := range out.Volumes {
		if v.UsageData != nil && v.UsageData.Size >= 0 {
			sizes[v.Name] = v.UsageData.Size
		}
	}
	return sizes, nil
}

// engineMessage is the one-line text of an Engine failure for an operation's
// error field. Engine answers keep their message; anything else its error text.
func engineMessage(err error) string {
	var jsonErr *dockerutil.JSONMessageError
	if errors.As(err, &jsonErr) {
		return dockerutil.SanitizeOneLine(jsonErr.Message, 300)
	}
	return dockerutil.SanitizeOneLine(err.Error(), 300)
}

package dockerhost

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"aurago/internal/dockerutil"
	"aurago/internal/tools"
)

// StatusError is a non-2xx answer of the Engine; Status is the HTTP status
// code. Callers classify Engine answers by it (404 gone, 409 conflict), never
// by matching the message.
type StatusError struct {
	Status int
	Err    error
}

func (e *StatusError) Error() string { return e.Err.Error() }
func (e *StatusError) Unwrap() error { return e.Err }

// StatusOf returns the HTTP status carried by an Engine answer error, or 0 when
// the error is not one (no answer arrived, or the caller's own validation).
func StatusOf(err error) int {
	var se *StatusError
	if errors.As(err, &se) {
		return se.Status
	}
	return 0
}

var (
	// ErrInvalidAction is returned for a container action the Docker Manager does not offer.
	ErrInvalidAction = errors.New("invalid container action")
	// ErrInvalidContainerRef is returned for a container id or name that is unsafe in an API path.
	ErrInvalidContainerRef = errors.New("invalid container id or name")
)

var reContainerRef = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func validateContainerRef(id string) error {
	if id == "" || len(id) > 128 || !reContainerRef.MatchString(id) || strings.Contains(id, "..") {
		return ErrInvalidContainerRef
	}
	return nil
}

// engineJSON is Client.DoJSON without a request body that wraps a non-2xx
// answer in a *StatusError.
func engineJSON(ctx context.Context, c *dockerutil.Client, method, path string, out any) error {
	status, err := c.DoJSON(ctx, method, path, nil, out)
	if err != nil && status != 0 && (status < 200 || status >= 300) {
		return &StatusError{Status: status, Err: err}
	}
	return err
}

// Port is one published or exposed container port.
type Port struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort int    `json:"private"`
	PublicPort  int    `json:"public,omitempty"`
	Type        string `json:"type"`
}

// Mount is one container mount. A bind's Source keeps only its last path
// element, like the admin inspect.
type Mount struct {
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination"`
}

// ContainerSummary is one row of the container list.
type ContainerSummary struct {
	ID                string            `json:"id"` // 12 chars
	FullID            string            `json:"full_id"`
	Names             []string          `json:"names"` // leading "/" stripped
	Image             string            `json:"image"`
	ImageID           string            `json:"image_id"`
	State             string            `json:"state"`
	Status            string            `json:"status"`
	Health            string            `json:"health,omitempty"` // healthy | unhealthy | starting
	Created           int64             `json:"created"`
	Ports             []Port            `json:"ports"`
	Mounts            []Mount           `json:"mounts"`
	Networks          []string          `json:"networks"`
	ComposeProject    string            `json:"compose_project,omitempty"`
	ComposeService    string            `json:"compose_service,omitempty"`
	ComposeWorkingDir string            `json:"compose_working_dir,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"` // credential-like values masked
}

const (
	labelComposeProject    = "com.docker.compose.project"
	labelComposeService    = "com.docker.compose.service"
	labelComposeWorkingDir = "com.docker.compose.project.working_dir"
)

type engineContainer struct {
	ID      string   `json:"Id"`
	Names   []string `json:"Names"`
	Image   string   `json:"Image"`
	ImageID string   `json:"ImageID"`
	Created int64    `json:"Created"`
	Ports   []struct {
		IP          string `json:"IP"`
		PrivatePort int    `json:"PrivatePort"`
		PublicPort  int    `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
	Labels     map[string]string `json:"Labels"`
	State      string            `json:"State"`
	Status     string            `json:"Status"`
	HostConfig struct {
		NetworkMode string `json:"NetworkMode"`
	} `json:"HostConfig"`
	NetworkSettings struct {
		Networks map[string]json.RawMessage `json:"Networks"`
	} `json:"NetworkSettings"`
	Mounts []struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	} `json:"Mounts"`
}

// ListContainers returns every container of the Engine, running or not.
func ListContainers(ctx context.Context, c *dockerutil.Client) ([]ContainerSummary, error) {
	out, _, err := listContainers(ctx, c, false)
	return out, err
}

// ListContainersWithEntries is ListContainers plus, from the same Engine answer, the compact entries the Containers page's
// protection classification reads: the full ID, the labels unmasked, the addresses of the container and its network mode. The
// entries equal what tools.DockerListContainerEntries(cfg, true) returns for the same Engine, so the local host's list needs one
// listing instead of two.
func ListContainersWithEntries(ctx context.Context, c *dockerutil.Client) ([]ContainerSummary, []tools.DockerContainerListEntry, error) {
	return listContainers(ctx, c, true)
}

// engineEndpoint is the part of a container's network endpoint the protection classification reads.
type engineEndpoint struct {
	IPAddress         string `json:"IPAddress"`
	GlobalIPv6Address string `json:"GlobalIPv6Address"`
}

// listEntry is the tools.DockerListContainerEntries entry of one container of the list.
func listEntry(rc engineContainer, id string) tools.DockerContainerListEntry {
	entry := tools.DockerContainerListEntry{
		ID: id, Names: rc.Names, Image: rc.Image, State: rc.State, Status: rc.Status, FullID: rc.ID, Labels: rc.Labels,
		NetworkMode: strings.TrimSpace(rc.HostConfig.NetworkMode),
	}
	for _, raw := range rc.NetworkSettings.Networks {
		var endpoint engineEndpoint
		if json.Unmarshal(raw, &endpoint) != nil {
			continue
		}
		for _, ip := range []string{endpoint.IPAddress, endpoint.GlobalIPv6Address} {
			if ip = strings.TrimSpace(ip); ip != "" {
				entry.NetworkIPs = append(entry.NetworkIPs, ip)
			}
		}
	}
	return entry
}

func listContainers(ctx context.Context, c *dockerutil.Client, withEntries bool) ([]ContainerSummary, []tools.DockerContainerListEntry, error) {
	var raw []engineContainer
	if err := engineJSON(ctx, c, http.MethodGet, "containers/json?all=true", &raw); err != nil {
		return nil, nil, err
	}
	out := make([]ContainerSummary, 0, len(raw))
	var entries []tools.DockerContainerListEntry
	if withEntries {
		entries = make([]tools.DockerContainerListEntry, 0, len(raw))
	}
	for _, rc := range raw {
		s := ContainerSummary{
			FullID:            rc.ID,
			Image:             rc.Image,
			ImageID:           rc.ImageID,
			State:             rc.State,
			Status:            rc.Status,
			Health:            healthFromStatus(rc.Status),
			Created:           rc.Created,
			Names:             make([]string, 0, len(rc.Names)),
			Ports:             make([]Port, 0, len(rc.Ports)),
			Mounts:            make([]Mount, 0, len(rc.Mounts)),
			Networks:          make([]string, 0, len(rc.NetworkSettings.Networks)),
			ComposeProject:    rc.Labels[labelComposeProject],
			ComposeService:    rc.Labels[labelComposeService],
			ComposeWorkingDir: rc.Labels[labelComposeWorkingDir],
			Labels:            tools.RedactDockerLabels(rc.Labels),
		}
		s.ID = rc.ID
		if len(s.ID) > 12 {
			s.ID = s.ID[:12]
		}
		for _, name := range rc.Names {
			s.Names = append(s.Names, strings.TrimPrefix(name, "/"))
		}
		for _, p := range rc.Ports {
			s.Ports = append(s.Ports, Port{IP: p.IP, PrivatePort: p.PrivatePort, PublicPort: p.PublicPort, Type: p.Type})
		}
		slices.SortStableFunc(s.Ports, func(a, b Port) int {
			if a.PrivatePort != b.PrivatePort {
				return a.PrivatePort - b.PrivatePort
			}
			if a.Type != b.Type {
				return strings.Compare(a.Type, b.Type)
			}
			if a.PublicPort != b.PublicPort {
				return a.PublicPort - b.PublicPort
			}
			return strings.Compare(a.IP, b.IP)
		})
		for _, m := range rc.Mounts {
			s.Mounts = append(s.Mounts, Mount{
				Type:        m.Type,
				Name:        m.Name,
				Source:      tools.RedactDockerMountSource(m.Type, m.Source),
				Destination: m.Destination,
			})
		}
		for name := range rc.NetworkSettings.Networks {
			s.Networks = append(s.Networks, name)
		}
		slices.Sort(s.Networks)
		out = append(out, s)
		if withEntries {
			entries = append(entries, listEntry(rc, s.ID))
		}
	}
	return out, entries, nil
}

// healthFromStatus reads the health state out of the list's status text
// ("Up 2 hours (healthy)", "Up 5 seconds (health: starting)"); the list API has
// no separate health field.
func healthFromStatus(status string) string {
	switch {
	case strings.Contains(status, "(unhealthy)"):
		return "unhealthy"
	case strings.Contains(status, "(healthy)"):
		return "healthy"
	case strings.Contains(status, "(health: starting)"):
		return "starting"
	}
	return ""
}

// InspectContainer returns the container's inspect document, trimmed and
// redacted exactly like the admin inspect: credential-like environment values,
// command-line secrets and labels are masked and bind sources are reduced to
// their last path element.
func InspectContainer(ctx context.Context, c *dockerutil.Client, id string) (json.RawMessage, error) {
	if err := validateContainerRef(id); err != nil {
		return nil, err
	}
	var raw json.RawMessage
	if err := engineJSON(ctx, c, http.MethodGet, "containers/"+url.PathEscape(id)+"/json", &raw); err != nil {
		return nil, err
	}
	projected, err := tools.ProjectDockerInspect(raw)
	if err != nil {
		return nil, err
	}
	return json.Marshal(projected)
}

// containerActionPaths maps an action to the Engine request suffix.
var containerActionPaths = map[string]string{
	"start":   "start",
	"stop":    "stop?t=10",
	"restart": "restart?t=10",
	"pause":   "pause",
	"unpause": "unpause",
	"kill":    "kill",
}

// ContainerAction runs start, stop, restart, pause, unpause or kill. The Engine
// answers 304 when the container is already in the requested state; that is
// success.
func ContainerAction(ctx context.Context, c *dockerutil.Client, id, action string) error {
	suffix, ok := containerActionPaths[action]
	if !ok {
		return ErrInvalidAction
	}
	if err := validateContainerRef(id); err != nil {
		return err
	}
	err := engineJSON(ctx, c, http.MethodPost, "containers/"+url.PathEscape(id)+"/"+suffix, nil)
	if StatusOf(err) == http.StatusNotModified {
		return nil
	}
	return err
}

// RemoveContainer deletes a container together with its anonymous volumes
// (v=true), like the admin remove; force also removes a running container.
func RemoveContainer(ctx context.Context, c *dockerutil.Client, id string, force bool) error {
	if err := validateContainerRef(id); err != nil {
		return err
	}
	query := "?v=true"
	if force {
		query += "&force=true"
	}
	return engineJSON(ctx, c, http.MethodDelete, "containers/"+url.PathEscape(id)+query, nil)
}

const (
	logTailDefault = 200
	logTailMax     = 5000
	logResultMax   = 1 << 20  // the result keeps at most the last 1 MiB
	logReadMax     = 64 << 20 // raw bytes read from the Engine
	logOmittedNote = "[earlier log lines omitted]\n"
)

// ContainerLogs returns the last tail lines (default 200, 1..5000) of a
// container's stdout and stderr with timestamps. A multiplexed stream is
// demultiplexed; a TTY container's stream is plain text. The result is capped
// at 1 MiB, keeping the end.
func ContainerLogs(ctx context.Context, c *dockerutil.Client, id string, tail int) (string, error) {
	if err := validateContainerRef(id); err != nil {
		return "", err
	}
	if tail <= 0 {
		tail = logTailDefault
	}
	if tail > logTailMax {
		tail = logTailMax
	}
	var inspect struct {
		Config struct {
			Tty bool `json:"Tty"`
		} `json:"Config"`
	}
	ref := url.PathEscape(id)
	if err := engineJSON(ctx, c, http.MethodGet, "containers/"+ref+"/json", &inspect); err != nil {
		return "", err
	}
	endpoint := dockerutil.Endpoint(fmt.Sprintf("containers/%s/logs?stdout=true&stderr=true&timestamps=true&tail=%d", ref, tail))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create Docker request: %w", err)
	}
	resp, err := c.HTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("Docker request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		message := dockerutil.SanitizeOneLine(dockerutil.EngineErrorMessage(dockerutil.ReadErrorBody(resp.Body)), 512)
		return "", &StatusError{Status: resp.StatusCode, Err: fmt.Errorf("Docker API returned %d: %s", resp.StatusCode, message)}
	}
	return readLogs(io.LimitReader(resp.Body, logReadMax), inspect.Config.Tty)
}

func readLogs(r io.Reader, tty bool) (string, error) {
	tb := &tailBuffer{max: logResultMax}
	var err error
	if tty {
		_, err = io.Copy(tb, r)
	} else {
		err = demuxLogs(r, tb)
	}
	if err != nil {
		return "", fmt.Errorf("read container logs: %w", err)
	}
	return tb.String(), nil
}

// demuxLogs copies the frames of a multiplexed Docker stream (8-byte header:
// stream 0..2, three zero bytes, big-endian payload size) to w, stdout and
// stderr in arrival order. A stream that does not start like one is copied
// verbatim, so a proxy that already demultiplexed it still works. A stream cut
// short ends quietly.
func demuxLogs(r io.Reader, w io.Writer) error {
	return demuxFrames(r, func(byte) io.Writer { return w })
}

// DemuxStreams splits a non-TTY Engine log or attach stream into stdout and
// stderr. Either writer may be nil, which discards that stream. A frame of
// stream 0 (stdin) goes to stdout, like Docker's own StdCopy. It shares the
// frame parser of demuxLogs: a stream that does not start like a frame is
// copied verbatim to stdout, and a stream cut short ends quietly.
func DemuxStreams(r io.Reader, stdout, stderr io.Writer) error {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	return demuxFrames(r, func(stream byte) io.Writer {
		if stream == 2 {
			return stderr
		}
		return stdout
	})
}

// demuxFrames is the frame parser behind demuxLogs and DemuxStreams. pick
// chooses the writer of a frame by its stream byte; the writer of stream 1
// also receives a stream that is not framed at all.
func demuxFrames(r io.Reader, pick func(stream byte) io.Writer) error {
	br := bufio.NewReader(r)
	plain := pick(1)
	var header [8]byte
	for {
		n, err := io.ReadFull(br, header[:])
		switch {
		case err == io.EOF: // no bytes left: a clean end
			return nil
		case err == io.ErrUnexpectedEOF: // fewer than 8 bytes cannot be a frame; keep them as text
			_, werr := plain.Write(header[:n])
			return werr
		case err != nil:
			return err
		}
		if header[0] > 2 || header[1] != 0 || header[2] != 0 || header[3] != 0 {
			if _, werr := plain.Write(header[:]); werr != nil {
				return werr
			}
			_, cerr := io.Copy(plain, br)
			return cerr
		}
		size := int64(binary.BigEndian.Uint32(header[4:]))
		if _, err := io.CopyN(pick(header[0]), br, size); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// tailBuffer keeps the last max bytes written to it.
type tailBuffer struct {
	max     int
	buf     []byte
	dropped bool
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > 2*t.max {
		t.trim()
	}
	return len(p), nil
}

func (t *tailBuffer) trim() {
	if len(t.buf) > t.max {
		t.buf = append(t.buf[:0], t.buf[len(t.buf)-t.max:]...)
		t.dropped = true
	}
}

// String returns the kept text. When the start was cut, it begins at the next
// whole line behind a short note.
func (t *tailBuffer) String() string {
	t.trim()
	s := t.buf
	if !t.dropped {
		return string(s)
	}
	for len(s) > 0 && !utf8.RuneStart(s[0]) {
		s = s[1:]
	}
	if i := bytes.IndexByte(s, '\n'); i >= 0 && i < 4096 {
		s = s[i+1:]
	}
	return logOmittedNote + string(s)
}

// ImageSummary is one row of the image list.
type ImageSummary struct {
	ID         string   `json:"id"`
	Tags       []string `json:"tags"`
	Size       int64    `json:"size"`
	Created    int64    `json:"created"`
	Containers int      `json:"containers"` // computed by Annotate from ContainerSummary.ImageID; -1 unknown
	Dangling   bool     `json:"dangling"`
}

// VolumeSummary is one row of the volume list.
type VolumeSummary struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Mountpoint string            `json:"mountpoint"`
	CreatedAt  string            `json:"created_at,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	UsedBy     []string          `json:"used_by"` // container names, computed by Annotate
}

// NetworkSummary is one row of the network list.
type NetworkSummary struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Driver     string   `json:"driver"`
	Scope      string   `json:"scope"`
	Subnets    []string `json:"subnets"`
	Containers int      `json:"containers"` // computed by Annotate from ContainerSummary.Networks
	Builtin    bool     `json:"builtin"`    // bridge, host, none
}

// EngineInfo is the host overview.
type EngineInfo struct {
	Name              string `json:"name"`
	ServerVersion     string `json:"server_version"`
	APIVersion        string `json:"api_version"`
	OS                string `json:"os"`
	OSType            string `json:"os_type"`
	Kernel            string `json:"kernel"`
	Arch              string `json:"arch"`
	DockerRootDir     string `json:"docker_root_dir"`
	NCPU              int    `json:"ncpu"`
	MemTotal          int64  `json:"mem_total"`
	Containers        int    `json:"containers"`
	ContainersRunning int    `json:"containers_running"`
	ContainersPaused  int    `json:"containers_paused"`
	ContainersStopped int    `json:"containers_stopped"`
	Images            int    `json:"images"`
}

// ListImages returns the Engine's images. Containers is -1 until Annotate.
func ListImages(ctx context.Context, c *dockerutil.Client) ([]ImageSummary, error) {
	var raw []struct {
		ID          string   `json:"Id"`
		RepoTags    []string `json:"RepoTags"`
		RepoDigests []string `json:"RepoDigests"`
		Created     int64    `json:"Created"`
		Size        int64    `json:"Size"`
	}
	if err := engineJSON(ctx, c, http.MethodGet, "images/json", &raw); err != nil {
		return nil, err
	}
	out := make([]ImageSummary, 0, len(raw))
	for _, ri := range raw {
		tags := make([]string, 0, len(ri.RepoTags))
		for _, tag := range ri.RepoTags {
			if tag != "<none>:<none>" {
				tags = append(tags, tag)
			}
		}
		out = append(out, ImageSummary{
			ID:         ri.ID,
			Tags:       tags,
			Size:       ri.Size,
			Created:    ri.Created,
			Containers: -1,
			Dangling:   len(tags) == 0 && len(ri.RepoDigests) == 0,
		})
	}
	return out, nil
}

// ListVolumes returns the Engine's volumes. UsedBy is empty until Annotate.
func ListVolumes(ctx context.Context, c *dockerutil.Client) ([]VolumeSummary, error) {
	var raw struct {
		Volumes []struct {
			Name       string            `json:"Name"`
			Driver     string            `json:"Driver"`
			Mountpoint string            `json:"Mountpoint"`
			CreatedAt  string            `json:"CreatedAt"`
			Labels     map[string]string `json:"Labels"`
		} `json:"Volumes"`
	}
	if err := engineJSON(ctx, c, http.MethodGet, "volumes", &raw); err != nil {
		return nil, err
	}
	out := make([]VolumeSummary, 0, len(raw.Volumes))
	for _, rv := range raw.Volumes {
		out = append(out, VolumeSummary{
			Name:       rv.Name,
			Driver:     rv.Driver,
			Mountpoint: rv.Mountpoint,
			CreatedAt:  rv.CreatedAt,
			Labels:     tools.RedactDockerLabels(rv.Labels),
			UsedBy:     []string{},
		})
	}
	return out, nil
}

// ListNetworks returns the Engine's networks. Containers is 0 until Annotate.
func ListNetworks(ctx context.Context, c *dockerutil.Client) ([]NetworkSummary, error) {
	var raw []struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Driver string `json:"Driver"`
		Scope  string `json:"Scope"`
		IPAM   struct {
			Config []struct {
				Subnet string `json:"Subnet"`
			} `json:"Config"`
		} `json:"IPAM"`
	}
	if err := engineJSON(ctx, c, http.MethodGet, "networks", &raw); err != nil {
		return nil, err
	}
	out := make([]NetworkSummary, 0, len(raw))
	for _, rn := range raw {
		n := NetworkSummary{
			ID:      rn.ID,
			Name:    rn.Name,
			Driver:  rn.Driver,
			Scope:   rn.Scope,
			Subnets: []string{},
			Builtin: rn.Name == "bridge" || rn.Name == "host" || rn.Name == "none",
		}
		for _, cfg := range rn.IPAM.Config {
			if cfg.Subnet != "" {
				n.Subnets = append(n.Subnets, cfg.Subnet)
			}
		}
		out = append(out, n)
	}
	return out, nil
}

// Info returns the host overview from /info plus the API version from /version.
func Info(ctx context.Context, c *dockerutil.Client) (EngineInfo, error) {
	var version struct {
		APIVersion string `json:"ApiVersion"`
	}
	if err := engineJSON(ctx, c, http.MethodGet, "version", &version); err != nil {
		return EngineInfo{}, err
	}
	var raw struct {
		Name              string `json:"Name"`
		ServerVersion     string `json:"ServerVersion"`
		OperatingSystem   string `json:"OperatingSystem"`
		OSType            string `json:"OSType"`
		KernelVersion     string `json:"KernelVersion"`
		Architecture      string `json:"Architecture"`
		DockerRootDir     string `json:"DockerRootDir"`
		NCPU              int    `json:"NCPU"`
		MemTotal          int64  `json:"MemTotal"`
		Containers        int    `json:"Containers"`
		ContainersRunning int    `json:"ContainersRunning"`
		ContainersPaused  int    `json:"ContainersPaused"`
		ContainersStopped int    `json:"ContainersStopped"`
		Images            int    `json:"Images"`
	}
	if err := engineJSON(ctx, c, http.MethodGet, "info", &raw); err != nil {
		return EngineInfo{}, err
	}
	return EngineInfo{
		Name:              raw.Name,
		ServerVersion:     raw.ServerVersion,
		APIVersion:        version.APIVersion,
		OS:                raw.OperatingSystem,
		OSType:            raw.OSType,
		Kernel:            raw.KernelVersion,
		Arch:              raw.Architecture,
		DockerRootDir:     raw.DockerRootDir,
		NCPU:              raw.NCPU,
		MemTotal:          raw.MemTotal,
		Containers:        raw.Containers,
		ContainersRunning: raw.ContainersRunning,
		ContainersPaused:  raw.ContainersPaused,
		ContainersStopped: raw.ContainersStopped,
		Images:            raw.Images,
	}, nil
}

// Annotate fills the counts that the Engine's list calls do not give: how many
// containers use each image and network, and which containers use each volume.
// A nil containers slice means the container list is unknown; image counts then
// stay -1 and the other counts stay empty. Matching is by the image id of a
// container, the volume name of a volume mount and the network names of a
// container.
func Annotate(containers []ContainerSummary, images []ImageSummary, volumes []VolumeSummary, networks []NetworkSummary) {
	if containers == nil {
		return
	}
	imageUse := map[string]int{}
	networkUse := map[string]int{}
	volumeUse := map[string][]string{}
	for _, ctr := range containers {
		imageUse[ctr.ImageID]++
		for _, name := range ctr.Networks {
			networkUse[name]++
		}
		label := ctr.ID
		if len(ctr.Names) > 0 {
			label = ctr.Names[0]
		}
		seen := map[string]bool{}
		for _, m := range ctr.Mounts {
			if m.Type == "volume" && m.Name != "" && !seen[m.Name] {
				seen[m.Name] = true
				volumeUse[m.Name] = append(volumeUse[m.Name], label)
			}
		}
	}
	for i := range images {
		images[i].Containers = imageUse[images[i].ID]
	}
	for i := range volumes {
		users := append([]string{}, volumeUse[volumes[i].Name]...)
		slices.Sort(users)
		volumes[i].UsedBy = users
	}
	for i := range networks {
		networks[i].Containers = networkUse[networks[i].Name]
	}
}

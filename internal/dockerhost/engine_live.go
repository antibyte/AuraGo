package dockerhost

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/dockerutil"
)

// The live Engine calls of the Docker Manager: the events feed, one stats
// sample of a container and the disk usage. Streaming reads use the client
// without a total timeout (Client.StreamingHTTPClient); the caller's context
// bounds them.

// eventLineMax bounds one line of the events feed; real events are a few
// hundred bytes, the limit only protects against a broken or hostile endpoint.
const eventLineMax = 1 << 20

// dfBodyMax bounds the /system/df answer.
const dfBodyMax = 32 << 20

// eventFilters limits the feed to the resource types the Docker Manager shows.
const eventFilters = `{"type":["container","image","volume","network"]}`

// EngineEvent is one entry of the Engine's events feed.
type EngineEvent struct {
	Type       string            // container | image | volume | network
	Action     string            // e.g. start, die, "health_status: healthy" (raw)
	ID         string            // Actor.ID
	Attributes map[string]string // Actor.Attributes; values are not masked and never go to the UI raw
	Time       time.Time         // from timeNano, else from time
}

type liveRawEvent struct {
	Type     string `json:"Type"`
	Action   string `json:"Action"`
	TimeSec  int64  `json:"time"`
	TimeNano int64  `json:"timeNano"`
	Actor    struct {
		ID         string            `json:"ID"`
		Attributes map[string]string `json:"Attributes"`
	} `json:"Actor"`
}

// liveGet sends a GET on the streaming client and turns a non-2xx answer into
// a *StatusError. The caller closes the body of a successful response.
func liveGet(ctx context.Context, c *dockerutil.Client, path string) (*http.Response, error) {
	hc := c.StreamingHTTPClient()
	if hc == nil {
		return nil, fmt.Errorf("docker client is not initialised")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dockerutil.Endpoint(path), nil)
	if err != nil {
		return nil, fmt.Errorf("create Docker request: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Docker request: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		message := dockerutil.SanitizeOneLine(dockerutil.EngineErrorMessage(dockerutil.ReadErrorBody(resp.Body)), 512)
		return nil, &StatusError{Status: resp.StatusCode, Err: fmt.Errorf("Docker API returned %d: %s", resp.StatusCode, message)}
	}
	return resp, nil
}

// StreamEvents follows GET /events (types container, image, volume, network;
// events since the given time when it is not zero) and calls fn for each
// decoded event. It returns when ctx ends (ctx.Err()), when fn returns an
// error (that error), or when the stream ends (an error wrapping io.EOF; the
// Engine never ends the feed on its own, so this means a dropped connection).
func StreamEvents(ctx context.Context, c *dockerutil.Client, since time.Time, fn func(EngineEvent) error) error {
	return StreamEventsOpen(ctx, c, since, nil, fn)
}

// StreamEventsOpen is StreamEvents with a callback that runs once, as soon as
// the Engine has accepted the request and before the first event: the point
// where the feed counts as established. opened may be nil.
func StreamEventsOpen(ctx context.Context, c *dockerutil.Client, since time.Time, opened func(), fn func(EngineEvent) error) error {
	query := url.Values{"filters": {eventFilters}}
	if !since.IsZero() {
		query.Set("since", fmt.Sprintf("%d.%09d", since.Unix(), since.Nanosecond()))
	}
	resp, err := liveGet(ctx, c, "events?"+query.Encode())
	if err != nil {
		return ctxOr(ctx, err)
	}
	defer resp.Body.Close()
	if opened != nil {
		opened()
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64<<10), eventLineMax)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var raw liveRawEvent
		if err := json.Unmarshal(line, &raw); err != nil {
			return fmt.Errorf("decode Docker event: %w", err)
		}
		ev := EngineEvent{Type: raw.Type, Action: raw.Action, ID: raw.Actor.ID, Attributes: raw.Actor.Attributes}
		switch {
		case raw.TimeNano > 0:
			ev.Time = time.Unix(0, raw.TimeNano).UTC()
		case raw.TimeSec > 0:
			ev.Time = time.Unix(raw.TimeSec, 0).UTC()
		}
		if err := fn(ev); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return ctxOr(ctx, fmt.Errorf("read Docker events: %w", err))
	}
	return ctxOr(ctx, fmt.Errorf("Docker events stream ended: %w", io.EOF))
}

// ctxOr prefers the context's error over the transport error it caused.
func ctxOr(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}

// RawStats is the decoded subset of one GET /containers/{id}/stats sample.
// The counters are cumulative; callers derive rates and percentages.
type RawStats struct {
	Read              time.Time // zero for a container that is not running
	CPUTotal          uint64    // cpu_stats.cpu_usage.total_usage
	SystemCPU         uint64    // cpu_stats.system_cpu_usage
	OnlineCPUs        int       // cpu_stats.online_cpus, else len(percpu_usage)
	PreCPUTotal       uint64    // precpu_stats.cpu_usage.total_usage
	PreSystem         uint64    // precpu_stats.system_cpu_usage
	MemUsage          uint64    // memory_stats.usage
	MemCache          uint64    // total_inactive_file or inactive_file, else cache
	MemLimit          uint64    // memory_stats.limit
	NetRx, NetTx      uint64    // summed over the networks
	BlkRead, BlkWrite uint64    // blkio read and write bytes, summed over devices
	Pids              uint64
}

type liveCPUStats struct {
	CPUUsage struct {
		Total  uint64   `json:"total_usage"`
		PerCPU []uint64 `json:"percpu_usage"`
	} `json:"cpu_usage"`
	System uint64 `json:"system_cpu_usage"`
	Online uint32 `json:"online_cpus"`
}

type liveRawStats struct {
	Read string `json:"read"`
	Pids struct {
		Current uint64 `json:"current"`
	} `json:"pids_stats"`
	Blkio struct {
		IOServiceBytes []struct {
			Op    string `json:"op"`
			Value uint64 `json:"value"`
		} `json:"io_service_bytes_recursive"`
	} `json:"blkio_stats"`
	CPU    liveCPUStats `json:"cpu_stats"`
	PreCPU liveCPUStats `json:"precpu_stats"`
	Memory struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		Rx uint64 `json:"rx_bytes"`
		Tx uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

// ContainerStats reads one stats sample (stream=false). oneShot adds
// one-shot=true, which the Engine honours from API 1.41: it answers at once
// instead of waiting a second to fill precpu_stats, and leaves precpu_stats
// empty. A container that is not running answers with an all-zero sample
// (Read is the zero time).
func ContainerStats(ctx context.Context, c *dockerutil.Client, id string, oneShot bool) (RawStats, error) {
	if err := validateContainerRef(id); err != nil {
		return RawStats{}, err
	}
	path := "containers/" + url.PathEscape(id) + "/stats?stream=false"
	if oneShot {
		path = "containers/" + url.PathEscape(id) + "/stats?one-shot=true&stream=false"
	}
	var raw liveRawStats
	if err := engineJSON(ctx, c, http.MethodGet, path, &raw); err != nil {
		return RawStats{}, err
	}
	out := RawStats{
		CPUTotal:    raw.CPU.CPUUsage.Total,
		SystemCPU:   raw.CPU.System,
		OnlineCPUs:  int(raw.CPU.Online),
		PreCPUTotal: raw.PreCPU.CPUUsage.Total,
		PreSystem:   raw.PreCPU.System,
		MemUsage:    raw.Memory.Usage,
		MemLimit:    raw.Memory.Limit,
		Pids:        raw.Pids.Current,
	}
	if out.OnlineCPUs == 0 {
		out.OnlineCPUs = len(raw.CPU.CPUUsage.PerCPU)
	}
	if t, err := time.Parse(time.RFC3339Nano, raw.Read); err == nil && t.Year() > 1 {
		out.Read = t.UTC()
	}
	// Page cache that the kernel can drop at once does not count as use, like
	// `docker stats`: cgroup v1 reports total_inactive_file, cgroup v2
	// inactive_file, very old kernels only cache.
	for _, key := range []string{"total_inactive_file", "inactive_file", "cache"} {
		if v, ok := raw.Memory.Stats[key]; ok {
			out.MemCache = v
			break
		}
	}
	for _, n := range raw.Networks {
		out.NetRx += n.Rx
		out.NetTx += n.Tx
	}
	for _, e := range raw.Blkio.IOServiceBytes {
		switch strings.ToLower(e.Op) {
		case "read":
			out.BlkRead += e.Value
		case "write":
			out.BlkWrite += e.Value
		}
	}
	return out, nil
}

// DiskUsageGroup is the usage of one kind of Docker data.
type DiskUsageGroup struct {
	Count       int   `json:"count"`
	Active      int   `json:"active"`
	Size        int64 `json:"size"`
	Reclaimable int64 `json:"reclaimable"`
}

// DiskUsage is Docker's own disk use in bytes (the Engine API reports no size
// for the file system itself).
type DiskUsage struct {
	Images     DiskUsageGroup `json:"images"`
	Containers DiskUsageGroup `json:"containers"`
	Volumes    DiskUsageGroup `json:"volumes"`
	BuildCache DiskUsageGroup `json:"build_cache"`
	Total      int64          `json:"total"` // sum of the four Size values
}

// nonNegative maps the Engine's "not computed" value -1 to 0.
func nonNegative(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}

// SystemDF reads GET /system/df and totals it the way `docker system df`
// does: image size is the size of the unique layers, container size the
// writable layers, volume size the reported usage, build cache without shared
// records. Reclaimable is what unused images, stopped containers, unreferenced
// volumes and idle build cache hold. The call can take a while on a large host.
func SystemDF(ctx context.Context, c *dockerutil.Client) (DiskUsage, error) {
	resp, err := liveGet(ctx, c, "system/df")
	if err != nil {
		return DiskUsage{}, ctxOr(ctx, err)
	}
	defer resp.Body.Close()
	var raw struct {
		LayersSize int64 `json:"LayersSize"`
		Images     []struct {
			Size       int64 `json:"Size"`
			SharedSize int64 `json:"SharedSize"`
			Containers int64 `json:"Containers"`
		} `json:"Images"`
		Containers []struct {
			State  string `json:"State"`
			SizeRw int64  `json:"SizeRw"`
		} `json:"Containers"`
		Volumes []struct {
			UsageData *struct {
				RefCount int64 `json:"RefCount"`
				Size     int64 `json:"Size"`
			} `json:"UsageData"`
		} `json:"Volumes"`
		BuildCache []struct {
			InUse  bool  `json:"InUse"`
			Shared bool  `json:"Shared"`
			Size   int64 `json:"Size"`
		} `json:"BuildCache"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, dfBodyMax)).Decode(&raw); err != nil {
		return DiskUsage{}, ctxOr(ctx, fmt.Errorf("decode Docker disk usage: %w", err))
	}

	var out DiskUsage
	used := int64(0)
	out.Images.Count = len(raw.Images)
	for _, img := range raw.Images {
		if img.Containers > 0 {
			out.Images.Active++
			if img.Size >= 0 && img.SharedSize >= 0 {
				used += img.Size - img.SharedSize
			}
		}
	}
	out.Images.Size = nonNegative(raw.LayersSize)
	out.Images.Reclaimable = nonNegative(out.Images.Size - used)

	out.Containers.Count = len(raw.Containers)
	for _, ctr := range raw.Containers {
		size := nonNegative(ctr.SizeRw)
		out.Containers.Size += size
		if strings.EqualFold(ctr.State, "running") {
			out.Containers.Active++
		} else {
			out.Containers.Reclaimable += size
		}
	}

	out.Volumes.Count = len(raw.Volumes)
	for _, vol := range raw.Volumes {
		if vol.UsageData == nil {
			continue
		}
		size := nonNegative(vol.UsageData.Size)
		out.Volumes.Size += size
		switch {
		case vol.UsageData.RefCount > 0:
			out.Volumes.Active++
		case vol.UsageData.RefCount == 0:
			out.Volumes.Reclaimable += size
		}
	}

	out.BuildCache.Count = len(raw.BuildCache)
	for _, rec := range raw.BuildCache {
		if rec.InUse {
			out.BuildCache.Active++
		}
		if rec.Shared {
			continue
		}
		size := nonNegative(rec.Size)
		out.BuildCache.Size += size
		if !rec.InUse {
			out.BuildCache.Reclaimable += size
		}
	}

	out.Total = out.Images.Size + out.Containers.Size + out.Volumes.Size + out.BuildCache.Size
	return out, nil
}

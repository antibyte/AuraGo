package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// OSProcessInfo holds detailed info about a single OS process.
type OSProcessInfo struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float32 `json:"mem_percent"`
	MemRSS     uint64  `json:"mem_rss_bytes"`
	Username   string  `json:"username,omitempty"`
	Cmdline    string  `json:"cmdline,omitempty"`
	CreateTime string  `json:"create_time,omitempty"`
	NumThreads int32   `json:"num_threads"`
	PPID       int32   `json:"ppid"`
}

// OSProcessTree represents a parent with its children.
type OSProcessTree struct {
	OSProcessInfo
	Children []OSProcessInfo `json:"children,omitempty"`
}

type processAnalyzerResult struct {
	Status    string      `json:"status"`
	Operation string      `json:"operation"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

type processMetricSample struct {
	proc       *process.Process
	cpuPercent float64
	memRSS     uint64
}

func processJSON(r processAnalyzerResult) string {
	b, _ := json.Marshal(r)
	return string(b)
}

// AnalyzeProcesses dispatches process analysis operations.
// Operations: top_cpu, top_memory, find, tree, info
func AnalyzeProcesses(operation string, name string, pid int, limit int) string {
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	switch operation {
	case "top_cpu":
		return processTopCPU(limit)
	case "top_memory":
		return processTopMemory(limit)
	case "find":
		return processFind(name, limit)
	case "tree":
		return processTree(int32(pid))
	case "info":
		if pid <= 0 {
			return processJSON(processAnalyzerResult{Status: "error", Message: "pid is required for info operation"})
		}
		return processInfo(int32(pid))
	default:
		return processJSON(processAnalyzerResult{
			Status:  "error",
			Message: fmt.Sprintf("unknown operation: %s (valid: top_cpu, top_memory, find, tree, info)", operation),
		})
	}
}

func gatherProcessInfo(p *process.Process) OSProcessInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	info := OSProcessInfo{PID: p.Pid}

	if n, err := p.NameWithContext(ctx); err == nil {
		info.Name = n
	}
	if s, err := p.StatusWithContext(ctx); err == nil && len(s) > 0 {
		info.Status = s[0]
	}
	if cpu, err := p.CPUPercentWithContext(ctx); err == nil {
		info.CPUPercent = cpu
	}
	if mp, err := p.MemoryPercentWithContext(ctx); err == nil {
		info.MemPercent = mp
	}
	if mi, err := p.MemoryInfoWithContext(ctx); err == nil && mi != nil {
		info.MemRSS = mi.RSS
	}
	if u, err := p.UsernameWithContext(ctx); err == nil {
		info.Username = u
	}
	if ct, err := p.CreateTimeWithContext(ctx); err == nil {
		info.CreateTime = time.UnixMilli(ct).Format(time.RFC3339)
	}
	if nt, err := p.NumThreadsWithContext(ctx); err == nil {
		info.NumThreads = nt
	}
	if ppid, err := p.PpidWithContext(ctx); err == nil {
		info.PPID = ppid
	}

	return info
}

func sampleProcessCPU(p *process.Process) (processMetricSample, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cpu, err := p.CPUPercentWithContext(ctx)
	if err != nil {
		return processMetricSample{}, false
	}

	return processMetricSample{proc: p, cpuPercent: cpu}, true
}

func sampleProcessMemory(p *process.Process) (processMetricSample, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mi, err := p.MemoryInfoWithContext(ctx)
	if err != nil || mi == nil {
		return processMetricSample{}, false
	}

	return processMetricSample{proc: p, memRSS: mi.RSS}, true
}

func selectTopProcessSamples(samples []processMetricSample, limit int, less func(a, b processMetricSample) bool) []processMetricSample {
	if len(samples) == 0 {
		return nil
	}

	sort.Slice(samples, func(i, j int) bool {
		return less(samples[i], samples[j])
	})

	if len(samples) > limit {
		samples = samples[:limit]
	}

	return samples
}

func collectTopProcessInfos(limit int, sampler func(*process.Process) (processMetricSample, bool), less func(a, b processMetricSample) bool) ([]OSProcessInfo, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}

	samples := make([]processMetricSample, 0, len(procs))
	for _, p := range procs {
		if sample, ok := sampler(p); ok {
			samples = append(samples, sample)
		}
	}

	topSamples := selectTopProcessSamples(samples, limit, less)
	infos := make([]OSProcessInfo, 0, len(topSamples))
	for _, sample := range topSamples {
		info := gatherProcessInfo(sample.proc)
		if sample.cpuPercent > 0 {
			info.CPUPercent = sample.cpuPercent
		}
		if sample.memRSS > 0 {
			info.MemRSS = sample.memRSS
		}
		infos = append(infos, info)
	}

	return infos, nil
}

func processTopCPU(limit int) string {
	infos, err := collectTopProcessInfos(limit, sampleProcessCPU, func(a, b processMetricSample) bool {
		return a.cpuPercent > b.cpuPercent
	})
	if err != nil {
		return processJSON(processAnalyzerResult{Status: "error", Message: fmt.Sprintf("failed to list processes: %v", err)})
	}

	return processJSON(processAnalyzerResult{
		Status:    "success",
		Operation: "top_cpu",
		Message:   fmt.Sprintf("Top %d processes by CPU usage", len(infos)),
		Data:      infos,
	})
}

func processTopMemory(limit int) string {
	infos, err := collectTopProcessInfos(limit, sampleProcessMemory, func(a, b processMetricSample) bool {
		return a.memRSS > b.memRSS
	})
	if err != nil {
		return processJSON(processAnalyzerResult{Status: "error", Message: fmt.Sprintf("failed to list processes: %v", err)})
	}

	return processJSON(processAnalyzerResult{
		Status:    "success",
		Operation: "top_memory",
		Message:   fmt.Sprintf("Top %d processes by memory usage", len(infos)),
		Data:      infos,
	})
}

func processFind(name string, limit int) string {
	if name == "" {
		return processJSON(processAnalyzerResult{Status: "error", Message: "name is required for find operation"})
	}

	procs, err := process.Processes()
	if err != nil {
		return processJSON(processAnalyzerResult{Status: "error", Message: fmt.Sprintf("failed to list processes: %v", err)})
	}

	var matches []OSProcessInfo
	for _, p := range procs {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		pname, err := p.NameWithContext(ctx)
		cancel()
		if err != nil {
			continue
		}

		// Also check cmdline for broader matching
		ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
		cmdline, _ := p.CmdlineWithContext(ctx2)
		cancel2()

		if containsIgnoreCase(pname, name) || containsIgnoreCase(cmdline, name) {
			info := gatherProcessInfo(p)
			info.Cmdline = cmdline
			matches = append(matches, info)
			if len(matches) >= limit {
				break
			}
		}
	}

	return processJSON(processAnalyzerResult{
		Status:    "success",
		Operation: "find",
		Message:   fmt.Sprintf("Found %d processes matching '%s'", len(matches), name),
		Data:      matches,
	})
}

func processTree(pid int32) string {
	if pid <= 0 {
		return processJSON(processAnalyzerResult{Status: "error", Message: "pid is required for tree operation"})
	}

	p, err := process.NewProcess(pid)
	if err != nil {
		return processJSON(processAnalyzerResult{Status: "error", Message: fmt.Sprintf("process %d not found: %v", pid, err)})
	}

	parent := gatherProcessInfo(p)

	children, err := p.Children()
	if err != nil {
		// No children or permission denied — not fatal
		return processJSON(processAnalyzerResult{
			Status:    "success",
			Operation: "tree",
			Message:   fmt.Sprintf("Process %d has no children", pid),
			Data:      OSProcessTree{OSProcessInfo: parent},
		})
	}

	childInfos := make([]OSProcessInfo, 0, len(children))
	for _, c := range children {
		childInfos = append(childInfos, gatherProcessInfo(c))
	}

	tree := OSProcessTree{
		OSProcessInfo: parent,
		Children:      childInfos,
	}

	return processJSON(processAnalyzerResult{
		Status:    "success",
		Operation: "tree",
		Message:   fmt.Sprintf("Process %d has %d children", pid, len(childInfos)),
		Data:      tree,
	})
}

func processInfo(pid int32) string {
	p, err := process.NewProcess(pid)
	if err != nil {
		return processJSON(processAnalyzerResult{Status: "error", Message: fmt.Sprintf("process %d not found: %v", pid, err)})
	}

	info := gatherProcessInfo(p)

	// Gather extra detail for single-process view
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if cmd, err := p.CmdlineWithContext(ctx); err == nil {
		info.Cmdline = cmd
	}

	return processJSON(processAnalyzerResult{
		Status:    "success",
		Operation: "info",
		Message:   fmt.Sprintf("Details for PID %d (%s)", pid, info.Name),
		Data:      info,
	})
}

func containsIgnoreCase(s, substr string) bool {
	if s == "" || substr == "" {
		return false
	}
	// Simple case-insensitive contains using lowercase comparison
	sl := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		sl[i] = c
	}
	subl := make([]byte, len(substr))
	for i := range substr {
		c := substr[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		subl[i] = c
	}
	return bytesContains(sl, subl)
}

func bytesContains(s, sub []byte) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		match := true
		for j := range sub {
			if s[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

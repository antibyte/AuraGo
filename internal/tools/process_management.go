package tools

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/shirou/gopsutil/v4/process"
)

// ProcResult is the JSON response returned to the LLM.
type ProcResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ProcessBasicInfo represents a summary of a running process.
type ProcessBasicInfo struct {
	PID           int32   `json:"pid"`
	Name          string  `json:"name"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float32 `json:"memory_percent"`
	Status        string  `json:"status"`
}

// ManageProcesses handles platform-independent process management.
func ManageProcesses(operation string, pid int32) string {
	encode := func(r ProcResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	switch operation {
	case "list":
		procs, err := process.Processes()
		if err != nil {
			return encode(ProcResult{Status: "error", Message: fmt.Sprintf("Failed to list processes: %v", err)})
		}

		var items []ProcessBasicInfo
		for _, p := range procs {
			name, _ := p.Name()
			cpu, _ := p.CPUPercent()
			mem, _ := p.MemoryPercent()
			status, _ := p.Status()

			displayStatus := "U"
			if len(status) > 0 && len(status[0]) > 0 {
				// Assuming status is []string based on lint feedback, safely get first char
				displayStatus = status[0][0:1]
			}

			items = append(items, ProcessBasicInfo{

				PID:           p.Pid,
				Name:          name,
				CPUPercent:    cpu,
				MemoryPercent: mem,
				Status:        displayStatus,
			})
		}

		// Sort by CPU usage descending
		sort.Slice(items, func(i, j int) bool {
			return items[i].CPUPercent > items[j].CPUPercent
		})

		// Return top 50 to avoid context flooding
		limit := 50
		if len(items) < limit {
			limit = len(items)
		}

		return encode(ProcResult{
			Status:  "success",
			Message: fmt.Sprintf("Listed %d processes (top %d by CPU)", len(items), limit),
			Data:    items[:limit],
		})

	case "kill":
		if pid == 0 {
			return encode(ProcResult{Status: "error", Message: "PID 0 cannot be killed"})
		}
		p, err := process.NewProcess(pid)
		if err != nil {
			return encode(ProcResult{Status: "error", Message: fmt.Sprintf("Process %d not found: %v", pid, err)})
		}
		if err := p.Kill(); err != nil {
			return encode(ProcResult{Status: "error", Message: fmt.Sprintf("Failed to kill process %d: %v", pid, err)})
		}
		return encode(ProcResult{Status: "success", Message: fmt.Sprintf("Process %d terminated", pid)})

	case "stats":
		p, err := process.NewProcess(pid)
		if err != nil {
			return encode(ProcResult{Status: "error", Message: fmt.Sprintf("Process %d not found: %v", pid, err)})
		}

		name, _ := p.Name()
		cmdline, _ := p.Cmdline()
		createTime, _ := p.CreateTime()
		memInfo, _ := p.MemoryInfo()
		cpuPercent, _ := p.CPUPercent()

		stats := map[string]interface{}{
			"pid":         pid,
			"name":        name,
			"command":     cmdline,
			"created_at":  createTime,
			"cpu_percent": cpuPercent,
			"memory":      memInfo,
		}

		return encode(ProcResult{Status: "success", Data: stats})

	default:
		return encode(ProcResult{Status: "error", Message: fmt.Sprintf("Unknown operation: %s. Valid: list, kill, stats", operation)})
	}
}

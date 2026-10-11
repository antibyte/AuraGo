package persondetect

// SelfTest is the worker's startup check on ReferenceImage(): 1 warm-up and 3
// measured runs per session, wall time and process CPU time per run.
type SelfTest struct {
	Passed           bool    `json:"passed"`
	Reason           string  `json:"reason,omitempty"`
	ReferencePersons int     `json:"reference_persons"`
	GPUWallMS        float64 `json:"gpu_wall_ms,omitempty"`
	GPUCPUMS         float64 `json:"gpu_cpu_ms,omitempty"`
	CPUWallMS        float64 `json:"cpu_wall_ms"`
	CPUCPUMS         float64 `json:"cpu_cpu_ms"`
}

// workerRequest is one JSON line on the worker's stdin.
type workerRequest struct {
	ID       uint64  `json:"id"`
	Command  string  `json:"command"`        // detect | close
	JPEG     []byte  `json:"jpeg,omitempty"` // base64 in JSON
	MinScore float64 `json:"min_score,omitempty"`
}

// helloMessage is the first line `serve` prints.
type helloMessage struct {
	Type      string    `json:"type"` // hello
	Version   string    `json:"version,omitempty"`
	Providers []string  `json:"providers,omitempty"`
	Provider  string    `json:"provider,omitempty"` // cpu | webgpu
	SelfTest  *SelfTest `json:"self_test,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// preparedMessage is the only line `prepare` prints.
type preparedMessage struct {
	Type         string `json:"type"` // prepared | error
	PatchedNodes int    `json:"patched_nodes,omitempty"`
	Error        string `json:"error,omitempty"`
}

type detectResponse struct {
	ID     uint64  `json:"id"`
	Type   string  `json:"type"`
	Boxes  []Box   `json:"boxes"`
	Width  int     `json:"width"`
	Height int     `json:"height"`
	WallMS float64 `json:"wall_ms"`
	CPUMS  float64 `json:"cpu_ms"`
}

type errorResponse struct {
	ID    uint64 `json:"id"`
	Type  string `json:"type"`
	Error string `json:"error"`
}

type closeResponse struct {
	ID   uint64 `json:"id"`
	Type string `json:"type"`
}

// workerMessage is the client-side union of every line a worker prints.
type workerMessage struct {
	ID           uint64    `json:"id"`
	Type         string    `json:"type"`
	Error        string    `json:"error"`
	Version      string    `json:"version"`
	Providers    []string  `json:"providers"`
	Provider     string    `json:"provider"`
	SelfTest     *SelfTest `json:"self_test"`
	PatchedNodes int       `json:"patched_nodes"`
	Boxes        []Box     `json:"boxes"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	WallMS       float64   `json:"wall_ms"`
	CPUMS        float64   `json:"cpu_ms"`
}

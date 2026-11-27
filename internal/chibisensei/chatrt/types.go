package chatrt

// MaxInstances is the number of llama-server children that run at the same time.
const MaxInstances = 2

// Config is the runtime configuration; the manager reads it through a func so config reloads apply at once.
type Config struct {
	Enabled     bool
	Backend     string // auto | cpu | vulkan | cuda
	IdleMinutes int
	ContextSize int
	CacheDir    string // <data>/chibi_sensei/runtime
}

// ModelRef names one GGUF the caller has verified.
type ModelRef struct {
	Key   string // stable identity of the file: "tuned:<run_id>" or "base:<rel_path of the shared file>"
	Path  string // absolute path of a GGUF the caller has verified
	Bytes int64
}

// Message is one chat turn.
type Message struct {
	Role    string `json:"role"` // system | user | assistant
	Content string `json:"content"`
}

// Sampling holds the sampling values of one request.
type Sampling struct {
	Temperature, TopP, RepeatPenalty, PresencePenalty float64
	TopK, MaxTokens                                   int
	Seed                                              int64 // 0 = random
}

// ChatRequest is one completion request.
type ChatRequest struct {
	Messages []Message // already prepared with PrepareMessages
	Sampling Sampling
	Stop     []string
}

// ChatStats describes one finished answer.
type ChatStats struct {
	FinishReason     string // stop | length
	PromptTokens     int
	CompletionTokens int
	MS               int64
	TokensPerSecond  float64
	Backend          string // cpu | vulkan | cuda | metal
}

// InstanceStatus is the public view of one child process.
type InstanceStatus struct {
	Key         string `json:"key"`
	State       string `json:"state"` // starting | ready | busy | stopping
	Backend     string `json:"backend"`
	GPUVerified bool   `json:"gpu_verified"`
	IdleSeconds int    `json:"idle_s"`
	Started     string `json:"started"`
}

// DownloadStatus describes the install set and the progress of its download.
type DownloadStatus struct {
	Bytes, DoneBytes int64  // total of the install set; bytes done so far
	Host             string // host of the asset URLs, e.g. "github.com"
	ErrorCode        string // "" | download_failed | checksum_mismatch | disk_full
}

// Status is the public view of the runtime.
type Status struct {
	State     string // disabled | unsupported | not_installed | installing | ready | failed
	Build     string
	Requested string // configured backend
	Selected  string // resolved backend; "" while unknown
	Fallback  bool   // a GPU start fell back to CPU since the last install or start
	Download  DownloadStatus
	Instances []InstanceStatus
}

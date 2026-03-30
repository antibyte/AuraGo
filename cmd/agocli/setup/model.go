package setup

import (
	"sync"

	"aurago/cmd/agocli/syscheck"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// Step represents the current step in the setup wizard.
type Step int

const (
	StepWelcome Step = iota
	StepDependencies
	StepMasterKey
	StepExtract
	StepNetwork
	StepConfig
	StepPassword
	StepService
	StepStart
	StepSummary
)

// Model is the setup wizard state.
type Model struct {
	CurrentStep Step

	// Output from setup process
	Output   []string
	outputMu sync.Mutex

	// Configuration
	ServerURL  string
	InstallDir string

	// Setup state
	Started bool // user has pressed Enter on Welcome, setup is running
	Running bool // a step is currently executing
	Done    bool
	Err     error

	// Dependency selection
	DepChecks    []syscheck.CheckResult
	SelectedDeps []string // command names of deps user wants to install
	DepForm      *huh.Form

	// Network / HTTPS
	NetworkMode string // "local", "lan", "https"
	HTTPS       HTTPSConfig
	NetworkForm *huh.Form

	// Service
	InstallSvc  bool
	ServiceForm *huh.Form

	// Results
	MasterKey    string
	KeyGenerated bool
	Password     string
	AccessURL    string

	// Dimensions
	Width  int
	Height int
}

// NewModel creates a new setup model.
func NewModel(serverURL string) *Model {
	return &Model{
		ServerURL:   serverURL,
		CurrentStep: StepWelcome,
		Output:      []string{},
		NetworkMode: "local",
	}
}

// AddOutput adds a line of output from the setup process.
func (m *Model) AddOutput(line string) {
	m.outputMu.Lock()
	defer m.outputMu.Unlock()
	m.Output = append(m.Output, line)
	if len(m.Output) > 100 {
		m.Output = m.Output[len(m.Output)-100:]
	}
}

// Init does not auto-start setup — user must press Enter on Welcome first.
func (m *Model) Init() tea.Cmd {
	return nil
}

// NextStep advances to the next step.
func (m *Model) NextStep() {
	m.CurrentStep++
}

// SetDone marks the setup as complete.
func (m *Model) SetDone(err error) {
	m.Done = true
	m.Err = err
	if err != nil {
		m.CurrentStep = StepSummary
	}
}

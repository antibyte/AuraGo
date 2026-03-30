package update

import (
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// Step represents the current step in the update wizard.
type Step int

const (
	StepCheck Step = iota
	StepChangelog
	StepConfirm
	StepApply
	StepKeyMigrate
	StepRestart
	StepSummary
)

// Model is the update wizard state.
type Model struct {
	CurrentStep Step

	// Update info
	UpdateAvailable bool
	CurrentVersion  string
	LatestVersion   string
	Changelog       string

	// Output from update process
	Output   []string
	outputMu sync.Mutex

	// Options
	NoRestart bool
	Yes       bool // auto-confirm

	// Update state
	Running bool
	Done    bool
	Err     error

	// Key migration
	MigrateKey  bool
	MigrateForm *huh.Form

	// Restart confirm
	DoRestart   bool
	RestartForm *huh.Form

	// Config
	InstallDir string
	ServerURL  string

	// Dimensions
	Width  int
	Height int
}

// NewModel creates a new update model.
func NewModel(serverURL string) *Model {
	return &Model{
		CurrentStep: StepCheck,
		Output:      []string{},
		ServerURL:   serverURL,
		DoRestart:   true,
	}
}

// NewModelWithOpts creates a new update model with options.
func NewModelWithOpts(serverURL string, noRestart, yes bool) *Model {
	m := NewModel(serverURL)
	m.NoRestart = noRestart
	m.Yes = yes
	return m
}

// AddOutput adds a line of output from the update process.
func (m *Model) AddOutput(line string) {
	m.outputMu.Lock()
	defer m.outputMu.Unlock()
	m.Output = append(m.Output, line)
	if len(m.Output) > 200 {
		m.Output = m.Output[len(m.Output)-200:]
	}
}

// SetDone marks the update as complete.
func (m *Model) SetDone(err error) {
	m.Done = true
	m.Err = err
	if err != nil {
		m.CurrentStep = StepSummary
	}
}

// NextStep advances to the next step.
func (m *Model) NextStep() {
	m.CurrentStep++
}

// Init initializes the update wizard and starts the version check.
func (m *Model) Init() tea.Cmd {
	m.resolveInstallDir()
	return m.checkForUpdates()
}

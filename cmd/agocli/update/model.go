package update

import (
	"sync"

	"github.com/charmbracelet/bubbletea"
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
	Changelog      string

	// Output from update process
	Output    []string
	outputMu  sync.Mutex

	// Update state
	Done     bool
	Err      error

	// Dimensions
	Width  int
	Height int
}

// NewModel creates a new update model.
func NewModel(serverURL string) *Model {
	return &Model{
		CurrentStep: StepCheck,
		Output:      []string{},
	}
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

// Init initializes the update wizard.
func (m *Model) Init() tea.Cmd {
	return nil
}

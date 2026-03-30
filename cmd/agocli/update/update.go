package update

import (
	"os"
	"path/filepath"

	"aurago/cmd/agocli/shared"
	"aurago/cmd/agocli/updater"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// ── Tea messages ─────────────────────────────────────────────────────

type stepDoneMsg struct{ err error }
type formDoneMsg struct{}

// ── Update (message dispatch) ────────────────────────────────────────

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		if form := m.activeForm(); form != nil {
			return m.updateForm(msg)
		}
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case stepDoneMsg:
		m.Running = false
		if msg.err != nil {
			m.SetDone(msg.err)
			return m, nil
		}
		return m, m.advanceAfterStep()

	case formDoneMsg:
		m.Running = false
		return m, m.advanceAfterForm()
	}

	return m, nil
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if m.Done {
			return m, tea.Quit
		}
		if m.CurrentStep == StepChangelog {
			m.NextStep() // → Confirm
			return m, m.startConfirm()
		}
		return m, nil
	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}

// ── Form helpers ─────────────────────────────────────────────────────

func (m *Model) activeForm() *huh.Form {
	switch m.CurrentStep {
	case StepConfirm:
		return nil // we use auto-advance for --yes
	case StepKeyMigrate:
		return m.MigrateForm
	case StepRestart:
		return m.RestartForm
	}
	return nil
}

func (m *Model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	form := m.activeForm()
	if form == nil {
		return m, nil
	}
	model, cmd := form.Update(msg)
	if f, ok := model.(*huh.Form); ok {
		switch m.CurrentStep {
		case StepKeyMigrate:
			m.MigrateForm = f
		case StepRestart:
			m.RestartForm = f
		}
	}
	if form.State == huh.StateCompleted {
		return m, func() tea.Msg { return formDoneMsg{} }
	}
	return m, cmd
}

// ── Step flow ────────────────────────────────────────────────────────

func (m *Model) resolveInstallDir() {
	exePath, err := os.Executable()
	if err == nil {
		exePath, _ = filepath.EvalSymlinks(exePath)
		m.InstallDir = filepath.Dir(filepath.Dir(exePath))
	}
	if m.InstallDir == "" {
		m.InstallDir, _ = os.Getwd()
	}
}

func (m *Model) checkForUpdates() tea.Cmd {
	m.Running = true
	return func() tea.Msg {
		cfg := &updater.Config{
			InstallDir: m.InstallDir,
			LogFn:      m.AddOutput,
		}
		avail, version, changelog, err := updater.CheckForUpdates(cfg)
		if err != nil {
			return stepDoneMsg{err: err}
		}

		m.UpdateAvailable = avail
		m.LatestVersion = version
		m.Changelog = changelog

		if !avail {
			m.AddOutput("Already running the latest version.")
			m.SetDone(nil)
		}
		return stepDoneMsg{}
	}
}

func (m *Model) advanceAfterStep() tea.Cmd {
	switch m.CurrentStep {
	case StepCheck:
		if !m.UpdateAvailable {
			m.CurrentStep = StepSummary
			m.SetDone(nil)
			return nil
		}
		m.CurrentStep = StepChangelog
		return nil // wait for Enter

	case StepApply:
		m.CurrentStep = StepKeyMigrate
		return m.startKeyMigrate()

	case StepKeyMigrate:
		// handled by form
		return nil

	case StepRestart:
		m.CurrentStep = StepSummary
		m.SetDone(nil)
		return nil
	}

	return nil
}

func (m *Model) advanceAfterForm() tea.Cmd {
	switch m.CurrentStep {
	case StepKeyMigrate:
		if m.MigrateKey {
			m.AddOutput("Migrating master key to /etc/aurago/...")
			done, err := shared.MigrateMasterKey(m.InstallDir)
			if err != nil {
				m.AddOutput("WARNING: Key migration failed: " + err.Error())
			} else if done {
				m.AddOutput("Master key migrated successfully")
				shared.PatchServiceEnvFile("/etc/aurago/master.key")
			}
		}
		m.CurrentStep = StepRestart
		return m.startRestart()

	case StepRestart:
		if m.DoRestart && !m.NoRestart {
			m.AddOutput("Restarting AuraGo...")
			if err := shared.RestartService(m.InstallDir); err != nil {
				m.AddOutput("WARNING: Restart failed: " + err.Error())
			} else {
				m.AddOutput("AuraGo restarted successfully!")
			}
		}
		m.CurrentStep = StepSummary
		m.SetDone(nil)
		return nil
	}
	return nil
}

func (m *Model) startConfirm() tea.Cmd {
	if m.Yes {
		// auto-confirm
		m.NextStep() // → StepApply
		return m.runUpdate()
	}
	// For non-yes mode, user presses Enter on the confirm step text
	return nil
}

func (m *Model) runUpdate() tea.Cmd {
	m.Running = true
	return func() tea.Msg {
		cfg := &updater.Config{
			InstallDir: m.InstallDir,
			NoRestart:  true, // we handle restart in the TUI
			LogFn:      m.AddOutput,
		}
		err := updater.Run(cfg)
		return stepDoneMsg{err: err}
	}
}

func (m *Model) startKeyMigrate() tea.Cmd {
	// Check if migration is needed
	envPath := filepath.Join(m.InstallDir, ".env")
	key := shared.ReadEnvKey(envPath, "AURAGO_MASTER_KEY")
	if key == "" || !shared.HasSystemd() {
		// No migration needed — skip
		m.CurrentStep = StepRestart
		return m.startRestart()
	}

	m.MigrateKey = true
	m.MigrateForm = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Migrate master key to /etc/aurago/master.key?").
				Description("Your key is currently in .env — moving it to /etc/aurago/ is more secure.").
				Affirmative("Yes").
				Negative("Skip").
				Value(&m.MigrateKey),
		),
	)
	m.MigrateForm.Init()
	return nil
}

func (m *Model) startRestart() tea.Cmd {
	if m.NoRestart {
		m.CurrentStep = StepSummary
		m.SetDone(nil)
		return nil
	}

	m.RestartForm = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Restart AuraGo now?").
				Affirmative("Yes").
				Negative("No").
				Value(&m.DoRestart),
		),
	)
	m.RestartForm.Init()
	return nil
}

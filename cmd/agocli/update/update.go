package update

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbletea"
)

// Update handles messages for the update wizard.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil
	}

	return m, nil
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if m.Done {
			return m, tea.Quit
		}

		switch m.CurrentStep {
		case StepCheck:
			return m, m.checkForUpdates()

		case StepChangelog:
			m.NextStep()
			return m, nil

		case StepConfirm:
			m.NextStep()
			return m, m.runUpdate()

		case StepKeyMigrate:
			m.NextStep()
			return m, nil

		case StepRestart:
			return m, m.restartServer()
		}

	case tea.KeyCtrlC:
		return m, tea.Quit
	}

	return m, nil
}

// checkForUpdates checks if updates are available.
func (m *Model) checkForUpdates() tea.Cmd {
	return func() tea.Msg {
		m.AddOutput("Checking for updates...")

		// Check if running inside a git repo
		if isGitRepo() {
			m.AddOutput("Git repository detected")
			if hasGitUpdates() {
				m.UpdateAvailable = true
				m.Changelog = getChangelog()
				m.CurrentStep = StepChangelog
				m.AddOutput("Update available!")
			} else {
				m.UpdateAvailable = false
				m.CurrentStep = StepSummary
				m.AddOutput("Already running the latest version.")
				m.SetDone(nil)
			}
		} else {
			// Binary mode - check GitHub releases
			m.AddOutput("Checking GitHub releases...")
			latestTag := getLatestReleaseTag()
			if latestTag != "" {
				m.LatestVersion = latestTag
				m.UpdateAvailable = true
				m.CurrentStep = StepChangelog
				m.AddOutput("Update available: " + latestTag)
			} else {
				m.UpdateAvailable = false
				m.CurrentStep = StepSummary
				m.AddOutput("Could not determine latest version.")
				m.SetDone(nil)
			}
		}

		return updateOutputMsg{}
	}
}

// runUpdate runs the update.sh script.
func (m *Model) runUpdate() tea.Cmd {
	return func() tea.Msg {
		m.AddOutput("Starting update process...")
		m.AddOutput("")

		// Determine the script name based on OS
		scriptName := "./update.sh"
		if runtime.GOOS == "windows" {
			scriptName = "./update.bat"
		}

		cmd := exec.Command("bash", scriptName, "--yes")
		cmd.Dir, _ = os.Getwd()

		output, err := cmd.CombinedOutput()
		if err != nil {
			m.SetDone(err)
			m.AddOutput("Update failed: " + err.Error())
		} else {
			m.SetDone(nil)
			m.AddOutput("Update completed successfully!")
		}

		if len(output) > 0 {
			m.AddOutput("")
			m.AddOutput("=== Update Output ===")
			for _, line := range strings.Split(string(output), "\n") {
				if line != "" {
					m.AddOutput(line)
				}
			}
		}

		m.CurrentStep = StepSummary
		return updateOutputMsg{}
	}
}

// restartServer restarts the AuraGo service.
func (m *Model) restartServer() tea.Cmd {
	return func() tea.Msg {
		m.AddOutput("Restarting AuraGo...")

		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("schtasks", "/Run", "/TN", "AuraGo")
		} else {
			cmd = exec.Command("systemctl", "start", "aurago")
		}

		if err := cmd.Run(); err != nil {
			m.AddOutput("Could not restart via service manager: " + err.Error())
			m.AddOutput("Please restart AuraGo manually.")
		} else {
			m.AddOutput("AuraGo service restarted.")
		}

		m.AddOutput("")
		m.AddOutput("===========================================")
		m.AddOutput("Update process complete!")
		m.AddOutput("===========================================")
		m.AddOutput("")
		m.AddOutput("Press Enter to exit...")

		m.SetDone(nil)
		return updateOutputMsg{}
	}
}

// updateOutputMsg is a message indicating update output.
type updateOutputMsg struct{}

// Helper functions

func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir, _ = os.Getwd()
	return cmd.Run() == nil
}

func hasGitUpdates() bool {
	cmd := exec.Command("git", "fetch", "origin", "main")
	cmd.Dir, _ = os.Getwd()
	if err := cmd.Run(); err != nil {
		return false
	}

	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir, _ = os.Getwd()
	currentHash, _ := cmd.Output()

	cmd = exec.Command("git", "rev-parse", "origin/main")
	cmd.Dir, _ = os.Getwd()
	remoteHash, _ := cmd.Output()

	return string(currentHash) != string(remoteHash)
}

func getChangelog() string {
	cmd := exec.Command("git", "log", "HEAD..origin/main", "--oneline")
	cmd.Dir, _ = os.Getwd()
	output, err := cmd.Output()
	if err != nil {
		return "Could not retrieve changelog."
	}
	return strings.TrimSpace(string(output))
}

func getLatestReleaseTag() string {
	cmd := exec.Command("bash", "-c", "curl -s https://api.github.com/repos/antibyte/AuraGo/releases/latest | grep '\"tag_name\"' | cut -d'\"' -f4")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

package setup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"aurago/cmd/agocli/shared"
	"aurago/cmd/agocli/syscheck"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// ── Tea messages ─────────────────────────────────────────────────────

type stepDoneMsg struct{ err error } // a background step finished
type formDoneMsg struct{}            // an interactive huh form finished

// ── Update (message dispatch) ────────────────────────────────────────

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		// If a huh form is active, delegate keys to it first
		if m.activeForm() != nil {
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
		m.NextStep()
		return m, m.startCurrentStep()

	case formDoneMsg:
		m.Running = false
		m.NextStep()
		return m, m.startCurrentStep()
	}

	return m, nil
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if m.Done {
			return m, tea.Quit
		}
		if m.CurrentStep == StepWelcome && !m.Started {
			m.Started = true
			m.resolveInstallDir()
			m.NextStep() // → StepDependencies
			return m, m.startCurrentStep()
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
	case StepDependencies:
		return m.DepForm
	case StepNetwork:
		return m.NetworkForm
	case StepService:
		return m.ServiceForm
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
		case StepDependencies:
			m.DepForm = f
		case StepNetwork:
			m.NetworkForm = f
		case StepService:
			m.ServiceForm = f
		}
	}

	// Check if form completed
	if form.State == huh.StateCompleted {
		return m, func() tea.Msg { return formDoneMsg{} }
	}

	return m, cmd
}

// ── Step dispatcher ──────────────────────────────────────────────────

func (m *Model) startCurrentStep() tea.Cmd {
	switch m.CurrentStep {
	case StepDependencies:
		return m.stepDependencies()
	case StepMasterKey:
		return m.stepMasterKey()
	case StepExtract:
		return m.stepExtract()
	case StepNetwork:
		return m.stepNetwork()
	case StepConfig:
		return m.stepConfig()
	case StepPassword:
		return m.stepPassword()
	case StepService:
		return m.stepService()
	case StepStart:
		return m.stepStart()
	case StepSummary:
		m.buildSummary()
		m.SetDone(nil)
		return nil
	}
	return nil
}

// ── Step implementations ─────────────────────────────────────────────

func (m *Model) stepDependencies() tea.Cmd {
	if runtime.GOOS != "linux" {
		m.AddOutput("Skipping dependency check (not Linux)")
		return func() tea.Msg { return stepDoneMsg{} }
	}

	m.Running = true
	m.AddOutput("Checking system dependencies...")
	m.DepChecks = syscheck.CheckAll()

	for _, r := range m.DepChecks {
		if r.Installed {
			m.AddOutput(fmt.Sprintf("  ✓ %s (%s)", r.Dependency.Name, r.Version))
		} else {
			m.AddOutput(fmt.Sprintf("  ✗ %s — not found", r.Dependency.Name))
		}
	}

	missing := syscheck.MissingOptional(m.DepChecks)
	if len(missing) == 0 {
		m.AddOutput("All dependencies satisfied!")
		m.Running = false
		return func() tea.Msg { return stepDoneMsg{} }
	}

	// Build multi-select options
	var options []huh.Option[string]
	for _, r := range missing {
		options = append(options, huh.NewOption(
			fmt.Sprintf("%s — %s", r.Dependency.Name, r.Dependency.Description),
			r.Dependency.Command,
		))
	}

	m.DepForm = huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select optional dependencies to install:").
				Options(options...).
				Value(&m.SelectedDeps),
		),
	)
	m.DepForm.Init()
	m.Running = false
	return nil // form will be driven by key events
}

func (m *Model) stepMasterKey() tea.Cmd {
	m.Running = true
	return func() tea.Msg {
		m.AddOutput("Ensuring master key exists...")

		key, generated, err := shared.EnsureMasterKey(m.InstallDir)
		if err != nil {
			m.AddOutput("ERROR: " + err.Error())
			return stepDoneMsg{err: err}
		}

		m.MasterKey = key
		m.KeyGenerated = generated

		if generated {
			m.AddOutput("New master key generated: " + shared.MasterKeyPath(m.InstallDir))
		} else {
			m.AddOutput("Existing master key found")
		}
		return stepDoneMsg{}
	}
}

func (m *Model) stepExtract() tea.Cmd {
	m.Running = true
	return func() tea.Msg {
		resPath := filepath.Join(m.InstallDir, "resources.dat")
		if _, err := os.Stat(resPath); os.IsNotExist(err) {
			m.AddOutput("No resources.dat found — skipping extraction (source mode)")
			return stepDoneMsg{}
		}

		m.AddOutput("Extracting resources.dat...")
		if err := extractTarGz(resPath, m.InstallDir); err != nil {
			m.AddOutput("ERROR: " + err.Error())
			return stepDoneMsg{err: err}
		}
		m.AddOutput("Resources extracted successfully")
		return stepDoneMsg{}
	}
}

func (m *Model) stepNetwork() tea.Cmd {
	m.Running = true

	m.NetworkForm = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("How should AuraGo be accessible?").
				Options(
					huh.NewOption("Localhost only (127.0.0.1:8080)", "local"),
					huh.NewOption("LAN access (0.0.0.0:8080)", "lan"),
					huh.NewOption("Internet-facing with HTTPS (Let's Encrypt)", "https"),
				).
				Value(&m.NetworkMode),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Domain name (e.g. aurago.example.com)").
				Value(&m.HTTPS.Domain),
			huh.NewInput().
				Title("Email for Let's Encrypt").
				Value(&m.HTTPS.Email),
		).WithHideFunc(func() bool { return m.NetworkMode != "https" }),
	)
	m.NetworkForm.Init()
	m.Running = false
	return nil
}

func (m *Model) stepConfig() tea.Cmd {
	m.Running = true
	return func() tea.Msg {
		// Ensure directory structure
		dirs := []string{
			"data", "data/vectordb", "log",
			"agent_workspace/workdir",
			"agent_workspace/workdir/attachments",
			"agent_workspace/tools",
			"agent_workspace/skills",
		}
		for _, d := range dirs {
			p := filepath.Join(m.InstallDir, d)
			if err := os.MkdirAll(p, 0750); err != nil {
				m.AddOutput("WARNING: Could not create " + p + ": " + err.Error())
			}
		}
		m.AddOutput("Directory structure created")

		// Apply network config
		switch m.NetworkMode {
		case "lan":
			m.HTTPS.BindAll = true
			m.AccessURL = "http://<YOUR-IP>:8080"
		case "https":
			m.HTTPS.Enabled = true
			m.HTTPS.BindAll = true
			m.AccessURL = "https://" + m.HTTPS.Domain
		default:
			m.AccessURL = "http://localhost:8080"
		}

		// Run config-merger if available
		mergerPath := filepath.Join(m.InstallDir, "bin", "config-merger")
		if runtime.GOOS == "linux" {
			if p := mergerPath + "_linux"; fileExists(p) {
				mergerPath = p
			}
		}
		templatePath := filepath.Join(m.InstallDir, "config_template.yaml")

		if fileExists(mergerPath) && fileExists(templatePath) {
			m.AddOutput("Running config-merger...")
			cmd := exec.Command(mergerPath, "-template", templatePath, "-output", filepath.Join(m.InstallDir, "config.yaml"))
			cmd.Dir = m.InstallDir
			if out, err := cmd.CombinedOutput(); err != nil {
				m.AddOutput("WARNING: config-merger failed: " + err.Error())
				if len(out) > 0 {
					m.AddOutput(string(out))
				}
			} else {
				m.AddOutput("Configuration merged successfully")
			}
		} else {
			m.AddOutput("Config-merger not found — using existing config.yaml")
		}

		return stepDoneMsg{}
	}
}

func (m *Model) stepPassword() tea.Cmd {
	m.Running = true
	return func() tea.Msg {
		// Generate random password
		pwBytes := make([]byte, 12)
		if _, err := rand.Read(pwBytes); err != nil {
			m.AddOutput("ERROR: Could not generate password: " + err.Error())
			return stepDoneMsg{err: err}
		}
		m.Password = base64.URLEncoding.EncodeToString(pwBytes)

		// Write to file
		pwFile := filepath.Join(m.InstallDir, "firstpassword.txt")
		if err := os.WriteFile(pwFile, []byte(m.Password+"\n"), 0600); err != nil {
			m.AddOutput("WARNING: Could not write password file: " + err.Error())
		}

		// Init aurago with password (run --init-only if binary supports it)
		auragoBin := findAuragoBinary(m.InstallDir)
		if auragoBin != "" {
			args := []string{"--init-only", "-password", m.Password}
			if m.HTTPS.Enabled {
				args = append(args, "-https", "-domain", m.HTTPS.Domain, "-email", m.HTTPS.Email)
			} else if m.HTTPS.BindAll {
				args = append(args, "-host", "0.0.0.0")
			}
			cmd := exec.Command(auragoBin, args...)
			cmd.Dir = m.InstallDir
			envKey := "AURAGO_MASTER_KEY=" + m.MasterKey
			cmd.Env = append(os.Environ(), envKey)
			if out, err := cmd.CombinedOutput(); err != nil {
				m.AddOutput("WARNING: init-only failed: " + err.Error())
				if len(out) > 0 {
					m.AddOutput(strings.TrimSpace(string(out)))
				}
			} else {
				m.AddOutput("Initial configuration applied")
			}
		}

		m.AddOutput("Initial password generated")
		return stepDoneMsg{}
	}
}

func (m *Model) stepService() tea.Cmd {
	if !shared.HasSystemd() {
		m.AddOutput("systemd not available — generating start.sh instead")
		shared.GenerateStartScript(m.InstallDir)
		return func() tea.Msg { return stepDoneMsg{} }
	}

	m.InstallSvc = true
	m.ServiceForm = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Install AuraGo as a systemd service?").
				Description("Enables automatic startup on boot with security hardening.").
				Affirmative("Yes").
				Negative("No").
				Value(&m.InstallSvc),
		),
	)
	m.ServiceForm.Init()
	return nil
}

func (m *Model) stepStart() tea.Cmd {
	m.Running = true
	return func() tea.Msg {
		// Install systemd service if chosen
		if m.InstallSvc && shared.HasSystemd() {
			m.AddOutput("Installing systemd service...")
			user := shared.ServiceUser()
			auragoBin := findAuragoBinary(m.InstallDir)
			if auragoBin == "" {
				auragoBin = filepath.Join(m.InstallDir, "bin", "aurago_linux")
			}

			cfg := shared.ServiceConfig{
				User:      user,
				Group:     user,
				WorkDir:   m.InstallDir,
				ExecStart: auragoBin,
				EnvFile:   shared.MasterKeyPath(m.InstallDir),
				HTTPS:     m.HTTPS.Enabled,
			}
			if err := shared.InstallService(cfg); err != nil {
				m.AddOutput("WARNING: Service installation failed: " + err.Error())
			} else {
				m.AddOutput("Service installed and enabled")
			}

			// Set CAP_NET_BIND_SERVICE if HTTPS
			if m.HTTPS.Enabled && shared.HasCommand("setcap") {
				exec.Command("sudo", "setcap", "cap_net_bind_service=+ep", auragoBin).Run()
				m.AddOutput("Network bind capability set for HTTPS")
			}
		}

		// Start the service
		m.AddOutput("Starting AuraGo...")
		if err := shared.StartService(m.InstallDir); err != nil {
			m.AddOutput("WARNING: Could not start AuraGo: " + err.Error())
			m.AddOutput("You can start it manually later.")
		} else {
			m.AddOutput("AuraGo started successfully!")
		}

		return stepDoneMsg{}
	}
}

func (m *Model) buildSummary() {
	m.AddOutput("")
	m.AddOutput("══════════════════════════════════════════")
	m.AddOutput("  Setup Complete!")
	m.AddOutput("══════════════════════════════════════════")
	m.AddOutput("")
	m.AddOutput("  Access URL: " + m.AccessURL)
	if m.Password != "" {
		m.AddOutput("  Password:   " + m.Password)
		m.AddOutput("  (also saved in firstpassword.txt)")
	}
	m.AddOutput("")
	m.AddOutput("  Next steps:")
	m.AddOutput("  1. Open the Web UI at the URL above")
	m.AddOutput("  2. Configure your LLM provider in Settings")
	m.AddOutput("  3. Or use 'agocli' for terminal chat")
	m.AddOutput("")
	m.AddOutput("Press Enter to exit...")
}

// ── Helper functions ─────────────────────────────────────────────────

func (m *Model) resolveInstallDir() {
	exePath, err := os.Executable()
	if err == nil {
		exePath, _ = filepath.EvalSymlinks(exePath)
		// agocli sits in bin/, so go up one level
		m.InstallDir = filepath.Dir(filepath.Dir(exePath))
	}
	if m.InstallDir == "" {
		m.InstallDir, _ = os.Getwd()
	}
}

func findAuragoBinary(installDir string) string {
	candidates := []string{
		filepath.Join(installDir, "bin", "aurago_linux_"+runtime.GOARCH),
		filepath.Join(installDir, "bin", "aurago_linux"),
		filepath.Join(installDir, "bin", "aurago"),
		filepath.Join(installDir, "aurago"),
	}
	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// extractTarGz extracts a tar.gz archive.
func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip open: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		target := filepath.Join(destDir, filepath.FromSlash(hdr.Name))

		// Security: prevent path traversal
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			return fmt.Errorf("illegal path in archive: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)|0750); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
				return err
			}
			// Don't overwrite config.yaml if it already exists
			if filepath.Base(target) == "config.yaml" {
				if _, err := os.Stat(target); err == nil {
					continue
				}
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)|0640)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}

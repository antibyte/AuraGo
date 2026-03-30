package setup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbletea"
)

// Update handles messages for the setup wizard.
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
		// Advance through welcome step and start setup
		if m.CurrentStep == StepWelcome && !m.Started {
			m.Started = true
			m.NextStep()
			return m, m.runSetup()
		}
		return m, nil

	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}

// runSetup executes the setup process in a goroutine.
func (m *Model) runSetup() tea.Cmd {
	return func() tea.Msg {
		exePath, err := os.Executable()
		if err != nil {
			m.SetDone(err)
			return setupOutputMsg{Text: "failed to determine executable path: " + err.Error()}
		}
		exePath, err = filepath.EvalSymlinks(exePath)
		if err != nil {
			m.SetDone(err)
			return setupOutputMsg{Text: "failed to resolve symlinks: " + err.Error()}
		}
		installDir := filepath.Dir(exePath)

		// Check for resources.dat
		m.AddOutput("Checking for resources.dat...")
		m.CurrentStep = StepPrerequisites
		resPath := filepath.Join(installDir, "resources.dat")
		if _, err := os.Stat(resPath); os.IsNotExist(err) {
			m.SetDone(err)
			return setupOutputMsg{Text: "resources.dat not found at " + resPath}
		}

		// Extract resources.dat
		m.AddOutput("Extracting resources.dat...")
		m.CurrentStep = StepExtract
		if err := extractTarGz(resPath, installDir); err != nil {
			m.SetDone(err)
			return setupOutputMsg{Text: "failed to extract resources: " + err.Error()}
		}
		m.AddOutput("Resources extracted successfully")

		// Generate master key
		m.AddOutput("Generating master key...")
		m.CurrentStep = StepMasterKey
		if err := ensureMasterKey(installDir); err != nil {
			m.SetDone(err)
			return setupOutputMsg{Text: "failed to generate master key: " + err.Error()}
		}

		// Ensure directories exist
		m.AddOutput("Creating directory structure...")
		m.CurrentStep = StepConfig
		dirs := []string{"data", "data/vectordb", "log", "agent_workspace/workdir", "agent_workspace/workdir/attachments"}
		for _, d := range dirs {
			p := filepath.Join(installDir, d)
			if err := os.MkdirAll(p, 0750); err != nil {
				m.SetDone(err)
				return setupOutputMsg{Text: "failed to create directory: " + p}
			}
		}

		// Install service
		m.AddOutput("Installing system service...")
		m.CurrentStep = StepService
		if err := installService(exePath, installDir); err != nil {
			m.AddOutput("WARNING: Service installation failed (non-fatal): " + err.Error())
		} else {
			m.AddOutput("Service installed successfully")
		}

		// Start the server
		m.AddOutput("Starting AuraGo...")
		m.CurrentStep = StepStart
		if err := startServer(exePath, installDir); err != nil {
			m.AddOutput("WARNING: Could not start server: " + err.Error())
		} else {
			m.AddOutput("AuraGo server started")
		}

		m.AddOutput("")
		m.AddOutput("===========================================")
		m.AddOutput("Setup complete!")
		m.AddOutput("===========================================")
		m.AddOutput("")
		m.AddOutput("Next steps:")
		m.AddOutput("  1. Edit config.yaml with your LLM API key")
		m.AddOutput("  2. Access the Web UI at http://localhost:8080")
		m.AddOutput("  3. Or use 'agocli' for terminal chat")
		m.AddOutput("")
		m.AddOutput("Press Enter to exit...")

		m.SetDone(nil)
		return setupOutputMsg{Text: "done"}
	}
}

// setupOutputMsg is a message indicating setup output.
type setupOutputMsg struct {
	Text string
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

// ensureMasterKey ensures the .env file has a master key.
func ensureMasterKey(installDir string) error {
	envPath := filepath.Join(installDir, ".env")
	if _, err := os.Stat(envPath); err == nil {
		// .env exists, check for key
		data, err := os.ReadFile(envPath)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "AURAGO_MASTER_KEY=") {
			return nil // Key exists
		}
	}

	// Generate new key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("failed to generate random key: %w", err)
	}
	hexKey := hex.EncodeToString(key)
	content := fmt.Sprintf("AURAGO_MASTER_KEY=%s\n", hexKey)
	return os.WriteFile(envPath, []byte(content), 0600)
}

// installService installs the system service.
func installService(exePath, installDir string) error {
	masterKey := readEnvKey(filepath.Join(installDir, ".env"), "AURAGO_MASTER_KEY")

	user := os.Getenv("SUDO_USER")
	if user == "" {
		user = os.Getenv("USER")
	}
	if user == "" {
		user = "root"
	}

	unit := fmt.Sprintf(`[Unit]
Description=AuraGo AI Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=%s
Group=%s
WorkingDirectory=%s
ExecStart=%s
Restart=on-failure
RestartSec=10
EnvironmentFile=-%s/.env
Environment="AURAGO_MASTER_KEY=%s"

[Install]
WantedBy=multi-user.target
`, user, user, installDir, exePath, installDir, masterKey)

	unitPath := "/etc/systemd/system/aurago.service"
	if err := os.WriteFile(unitPath, []byte(unit), 0600); err != nil {
		return fmt.Errorf("failed to write systemd unit: %w", err)
	}

	for _, cmd := range [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", "aurago.service"},
	} {
		if out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput(); err != nil {
			// Log but don't fail - this might be running without systemd
			_ = string(out)
		}
	}
	return nil
}

// readEnvKey reads a key from a .env file.
func readEnvKey(envPath, key string) string {
	data, err := os.ReadFile(envPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+"=") {
			return strings.TrimPrefix(line, key+"=")
		}
	}
	return ""
}

// startServer starts the AuraGo server.
func startServer(exePath, installDir string) error {
	// Start aurago in background
	cmd := exec.Command(exePath)
	cmd.Dir = installDir
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = nil

	return cmd.Start()
}

// setupLogger is a logger that captures output for TUI display.
type setupLogger struct {
	model *Model
}

func (l *setupLogger) Info(msg string) {
	l.model.AddOutput(msg)
}

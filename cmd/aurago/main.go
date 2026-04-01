package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"aurago/internal/config"
	"aurago/internal/contacts"
	"aurago/internal/credentials"
	"aurago/internal/invasion"
	"aurago/internal/invasion/bridge"
	"aurago/internal/inventory"
	"aurago/internal/llm"
	"aurago/internal/logger"
	"aurago/internal/memory"
	promptspkg "aurago/internal/prompts"
	"aurago/internal/push"
	"aurago/internal/remote"
	"aurago/internal/sandbox"
	"aurago/internal/security"
	"aurago/internal/server"
	"aurago/internal/setup"
	"aurago/internal/sqlconnections"
	"aurago/internal/tools"

	"github.com/gofrs/flock"
)

// cronHTTPClient is used for cron loopback requests with a bounded timeout.
var cronHTTPClient = &http.Client{Timeout: 2 * time.Minute}

func main() {
	// ── Sandbox helper mode ──────────────────────────────────────────────
	// When invoked with --sandbox-exec, this process applies Landlock + rlimits
	// and exec's the shell command. Must happen before ANY other initialization.
	if len(os.Args) > 2 && os.Args[1] == "--sandbox-exec" {
		sandbox.RunHelper(os.Args[2])
		os.Exit(126) // only reached if RunHelper's exec fails
	}
	if len(os.Args) > 1 && os.Args[1] == "--sandbox-exec-bin" {
		sandbox.RunExecHelper()
		os.Exit(126)
	}

	var debug bool
	var runSetup bool
	var initOnly bool
	var checkConfig bool
	var configFile string
	var recoveryContext string
	var enableHTTPS bool
	var httpsDomain string
	var httpsEmail string
	var initialPassword string
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.BoolVar(&runSetup, "setup", false, "Extract resources.dat, install service, and exit")
	flag.BoolVar(&initOnly, "init-only", false, "Apply -password/-https flags to config/vault, then exit immediately (used by installer)")
	flag.BoolVar(&checkConfig, "check-config", false, "Validate config file syntax and exit (used by Docker entrypoint)")
	flag.StringVar(&configFile, "config", "config.yaml", "Path to config file (default: config.yaml)")
	flag.StringVar(&recoveryContext, "recovery-context", "", "Recovery context after maintenance (Base64)")
	flag.BoolVar(&enableHTTPS, "https", false, "Enable HTTPS (Let's Encrypt) and update config")
	flag.StringVar(&httpsDomain, "domain", "", "Domain for Let's Encrypt")
	flag.StringVar(&httpsEmail, "email", "", "Email for Let's Encrypt")
	flag.StringVar(&initialPassword, "password", "", "Set initial login password (hashes and stores in vault)")
	flag.Parse()

	appLog := logger.Setup(debug)
	slog.SetDefault(appLog)
	webAccessLog := appLog.With("component", "web-access")

	// Load secrets in priority order — each step only sets vars not already present:
	//   1. systemd EnvironmentFile (already in env before process starts)
	//   2. Docker Compose secret  (/run/secrets/aurago_master_key)
	//   3. System credential file (/etc/aurago/master.key)  ← manual starts post-migration
	//   4. Local .env             ($configDir/.env)          ← dev / non-systemd installs
	loadDockerSecret("/run/secrets/aurago_master_key", "AURAGO_MASTER_KEY", appLog)
	loadDotEnv("/etc/aurago/master.key", appLog)
	loadDotEnv(filepath.Join(filepath.Dir(configFile), ".env"), appLog)

	// ── Config-check mode: validate YAML and exit (used by Docker entrypoint) ─
	if checkConfig {
		if _, err := config.Load(configFile); err != nil {
			fmt.Fprintf(os.Stderr, "CONFIG ERROR: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("OK")
		os.Exit(0)
	}

	// ── Early Config Load for Path Resolution ────────────────────────────
	cfg, err := config.Load(configFile)
	if err != nil && !runSetup {
		// If we can't load config and we're not in setup, we can't safely proceed
		log.Fatalf("❌ CONFIG ERROR: %v", err)
	}

	// ── Apply CLI flags for HTTPS ──────────────────────────────────────────
	if enableHTTPS && cfg != nil {
		saveNeeded := false
		if !cfg.Server.HTTPS.Enabled {
			cfg.Server.HTTPS.Enabled = true
			saveNeeded = true
		}
		if httpsDomain != "" && cfg.Server.HTTPS.Domain != httpsDomain {
			cfg.Server.HTTPS.Domain = httpsDomain
			saveNeeded = true
		}
		if httpsEmail != "" && cfg.Server.HTTPS.Email != httpsEmail {
			cfg.Server.HTTPS.Email = httpsEmail
			saveNeeded = true
		}
		if cfg.Server.Host != "0.0.0.0" {
			cfg.Server.Host = "0.0.0.0"
			saveNeeded = true
		}
		if saveNeeded {
			appLog.Info("Updating config.yaml with HTTPS settings from CLI flags")
			if err := cfg.Save(configFile); err != nil {
				appLog.Error("Failed to save config with HTTPS settings", "error", err)
			}
		}
	}

	// ── Apply initial password ───────────────────────────────────────────
	if initialPassword != "" && cfg != nil {
		masterKey := os.Getenv("AURAGO_MASTER_KEY")
		if masterKey != "" && len(masterKey) == 64 {
			vaultPath := filepath.Join(cfg.Directories.DataDir, "vault.bin")
			if v, err := security.NewVault(masterKey, vaultPath); err == nil {
				hash, err := server.HashPassword(initialPassword)
				if err == nil {
					_ = v.WriteSecret("auth_password_hash", hash)
					appLog.Info("Initial password hash stored in vault")

					// Setup session secret if not exists
					if sec, _ := v.ReadSecret("auth_session_secret"); sec == "" {
						if newSec, e := server.GenerateRandomHex(32); e == nil {
							_ = v.WriteSecret("auth_session_secret", newSec)
						}
					}

					if !cfg.Auth.Enabled {
						cfg.Auth.Enabled = true
						if err := cfg.Save(configFile); err != nil {
							appLog.Error("Failed to enable auth in config.yaml", "error", err)
						}
					}
				} else {
					appLog.Error("Failed to hash initial password", "error", err)
				}
			} else {
				appLog.Error("Failed to open vault for password setup", "error", err)
			}
		} else {
			appLog.Warn("AURAGO_MASTER_KEY missing or invalid to set initial password")
		}
	}

	// ── Init-only mode: apply flags and exit without starting the server ──
	// Used by the installer to set the initial password / HTTPS config.
	if initOnly {
		appLog.Info("Init-only mode: configuration applied, exiting.")
		os.Exit(0)
	}

	// ── Robust File Locking ──────────────────────────────────────────────
	var lockPath string
	if cfg != nil && cfg.Directories.DataDir != "" {
		lockPath = filepath.Join(cfg.Directories.DataDir, "aurago.lock")
		_ = os.MkdirAll(cfg.Directories.DataDir, 0755)
	} else {
		lockPath = "aurago.lock"
	}

	absLockPath, _ := filepath.Abs(lockPath)
	appLog.Info("Checking application lock", "path", absLockPath)

	fileLock := flock.New(absLockPath)
	locked, err := fileLock.TryLock()
	if err != nil || !locked {
		appLog.Error("❌ BLOCKIERT: AuraGo läuft bereits!", "lock_path", absLockPath)
		os.Exit(1)
	}
	defer fileLock.Unlock()
	appLog.Info("Application lock acquired", "path", absLockPath)

	// ── Setup mode: extract resources and install service ────────────────
	if runSetup {
		appLog.Info("Running AuraGo first-time setup ...")
		if err := setup.Run(appLog); err != nil {
			appLog.Error("Setup failed", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// ── Auto-detect missing resources ────────────────────────────────────
	exePath, _ := os.Executable()
	installDir := filepath.Dir(exePath)
	if setup.NeedsSetup(installDir) {
		resPath := filepath.Join(installDir, "resources.dat")
		if _, err := os.Stat(resPath); err == nil {
			appLog.Warn("Essential files missing — running automatic setup from resources.dat")
			if err := setup.Run(appLog); err != nil {
				appLog.Error("Auto-setup failed", "error", err)
				os.Exit(1)
			}
			appLog.Info("Auto-setup complete, continuing startup ...")
		}
	}

	appLog.Info("Starting AuraGo")

	// ── Bootstrap embedded prompt defaults if PromptsDir is empty ────────
	promptspkg.EnsurePromptsDir(cfg.Directories.PromptsDir, appLog)

	// Configure execution timeouts from config
	tools.ConfigureTimeouts(cfg.Tools.PythonTimeoutSeconds, cfg.Tools.SkillTimeoutSeconds, cfg.Tools.BackgroundTimeoutSeconds)

	// Maintenance lock setup (uses DataDir)
	tools.SetBusyFilePath(filepath.Join(cfg.Directories.DataDir, "maintenance.lock"))
	// Clean up stale maintenance lock from previous unclean shutdown
	if tools.IsBusy() {
		appLog.Warn("Stale maintenance lock detected at startup — clearing")
		tools.SetBusy(false)
	}
	appLog.Info("Maintenance lock path initialized", "path", tools.GetBusyFilePath())
	appLog.Info("Current Maintenance Status", "is_busy", tools.IsBusy())

	// Phase 82: Re-initialize logger with file support if enabled
	if cfg.Logging.EnableFileLog {
		logPath := filepath.Join(cfg.Logging.LogDir, "aurago.log")
		if lf, err := logger.SetupWithFile(debug, logPath, false); err == nil {
			appLog = lf.Logger
			slog.SetDefault(appLog)
			defer lf.Close()
			appLog.Info("File logging enabled", "path", logPath)
		} else {
			appLog.Warn("Failed to setup file logging", "error", err)
		}

		webAccessLogPath := filepath.Join(cfg.Logging.LogDir, "web_access.log")
		if lf, err := logger.SetupFileOnly(debug, webAccessLogPath, false); err == nil {
			webAccessLog = lf.Logger
			defer lf.Close()
			appLog.Info("Web UI access logging enabled", "path", webAccessLogPath)
		} else {
			appLog.Warn("Failed to setup Web UI access log", "error", err)
		}
	}

	dirs := []string{
		cfg.Directories.DataDir,
		cfg.Directories.WorkspaceDir,
		cfg.Directories.ToolsDir,
		cfg.Directories.PromptsDir,
		cfg.Directories.SkillsDir,
		cfg.Directories.VectorDBDir,
		cfg.Logging.LogDir,
		cfg.Tools.DocumentCreator.OutputDir,
	}

	appLog.Debug("Resolved absolute paths",
		"data_dir", cfg.Directories.DataDir,
		"workspace_dir", cfg.Directories.WorkspaceDir,
		"tools_dir", cfg.Directories.ToolsDir,
		"prompts_dir", cfg.Directories.PromptsDir,
		"skills_dir", cfg.Directories.SkillsDir,
		"vectordb_dir", cfg.Directories.VectorDBDir,
	)

	for _, dir := range dirs {
		if dir != "" {
			if err := os.MkdirAll(dir, 0750); err != nil {
				appLog.Error("Failed to create directory", "dir", dir, "error", err)
				os.Exit(1)
			}
		}
	}

	venvDir := filepath.Join(cfg.Directories.WorkspaceDir, "venv")
	venvPython := tools.GetPythonBin(cfg.Directories.WorkspaceDir)
	if _, err := os.Stat(venvPython); os.IsNotExist(err) {
		appLog.Info("Creating Python virtual environment...", "dir", venvDir)

		// Try 'python3' first on Linux/macOS, then 'python'
		pythonExe := "python"
		if runtime.GOOS != "windows" {
			if _, err := exec.LookPath("python3"); err == nil {
				pythonExe = "python3"
			}
		}

		cmd := exec.Command(pythonExe, "-m", "venv", venvDir)
		if err := cmd.Run(); err != nil {
			appLog.Error("Failed to create virtual environment", "error", err, "python", pythonExe)
			os.Exit(1)
		}
		appLog.Info("Virtual environment created successfully.", "python", pythonExe)
	}

	// Phase 26.1: Provision skill dependencies into the venv in the background
	// so the server starts immediately without waiting for pip.
	go tools.ProvisionSkillDependencies(cfg.Directories.SkillsDir, cfg.Directories.WorkspaceDir, appLog)

	shortTermMem, err := memory.NewSQLiteMemory(cfg.SQLite.ShortTermPath, appLog)
	if err != nil {
		appLog.Error("Failed to initialize Short-Term memory", "error", err)
		os.Exit(1)
	}
	defer shortTermMem.Close()

	if _, err := server.ApplyPendingEmbeddingsReset(cfg, shortTermMem, appLog); err != nil {
		appLog.Error("Failed to apply pending embeddings reset", "error", err)
		os.Exit(1)
	}

	// Migrate core_memory.md → SQLite (no-op if already done); returns true on first start
	isFirstStart := shortTermMem.MigrateCoreMemoryFromMarkdown(cfg.Directories.DataDir, appLog)

	inventoryDB, err := inventory.InitDB(cfg.SQLite.InventoryPath)
	if err != nil {
		appLog.Error("Failed to initialize Inventory DB", "error", err)
		os.Exit(1)
	}
	defer inventoryDB.Close()
	if err := credentials.EnsureSchema(inventoryDB); err != nil {
		appLog.Error("Failed to initialize Credentials schema", "error", err)
		os.Exit(1)
	}

	// Invasion Control DB (nests & eggs) — always initialized so the UI works
	// after binary updates even if the server's config.yaml predates the feature.
	// The agent tool still respects InvasionControl.Enabled for prompt inclusion.
	invasionDB, invasionDBErr := invasion.InitDB(cfg.SQLite.InvasionPath)
	if invasionDBErr != nil {
		appLog.Warn("Failed to initialize Invasion Control DB; feature disabled", "error", invasionDBErr, "path", cfg.SQLite.InvasionPath)
		invasionDB = nil
	} else {
		defer invasionDB.Close()
		appLog.Info("Invasion Control DB initialized", "path", cfg.SQLite.InvasionPath)
		cfg.InvasionControl.Enabled = true
	}

	// Cheat Sheets DB
	cheatsheetDB, cheatsheetDBErr := tools.InitCheatsheetDB(cfg.SQLite.CheatsheetPath)
	if cheatsheetDBErr != nil {
		appLog.Warn("Failed to initialize Cheatsheet DB", "error", cheatsheetDBErr, "path", cfg.SQLite.CheatsheetPath)
		cheatsheetDB = nil
	} else {
		defer cheatsheetDB.Close()
		appLog.Info("Cheatsheet DB initialized", "path", cfg.SQLite.CheatsheetPath)
	}

	// Image Gallery DB
	imageGalleryDB, imageGalleryDBErr := tools.InitImageGalleryDB(cfg.SQLite.ImageGalleryPath)
	if imageGalleryDBErr != nil {
		appLog.Warn("Failed to initialize Image Gallery DB", "error", imageGalleryDBErr, "path", cfg.SQLite.ImageGalleryPath)
		imageGalleryDB = nil
	} else {
		defer imageGalleryDB.Close()
		appLog.Info("Image Gallery DB initialized", "path", cfg.SQLite.ImageGalleryPath)
	}

	// Remote Control DB
	var remoteControlDB *sql.DB
	if cfg.SQLite.RemoteControlPath != "" {
		var rcErr error
		remoteControlDB, rcErr = remote.InitDB(cfg.SQLite.RemoteControlPath)
		if rcErr != nil {
			appLog.Warn("Failed to initialize Remote Control DB; feature disabled", "error", rcErr, "path", cfg.SQLite.RemoteControlPath)
			remoteControlDB = nil
		} else {
			defer remoteControlDB.Close()
			appLog.Info("Remote Control DB initialized", "path", cfg.SQLite.RemoteControlPath)
		}
	}

	// Media Registry DB
	mediaRegistryDB, mediaRegistryDBErr := tools.InitMediaRegistryDB(cfg.SQLite.MediaRegistryPath)
	if mediaRegistryDBErr != nil {
		appLog.Warn("Failed to initialize Media Registry DB", "error", mediaRegistryDBErr, "path", cfg.SQLite.MediaRegistryPath)
		mediaRegistryDB = nil
	} else {
		defer mediaRegistryDB.Close()
		appLog.Info("Media Registry DB initialized", "path", cfg.SQLite.MediaRegistryPath)
	}

	// Seed the media registry with sample files bundled in resources.dat.
	// Runs on every start but is idempotent: existing files and DB entries are skipped.
	if mediaRegistryDB != nil {
		tools.SeedWelcomeMedia(mediaRegistryDB, cfg.Directories.DataDir, installDir, appLog)
	}

	// Homepage Registry DB
	homepageRegistryDB, homepageRegistryDBErr := tools.InitHomepageRegistryDB(cfg.SQLite.HomepageRegistryPath)
	if homepageRegistryDBErr != nil {
		appLog.Warn("Failed to initialize Homepage Registry DB", "error", homepageRegistryDBErr, "path", cfg.SQLite.HomepageRegistryPath)
		homepageRegistryDB = nil
	} else {
		defer homepageRegistryDB.Close()
		appLog.Info("Homepage Registry DB initialized", "path", cfg.SQLite.HomepageRegistryPath)
	}

	// Contacts (Address Book) DB
	contactsDB, contactsDBErr := contacts.InitDB(cfg.SQLite.ContactsPath)
	if contactsDBErr != nil {
		appLog.Warn("Failed to initialize Contacts DB", "error", contactsDBErr, "path", cfg.SQLite.ContactsPath)
		contactsDB = nil
	} else {
		defer contactsDB.Close()
		appLog.Info("Contacts DB initialized", "path", cfg.SQLite.ContactsPath)
	}

	var sqlConnectionsDB *sql.DB
	var sqlConnectionPool *sqlconnections.ConnectionPool
	// Always initialize the metadata DB so the UI can manage connection configs
	// regardless of whether the feature is enabled. The pool (external DB connections)
	// is only created when explicitly enabled.
	{
		var scErr error
		sqlConnectionsDB, scErr = sqlconnections.InitDB(cfg.SQLite.SQLConnectionsPath)
		if scErr != nil {
			appLog.Warn("Failed to initialize SQL Connections DB; feature disabled", "error", scErr, "path", cfg.SQLite.SQLConnectionsPath)
			sqlConnectionsDB = nil
		} else {
			defer sqlConnectionsDB.Close()
			appLog.Info("SQL Connections DB initialized", "path", cfg.SQLite.SQLConnectionsPath)
		}
	}

	masterKey := os.Getenv("AURAGO_MASTER_KEY")
	if masterKey == "" || len(masterKey) != 64 {
		appLog.Error("CRITICAL: AURAGO_MASTER_KEY environment variable is missing or not exactly 64 hex characters (32 bytes). Refusing to start.")
		os.Exit(1)
	}
	// Ensure the master key is never leaked through any outgoing communication channel.
	security.RegisterSensitive(masterKey)

	vaultPath := filepath.Join(cfg.Directories.DataDir, "vault.bin")
	vault, err := security.NewVault(masterKey, vaultPath)
	if err != nil {
		appLog.Error("Failed to initialize security vault", "error", err)
		os.Exit(1)
	}

	// Initialize SQL Connections pool now that vault is available.
	// Pool is only created when the feature is explicitly enabled — it manages
	// live connections to external databases, which should be opt-in.
	if sqlConnectionsDB != nil && cfg.SQLConnections.Enabled {
		sqlConnectionPool = sqlconnections.NewConnectionPool(
			sqlConnectionsDB, vault,
			cfg.SQLConnections.MaxPoolSize,
			cfg.SQLConnections.ConnectionTimeoutSec,
			appLog,
		)
		defer sqlConnectionPool.CloseAll()
	}

	// One-time migration: auth secrets (password_hash, session_secret) may have been
	// stored in config.yaml by older versions. Move them to the encrypted vault so that
	// they are no longer kept in plaintext on disk.
	config.MigrateAuthSecretsToVault(configFile, vault, appLog)
	config.MigrateEggModeSharedKeyToVault(configFile, vault, appLog)

	// Apply all vault-stored secrets into the runtime config, then re-resolve
	// provider references so that API keys propagate to the LLM/Vision/etc. slots.
	cfg.ApplyVaultSecrets(vault)
	cfg.ResolveProviders()

	// Apply OAuth2 access tokens from vault into provider API keys
	cfg.ApplyOAuthTokens(vault)

	// Web Push (PWA notifications) — init after vault so VAPID keys can be stored/loaded
	if _, err := push.NewManager(cfg.Directories.DataDir, vault, appLog); err != nil {
		appLog.Warn("Web Push manager initialization failed — push notifications disabled", "error", err)
	}

	// Initialize Long-Term memory (VectorDB) after vault secrets are applied
	// so that the embedding provider API key is available.
	longTermMem, err := memory.NewChromemVectorDB(cfg, appLog)
	if err != nil {
		appLog.Error("Failed to initialize Long-Term memory (VectorDB)", "error", err)
		os.Exit(1)
	}

	// Tool guide indexing (async at startup for faster boot)
	toolGuidesDir := filepath.Join(cfg.Directories.PromptsDir, "tools_manuals")
	longTermMem.IndexToolGuidesAsync(toolGuidesDir, false)

	// Documentation indexing (async RAG)
	docDir := filepath.Join(filepath.Dir(cfg.ConfigPath), "documentation")
	if _, err := os.Stat(docDir); err == nil {
		longTermMem.IndexDirectoryAsync(docDir, "documentation", shortTermMem, false)
	} else {
		appLog.Debug("Documentation directory not found, skipping indexing", "path", docDir)
	}

	// Detect runtime environment capabilities (Docker, socket, broadcast, firewall)
	cfg.Runtime = config.DetectRuntime(appLog)

	llmClient := llm.NewFailoverManager(cfg, appLog)

	// Auto-detect context window and configure token budget
	if cfg.Agent.ContextWindow == 0 {
		detected := llm.DetectContextWindow(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.ProviderType, appLog)
		if detected > 0 {
			cfg.Agent.SystemPromptTokenBudget, cfg.Agent.ContextWindow = llm.AutoConfigureBudget(detected, cfg.Agent.SystemPromptTokenBudget, appLog)
		}
	}

	// Process Registry for background daemon management
	registry := tools.NewProcessRegistry(appLog)

	backgroundTaskManager := tools.NewBackgroundTaskManager(cfg.Directories.DataDir, appLog)
	backgroundTaskManager.SetProcessRegistry(registry)
	backgroundTaskManager.SetNotifier(func(title, body string) {
		if shortTermMem == nil {
			return
		}
		msg := title
		if strings.TrimSpace(body) != "" {
			msg += ": " + body
		}
		if err := shortTermMem.AddNotification(msg); err != nil {
			appLog.Warn("Failed to store background task notification", "error", err)
		}
	})
	backgroundTaskManager.SetLoopbackExecutor(func(prompt string, timeout time.Duration) error {
		url := server.InternalAPIURL(cfg) + "/v1/chat/completions"
		payload := map[string]interface{}{
			"model":  "aurago",
			"stream": false,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal background loopback payload: %w", err)
		}
		client := &http.Client{Timeout: timeout}
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			return fmt.Errorf("create background loopback request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-FollowUp", "true")
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("execute background loopback request: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("background loopback returned %d: %s", resp.StatusCode, string(body))
		}
		return nil
	})
	backgroundTaskManager.Start()
	tools.SetDefaultBackgroundTaskManager(backgroundTaskManager)

	// Shell Sandbox (Landlock + rlimits on Linux)
	{
		var allowedPaths []sandbox.PathRule
		for _, p := range cfg.ShellSandbox.AllowedPaths {
			allowedPaths = append(allowedPaths, sandbox.PathRule{Path: p.Path, ReadOnly: p.ReadOnly})
		}

		// Automatically grant read-write access to the agent's output directories
		// so that shell commands and Python scripts can write files that are then
		// served via the web UI (/files/documents/, /files/audio/, etc.).
		// We deliberately do NOT expose the entire data/ dir — vault.bin and the
		// SQLite databases must remain inaccessible from within the sandbox.
		dataDir := cfg.Directories.DataDir
		for _, subDir := range []string{"documents", "audio", "generated_images", "tts"} {
			allowedPaths = append(allowedPaths, sandbox.PathRule{
				Path:     filepath.Join(dataDir, subDir),
				ReadOnly: false,
			})
		}

		// Grant read-only access to the agent's tools and skills directories so
		// that saved tools (save_tool) and skills can be executed via execute_shell.
		// These directories are outside workspaceDir (workdir) and would otherwise
		// be blocked by Landlock — causing "No such file or directory" errors even
		// though the files actually exist.
		if cfg.Directories.ToolsDir != "" {
			allowedPaths = append(allowedPaths, sandbox.PathRule{
				Path:     cfg.Directories.ToolsDir,
				ReadOnly: true,
			})
		}
		if cfg.Directories.SkillsDir != "" {
			allowedPaths = append(allowedPaths, sandbox.PathRule{
				Path:     cfg.Directories.SkillsDir,
				ReadOnly: true,
			})
		}

		// If Docker is enabled, grant the sandbox access to the Docker socket so
		// that docker CLI commands (docker ps, docker images, etc.) work inside
		// the sandbox.  The socket path is a Unix domain socket — Landlock's
		// LANDLOCK_ACCESS_FS_MAKE_SOCK right controls access to it, so the socket
		// path must be explicitly added to the ruleset.
		// We also inject DOCKER_HOST so the docker CLI can find the socket even
		// with the stripped minimal environment built by buildEnv().
		var extraEnv []string
		if cfg.Docker.Enabled {
			dockerHost := cfg.Docker.Host
			if dockerHost == "" {
				dockerHost = "unix:///var/run/docker.sock"
			}
			// Pass DOCKER_HOST into the sandboxed process environment
			extraEnv = append(extraEnv, "DOCKER_HOST="+dockerHost)
			// Add the Unix socket file itself to the allowed-path ruleset
			if strings.HasPrefix(dockerHost, "unix://") {
				socketPath := strings.TrimPrefix(dockerHost, "unix://")
				allowedPaths = append(allowedPaths, sandbox.PathRule{
					Path:     socketPath,
					ReadOnly: false,
				})
			}
		}

		sandbox.Init(sandbox.ShellSandboxConfig{
			Enabled:       cfg.ShellSandbox.Enabled,
			MaxMemoryMB:   cfg.ShellSandbox.MaxMemoryMB,
			MaxCPUSeconds: cfg.ShellSandbox.MaxCPUSeconds,
			MaxProcesses:  cfg.ShellSandbox.MaxProcesses,
			MaxFileSizeMB: cfg.ShellSandbox.MaxFileSizeMB,
			AllowedPaths:  allowedPaths,
			ExtraEnv:      extraEnv,
		}, cfg.Directories.WorkspaceDir, appLog)
	}

	// Cron Manager for autonomous triggers
	cronManager := tools.NewCronManager(cfg.Directories.DataDir)
	err = cronManager.Start(func(prompt string) {
		appLog.Info("Scheduling autonomous cron task", "prompt", prompt)
		if backgroundTaskManager == nil {
			appLog.Error("Background task manager unavailable for cron task")
			return
		}
		cronPrompt := fmt.Sprintf("[SYSTEM CRON TRIGGER] It is time to execute the following scheduled task: %s", prompt)
		if _, scheduleErr := backgroundTaskManager.ScheduleCronPrompt(cronPrompt, tools.BackgroundTaskScheduleOptions{
			Source:      "cron",
			Description: "Scheduled cron task",
			MaxRetries:  cfg.Agent.BackgroundTasks.MaxRetries,
			RetryDelay:  time.Duration(cfg.Agent.BackgroundTasks.RetryDelaySeconds) * time.Second,
			Timeout:     time.Duration(cfg.Agent.BackgroundTasks.HTTPTimeoutSeconds) * time.Second,
		}); scheduleErr != nil {
			appLog.Error("Failed to schedule cron task", "error", scheduleErr, "prompt", prompt)
		}
	})
	if err != nil {
		appLog.Error("Failed to load crontab", "error", err)
	}

	// Graceful shutdown: kill all background processes on SIGINT/SIGTERM
	shutdownCh := setupGracefulShutdown(appLog, registry, llmClient)
	go func() {
		<-shutdownCh
		backgroundTaskManager.Stop()
	}()

	// History Manager for persistent conversational memory array
	historyManager := memory.NewHistoryManager(filepath.Join(cfg.Directories.DataDir, "chat_history.json"))
	defer historyManager.Close()

	// Phase 36: Native Knowledge Graph (SQLite-backed with FTS5)
	kg, err := memory.NewKnowledgeGraph(
		filepath.Join(cfg.Directories.DataDir, "knowledge_graph.db"),
		filepath.Join(cfg.Directories.DataDir, "graph.json"),
		appLog,
	)
	if err != nil {
		appLog.Error("Failed to initialize knowledge graph", "error", err)
		return
	}
	defer kg.Close()

	// Handle Recovery Context
	if recoveryContext != "" {
		decoded, err := base64.StdEncoding.DecodeString(recoveryContext)
		if err != nil {
			appLog.Error("Failed to decode recovery context", "error", err)
		} else {
			msg := fmt.Sprintf("SYSTEM: Neustart nach Wartung abgeschlossen. Zusammenfassung der Änderungen: %s. Setze deinen Plan fort.", string(decoded))
			mid, _ := shortTermMem.InsertMessage("default", "system", msg, false, false)
			historyManager.Add("system", msg, mid, false, false)
			appLog.Info("Recovery context injected into history")
		}
	}

	// Start Lifeboat Sidecar if enabled
	if cfg.Maintenance.LifeboatEnabled {
		startLifeboatSidecar(appLog, cfg)
	}

	// ── Egg Mode: start WebSocket client to master ──
	if cfg.EggMode.Enabled {
		appLog.Info("Egg mode enabled — connecting to master", "master_url", cfg.EggMode.MasterURL)
		eggClient := bridge.NewEggClient(
			cfg.EggMode.MasterURL,
			cfg.EggMode.EggID,
			cfg.EggMode.NestID,
			cfg.EggMode.SharedKey,
			"1.0.0",
			appLog,
		)
		eggClient.TLSSkipVerify = cfg.EggMode.TLSSkipVerify
		eggClient.OnTask = func(task bridge.TaskPayload) {
			appLog.Info("Task received from master", "task_id", task.TaskID, "desc", task.Description)
			// Execute task via loopback to local agent API
			eggPort := cfg.Server.Port
			if eggPort == 0 {
				eggPort = 8099
			}
			taskMsg := map[string]interface{}{
				"model": cfg.LLM.Model,
				"messages": []map[string]string{
					{"role": "user", "content": fmt.Sprintf("[MASTER TASK %s] %s", task.TaskID, task.Description)},
				},
			}
			payload, _ := json.Marshal(taskMsg)
			url := server.InternalAPIURL(cfg) + "/v1/chat/completions"
			req, reqErr := http.NewRequest("POST", url, bytes.NewBuffer(payload))
			result := bridge.ResultPayload{TaskID: task.TaskID}
			if reqErr != nil {
				result.Status = "failure"
				result.Error = reqErr.Error()
			} else {
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Internal-FollowUp", "true")
				resp, err := cronHTTPClient.Do(req)
				if err != nil {
					result.Status = "failure"
					result.Error = err.Error()
				} else {
					defer resp.Body.Close()
					body, _ := io.ReadAll(resp.Body)
					if resp.StatusCode == 200 {
						result.Status = "success"
						result.Output = string(body)
					} else {
						result.Status = "failure"
						result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
					}
				}
			}
			if err := eggClient.SendResult(result); err != nil {
				appLog.Error("Failed to send task result to master", "error", err)
			}
		}
		eggClient.OnSecret = func(secret bridge.SecretPayload) {
			appLog.Info("Secret received from master", "key", secret.Key)
			// Decrypt the value with the shared key, then store in local vault
			plaintext, err := bridge.DecryptWithSharedKey(secret.EncryptedValue, cfg.EggMode.SharedKey)
			if err != nil {
				appLog.Error("Failed to decrypt received secret", "key", secret.Key, "error", err)
				return
			}
			if err := vault.WriteSecret(secret.Key, string(plaintext)); err != nil {
				appLog.Error("Failed to store received secret", "key", secret.Key, "error", err)
			}
		}
		eggClient.OnStop = func() {
			appLog.Warn("Stop command from master — initiating shutdown")
			close(shutdownCh)
		}
		go eggClient.Start()
	}

	// Startup security audit — log warnings for any critical/warning issues
	// so admins notice them immediately in the log even before opening the UI.
	if secHints := server.CheckSecurity(cfg); len(secHints) > 0 {
		critCount := 0
		for _, h := range secHints {
			if h.Severity == server.SevCritical {
				critCount++
			}
		}
		if critCount > 0 {
			appLog.Warn("[Security] CRITICAL security issues detected — open /config to review and fix",
				"critical", critCount, "total", len(secHints))
		} else {
			appLog.Warn("[Security] Security recommendations detected — open /config to review",
				"total", len(secHints))
		}
		for _, h := range secHints {
			appLog.Warn("[Security] "+h.Title, "id", h.ID, "severity", h.Severity)
		}
	}

	if err := server.Start(cfg, appLog, webAccessLog, llmClient, shortTermMem, longTermMem, vault, registry, cronManager, historyManager, kg, inventoryDB, invasionDB, cheatsheetDB, imageGalleryDB, remoteControlDB, mediaRegistryDB, homepageRegistryDB, contactsDB, sqlConnectionsDB, sqlConnectionPool, backgroundTaskManager, isFirstStart, shutdownCh); err != nil {
		appLog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

func setupGracefulShutdown(log *slog.Logger, registry *tools.ProcessRegistry, llmClient llm.ChatClient) chan struct{} {
	done := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Warn("Received shutdown signal, cleaning up...", "signal", sig)
		registry.KillAll()
		// Stop the LLM failover probe goroutine cleanly
		if fm, ok := llmClient.(*llm.FailoverManager); ok {
			fm.Stop()
		}
		close(done)
	}()
	return done
}

func loadDotEnv(path string, log *slog.Logger) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // Ignore if file doesn't exist
	}
	log.Info("Loading environment from credential file", "path", path)
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || bytes.HasPrefix(line, []byte("#")) {
			continue
		}
		parts := bytes.SplitN(line, []byte("="), 2)
		if len(parts) == 2 {
			key := string(bytes.TrimSpace(parts[0]))
			val := string(bytes.TrimSpace(parts[1]))
			// Remove quotes if present
			val = strings.Trim(val, `"'`)
			// Only set if not already provided by a higher-priority source
			// (systemd EnvironmentFile, Docker secret, or earlier file in the chain).
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

// loadDockerSecret reads a Docker secret file (mounted at /run/secrets/) and
// sets the given environment variable if it is not already set. This allows
// Docker Compose file-based secrets to inject credentials without .env files
// or plaintext environment variables visible in `docker inspect`.
func loadDockerSecret(path, envVar string, log *slog.Logger) {
	if os.Getenv(envVar) != "" {
		return // Already set via env — don't override
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return // Secret file doesn't exist — not a Docker deployment
	}
	val := strings.TrimSpace(string(data))
	if val == "" {
		return
	}
	os.Setenv(envVar, val)
	log.Info("Loaded secret from Docker secret file", "env", envVar, "path", path)
}

func startLifeboatSidecar(log *slog.Logger, cfg *config.Config) {
	// Candidate paths in priority order:
	//   1. ./lifeboat         – Docker image layout (/app/lifeboat next to /app/aurago)
	//   2. ./bin/lifeboat     – native Linux install via install.sh
	//   3. ./bin/lifeboat_linux – pre-built binary distributed in the repo
	//   4. ./bin/lifeboat.exe – Windows
	var lifeboatPath string
	if runtime.GOOS == "windows" {
		lifeboatPath = "./bin/lifeboat.exe"
	} else {
		candidates := []string{"./lifeboat", "./bin/lifeboat", "./bin/lifeboat_linux"}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				lifeboatPath = c
				break
			}
		}
		if lifeboatPath == "" {
			lifeboatPath = "./bin/lifeboat_linux" // use last candidate for the warning message
		}
	}

	if _, err := os.Stat(lifeboatPath); os.IsNotExist(err) {
		log.Warn("Lifeboat binary not found, sidecar not started", "path", lifeboatPath)
		return
	}

	log.Info("Starting Lifeboat Sidecar...", "path", lifeboatPath)

	planPath := filepath.Join(cfg.Directories.DataDir, "current_plan.md")
	statePath := filepath.Join(cfg.Directories.DataDir, "state.json")
	cmd := exec.Command(lifeboatPath, "--state", statePath, "--plan", planPath, "--sidecar")

	// Platform specific detachment
	attachDetachedAttributes(cmd)

	if err := cmd.Start(); err != nil {
		log.Error("Failed to start Lifeboat Sidecar", "error", err)
	} else {
		log.Info("Lifeboat Sidecar started", "pid", cmd.Process.Pid)
	}
}

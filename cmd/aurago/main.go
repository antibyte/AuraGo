package main

import (
	"bytes"
	"context"
	"crypto/rand"
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
	"aurago/internal/dockerutil"
	"aurago/internal/invasion"
	"aurago/internal/invasion/bridge"
	"aurago/internal/inventory"
	"aurago/internal/llm"
	"aurago/internal/logger"
	"aurago/internal/memory"
	"aurago/internal/planner"
	promptspkg "aurago/internal/prompts"
	"aurago/internal/push"
	"aurago/internal/remote"
	"aurago/internal/sandbox"
	"aurago/internal/security"
	"aurago/internal/server"
	"aurago/internal/services/optimizer"
	"aurago/internal/setup"
	"aurago/internal/sqlconnections"
	"aurago/internal/tools"
	"aurago/internal/warnings"

	"github.com/gofrs/flock"
)

// cronHTTPClient is used for cron loopback requests with a bounded timeout.
var cronHTTPClient = &http.Client{Timeout: 2 * time.Minute}

func main() {
	// â”€â”€ Sandbox helper mode â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
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
	exePath, _ := os.Executable()
	installDir := filepath.Dir(exePath)

	// Load secrets in priority order -- each step only sets vars not already present:
	//   1. systemd EnvironmentFile (already in env before process starts)
	//   2. Docker Compose secret  (/run/secrets/aurago_master_key)
	//   3. System credential file (/etc/aurago/master.key)  â† manual starts post-migration
	//   4. Local .env             ($configDir/.env)          â† dev / non-systemd installs
	loadDockerSecret("/run/secrets/aurago_master_key", "AURAGO_MASTER_KEY", appLog)
	loadDotEnv("/etc/aurago/master.key", appLog)
	loadDotEnv(filepath.Join(filepath.Dir(configFile), ".env"), appLog)

	// â”€â”€ Config-check mode: validate YAML and exit (used by Docker entrypoint) â”€
	if checkConfig {
		if _, err := config.Load(configFile); err != nil {
			fmt.Fprintf(os.Stderr, "CONFIG ERROR: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("OK")
		os.Exit(0)
	}

	// â”€â”€ Early Config Load for Path Resolution â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
	cfg, err := config.Load(configFile)
	if err != nil && !runSetup {
		if setup.NeedsSetup(installDir, configFile) {
			resPath := filepath.Join(installDir, "resources.dat")
			if _, statErr := os.Stat(resPath); statErr != nil {
				appLog.Warn("resources.dat not found in install directory — bootstrapping from local defaults", "path", resPath)
			} else {
				appLog.Info("Running automatic setup from resources.dat")
			}
			if setupErr := setup.Run(appLog); setupErr != nil {
				appLog.Error("Auto-setup failed", "error", setupErr)
				os.Exit(1)
			}
			cfg, err = config.Load(configFile)
		}
		if err != nil {
			// If we can't load config and we're not in setup, we can't safely proceed
			log.Fatalf("âŒ CONFIG ERROR: %v", err)
		}
	}

	// â”€â”€ Apply CLI flags for HTTPS â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
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

	// â”€â”€ Apply initial password â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
	if initialPassword != "" && cfg != nil {
		masterKey := os.Getenv("AURAGO_MASTER_KEY")
		if masterKey != "" && len(masterKey) == 64 {
			vaultPath := filepath.Join(cfg.Directories.DataDir, "vault.bin")
			if v, err := security.NewVault(masterKey, vaultPath); err == nil {
				hash, err := server.HashPassword(initialPassword)
				if err == nil {
					if err := v.WriteSecret("auth_password_hash", hash); err != nil {
						appLog.Error("Failed to store password hash in vault", "error", err)
					} else {
						appLog.Info("Initial password hash stored in vault")
					}

					// Setup session secret if not exists
					if sec, _ := v.ReadSecret("auth_session_secret"); sec == "" {
						if newSec, e := server.GenerateRandomHex(32); e == nil {
							if err := v.WriteSecret("auth_session_secret", newSec); err != nil {
								appLog.Error("Failed to store session secret in vault", "error", err)
							}
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

	// â”€â”€ Init-only mode: apply flags and exit without starting the server â”€â”€
	// Used by the installer to set the initial password / HTTPS config.
	if initOnly {
		appLog.Info("Init-only mode: configuration applied, exiting.")
		os.Exit(0)
	}

	// â”€â”€ Robust File Locking â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
	var lockPath string
	if cfg != nil && cfg.Directories.DataDir != "" {
		lockPath = filepath.Join(cfg.Directories.DataDir, "aurago.lock")
		if err := os.MkdirAll(cfg.Directories.DataDir, 0755); err != nil {
			appLog.Error("Failed to create data directory", "path", cfg.Directories.DataDir, "error", err)
		}
	} else {
		lockPath = "aurago.lock"
	}

	absLockPath, _ := filepath.Abs(lockPath)
	appLog.Info("Checking application lock", "path", absLockPath)

	fileLock := flock.New(absLockPath)
	locked, err := fileLock.TryLock()
	if err != nil || !locked {
		appLog.Error("âŒ BLOCKIERT: AuraGo lÃ¤uft bereits!", "lock_path", absLockPath)
		os.Exit(1)
	}
	defer fileLock.Unlock()
	appLog.Info("Application lock acquired", "path", absLockPath)

	// â”€â”€ Setup mode: extract resources and install service â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
	if runSetup {
		appLog.Info("Running AuraGo first-time setup ...")
		if err := setup.Run(appLog); err != nil {
			appLog.Error("Setup failed", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// â”€â”€ Runtime directories â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
	setup.EnsureDirectories(installDir, appLog)

	appLog.Info("Starting AuraGo")

	// â”€â”€ Bootstrap embedded prompt defaults if PromptsDir is empty â”€â”€â”€â”€â”€â”€â”€â”€
	promptspkg.EnsurePromptsDir(cfg.Directories.PromptsDir, appLog)

	// Configure execution timeouts from config
	tools.ConfigureTimeouts(cfg.Tools.PythonTimeoutSeconds, cfg.Tools.SkillTimeoutSeconds, cfg.Tools.BackgroundTimeoutSeconds)

	// Maintenance lock setup (uses DataDir)
	tools.SetBusyFilePath(filepath.Join(cfg.Directories.DataDir, "maintenance.lock"))
	// Clean up stale maintenance lock from previous unclean shutdown
	if tools.IsBusy() {
		appLog.Warn("Stale maintenance lock detected at startup -- clearing")
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

		// Truncate prompts.log on each startup so it only contains entries from the current session
		if cfg.Logging.EnablePromptLog {
			promptLogPath := filepath.Join(cfg.Logging.LogDir, "prompts.log")
			if err := os.WriteFile(promptLogPath, nil, 0644); err != nil {
				appLog.Warn("Failed to truncate prompts.log", "error", err)
			}
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

	// Initialize Knowledge Graph early so ApplyPendingEmbeddingsReset can use it
	// instead of opening a separate database connection
	kg, err := memory.NewKnowledgeGraph(
		cfg.SQLite.KnowledgeGraphPath,
		filepath.Join(cfg.Directories.DataDir, "graph.json"),
		appLog,
	)
	if err != nil {
		appLog.Error("Failed to initialize knowledge graph", "error", err)
		os.Exit(1)
	}
	defer kg.Close()

	if _, err := server.ApplyPendingEmbeddingsReset(cfg, shortTermMem, kg, appLog); err != nil {
		appLog.Error("Failed to apply pending embeddings reset", "error", err)
		os.Exit(1)
	}

	// Migrate core_memory.md â†’ SQLite (no-op if already done); returns true on first start
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

	// Invasion Control DB (nests & eggs) -- always initialized so the UI works
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

	// Planner (Appointments & Todos) DB
	plannerDB, plannerDBErr := planner.InitDB(cfg.SQLite.PlannerPath)
	if plannerDBErr != nil {
		appLog.Warn("Failed to initialize Planner DB", "error", plannerDBErr, "path", cfg.SQLite.PlannerPath)
		plannerDB = nil
	} else {
		defer plannerDB.Close()
		appLog.Info("Planner DB initialized", "path", cfg.SQLite.PlannerPath)
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
	if legacyVaultPath := findLegacyVaultPath(configFile, cfg.Directories.DataDir); legacyVaultPath != "" {
		appLog.Warn("Legacy vault file found at previous data directory", "legacy_path", legacyVaultPath, "current_path", vaultPath)
	}
	vault, err := security.NewVault(masterKey, vaultPath)
	if err != nil {
		appLog.Error("Failed to initialize security vault", "error", err)
		os.Exit(1)
	}

	// Initialize SQL Connections pool now that vault is available.
	// Pool is only created when the feature is explicitly enabled -- it manages
	// live connections to external databases, which should be opt-in.
	if sqlConnectionsDB != nil && cfg.SQLConnections.Enabled {
		sqlConnectionPool = sqlconnections.NewConnectionPool(
			sqlConnectionsDB, vault,
			cfg.SQLConnections.MaxPoolSize,
			cfg.SQLConnections.ConnectionTimeoutSec,
			appLog,
		)
		// Configure rate limiting: minimum seconds between accesses per connection
		if cfg.SQLConnections.RateLimitWindowSec > 0 {
			sqlConnectionPool.SetRateLimit(cfg.SQLConnections.RateLimitWindowSec)
			appLog.Info("SQL connection pool rate limiting enabled", "window_sec", cfg.SQLConnections.RateLimitWindowSec)
		}
		// Configure idle TTL: how long to keep idle connections before eviction
		if cfg.SQLConnections.IdleTTLSec > 0 {
			sqlConnectionPool.SetIdleTTL(time.Duration(cfg.SQLConnections.IdleTTLSec) * time.Second)
			appLog.Info("SQL connection pool idle TTL configured", "idle_ttl_sec", cfg.SQLConnections.IdleTTLSec)
		}
		defer sqlConnectionPool.CloseAll()
	}

	// One-time migration: auth secrets (password_hash, session_secret) may have been
	// stored in config.yaml by older versions. Move them to the encrypted vault so that
	// they are no longer kept in plaintext on disk.
	config.MigratePlaintextSecretsToVault(configFile, vault, appLog)

	// Apply all vault-stored secrets into the runtime config, then re-resolve
	// provider references so that API keys propagate to the LLM/Vision/etc. slots.
	cfg.ApplyVaultSecrets(vault)
	cfg.ResolveProviders()

	// Apply OAuth2 access tokens from vault into provider API keys
	cfg.ApplyOAuthTokens(vault)

	// Web Push (PWA notifications) -- init after vault so VAPID keys can be stored/loaded
	if _, err := push.NewManager(cfg.SQLite.PushPath, vault, appLog); err != nil {
		appLog.Warn("Web Push manager initialization failed -- push notifications disabled", "error", err)
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
	if cfg.Agent.ContextWindow == 0 || cfg.Agent.SystemPromptTokenBudgetAuto {
		if cfg.Agent.ContextWindow == 0 {
			cfg.Agent.ContextWindow = llm.DetectContextWindow(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.ProviderType, appLog)
		}
		if cfg.Agent.SystemPromptTokenBudgetAuto {
			cfg.Agent.SystemPromptTokenBudget, cfg.Agent.ContextWindow = llm.AutoConfigureBudget(cfg.Agent.ContextWindow, cfg.Agent.SystemPromptTokenBudget, appLog)
		}
	}

	// Warnings Registry for runtime health / security issue tracking
	warningsRegistry := warnings.NewRegistry()

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

	// Generate a per-process crypto token for loopback authentication.
	// This token is only valid for the lifetime of this process and never persisted.
	loopbackTokenBytes := make([]byte, 32)
	if _, err := rand.Read(loopbackTokenBytes); err != nil {
		appLog.Error("Failed to generate loopback auth token", "error", err)
		os.Exit(1)
	}
	loopbackToken := base64.StdEncoding.EncodeToString(loopbackTokenBytes)
	backgroundTaskManager.SetInternalToken(loopbackToken)

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
		client := server.NewInternalHTTPClient(timeout)
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			return fmt.Errorf("create background loopback request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-FollowUp", "true")
		req.Header.Set("X-Internal-Token", loopbackToken)
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
		// We deliberately do NOT expose the entire data/ dir -- vault.bin and the
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
		// be blocked by Landlock -- causing "No such file or directory" errors even
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
		// the sandbox.  The socket path is a Unix domain socket -- Landlock's
		// LANDLOCK_ACCESS_FS_MAKE_SOCK right controls access to it, so the socket
		// path must be explicitly added to the ruleset.
		// We also inject DOCKER_HOST so the docker CLI can find the socket even
		// with the stripped minimal environment built by buildEnv().
		var extraEnv []string
		if cfg.Docker.Enabled {
			dockerHost := cfg.Docker.Host
			if dockerHost == "" {
				dockerHost = dockerutil.DefaultHost()
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
	// Note: KG was already initialized earlier for ApplyPendingEmbeddingsReset
	optDB, optErr := optimizer.InitDB(cfg.SQLite.OptimizationPath)
	if optErr != nil {
		appLog.Warn("Failed to initialize optimizer trace database", "error", optErr)
	} else {
		defer optDB.Close()
		optWorker := optimizer.NewOptimizerWorker(optDB, llmClient, llmClient, 6*time.Hour)
		if cfg.Agent.OptimizerEnabled {
			go optWorker.Start(context.Background())
		}
	}

	// Enable semantic search if embeddings are enabled (kg was initialized earlier)
	if !longTermMem.IsDisabled() {
		go func() {
			if err := kg.EnableSemanticSearchShared(longTermMem.GetDB(), longTermMem.GetEmbeddingFunc()); err != nil {
				appLog.Warn("Failed to enable KG semantic search", "error", err)
			}
		}()
	} else {
		appLog.Info("KG semantic search skipped (embeddings disabled)")
	}

	// Handle Recovery Context
	if recoveryContext != "" {
		decoded, err := base64.StdEncoding.DecodeString(recoveryContext)
		if err != nil {
			appLog.Error("Failed to decode recovery context", "error", err)
		} else {
			msg := fmt.Sprintf("SYSTEM: Neustart nach Wartung abgeschlossen. Zusammenfassung der Ã„nderungen: %s. Setze deinen Plan fort.", string(decoded))
			mid, _ := shortTermMem.InsertMessage("default", "system", msg, false, false)
			historyManager.Add("system", msg, mid, false, false)
			appLog.Info("Recovery context injected into history")
		}
	}

	// Start Lifeboat Sidecar if enabled
	if cfg.Maintenance.LifeboatEnabled {
		startLifeboatSidecar(appLog, cfg, loopbackToken)
	}

	// â”€â”€ Egg Mode: start WebSocket client to master â”€â”€
	var eggMissionResultSink func(result bridge.MissionResultPayload) error
	if cfg.EggMode.Enabled {
		appLog.Info("Egg mode enabled -- connecting to master", "master_url", cfg.EggMode.MasterURL)
		eggClient := bridge.NewEggClient(
			cfg.EggMode.MasterURL,
			cfg.EggMode.EggID,
			cfg.EggMode.NestID,
			cfg.EggMode.SharedKey,
			"1.0.0",
			appLog,
		)
		eggClient.TLSSkipVerify = cfg.EggMode.TLSSkipVerify
		internalHTTPClient := server.NewInternalHTTPClient(2 * time.Minute)
		eggMissionAPI := func(method, path string, body interface{}) error {
			var reader io.Reader
			if body != nil {
				payload, err := json.Marshal(body)
				if err != nil {
					return err
				}
				reader = bytes.NewBuffer(payload)
			}
			req, err := http.NewRequest(method, server.InternalAPIURL(cfg)+path, reader)
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Internal-FollowUp", "true")
			req.Header.Set("X-Internal-Token", loopbackToken)
			resp, err := internalHTTPClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			respBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		}
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
				req.Header.Set("X-Internal-Token", loopbackToken)
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
		eggClient.OnMissionSync = func(payload bridge.MissionSyncPayload) error {
			appLog.Info("Mission sync received from master", "mission_id", payload.MissionID, "revision", payload.Revision)
			return eggMissionAPI(http.MethodPost, "/api/internal/missions/sync", payload)
		}
		eggClient.OnMissionRun = func(payload bridge.MissionRunPayload) error {
			appLog.Info("Mission run received from master", "mission_id", payload.MissionID, "trigger_type", payload.TriggerType)
			body := map[string]string{
				"trigger_type": payload.TriggerType,
				"trigger_data": payload.TriggerData,
			}
			return eggMissionAPI(http.MethodPost, "/api/internal/missions/"+payload.MissionID+"/run", body)
		}
		eggClient.OnMissionDelete = func(payload bridge.MissionDeletePayload) error {
			appLog.Info("Mission delete received from master", "mission_id", payload.MissionID)
			return eggMissionAPI(http.MethodDelete, "/api/internal/missions/"+payload.MissionID, nil)
		}
		eggMissionResultSink = func(result bridge.MissionResultPayload) error {
			if err := eggClient.SendMissionResult(result); err != nil {
				return err
			}
			return nil
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
			appLog.Warn("Stop command from master -- initiating shutdown")
			close(shutdownCh)
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := waitForInternalAPIReady(ctx, cfg, loopbackToken, internalHTTPClient); err != nil {
				appLog.Error("Egg client startup aborted because internal API is not ready", "error", err)
				return
			}
			eggClient.Start()
		}()
	}

	// Startup security audit -- log warnings for any critical/warning issues
	// so admins notice them immediately in the log even before opening the UI.
	if secHints := server.CheckSecurity(cfg); len(secHints) > 0 {
		critCount := 0
		for _, h := range secHints {
			if h.Severity == server.SevCritical {
				critCount++
			}
		}
		if critCount > 0 {
			appLog.Warn("[Security] CRITICAL security issues detected -- open /config to review and fix",
				"critical", critCount, "total", len(secHints))
		} else {
			appLog.Warn("[Security] Security recommendations detected -- open /config to review",
				"total", len(secHints))
		}
		for _, h := range secHints {
			appLog.Warn("[Security] "+h.Title, "id", h.ID, "severity", h.Severity)
		}

		// Bridge security hints into the warnings registry.
		for _, h := range secHints {
			warningsRegistry.Add(warnings.Warning{
				ID:          "sec_" + h.ID,
				Severity:    h.Severity,
				Title:       h.Title,
				Description: h.Description,
				Category:    warnings.CategorySecurity,
			})
		}
	}

	// Register built-in warning producers (token budget fallback, vectordb, etc.).
	warnings.RegisterBuiltinProducers(warningsRegistry, cfg, appLog)

	if err := server.Start(server.StartOptions{
		Cfg:                  cfg,
		Logger:               appLog,
		AccessLogger:         webAccessLog,
		LLMClient:            llmClient,
		ShortTermMem:         shortTermMem,
		LongTermMem:          longTermMem,
		Vault:                vault,
		Registry:             registry,
		CronManager:          cronManager,
		HistoryManager:       historyManager,
		KG:                   kg,
		InventoryDB:          inventoryDB,
		InvasionDB:           invasionDB,
		CheatsheetDB:         cheatsheetDB,
		ImageGalleryDB:       imageGalleryDB,
		RemoteControlDB:      remoteControlDB,
		MediaRegistryDB:      mediaRegistryDB,
		HomepageRegistryDB:   homepageRegistryDB,
		ContactsDB:           contactsDB,
		PlannerDB:            plannerDB,
		SQLConnectionsDB:     sqlConnectionsDB,
		SQLConnectionPool:    sqlConnectionPool,
		BackgroundTasks:      backgroundTaskManager,
		EggMissionResultSink: eggMissionResultSink,
		WarningsRegistry:     warningsRegistry,
		IsFirstStart:         isFirstStart,
		ShutdownCh:           shutdownCh,
		InstallDir:           installDir,
	}); err != nil {
		appLog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

func waitForInternalAPIReady(ctx context.Context, cfg *config.Config, token string, client *http.Client) error {
	if client == nil {
		client = server.NewInternalHTTPClient(10 * time.Second)
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	url := server.InternalAPIURL(cfg) + "/api/health"
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-Internal-FollowUp", "true")
		req.Header.Set("X-Internal-Token", token)
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
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

		// Close site monitor DB if initialized
		_ = tools.CloseSiteMonitorDB()

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
		return // Already set via env -- don't override
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return // Secret file doesn't exist -- not a Docker deployment
	}
	val := strings.TrimSpace(string(data))
	if val == "" {
		return
	}
	os.Setenv(envVar, val)
	log.Info("Loaded secret from Docker secret file", "env", envVar, "path", path)
}

func findLegacyVaultPath(configPath, currentDataDir string) string {
	if strings.TrimSpace(configPath) == "" || strings.TrimSpace(currentDataDir) == "" {
		return ""
	}
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return ""
	}
	configDir := filepath.Dir(absConfigPath)
	legacyVaultPath := filepath.Join(configDir, "data", "vault.bin")
	currentVaultPath := filepath.Join(currentDataDir, "vault.bin")
	if filepath.Clean(legacyVaultPath) == filepath.Clean(currentVaultPath) {
		return ""
	}
	if _, err := os.Stat(legacyVaultPath); err == nil {
		return legacyVaultPath
	}
	return ""
}

func startLifeboatSidecar(log *slog.Logger, cfg *config.Config, bridgeToken string) {
	// Candidate paths in priority order:
	//   1. ./lifeboat         " Docker image layout (/app/lifeboat next to /app/aurago)
	//   2. ./bin/lifeboat     " native Linux install via install.sh
	//   3. ./bin/lifeboat_linux " pre-built binary distributed in the repo
	//   4. ./bin/lifeboat.exe " Windows
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
	// Pass the bridge authentication token via environment variable so lifeboat
	// can authenticate itself when sending commands over the TCP bridge.
	cmd.Env = append(os.Environ(), "AURAGO_BRIDGE_TOKEN="+bridgeToken)

	// Platform specific detachment
	attachDetachedAttributes(cmd)

	if err := cmd.Start(); err != nil {
		log.Error("Failed to start Lifeboat Sidecar", "error", err)
	} else {
		log.Info("Lifeboat Sidecar started", "pid", cmd.Process.Pid)
	}
}

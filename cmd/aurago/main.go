package main

import (
	"bytes"
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
	"aurago/internal/invasion"
	"aurago/internal/invasion/bridge"
	"aurago/internal/inventory"
	"aurago/internal/llm"
	"aurago/internal/logger"
	"aurago/internal/memory"
	promptspkg "aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/server"
	"aurago/internal/setup"
	"aurago/internal/tools"

	"github.com/gofrs/flock"
)

// cronHTTPClient is used for cron loopback requests with a bounded timeout.
var cronHTTPClient = &http.Client{Timeout: 2 * time.Minute}

func main() {
	var debug bool
	var runSetup bool
	var checkConfig bool
	var configFile string
	var recoveryContext string
	var enableHTTPS bool
	var httpsDomain string
	var httpsEmail string
	var initialPassword string
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.BoolVar(&runSetup, "setup", false, "Extract resources.dat, install service, and exit")
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

	// Load secrets: Docker secret → /etc/aurago/master.key → local .env
	loadDockerSecret("/run/secrets/aurago_master_key", "AURAGO_MASTER_KEY", appLog)
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
		logPath := filepath.Join(cfg.Logging.LogDir, "supervisor.log")
		if lf, err := logger.SetupWithFile(debug, logPath, false); err == nil {
			appLog = lf.Logger
			slog.SetDefault(appLog)
			defer lf.Close()
			appLog.Info("File logging enabled", "path", logPath)
		} else {
			appLog.Warn("Failed to setup file logging", "error", err)
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
	go tools.ProvisionSkillDependencies(cfg.Directories.SkillsDir, cfg.Directories.WorkspaceDir, appLog, cfg.Agent.EnableGoogleWorkspace)

	shortTermMem, err := memory.NewSQLiteMemory(cfg.SQLite.ShortTermPath, appLog)
	if err != nil {
		appLog.Error("Failed to initialize Short-Term memory", "error", err)
		os.Exit(1)
	}
	defer shortTermMem.Close()

	// Migrate core_memory.md → SQLite (no-op if already done); returns true on first start
	isFirstStart := shortTermMem.MigrateCoreMemoryFromMarkdown(cfg.Directories.DataDir, appLog)

	longTermMem, err := memory.NewChromemVectorDB(cfg, appLog)
	if err != nil {
		appLog.Error("Failed to initialize Long-Term memory (VectorDB)", "error", err)
		os.Exit(1)
	}

	inventoryDB, err := inventory.InitDB(cfg.SQLite.InventoryPath)
	if err != nil {
		appLog.Error("Failed to initialize Inventory DB", "error", err)
		os.Exit(1)
	}
	defer inventoryDB.Close()

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

	// Tool guide indexing (at startup for performance)
	toolGuidesDir := filepath.Join(cfg.Directories.PromptsDir, "tools_manuals")
	if err := longTermMem.IndexToolGuides(toolGuidesDir, false); err != nil {
		appLog.Warn("Tool guide indexing failed", "error", err)
	}

	// Documentation indexing (RAG)
	docDir := filepath.Join(filepath.Dir(cfg.ConfigPath), "documentation")
	if _, err := os.Stat(docDir); err == nil {
		if err := longTermMem.IndexDirectory(docDir, "documentation", shortTermMem, false); err != nil {
			appLog.Warn("Documentation indexing failed", "error", err)
		}
	} else {
		appLog.Debug("Documentation directory not found, skipping indexing", "path", docDir)
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

	// Apply all vault-stored secrets into the runtime config, then re-resolve
	// provider references so that API keys propagate to the LLM/Vision/etc. slots.
	cfg.ApplyVaultSecrets(vault)
	cfg.ResolveProviders()

	// Apply OAuth2 access tokens from vault into provider API keys
	cfg.ApplyOAuthTokens(vault)

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

	// Cron Manager for autonomous triggers
	cronManager := tools.NewCronManager(cfg.Directories.DataDir)
	err = cronManager.Start(func(prompt string) {
		appLog.Info("Executing autonomous cron task", "prompt", prompt)

		// Send a loopback request to our own API
		url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", cfg.Server.Port)

		msg := map[string]interface{}{
			"model": cfg.LLM.Model,
			"messages": []map[string]string{
				{"role": "user", "content": fmt.Sprintf("[SYSTEM CRON TRIGGER] It is time to execute the following scheduled task: %s", prompt)},
			},
		}

		scheduleRetry := func(reason string) {
			appLog.Warn("Cron task failed, scheduling retry in 5 minutes", "reason", reason, "prompt", prompt)
			time.AfterFunc(5*time.Minute, func() {
				appLog.Info("Retrying failed cron task", "prompt", prompt)
				retryPayload, _ := json.Marshal(msg)
				retryResp, retryErr := cronHTTPClient.Post(url, "application/json", bytes.NewBuffer(retryPayload))
				if retryErr != nil {
					appLog.Error("Cron retry also failed", "error", retryErr)
				} else {
					retryResp.Body.Close()
				}
			})
		}

		payload, _ := json.Marshal(msg)
		resp, reqErr := cronHTTPClient.Post(url, "application/json", bytes.NewBuffer(payload))
		if reqErr != nil {
			scheduleRetry(reqErr.Error())
		} else {
			if resp.StatusCode != 200 {
				scheduleRetry(fmt.Sprintf("non-200 status: %d", resp.StatusCode))
			}
			resp.Body.Close()
		}
	})
	if err != nil {
		appLog.Error("Failed to load crontab", "error", err)
	}

	// Graceful shutdown: kill all background processes on SIGINT/SIGTERM
	shutdownCh := setupGracefulShutdown(appLog, registry, llmClient)

	// History Manager for persistent conversational memory array
	historyManager := memory.NewHistoryManager(filepath.Join(cfg.Directories.DataDir, "chat_history.json"))
	defer historyManager.Close()

	// Phase 36: Native Knowledge Graph
	kg := memory.NewKnowledgeGraph(filepath.Join(cfg.Directories.DataDir, "graph.json"))
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
			url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", eggPort)
			resp, err := cronHTTPClient.Post(url, "application/json", bytes.NewBuffer(payload))
			result := bridge.ResultPayload{TaskID: task.TaskID}
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

	if err := server.Start(cfg, appLog, llmClient, shortTermMem, longTermMem, vault, registry, cronManager, historyManager, kg, inventoryDB, invasionDB, isFirstStart, shutdownCh); err != nil {
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
	log.Info("Loading environment from .env", "path", path)
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
			os.Setenv(key, val)
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

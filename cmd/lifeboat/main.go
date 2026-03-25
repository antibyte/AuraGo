package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/llm"
	"aurago/internal/logger"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/gofrs/flock"
	"github.com/sashabaranov/go-openai"
)

func main() {
	fileLock := flock.New("lifeboat.lock")
	locked, err := fileLock.TryLock()
	if err != nil || !locked {
		log.Fatalf("❌ BLOCKIERT: Lifeboat läuft bereits! (Nova, lass das...)")
	}
	defer fileLock.Unlock()

	statePath := flag.String("state", "", "Pfad zur State-Datei")
	planPath := flag.String("plan", "", "Pfad zum Operationsplan")
	configPath := flag.String("config", "config.yaml", "Pfad zur config.yaml")
	sidecar := flag.Bool("sidecar", false, "Start as a persistent sidecar process")
	debug := flag.Bool("debug", false, "Enable debug mode")
	flag.Parse()

	if *statePath == "" || *planPath == "" {
		log.Println("Fehler: --state und --plan Argumente sind erforderlich.")
		flag.Usage()
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Printf("Fehler beim Laden der Config: %v", err)
		os.Exit(1)
	}

	l := logger.Setup(*debug)
	if cfg.Logging.EnableFileLog {
		logPath := filepath.Join(cfg.Logging.LogDir, "lifeboat.log")
		if fl, err := logger.SetupWithFile(*debug, logPath, false); err == nil {
			l = fl.Logger
			defer fl.Close()
			l.Info("File logging enabled for lifeboat", "path", logPath)
		}
	}
	slog.SetDefault(l)
	tools.SetBusyFilePath(filepath.Join(cfg.Directories.DataDir, "maintenance.lock"))
	l.Info("Lifeboat (Sidecar) gestartet", "state", *statePath, "plan", *planPath, "lock", tools.GetBusyFilePath())

	if *sidecar {
		runSidecarLoop(cfg, *statePath, *planPath, l)
	} else {
		l.Info("Notice: Lifeboat should be started with --sidecar in this architecture. Running in one-shot mode as fallback.")
		if err := runOperation(cfg, *statePath, *planPath, l); err != nil {
			l.Error("Operation failed", "error", err)
			os.Exit(1)
		}
	}
}

func runSidecarLoop(cfg *config.Config, statePath, planPath string, l *slog.Logger) {
	addr := fmt.Sprintf("localhost:%d", cfg.Maintenance.LifeboatPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		l.Error("Sidecar: Failed to listen", "addr", addr, "error", err)
		os.Exit(1)
	}
	l.Info("Lifeboat Sidecar listening", "addr", addr)
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			l.Warn("Sidecar: Accept error", "error", err)
			continue
		}

		go handleSidecarConnection(conn, cfg, statePath, planPath, l)
	}
}

func handleSidecarConnection(conn net.Conn, cfg *config.Config, statePath, planPath string, l *slog.Logger) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(10 * time.Minute))
	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return
	}

	var cmd struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(line, &cmd); err != nil {
		l.Error("Sidecar: Failed to unmarshal command", "error", err, "raw", string(line))
		return
	}
	l.Debug("Sidecar: Received command", "command", cmd.Command)

	if cmd.Command == "start_operation" {
		l.Info("Sidecar: Received start_operation signal!")
		if err := runOperation(cfg, statePath, planPath, l); err != nil {
			l.Error("Sidecar: Operation failed", "error", err)
		} else {
			l.Info("Sidecar: Operation completed successfully. Exiting to allow port reuse.")
			os.Exit(0)
		}
	}
}

func runOperation(cfg *config.Config, statePath, planPath string, l *slog.Logger) error {
	l.Info("Lifeboat Operation gestartet", "state", statePath, "plan", planPath)

	// 1. Dependencies initialisieren (Unified with Supervisor)
	dirs := []string{
		cfg.Directories.DataDir,
		cfg.Directories.WorkspaceDir,
		cfg.Directories.ToolsDir,
		cfg.Directories.PromptsDir,
		cfg.Directories.SkillsDir,
		cfg.Directories.VectorDBDir,
		cfg.Logging.LogDir,
	}

	for _, dir := range dirs {
		if dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				l.Warn("Failed to create directory", "path", dir, "error", err)
			}
		}
	}

	l.Info("Initializing STM...", "path", cfg.SQLite.ShortTermPath)
	shortTermMem, err := memory.NewSQLiteMemory(cfg.SQLite.ShortTermPath, l)
	if err != nil {
		return fmt.Errorf("STM init failed: %w", err)
	}
	defer shortTermMem.Close()

	l.Info("Initializing LTM (VectorDB)...")
	longTermMem, err := memory.NewChromemVectorDB(cfg, l)
	if err != nil {
		return fmt.Errorf("LTM init failed: %w", err)
	}

	masterKey := os.Getenv("AURAGO_MASTER_KEY")
	if masterKey == "" || len(masterKey) != 64 {
		return fmt.Errorf("AURAGO_MASTER_KEY is missing or not exactly 64 hex characters (32 bytes)")
	}
	l.Info("AURAGO_MASTER_KEY found", "len", len(masterKey))

	vaultPath := filepath.Join(cfg.Directories.DataDir, "vault.bin")
	l.Info("Initializing Vault...", "path", vaultPath)
	vault, err := security.NewVault(masterKey, vaultPath)
	if err != nil {
		return fmt.Errorf("vault init failed: %w", err)
	}

	llmClient := llm.NewClient(cfg)
	registry := tools.NewProcessRegistry(l)
	cronManager := tools.NewCronManager(cfg.Directories.DataDir)
	historyManager := memory.NewHistoryManager(filepath.Join(cfg.Directories.DataDir, "chat_history.json"))
	kg, err := memory.NewKnowledgeGraph(
		filepath.Join(cfg.Directories.DataDir, "knowledge_graph.db"),
		filepath.Join(cfg.Directories.DataDir, "graph.json"),
		l,
	)
	if err != nil {
		return fmt.Errorf("knowledge graph init failed: %w", err)
	}
	manifest := tools.NewManifest(cfg.Directories.ToolsDir)

	inventoryDB, err := inventory.InitDB(cfg.SQLite.InventoryPath)
	if err != nil {
		return fmt.Errorf("inventory init failed: %w", err)
	}
	defer inventoryDB.Close()
	if err := credentials.EnsureSchema(inventoryDB); err != nil {
		return fmt.Errorf("credentials schema init failed: %w", err)
	}

	// 2. Plan laden
	planContent, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("fehler beim Lesen des Plans: %w", err)
	}
	l.Info("Plan geladen", "len", len(planContent), "preview", strings.Split(string(planContent), "\n")[0])

	// 3. Unified Agent Loop ausführen
	l.Info("Starte AI Surgery Loop...")
	req := openai.ChatCompletionRequest{
		Model: cfg.LLM.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "You are now in the lifeboat and ready to execute your plan."},
		},
	}

	// Use NoopBroker for CLI output
	broker := &CLIBroker{logger: l}

	runCfg := agent.RunConfig{
		Config:          cfg,
		Logger:          l,
		LLMClient:       llmClient,
		ShortTermMem:    shortTermMem,
		HistoryManager:  historyManager,
		LongTermMem:     longTermMem,
		KG:              kg,
		InventoryDB:     inventoryDB,
		Vault:           vault,
		Registry:        registry,
		Manifest:        manifest,
		CronManager:     cronManager,
		CoAgentRegistry: nil,
		BudgetTracker:   nil,
		SessionID:       "lifeboat",
		IsMaintenance:   true,
		SurgeryPlan:     string(planContent),
	}

	_, err = agent.ExecuteAgentLoop(context.Background(), req, runCfg, false, broker)
	if err != nil {
		return fmt.Errorf("AI surgery failed: %w", err)
	}

	// 4. Main Agent neu bauen
	if err := rebuildMainAgent(l); err != nil {
		return fmt.Errorf("fehler beim Neubauen: %w", err)
	}

	// 5. Vitality Check via TCP (gegen den NOCH LAUFENDEN alten Agenten)
	if err := checkVitality(string(planContent), l); err != nil {
		l.Warn("Vitality Check failed (expected if old agent already quit or stuck)", "error", err)
	}

	// 6. Shutdown des alten Agenten
	if err := sendShutdownAndReload(l); err != nil {
		l.Warn("Shutdown signal failed (expected if old agent already quit)", "error", err)
	}

	// 7. Kurze Pause, damit der Port frei wird
	l.Info("Warte auf Port-Freigabe...")
	time.Sleep(2 * time.Second)

	// 8. Neuen Main Agent starten (mit Recovery Context)
	recoveryContext := base64.StdEncoding.EncodeToString(planContent)
	if err := restartMainAgent(recoveryContext, l); err != nil {
		return fmt.Errorf("fehler beim Neustart: %w", err)
	}

	l.Info("Operation erfolgreich abgeschlossen. Neuer Agent läuft.")
	tools.SetBusy(false)
	return nil
}

type CLIBroker struct {
	logger *slog.Logger
}

func (b *CLIBroker) Send(event, message string) {
	b.logger.Info("[Surgery Event]", "event", event, "message", message)
	fmt.Printf("[%s] %s\n", strings.ToUpper(event), message)
}

func (b *CLIBroker) SendJSON(jsonStr string) {
	b.logger.Debug("[Surgery JSON]", "data", jsonStr)
	fmt.Printf("[JSON] %s\n", jsonStr)
}

func checkVitality(summary string, l *slog.Logger) error {
	l.Info("Führe Vitality Check durch (localhost:8089)...")

	// Kurze Pause, um dem Agenten Zeit zum Starten zu geben
	time.Sleep(3 * time.Second)

	conn, err := net.DialTimeout("tcp", "localhost:8089", 5*time.Second)
	if err != nil {
		return fmt.Errorf("verbindung zu localhost:8089 fehlgeschlagen: %w", err)
	}
	defer conn.Close()

	// 1. Challenge generieren
	challenge := make([]byte, 8)
	if _, err := rand.Read(challenge); err != nil {
		return fmt.Errorf("fehler beim Generieren der Challenge: %w", err)
	}
	challengeHex := hex.EncodeToString(challenge)

	// 2. JSON Command senden
	cmd := map[string]string{
		"command":   "vitality_check",
		"challenge": challengeHex,
		"summary":   summary,
	}
	data, _ := json.Marshal(cmd)
	fmt.Fprintf(conn, "%s\n", string(data))

	// 3. Antwort lesen
	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("fehler beim Lesen der Antwort: %w", err)
	}

	var res struct {
		Status string `json:"status"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal(line, &res); err != nil {
		return fmt.Errorf("fehler beim Dekodieren der Antwort: %w", err)
	}

	// 4. Vergleichen
	if res.Status != "ok" || strings.TrimSpace(res.Result) != challengeHex {
		l.Error("Vitality Mismatch", "expected", challengeHex, "got", res.Result, "status", res.Status)
		return fmt.Errorf("vitality Check fehlgeschlagen: Status=%s, Challenge %s != Result %s", res.Status, challengeHex, res.Result)
	}

	l.Info("Vitality Check erfolgreich.")
	return nil
}

func sendShutdownAndReload(l *slog.Logger) error {
	l.Info("Sende shutdown_and_reload Befehl...")
	conn, err := net.DialTimeout("tcp", "localhost:8089", 5*time.Second)
	if err != nil {
		return fmt.Errorf("verbindung zu localhost:8089 fehlgeschlagen: %w", err)
	}
	defer conn.Close()

	cmd := map[string]string{"command": "shutdown_and_reload"}
	data, _ := json.Marshal(cmd)
	fmt.Fprintf(conn, "%s\n", string(data))
	return nil
}

func rebuildMainAgent(l *slog.Logger) error {
	binPath := "aurago" + EXE_SUFFIX
	l.Info("Führe Build aus", "command", fmt.Sprintf("go build -o %s ./cmd/aurago", binPath))

	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/aurago")
	output, err := buildCmd.CombinedOutput()
	if len(output) > 0 {
		l.Info("Build Output", "content", string(output))
	}
	if err != nil {
		l.Error("Build aborted with error", "error", err)
		return fmt.Errorf("build failed: %w", err)
	}
	l.Info("Build erfolgreich completed")
	return nil
}

func restartMainAgent(recoveryContext string, l *slog.Logger) error {
	exeName := "aurago" + EXE_SUFFIX
	exePath, err := filepath.Abs(exeName)
	if err != nil {
		l.Warn("Failed to resolve absolute path for restart", "error", err)
		exePath = "./" + exeName // Fallback to explicitly relative
	}
	l.Info("Starte Main Agent neu", "path", exePath)

	args := []string{}
	if recoveryContext != "" {
		args = append(args, "--recovery-context", recoveryContext)
	}

	cmd := prepareCommand(exePath, args...)

	l.Info("Executing restart command", "exe", exePath, "args", args)
	err = cmd.Start()
	if err != nil {
		l.Error("Restart failed", "error", err)
		return fmt.Errorf("start failed: %w", err)
	}

	l.Info("Main Agent wurde gestartet", "pid", cmd.Process.Pid)
	return nil
}

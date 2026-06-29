package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
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
	"runtime"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/kgquality"
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
		logPath := lifeboatLogPath(cfg)
		if fl, err := logger.SetupWithFile(*debug, logPath, false); err == nil {
			l = fl.Logger
			defer fl.Close()
			l.Info("File logging enabled for lifeboat", "path", logPath)
		}
	}
	slog.SetDefault(l)
	tools.SetBusyFilePath(lifeboatBusyFilePath(cfg))
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
		Token   string `json:"token"`
	}
	if err := json.Unmarshal(line, &cmd); err != nil {
		l.Error("Sidecar: Failed to unmarshal command", "error", err, "raw", string(line))
		return
	}
	l.Debug("Sidecar: Received command", "command", cmd.Command)

	if cmd.Command == "start_operation" {
		if !lifeboatCommandAuthorized(os.Getenv("AURAGO_BRIDGE_TOKEN"), cmd.Token) {
			l.Warn("Sidecar: rejected unauthorized start_operation command")
			return
		}
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
	for _, dir := range lifeboatRuntimeDirs(cfg) {
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
	defer func() {
		if err := longTermMem.Close(); err != nil {
			l.Warn("Failed to close LTM", "error", err)
		}
	}()

	// Load existing vector DB collections from SQLite and register them
	if cols, colsErr := shortTermMem.GetIndexedCollections(); colsErr == nil {
		longTermMem.RegisterCollections(cols)
		l.Info("Loaded existing vector DB collections from SQLite", "collections", cols)
	} else {
		l.Warn("Failed to load existing vector DB collections from SQLite", "error", colsErr)
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
	defer func() { _ = cronManager.Close() }()
	historyManager := memory.NewHistoryManager(filepath.Join(cfg.Directories.DataDir, "chat_history.json"))
	defer historyManager.Close()
	kg, err := memory.NewKnowledgeGraph(
		cfg.SQLite.KnowledgeGraphPath,
		filepath.Join(cfg.Directories.DataDir, "graph.json"),
		l,
	)
	if err != nil {
		return fmt.Errorf("knowledge graph init failed: %w", err)
	}
	kg.SetMinSemanticSimilarity(cfg.Tools.KnowledgeGraph.MinSemanticSimilarity)
	kg.SetExcludedNodeTypes(cfg.Tools.KnowledgeGraph.ExcludeNodeTypes)
	kg.SetSemanticReindexInterval(cfg.Tools.KnowledgeGraph.SemanticReindexInterval)
	kg.SetProtectOptimizeSources(cfg.Tools.KnowledgeGraph.ProtectOptimizeSources)
	kg.SetProtectIDPrefixes(cfg.Tools.KnowledgeGraph.ProtectIDPrefixes)
	kg.SetQualityPolicy(kgquality.Policy{
		PendingCoMentionTTLDays:         cfg.Tools.KnowledgeGraph.PendingCoMentionTTLDays,
		LowConfidenceCoMentionMinWeight: cfg.Tools.KnowledgeGraph.LowConfidenceCoMentionMinWeight,
		HideLowConfidenceByDefault:      cfg.Tools.KnowledgeGraph.HideLowConfidenceByDefault,
	})
	if !longTermMem.IsDisabled() {
		if err := kg.EnableSemanticSearchShared(longTermMem.GetDB(), longTermMem.GetEmbeddingFunc()); err != nil {
			l.Warn("Failed to enable KG semantic search", "error", err)
		}
	} else {
		l.Info("KG semantic search skipped (embeddings disabled)")
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
		Config:             cfg,
		Logger:             l,
		LLMClient:          llmClient,
		ShortTermMem:       shortTermMem,
		HistoryManager:     historyManager,
		LongTermMem:        longTermMem,
		KG:                 kg,
		InventoryDB:        inventoryDB,
		Vault:              vault,
		Registry:           registry,
		Manifest:           manifest,
		CronManager:        cronManager,
		CoAgentRegistry:    nil,
		BudgetTracker:      nil,
		PreparationService: nil,
		SessionID:          "lifeboat",
		IsMaintenance:      true,
		SurgeryPlan:        string(planContent),
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

func (b *CLIBroker) SendLLMStreamDelta(content, toolName, toolID string, index int, finishReason string) {
}

func (b *CLIBroker) SendLLMStreamDone(finishReason string) {}

func (b *CLIBroker) SendTokenUpdate(prompt, completion, total, sessionTotal, globalTotal int, isEstimated, isFinal bool, source string) {
}

func (b *CLIBroker) SendThinkingBlock(provider, content, state string) {
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

	cmd := map[string]string{"command": "shutdown_and_reload", "token": os.Getenv("AURAGO_BRIDGE_TOKEN")}
	data, _ := json.Marshal(cmd)
	fmt.Fprintf(conn, "%s\n", string(data))
	return nil
}

func rebuildMainAgent(l *slog.Logger) error {
	buildArgs := lifeboatBuildCommandArgs()
	l.Info("Führe Build aus", "command", "go "+strings.Join(buildArgs, " "))
	if len(buildArgs) >= 3 {
		if dir := filepath.Dir(buildArgs[2]); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create build output directory: %w", err)
			}
		}
	}

	buildCmd := exec.Command("go", buildArgs...)
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
	exePath, args, err := lifeboatRestartSpec(recoveryContext, filepath.Abs)
	if err != nil {
		l.Warn("Failed to resolve absolute path for restart", "error", err)
	}
	l.Info("Starte Main Agent neu", "path", exePath)

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

func lifeboatLogPath(cfg *config.Config) string {
	return filepath.Join(cfg.Logging.LogDir, "lifeboat.log")
}

func lifeboatBusyFilePath(cfg *config.Config) string {
	return filepath.Join(cfg.Directories.DataDir, "maintenance.lock")
}

func lifeboatRuntimeDirs(cfg *config.Config) []string {
	return []string{
		cfg.Directories.DataDir,
		cfg.Directories.WorkspaceDir,
		cfg.Directories.ToolsDir,
		cfg.Directories.PromptsDir,
		cfg.Directories.SkillsDir,
		cfg.Directories.VectorDBDir,
		cfg.Logging.LogDir,
	}
}

func lifeboatMainBinaryName() string {
	return "aurago" + EXE_SUFFIX
}

func lifeboatBuildCommandArgs() []string {
	return lifeboatBuildCommandArgsForTarget(lifeboatMainBinaryPath())
}

func lifeboatBuildCommandArgsForTarget(target string) []string {
	return []string{"build", "-o", target, "./cmd/aurago"}
}

func lifeboatRestartSpec(recoveryContext string, absFn func(string) (string, error)) (string, []string, error) {
	return lifeboatRestartSpecForTarget(lifeboatMainBinaryPath(), recoveryContext, absFn)
}

func lifeboatRestartSpecForTarget(target string, recoveryContext string, absFn func(string) (string, error)) (string, []string, error) {
	exePath, err := absFn(target)
	if err != nil {
		exePath = "." + string(filepath.Separator) + target
	}
	args := []string{}
	if recoveryContext != "" {
		args = append(args, "--recovery-context", recoveryContext)
	}
	return exePath, args, err
}

func lifeboatMainBinaryPath() string {
	return lifeboatSelectMainBinaryPath(runtime.GOOS, func(name string) bool {
		_, err := os.Stat(name)
		return err == nil
	})
}

func lifeboatSelectMainBinaryPath(goos string, exists func(string) bool) string {
	candidates := lifeboatMainBinaryCandidates(goos)
	for _, candidate := range candidates {
		if exists(candidate) {
			return candidate
		}
	}
	return candidates[0]
}

func lifeboatMainBinaryCandidates(goos string) []string {
	if goos == "windows" {
		return []string{
			filepath.Join("bin", "aurago_windows.exe"),
			filepath.Join("bin", "aurago.exe"),
			"aurago.exe",
		}
	}
	return []string{
		filepath.Join("bin", "aurago_linux"),
		filepath.Join("bin", "aurago"),
		lifeboatMainBinaryName(),
	}
}

func lifeboatCommandAuthorized(expectedToken, suppliedToken string) bool {
	expectedToken = strings.TrimSpace(expectedToken)
	suppliedToken = strings.TrimSpace(suppliedToken)
	if expectedToken == "" || suppliedToken == "" {
		return false
	}
	if len(expectedToken) != len(suppliedToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expectedToken), []byte(suppliedToken)) == 1
}

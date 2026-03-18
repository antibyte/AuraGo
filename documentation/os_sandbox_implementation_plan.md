# OS Sandbox Implementation Plan für AuraGo

**Ziel:** Ein einfach zu integrierendes, optionales Sandbox-System mit einem Toggle-Prinzip  
**Priorität:** Linux (vollständig) → macOS/Windows (alternative Lösungen)  
**Philosophie:** "Ein Schalter, automatisch erkannt, sofort geschützt"

---

## Kritische Überprüfung & Korrekturen (v2)

### ⚠️ Identifizierte Probleme & Lösungen

| Problem | Schwere | Lösung |
|---------|---------|--------|
| AppArmor Auto-Install braucht root | 🔴 Hoch | Kein Auto-Install, nur Detection + Hinweis |
| Seatbelt ist deprecated auf macOS | 🔴 Hoch | Docker als primäre Option für macOS |
| Windows Sandbox ist Session-basiert | 🔴 Hoch | Entfernt - nur Job Objects |
| Docker-Overhead pro Shell-Befehl | 🟡 Mittel | Optionales Caching/Pooling |
| Datenpersistenz unklar | 🟡 Mittel | Bind Mounts dokumentieren |
| Config zu komplex | 🟢 Niedrig | Vereinfacht auf Essentials |

---

## 1. Executive Summary

### Aktueller Stand
- ✅ Docker-basierte Python-Sandbox existiert bereits (`llm-sandbox` MCP Server)
- ✅ Shell/FS-Tools laufen direkt auf dem Host (unsicher)
- ✅ Config-Struktur vorhanden, aber erweiterungsbedürftig

### Korrigierte Zielarchitektur
```
┌─────────────────────────────────────────────────────────────────────┐
│  Toggle: sandbox.os.enabled = true                                  │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  Sandbox Router (automatische Backend-Selektion)            │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │   │
│  │  │ Linux Tier  │  │ macOS Tier  │  │ Windows Tier        │  │   │
│  │  │ 1-3         │  │ 2-3         │  │ 1-2                 │  │   │
│  │  ├─────────────┤  ├─────────────┤  ├─────────────────────┤  │   │
│  │  │ • AppArmor* │  │ • Docker    │  │ • Docker (WSL2)     │  │   │
│  │  │ • Seccomp   │  │ • Seatbelt° │  │ • Job Objects       │  │   │
│  │  │ • Docker    │  │   (legacy)  │  │ • AppContainer      │  │   │
│  │  └─────────────┘  └─────────────┘  └─────────────────────┘  │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  * = Benötigt root für Setup    ° = Deprecated, nicht empfohlen    │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. Konfiguration (Vereinfacht)

```yaml
# config.yaml - OS Sandbox Section (VEREINFACHT)
sandbox:
  # Bereits existierend für Python-Sandbox
  enabled: true
  backend: "docker"
  
  # NEU: OS-Level Sandbox für Shell/FS-Befehle
  os:
    enabled: false                    # THE TOGGLE - default: OFF (breaking change vermeiden)
    mode: "auto"                      # "auto" | "enforce" | "disabled"
    
    # Backend-Präferenz (optional, für Power-User)
    preferred_backend: ""             # "" = auto, "docker", "apparmor", "seccomp"
    
    # Fallback-Verhalten
    fallback:
      on_unavailable: "warn"          # "warn" | "block" | "allow"
      warning_message: "Sandbox nicht verfügbar - Befehl wird direkt ausgeführt"
    
    # Docker-spezifisch (plattformübergreifend nutzbar)
    docker:
      image: "aurago/os-sandbox:latest"
      runtime: "runc"                 # "runc" | "runsc" (gVisor, Linux only)
      network_mode: "none"            # "none" | "host" | "bridge"
      keep_alive: true                # Container am Leben halten für schnellere Ausführung
      pool_size: 2                    # Anzahl vorgehaltener Container
      
    # Linux-spezifisch (nur wenn Docker nicht verfügbar)
    linux:
      apparmor:
        profile: "aurago-sandbox"     # Benutzer muss selbst installieren!
        enforce: true
      seccomp:
        profile: "homelab"            # "homelab" | "strict" | "custom"
      landlock:
        enabled: true

# Danger Zone Override
agent:
  sandbox_override:
    execute_shell: false              # Default: false (kein Breaking Change)
    execute_python: true              # Bereits default via llm-sandbox
    filesystem_write: false           # Schreibzugriff in Sandbox
```

### Warum diese Vereinfachungen?

1. **Kein Auto-Install**: AppArmor-Profile, Seccomp-Profile etc. brauchen root-Rechte. Das sollte nicht automatisch passieren.

2. **Docker als primäre Option**: Funktioniert auf Linux, macOS (Docker Desktop) und Windows (Docker Desktop/WSL2). Einheitliches Verhalten.

3. **macOS Seatbelt entfernt**: `sandbox-exec` ist deprecated seit macOS 10.15+. Apple empfiehlt es nicht mehr.

4. **Windows Sandbox entfernt**: Windows Sandbox ist eine komplette VM pro Sitzung - nicht für einzelne Befehle geeignet.

---

## 3. Architektur & Integration (Korrigiert)

### 3.1 Neue Package-Struktur

```
internal/
├── sandbox/
│   ├── manager.go               # Zentraler Sandbox Manager
│   ├── router.go                # Backend-Router
│   ├── config.go                # Config-Strukturen
│   ├── pool.go                  # Container Pooling für Performance
│   ├── 
│   ├── backends/                # Backend-Implementierungen
│   │   ├── docker.go            # Docker (Linux/macOS/Windows)
│   │   ├── apparmor_linux.go    # AppArmor (nur Linux)
│   │   ├── seccomp_linux.go     # Seccomp (nur Linux)
│   │   ├── jobobject_windows.go # Job Objects (nur Windows)
│   │   └── none.go              # Fallback (direkte Ausführung)
│   ├── 
│   └── detection/               # Verfügbarkeits-Prüfung
│       ├── detect_linux.go
│       ├── detect_darwin.go
│       └── detect_windows.go
│   
└── tools/
    ├── shell.go                 # Modifiziert: Sandbox-Routing
    ├── python.go                # Nutzt existierende llm-sandbox
    └── filesystem.go            # Modifiziert: Sandbox für writes
```

### 3.2 Korrigierter Sandbox Manager

```go
// internal/sandbox/manager.go
package sandbox

// ExecutionRequest definiert eine auszuführende Operation
type ExecutionRequest struct {
    Tool        string            // "shell", "filesystem"
    Command     string            // Der Befehl (für Shell)
    Args        []string          // Argumente
    Env         map[string]string
    WorkDir     string            // Host-Pfad (wird gemountet)
    
    // Für Filesystem-Operationen
    FSOp        FSOperation      // "read", "write", "delete"
    Path        string
    Content     []byte
    
    // Sicherheitsanforderungen
    NeedNetwork bool              // Netzwerkzugriff nötig?
    Timeout     time.Duration
}

type FSOperation string

const (
    FSOpRead   FSOperation = "read"
    FSOpWrite  FSOperation = "write"
    FSOpDelete FSOperation = "delete"
)

// Manager ist der zentrale Sandbox-Manager
type Manager struct {
    config      *OSSandboxConfig
    router      *Router
    logger      *slog.Logger
    
    // Docker-Pool für schnelle Wiederverwendung
    dockerPool  *ContainerPool  // nur für Docker-Backend
    
    // Aktives Backend
    backend     Backend
    backendMu   sync.RWMutex
}

// Execute führt einen Befehl in der Sandbox aus
func (m *Manager) Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error) {
    // 1. Prüfe ob Sandbox aktiviert
    if !m.config.Enabled {
        return m.executeDirect(req)
    }
    
    // 2. Prüfe Backend
    backend := m.getBackend()
    if backend == nil || !backend.IsAvailable() {
        return m.handleUnavailable(req)
    }
    
    // 3. Führe in Sandbox aus
    result, err := backend.Execute(ctx, req)
    if err != nil {
        return nil, err
    }
    
    result.Sandboxed = true
    return result, nil
}

// getBackend initialisiert lazy das beste verfügbare Backend
func (m *Manager) getBackend() Backend {
    m.backendMu.RLock()
    if m.backend != nil {
        m.backendMu.RUnlock()
        return m.backend
    }
    m.backendMu.RUnlock()
    
    // Lazy initialization
    m.backendMu.Lock()
    defer m.backendMu.Unlock()
    
    if m.backend != nil {
        return m.backend
    }
    
    m.backend = m.router.SelectBackend(m.config)
    return m.backend
}
```

### 3.3 Backend Interface (Stabilisiert)

```go
// internal/sandbox/backends/backend.go
package backends

// Backend ist das Interface für alle Sandbox-Implementierungen
type Backend interface {
    Name() string
    Platform() string
    IsAvailable() bool
    
    // Lifecycle
    Initialize(config map[string]interface{}) error
    Shutdown() error
    
    // Ausführung
    Execute(ctx context.Context, req sandbox.ExecutionRequest) (*sandbox.ExecutionResult, error)
    
    // Für Docker: Pooling-Unterstützung
    SupportsPooling() bool
}

// DetectedBackend enthält Info über verfügbare Backends
type DetectedBackend struct {
    Name        string
    Available   bool
    Priority    int           // Niedriger = höhere Priorität
    Tier        int           // 0-4 (Isolation-Level)
    Description string
    SetupNeeded string        // Anweisungen falls Setup nötig
}
```

---

## 4. Plattform-spezifische Implementierungen (Korrigiert)

### 4.1 Docker-Backend (Plattform-übergreifend)

**Warum Docker als primäre Option?**
- ✅ Funktioniert auf Linux, macOS (Docker Desktop), Windows (Docker Desktop)
- ✅ Einheitliches Verhalten über alle Plattformen
- ✅ Einfach zu installieren (ein Befehl)
- ✅ Bereits für Python-Sandbox vorhanden

```go
// internal/sandbox/backends/docker.go
package backends

type DockerBackend struct {
    client      *client.Client
    image       string
    runtime     string           // "runc" oder "runsc" (nur Linux)
    networkMode string
    pool        *ContainerPool   // Für Performance
    logger      *slog.Logger
}

func (b *DockerBackend) Execute(ctx context.Context, req sandbox.ExecutionRequest) (*sandbox.ExecutionResult, error) {
    // Prüfe ob wir einen gepoolten Container haben
    if b.pool != nil && req.Tool == "shell" {
        return b.executeWithPool(ctx, req)
    }
    
    // Einzelne Container-Ausführung (für FS-Ops)
    return b.executeEphemeral(ctx, req)
}

func (b *DockerBackend) executeWithPool(ctx context.Context, req sandbox.ExecutionRequest) (*sandbox.ExecutionResult, error) {
    // Hole Container aus Pool
    container := b.pool.Get()
    if container == nil {
        return b.executeEphemeral(ctx, req)
    }
    defer b.pool.Put(container)
    
    // Führe Befehl im laufenden Container aus (docker exec)
    execConfig := types.ExecConfig{
        Cmd:          append([]string{"/bin/sh", "-c"}, req.Command),
        AttachStdout: true,
        AttachStderr: true,
        WorkingDir:   "/workspace",
    }
    
    // ... exec im Container
}

func (b *DockerBackend) executeEphemeral(ctx context.Context, req sandbox.ExecutionRequest) (*sandbox.ExecutionResult, error) {
    // Erstelle temporären Container (wie existierende Python-Sandbox)
    config := &container.Config{
        Image:      b.image,
        WorkingDir: "/workspace",
        User:       "1000:1000",
    }
    
    // Tool-spezifische Konfiguration
    switch req.Tool {
    case "shell":
        config.Cmd = append([]string{"/bin/sh", "-c"}, req.Command)
        config.Entrypoint = []string{}
    case "filesystem":
        // Für FS-Ops nutzen wir ein spezielles Script
        config.Cmd = []string{"/bin/sh", "-c", b.buildFSCommand(req)}
    }
    
    hostConfig := &container.HostConfig{
        Binds: []string{
            fmt.Sprintf("%s:/workspace:rw", req.WorkDir),
        },
        NetworkMode:    container.NetworkMode(b.networkMode),
        CapDrop:        []string{"ALL"},
        SecurityOpt:    []string{"no-new-privileges:true"},
        ReadonlyRootfs: true,
        Resources: container.Resources{
            Memory:     256 * 1024 * 1024,
            MemorySwap: 256 * 1024 * 1024,
            CpuQuota:   50000,
            PidsLimit:  50,
        },
    }
    
    // Nur Linux: gVisor Option
    if b.runtime != "" && runtime.GOOS == "linux" {
        hostConfig.Runtime = b.runtime
    }
    
    // Create, Start, Wait, Logs, Remove
    // ...
}

func (b *DockerBackend) IsAvailable() bool {
    // Prüfe Docker-Client
    if b.client == nil {
        return false
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    _, err := b.client.Ping(ctx)
    return err == nil
}

func (b *DockerBackend) SupportsPooling() bool {
    return true  // Docker unterstützt Container-Pooling
}
```

### 4.2 Container Pooling (Performance-Optimierung)

**Problem:** Docker-Overhead von ~100ms pro Befehl ist zu langsam für iterative Shell-Operationen.

**Lösung:** Pool von laufenden Containern

```go
// internal/sandbox/pool.go
package sandbox

// ContainerPool hält vorgewärmte Container bereit
type ContainerPool struct {
    client    *client.Client
    image     string
    size      int
    
    available chan *PooledContainer
    inUse     map[string]bool
    mu        sync.Mutex
}

type PooledContainer struct {
    ID        string
    CreatedAt time.Time
}

func NewContainerPool(client *client.Client, image string, size int) *ContainerPool {
    pool := &ContainerPool{
        client:    client,
        image:     image,
        size:      size,
        available: make(chan *PooledContainer, size),
        inUse:     make(map[string]bool),
    }
    
    // Pre-warm Container
    go pool.warmup()
    
    return pool
}

func (p *ContainerPool) warmup() {
    for i := 0; i < p.size; i++ {
        container, err := p.createContainer()
        if err != nil {
            continue
        }
        p.available <- container
    }
}

func (p *ContainerPool) Get() *PooledContainer {
    select {
    case container := <-p.available:
        p.mu.Lock()
        p.inUse[container.ID] = true
        p.mu.Unlock()
        return container
    case <-time.After(100 * time.Millisecond):
        return nil  // Fallback zu ephemeral
    }
}

func (p *ContainerPool) Put(container *PooledContainer) {
    p.mu.Lock()
    delete(p.inUse, container.ID)
    p.mu.Unlock()
    
    // Container zurücksetzen (rm -rf /workspace/*)
    p.resetContainer(container)
    
    select {
    case p.available <- container:
    default:
        // Pool voll, Container entfernen
        p.removeContainer(container)
    }
}
```

### 4.3 Linux-Natives Backend (Fallback)

```go
// internal/sandbox/backends/apparmor_linux.go
package backends

// AppArmorBackend - nur wenn Docker nicht verfügbar
type AppArmorBackend struct {
    profile     string
    aaExecPath  string
    logger      *slog.Logger
}

func (b *AppArmorBackend) IsAvailable() bool {
    // Prüfe: aa-exec verfügbar?
    _, err := exec.LookPath("aa-exec")
    if err != nil {
        return false
    }
    
    // Prüke: Profil geladen?
    // WICHTIG: Wir prüfen NICHT ob unser spezifisches Profil existiert
    // Das ist Aufgabe des Admins!
    profiles, _ := os.ReadFile("/sys/kernel/security/apparmor/profiles")
    return len(profiles) > 0
}

func (b *AppArmorBackend) Execute(ctx context.Context, req sandbox.ExecutionRequest) (*sandbox.ExecutionResult, error) {
    // Prüfe ob Profil existiert
    if !b.profileExists(b.profile) {
        return nil, fmt.Errorf(
            "AppArmor Profil '%s' nicht gefunden. " +
            "Bitte installieren: sudo cp %s /etc/apparmor.d/ && sudo apparmor_parser -r /etc/apparmor.d/%s",
            b.profile, b.profile, b.profile)
    }
    
    // Wrappe mit aa-exec
    args := append([]string{"-p", b.profile, "/bin/sh", "-c", req.Command}, req.Args...)
    cmd := exec.CommandContext(ctx, b.aaExecPath, args...)
    cmd.Dir = req.WorkDir
    
    output, err := cmd.CombinedOutput()
    
    return &sandbox.ExecutionResult{
        Stdout:    string(output),
        Stderr:    "",
        ExitCode:  cmd.ProcessState.ExitCode(),
        Backend:   "apparmor",
        Sandboxed: true,
    }, err
}

func (b *AppArmorBackend) profileExists(name string) bool {
    profiles, _ := os.ReadFile("/sys/kernel/security/apparmor/profiles")
    return strings.Contains(string(profiles), name)
}
```

**Setup-Anweisung für AppArmor (NICHT automatisch!):**
```bash
# 1. Profil erstellen: /etc/apparmor.d/aurago-sandbox
sudo tee /etc/apparmor.d/aurago-sandbox > /dev/null << 'EOF'
#include <tunables/global>

profile aurago-sandbox flags=(enforce) {
  #include <abstractions/base>
  #include <abstractions/python>
  #include <abstractions/bash>
  
  # Workspace (anpassen an tatsächlichen Pfad)
  /opt/aurago/agent_workspace/** rwk,
  /tmp/** rw,
  
  # System-Binaries
  /usr/bin/** ix,
  /bin/** ix,
  
  # Netzwerk (konfigurierbar)
  network inet stream,
  network inet6 stream,
  
  # Verboten
  deny /etc/shadow r,
  deny /etc/passwd r,
  deny /root/** r,
  deny /home/*/.ssh/** r,
  deny /proc/sys/** w,
  deny /sys/** w,
  deny capability sys_admin,
  deny capability sys_ptrace,
}
EOF

# 2. Profil laden
sudo apparmor_parser -r /etc/apparmor.d/aurago-sandbox

# 3. Verifizieren
sudo aa-status | grep aurago-sandbox
```

### 4.4 Windows-Backend (Korrigiert)

```go
// internal/sandbox/backends/jobobject_windows.go
package backends

import (
    "golang.org/x/sys/windows"
)

// JobObjectBackend - Windows-spezifisch
type JobObjectBackend struct {
    restrictUI      bool
    maxMemory       uint64
    maxProcesses    uint32
}

func (b *JobObjectBackend) IsAvailable() bool {
    // Job Objects sind immer verfügbar auf Windows
    return true
}

func (b *JobObjectBackend) Execute(ctx context.Context, req sandbox.ExecutionRequest) (*sandbox.ExecutionResult, error) {
    cmd := exec.CommandContext(ctx, "cmd", "/c", req.Command)
    cmd.Dir = req.WorkDir
    
    // Erstelle Job Object
    jobHandle, err := windows.CreateJobObject(nil, nil)
    if err != nil {
        return nil, fmt.Errorf("CreateJobObject failed: %w", err)
    }
    defer windows.CloseHandle(jobHandle)
    
    // Setze Limits
    limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
        BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
            LimitFlags: windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS |
                       windows.JOB_OBJECT_LIMIT_PROCESS_MEMORY |
                       windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
            ActiveProcessLimit: b.maxProcesses,
        },
        ProcessMemoryLimit: b.maxMemory,
    }
    
    err = windows.SetInformationJobObject(
        jobHandle,
        windows.JobObjectExtendedLimitInformation,
        unsafe.Pointer(&limits),
        uint32(unsafe.Sizeof(limits)),
    )
    if err != nil {
        return nil, err
    }
    
    // UI Restrictions (optional)
    if b.restrictUI {
        uiRestrictions := windows.JOBOBJECT_BASIC_UI_RESTRICTIONS{
            UIRestrictionsClass: windows.JOB_OBJECT_UILIMIT_DESKTOP |
                                windows.JOB_OBJECT_UILIMIT_EXITWINDOWS |
                                windows.JOB_OBJECT_UILIMIT_HANDLES,
        }
        windows.SetInformationJobObject(
            jobHandle,
            windows.JobObjectBasicUIRestrictions,
            unsafe.Pointer(&uiRestrictions),
            uint32(unsafe.Sizeof(uiRestrictions)),
        )
    }
    
    // Starte Prozess
    cmd.SysProcAttr = &syscall.SysProcAttr{
        CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_SUSPENDED,
    }
    
    if err := cmd.Start(); err != nil {
        return nil, err
    }
    
    // Weise Job zu
    processHandle, _ := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(cmd.Process.Pid))
    defer windows.CloseHandle(processHandle)
    
    windows.AssignProcessToJobObject(jobHandle, processHandle)
    
    // Resume
    threadHandle, _ := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, uint32(cmd.Process.Pid))
    windows.ResumeThread(threadHandle)
    windows.CloseHandle(threadHandle)
    
    // Warte
    err = cmd.Wait()
    
    return &sandbox.ExecutionResult{
        Stdout:    "",  // Umleitung nötig
        Sandboxed: true,
        Backend:   "job_objects",
    }, nil
}

// WICHTIG: Docker auf Windows nutzt WSL2-Backend
// Das ist langsamer, aber bietet echte Isolation
```

---

## 5. Auto-Detection (Korrigiert)

```go
// internal/sandbox/detection/detect.go
package detection

// DetectionResult enthält verfügbare Backends
type Result struct {
    Platform        string
    Recommended     string              // Bestes verfügbares Backend
    Available       []BackendInfo
    DockerAvailable bool
    SetupRequired   map[string]string   // Backend → Setup-Anweisung
}

type BackendInfo struct {
    Name        string
    Available   bool
    Priority    int
    Tier        int
    Description string
}

// Detect untersucht das System
func Detect() *Result {
    switch runtime.GOOS {
    case "linux":
        return detectLinux()
    case "darwin":
        return detectDarwin()
    case "windows":
        return detectWindows()
    default:
        return &Result{Platform: runtime.GOOS}
    }
}

func detectLinux() *Result {
    result := &Result{
        Platform:      "linux",
        Available:     []BackendInfo{},
        SetupRequired: make(map[string]string),
    }
    
    // 1. Docker (höchste Priorität = 1)
    if hasDocker() {
        result.Available = append(result.Available, BackendInfo{
            Name:        "docker",
            Available:   true,
            Priority:    1,
            Tier:        3,
            Description: "Docker Container (empfohlen)",
        })
        result.Recommended = "docker"
        result.DockerAvailable = true
    } else {
        result.SetupRequired["docker"] = "Installiere Docker: https://docs.docker.com/engine/install/"
    }
    
    // 2. AppArmor (niedrigere Priorität = 2, braucht Setup)
    if hasAppArmor() {
        hasProfile := hasAppArmorProfile("aurago-sandbox")
        result.Available = append(result.Available, BackendInfo{
            Name:        "apparmor",
            Available:   hasProfile,
            Priority:    2,
            Tier:        1,
            Description: "AppArmor MAC (Linux-native)",
        })
        if !hasProfile {
            result.SetupRequired["apparmor"] = "Profil erstellen: sudo tee /etc/apparmor.d/aurago-sandbox ..."
        }
        if result.Recommended == "" && hasProfile {
            result.Recommended = "apparmor"
        }
    }
    
    // 3. Seccomp (Priority 3)
    if hasSeccomp() {
        result.Available = append(result.Available, BackendInfo{
            Name:        "seccomp",
            Available:   true,
            Priority:    3,
            Tier:        2,
            Description: "Seccomp + Landlock",
        })
        if result.Recommended == "" {
            result.Recommended = "seccomp"
        }
    }
    
    return result
}

func detectDarwin() *Result {
    result := &Result{
        Platform:      "darwin",
        Available:     []BackendInfo{},
        SetupRequired: make(map[string]string),
    }
    
    // macOS: Docker ist primäre Option
    if hasDocker() {
        result.Available = append(result.Available, BackendInfo{
            Name:        "docker",
            Available:   true,
            Priority:    1,
            Tier:        3,
            Description: "Docker Desktop (empfohlen)",
        })
        result.Recommended = "docker"
        result.DockerAvailable = true
    } else {
        result.SetupRequired["docker"] = "Installiere Docker Desktop: https://docs.docker.com/desktop/mac/install/"
        
        // Fallback: Keine gute Sandbox verfügbar
        result.Available = append(result.Available, BackendInfo{
            Name:        "none",
            Available:   true,
            Priority:    99,
            Tier:        0,
            Description: "Keine Sandbox verfügbar (Docker empfohlen)",
        })
    }
    
    return result
}

func detectWindows() *Result {
    result := &Result{
        Platform:      "windows",
        Available:     []BackendInfo{},
        SetupRequired: make(map[string]string),
    }
    
    // 1. Docker (WSL2)
    if hasDocker() {
        result.Available = append(result.Available, BackendInfo{
            Name:        "docker",
            Available:   true,
            Priority:    1,
            Tier:        3,
            Description: "Docker Desktop (WSL2)",
        })
        result.Recommended = "docker"
        result.DockerAvailable = true
    } else {
        result.SetupRequired["docker"] = "Installiere Docker Desktop: https://docs.docker.com/desktop/windows/install/"
    }
    
    // 2. Job Objects (immer verfügbar, aber schwächer)
    result.Available = append(result.Available, BackendInfo{
        Name:        "job_objects",
        Available:   true,
        Priority:    2,
        Tier:        1,
        Description: "Windows Job Objects (eingeschränkt)",
    })
    
    if result.Recommended == "" {
        result.Recommended = "job_objects"
    }
    
    return result
}
```

---

## 6. Tool-Integration (Präzisiert)

### 6.1 Shell-Tool

```go
// internal/tools/shell.go

func ExecuteShell(command, workspaceDir string) (string, string, error) {
    // Prüfe Sandbox-Override
    if shouldUseSandbox("execute_shell") {
        return executeShellSandboxed(command, workspaceDir)
    }
    
    return executeShellDirect(command, workspaceDir)
}

func executeShellSandboxed(command, workspaceDir string) (string, string, error) {
    mgr := sandbox.GetManager()
    
    result, err := mgr.Execute(context.Background(), sandbox.ExecutionRequest{
        Tool:        "shell",
        Command:     command,
        WorkDir:     workspaceDir,
        NeedNetwork: true,
        Timeout:     ForegroundTimeout,
    })
    
    if err != nil {
        return "", result.Stderr, err
    }
    
    return result.Stdout, result.Stderr, nil
}

func shouldUseSandbox(tool string) bool {
    cfg := config.Get()
    
    // Globaler Toggle
    if !cfg.Sandbox.OS.Enabled {
        return false
    }
    
    // Tool-spezifischer Override
    switch tool {
    case "execute_shell":
        return cfg.Agent.SandboxOverride.ExecuteShell
    case "execute_python":
        return cfg.Agent.SandboxOverride.ExecutePython
    case "filesystem_write":
        return cfg.Agent.SandboxOverride.FilesystemWrite
    }
    
    return false
}
```

### 6.2 Datenpersistenz-Konzept

**Wichtig:** Die Sandbox muss Daten zurückgeben können!

```go
// Beispiel: Shell-Befehl erzeugt Datei

// 1. Host-Verzeichnis wird in Container gemountet
//    /opt/aurago/agent_workspace/workdir:/workspace:rw

// 2. Befehl läuft im Container, schreibt nach /workspace/output.txt

// 3. Datei ist sofort auf Host verfügbar unter 
//    /opt/aurago/agent_workspace/workdir/output.txt

// 4. Kein Kopieren nötig!
```

**Für temporäre Daten:**
```go
// Erstelle temporäres Verzeichnis auf Host
// Mounte es in Container
// Lösche nach Ausführung (optional)
```

---

## 7. Fehlerbehandlung & Fallbacks

```go
// internal/sandbox/manager.go

func (m *Manager) handleUnavailable(req ExecutionRequest) (*ExecutionResult, error) {
    switch m.config.Fallback.OnUnavailable {
    case "block":
        return nil, fmt.Errorf(
            "Sandbox nicht verfügbar und fallback 'block' konfiguriert. " +
            "Bitte Docker installieren oder Sandbox deaktivieren.")
    
    case "warn":
        slog.Warn("Sandbox nicht verfügbar - führe direkt aus",
            "tool", req.Tool,
            "command", req.Command)
        return m.executeDirect(req)
    
    case "allow":
        // Stilles Fallback
        return m.executeDirect(req)
    
    default:
        return m.executeDirect(req)
    }
}

func (m *Manager) executeDirect(req ExecutionRequest) (*ExecutionResult, error) {
    // Direkte Ausführung wie bisher
    switch req.Tool {
    case "shell":
        return executeShellDirect(req.Command, req.WorkDir)
    case "filesystem":
        return executeFilesystemDirect(req)
    }
    return nil, fmt.Errorf("unknown tool: %s", req.Tool)
}
```

---

## 8. UI-Integration (Vereinfacht)

```javascript
// ui/cfg/sandbox.js - Vereinfachte Config

{
  "section": "os_sandbox",
  "label": {
    "de": "OS-Sandbox",
    "en": "OS Sandbox"
  },
  "fields": [
    {
      "key": "sandbox.os.enabled",
      "type": "toggle",
      "label": {
        "de": "OS-Sandbox aktivieren",
        "en": "Enable OS Sandbox"
      },
      "help": {
        "de": "Shell-Befehle in isolierter Umgebung ausführen. Erfordert Docker (empfohlen) oder natives Linux-Security.",
        "en": "Execute shell commands in isolated environment. Requires Docker (recommended) or native Linux security."
      },
      "default": false
    },
    {
      "key": "_sandbox_status",
      "type": "readonly",
      "label": {
        "de": "Status",
        "en": "Status"
      },
      "value": "${api:sandbox.status}",  // API-Endpunkt
      "showIf": {"sandbox.os.enabled": true}
    },
    {
      "key": "sandbox.os.preferred_backend",
      "type": "select",
      "label": {
        "de": "Backend",
        "en": "Backend"
      },
      "help": {
        "de": "Leer lassen für automatische Erkennung",
        "en": "Leave empty for auto-detection"
      },
      "options": [
        {"value": "", "label": "Automatisch (empfohlen)"},
        {"value": "docker", "label": "Docker"},
        {"value": "apparmor", "label": "AppArmor (Linux only)"},
        {"value": "seccomp", "label": "Seccomp (Linux only)"},
        {"value": "job_objects", "label": "Job Objects (Windows only)"}
      ],
      "showIf": {"sandbox.os.enabled": true}
    }
  ]
}
```

---

## 9. Implementierungs-Roadmap (Korrigiert)

### Phase 1: Foundation & Docker (Woche 1-2)
- [ ] Sandbox-Package Struktur
- [ ] Backend-Interface
- [ ] Docker-Backend (plattformübergreifend)
- [ ] Container Pooling für Performance
- [ ] Auto-Detection Framework
- [ ] Shell-Tool Integration

### Phase 2: Linux-Native (Woche 3)
- [ ] AppArmor Backend (Setup-Doku, kein Auto-Install!)
- [ ] Seccomp Backend (optional)
- [ ] Detection & Setup-Anweisungen

### Phase 3: Windows (Woche 4)
- [ ] Job Objects Backend
- [ ] Windows Detection
- [ ] Testing

### Phase 4: UI & Integration (Woche 5)
- [ ] Config UI
- [ ] Status-Anzeige
- [ ] Setup-Wizard/Anleitungen
- [ ] Filesystem-Tool Integration

### Phase 5: Testing & Dokumentation (Woche 6)
- [ ] Integrationstests
- [ ] Performance-Tests
- [ ] Security-Dokumentation
- [ ] Setup-Guides pro Plattform

---

## 10. Setup-Guides (Wichtig!)

### Linux (Docker - Empfohlen)
```bash
# Schnell-Setup
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# Relogin
```

### Linux (AppArmor - Ohne Docker)
```bash
# 1. Profil erstellen
sudo tee /etc/apparmor.d/aurago-sandbox << 'EOF'
#include <tunables/global>
profile aurago-sandbox flags=(enforce) {
  #include <abstractions/base>
  /opt/aurago/agent_workspace/** rwk,
  /tmp/** rw,
  /usr/bin/** ix,
  /bin/** ix,
  network inet stream,
  deny /etc/shadow r,
  deny capability sys_admin,
}
EOF

# 2. Profil laden
sudo apparmor_parser -r /etc/apparmor.d/aurago-sandbox

# 3. Verifizieren
sudo aa-status | grep aurago-sandbox
```

### macOS
```bash
# Docker Desktop installieren
brew install --cask docker
# Oder manuell von https://www.docker.com/products/docker-desktop
```

### Windows
```powershell
# Docker Desktop installieren (WSL2 Backend)
winget install Docker.DockerDesktop
# Oder manuell von https://www.docker.com/products/docker-desktop
```

---

## Zusammenfassung der Korrekturen

| Aspekt | Ursprünglich | Korrigiert |
|--------|--------------|------------|
| **Auto-Install** | AppArmor auto-install | ❌ Entfernt - braucht root |
| **macOS Primär** | Seatbelt | ✅ Docker (plattformübergreifend) |
| **Windows Sandbox** | Windows Sandbox Feature | ❌ Entfernt - Session-basiert |
| **WSL2** | Als Backend | ❌ Entfernt - Docker nutzt es intern |
| **Performance** | Keine Lösung | ✅ Container Pooling |
| **Setup** | Automatisch | ✅ Anleitungen + Detection |
| **Default** | enabled: true | ✅ enabled: false (kein Breaking Change) |

**Die korrigierte Version ist:**
- ✅ Sicherer (kein automatisches root-Setup)
- ✅ Einheitlicher (Docker als primäre Option)
- ✅ Performanter (Container Pooling)
- ✅ Realistischer (keine deprecated Technologien)
- ✅ Benutzerfreundlicher (Klare Setup-Anleitungen)

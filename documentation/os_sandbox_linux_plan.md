# Native OS Sandbox - Linux Only

**Scope:** Nur Linux (kein macOS/Windows)  
**Ziel:** Maximale Konfigurierbarkeit mit automatischer Konflikterkennung  
**Priorität:** Sicherheit durch Isolation > Convenience

---

## 1. Warum Linux Only?

```
┌─────────────────────────────────────────────────────────────┐
│  Linux-Only Vorteile                                        │
├─────────────────────────────────────────────────────────────┤
│  ✅ Vollständige Namespaces (PID, Mount, Net, IPC, UTS,     │
│     User, Cgroup)                                          │
│  ✅ Seccomp-BPF für Syscall-Filterung                       │
│  ✅ Landlock LSM (Kernel 5.13+)                             │
│  ✅ AppArmor / SELinux Integration                          │
│  ✅ cgroups v2 für Ressourcen-Limits                        │
│  ✅ Pivot_root für echte Filesystem-Isolation               │
│  ✅ User Namespaces (unprivileged containers)               │
└─────────────────────────────────────────────────────────────┘
```

**Für macOS/Windows:** Docker bleibt die empfohlene Lösung (bereits vorhanden via `llm-sandbox`).

---

## 2. Konfiguration (Erweitert & Validierbar)

```yaml
sandbox:
  # Bestehend: Python llm-sandbox (Docker)
  enabled: true
  backend: "docker"
  
  # NEU: Linux Native Shell Sandbox
  os:
    enabled: false
    
    # Auswahl des Sandbox-Modus
    # "auto" = wählt beste verfügbare Kombination
    # "strict" = maximale Isolation (kann kompatibilität beeinträchtigen)
    # "custom" = manuelle Konfiguration aller Layer
    mode: "auto"
    
    # Auto-Modus Einstellungen
    auto:
      # Prioritätsreihenfolge für Auto-Selektion
      priority: ["namespace_seccomp_landlock", "namespace_seccomp", "namespace_basic", "chroot"]
      
      # Mindestanforderungen
      require:
        namespaces: true           # Mindestens Mount + PID NS
        seccomp: false             # Nicht zwingend erforderlich
        network_isolation: false   # Host-Netzwerk erlaubt
    
    # Custom-Modus: Detaillierte Layer-Konfiguration
    custom:
      # Layer 0: Namespaces (Basis)
      namespaces:
        enabled: true
        mount: true                # Mount namespace (FS-Isolation)
        pid: true                  # PID namespace (Prozess-Isolation)
        uts: true                  # UTS namespace (Hostname)
        ipc: true                  # IPC namespace (Shared Memory)
        net: "host"                # "host" | "isolated" | "restricted"
        user: true                 # User namespace (root-mapping)
        cgroup: false              # cgroup namespace
      
      # Layer 1: Capabilities
      capabilities:
        enabled: true
        drop_all: true             # Drop alle Capabilities
        add: []                    # Hinzufügen: ["NET_BIND_SERVICE"]
        ambient: []                # Ambient caps
      
      # Layer 2: Seccomp
      seccomp:
        enabled: true
        profile: "homelab"         # "homelab" | "strict" | "custom" | "disabled"
        custom_profile_path: ""    # Pfad zu custom .json
        default_action: "errno"    # "errno" | "kill" | "trap"
        
        # Zusätzliche Syscalls (zum Homelab-Profil)
        allow_extra: []            # ["ptrace"] für Debugging
        deny_extra: []             # Zusätzlich blockieren
      
      # Layer 3: Landlock (Kernel 5.13+)
      landlock:
        enabled: "auto"            # "auto" | "true" | "false"
        
        # Pfad-Regeln
        paths:
          read:
            - "/usr"
            - "/bin"
            - "/lib"
            - "/lib64"
            - "/opt/aurago/agent_workspace"
          write:
            - "/workspace"
            - "/tmp"
          execute:
            - "/bin/sh"
            - "/usr/bin/*"
      
      # Layer 4: AppArmor (optional, wenn verfügbar)
      apparmor:
        enabled: false
        profile: "aurago-sandbox"
        enforce: true              # true = enforce, false = complain
      
      # Layer 5: cgroups v2
      cgroups:
        enabled: true
        max_memory_mb: 512
        max_cpu_percent: 50
        max_pids: 50
        max_disk_io_mbps: 100
    
    # Ressourcen-Limits (unabhängig von cgroups)
    resources:
      max_execution_time: "30s"
      max_output_size_mb: 10
      max_open_files: 1024
    
    # Session-Verhalten
    session:
      enabled: true                # Persistente Shell-Session
      timeout_minutes: 30
      max_idle_seconds: 300
      
      # Session-Isolation
      preserve_env: false          # Umgebungsvariablen übernehmen?
      inherit_cwd: true            # Working Directory beibehalten?
      share_history: false         # Command-History teilen?

# ─────────────────────────────────────────────────────────────────
# DANGER ZONE - Muss mit Sandbox kompatibel sein!
# ─────────────────────────────────────────────────────────────────
agent:
  # Bereits existierend
  sudo_enabled: false            # ⚠️ Widerspruch zu Sandbox!
  
  # Sandbox-Overrides (werden validiert!)
  sandbox_override:
    execute_shell: true
    execute_python: true
    filesystem_write: true
```

---

## 3. Validierung & Konflikterkennung

### 3.1 Validierungs-Regeln

```go
// internal/sandbox/validation.go
package sandbox

type ValidationError struct {
    Field       string
    Message     string
    Severity    string  // "error" | "warning"
    Suggestion  string
}

// ValidateConfig prüft auf Widersprüche
func ValidateConfig(cfg *OSSandboxConfig) []ValidationError {
    var errors []ValidationError
    
    // ─────────────────────────────────────────────────────────────
    // 1. Danger Zone Konflikte
    // ─────────────────────────────────────────────────────────────
    
    if cfg.Enabled {
        // Sudo + Sandbox = Widerspruch
        if agentCfg.SudoEnabled {
            errors = append(errors, ValidationError{
                Field:      "agent.sudo_enabled",
                Message:    "sudo_enabled ist aktiv, aber sandbox.os.enabled ist auch aktiv",
                Severity:   "error",
                Suggestion: "sudo_enabled deaktivieren ODER sandbox.os.enabled deaktivieren. " +
                           "Sudo überschreibt die Sandbox-Isolation.",
            })
        }
        
        // Unrestricted Shell + Strict Sandbox
        if cfg.Custom.Namespaces.Net == "isolated" && 
           agentCfg.AllowNetworkRequests {
            errors = append(errors, ValidationError{
                Field:      "sandbox.os.custom.namespaces.net",
                Message:    "Netzwerk-Isolation aktiv, aber allow_network_requests ist true",
                Severity:   "warning",
                Suggestion: "Sandbox erlaubt keinen Netzwerkzugriff, " +
                           "Agent erwartet aber Netzwerk-Requests.",
            })
        }
        
        // Filesystem-Isolation + Filesystem-Write erlaubt
        if !cfg.Custom.Namespaces.Mount && 
           agentCfg.AllowFilesystemWrite {
            errors = append(errors, ValidationError{
                Field:      "sandbox.os.custom.namespaces.mount",
                Message:    "Mount namespace deaktiviert, aber filesystem_write erlaubt",
                Severity:   "warning",
                Suggestion: "Mount namespace aktivieren für FS-Isolation, " +
                           "oder Filesystem-Write deaktivieren.",
            })
        }
    }
    
    // ─────────────────────────────────────────────────────────────
    // 2. Sandbox-Interne Konflikte
    // ─────────────────────────────────────────────────────────────
    
    // Seccomp + Capabilities Widerspruch
    if cfg.Custom.Seccomp.Enabled && cfg.Custom.Capabilities.DropAll {
        // Prüfe ob seccomp Capabilities erwartet
        if contains(cfg.Custom.Seccomp.AllowExtra, "capset") {
            errors = append(errors, ValidationError{
                Field:      "sandbox.os.custom.seccomp.allow_extra",
                Message:    "capset in allow_extra, aber capabilities.drop_all ist true",
                Severity:   "error",
                Suggestion: "capset entfernen ODER drop_all deaktivieren.",
            })
        }
    }
    
    // Landlock ohne Mount namespace
    if cfg.Custom.Landlock.Enabled == "true" && !cfg.Custom.Namespaces.Mount {
        errors = append(errors, ValidationError{
            Field:      "sandbox.os.custom.landlock.enabled",
            Message:    "Landlock aktiviert ohne Mount namespace",
            Severity:   "warning",
            Suggestion: "Mount namespace aktivieren für korrekte Landlock-Isolation.",
        })
    }
    
    // AppArmor Profile existiert nicht
    if cfg.Custom.AppArmor.Enabled {
        if !apparmorProfileExists(cfg.Custom.AppArmor.Profile) {
            errors = append(errors, ValidationError{
                Field:      "sandbox.os.custom.apparmor.profile",
                Message:    fmt.Sprintf("AppArmor Profil '%s' nicht gefunden", cfg.Custom.AppArmor.Profile),
                Severity:   "error",
                Suggestion: "Profil erstellen mit: sudo apparmor_parser -r /etc/apparmor.d/" + cfg.Custom.AppArmor.Profile,
            })
        }
    }
    
    // cgroups v2 Verfügbarkeit
    if cfg.Custom.Cgroups.Enabled && !cgroupsV2Available() {
        errors = append(errors, ValidationError{
            Field:      "sandbox.os.custom.cgroups.enabled",
            Message:    "cgroups v2 nicht verfügbar",
            Severity:   "warning",
            Suggestion: "Kernel-Parameter prüfen: systemd.unified_cgroup_hierarchy=1",
        })
    }
    
    // ─────────────────────────────────────────────────────────────
    // 3. Kernel-Anforderungen
    // ─────────────────────────────────────────────────────────────
    
    kernelVersion := getKernelVersion()
    
    // Landlock benötigt Kernel 5.13+
    if cfg.Custom.Landlock.Enabled == "true" && kernelVersion < 5.13 {
        errors = append(errors, ValidationError{
            Field:      "sandbox.os.custom.landlock.enabled",
            Message:    fmt.Sprintf("Landlock benötigt Kernel 5.13+, aktuell: %.2f", kernelVersion),
            Severity:   "error",
            Suggestion: "Landlock deaktivieren oder Kernel aktualisieren.",
        })
    }
    
    // User namespaces
    if cfg.Custom.Namespaces.User && !userNamespacesAvailable() {
        errors = append(errors, ValidationError{
            Field:      "sandbox.os.custom.namespaces.user",
            Message:    "User namespaces nicht verfügbar",
            Severity:   "error",
            Suggestion: "Kernel-Config prüfen: CONFIG_USER_NS=y",
        })
    }
    
    // ─────────────────────────────────────────────────────────────
    // 4. Performance-Warnungen
    // ─────────────────────────────────────────────────────────────
    
    // Zu viele Layer
    layerCount := countEnabledLayers(cfg)
    if layerCount > 5 {
        errors = append(errors, ValidationError{
            Field:      "sandbox.os.custom",
            Message:    fmt.Sprintf("Viele Sandbox-Layer aktiv (%d), Performance-Impact möglich", layerCount),
            Severity:   "warning",
            Suggestion: "Nur notwendige Layer aktivieren für bessere Performance.",
        })
    }
    
    return errors
}

// ValidateRuntime prüft zur Laufzeit
func ValidateRuntime(cfg *OSSandboxConfig) error {
    // Prüfe ob Sandbox überhaupt funktionieren kann
    if !namespaceSupportAvailable() {
        return fmt.Errorf("Kernel unterstützt keine Namespaces")
    }
    
    // Prüfe unprivileged user namespaces (wichtig!)
    if cfg.Custom.Namespaces.User && !unprivilegedUserNSAllowed() {
        return fmt.Errorf("unprivileged user namespaces nicht erlaubt. " +
            "Prüfe /proc/sys/kernel/unprivileged_userns_clone oder AppArmor-Profile")
    }
    
    return nil
}
```

### 3.2 UI-Anzeige für Konflikte

```javascript
// Beispiel UI-Validierung
{
  "validation": {
    "errors": [
      {
        "field": "agent.sudo_enabled",
        "message": "sudo_enabled überschreibt Sandbox-Isolation",
        "severity": "error",
        "suggestion": "sudo_enabled deaktivieren für Sandbox-Nutzung"
      }
    ],
    "warnings": [
      {
        "field": "sandbox.os.custom.seccomp.enabled",
        "message": "Seccomp deaktiviert, reduzierte Sicherheit",
        "severity": "warning"
      }
    ]
  }
}
```

---

## 4. Sandbox-Layer-Architektur

```
┌─────────────────────────────────────────────────────────────────┐
│  Sandbox Stack (von innen nach außen)                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Layer 5: cgroups v2                                           │
│  ├── Memory-Limit: 512MB                                       │
│  ├── CPU-Limit: 50%                                            │
│  └── PID-Limit: 50                                             │
│                                                                 │
│  Layer 4: AppArmor (optional)                                  │
│  ├── MAC-Policy Enforcement                                    │
│  └── Profil: aurago-sandbox                                    │
│                                                                 │
│  Layer 3: Landlock (Kernel 5.13+)                              │
│  ├── Filesystem Access Control                                 │
│  ├── Read: /usr, /bin, /workspace                              │
│  └── Write: /workspace, /tmp                                   │
│                                                                 │
│  Layer 2: Seccomp-BPF                                          │
│  ├── Syscall Whitelist                                         │
│  ├── Block: mount, chroot, ptrace, capset                      │
│  └── Allow: read, write, exec, socket, ...                     │
│                                                                 │
│  Layer 1: Capabilities                                         │
│  ├── Drop: ALL                                                 │
│  └── Add: (none)                                               │
│                                                                 │
│  Layer 0: Namespaces                                           │
│  ├── Mount: Private FS view                                    │
│  ├── PID: Isolierte Prozess-Hierarchie                         │
│  ├── UTS: Eigener Hostname                                     │
│  ├── IPC: Isolierter Shared Memory                             │
│  ├── Net: Isoliertes Netzwerk (optional)                       │
│  └── User: UID/GID Mapping                                     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 5. Implementierungs-Klassen

### 5.1 Layer-Interface

```go
// internal/sandbox/layer.go
package sandbox

// Layer repräsentiert einen Sandbox-Layer
type Layer interface {
    Name() string
    Priority() int           // Ausführungsreihenfolge (0 = zuerst)
    IsAvailable() bool
    
    // Setup wird im Parent-Prozess aufgerufen
    Setup(config interface{}) error
    
    // Apply wird im Child-Prozess aufgerufen (vor exec)
    Apply() error
    
    // Cleanup nach Beendigung
    Cleanup() error
}

// LayerStack kombiniert mehrere Layer
type LayerStack struct {
    layers []Layer
}

func (s *LayerStack) Execute(command string) (*Result, error) {
    // 1. Setup alle Layer (Parent)
    for _, layer := range s.layers {
        if err := layer.Setup(nil); err != nil {
            return nil, fmt.Errorf("layer %s setup failed: %w", layer.Name(), err)
        }
    }
    
    // 2. Fork + Apply Layer (Child)
    cmd := exec.Command("/bin/sh", "-c", command)
    cmd.SysProcAttr = s.buildSysProcAttr()
    
    // Pre-Exec Hook für Layer
    // (werden in richtiger Reihenfolge angewendet)
    
    // 3. Ausführen
    return cmd.CombinedOutput()
}
```

### 5.2 Konkrete Layer-Implementierungen

```go
// internal/sandbox/layers/namespace.go
package layers

type NamespaceLayer struct {
    config NamespacesConfig
}

func (l *NamespaceLayer) Apply() error {
    // Wird via SysProcAttr.Cloneflags gesetzt
    // Zusätzliche Mounts hier
    
    if l.config.Mount {
        // Mount proc
        if err := unix.Mount("proc", "/proc", "proc", 0, ""); err != nil {
            return err
        }
        
        // Mount tmpfs auf /tmp
        if err := unix.Mount("tmpfs", "/tmp", "tmpfs", 0, "size=100m"); err != nil {
            return err
        }
        
        // Pivot root
        if err := l.setupPivotRoot(); err != nil {
            return err
        }
    }
    
    return nil
}

// internal/sandbox/layers/seccomp.go
package layers

type SeccompLayer struct {
    filter *SeccompFilter
}

func (l *SeccompLayer) Apply() error {
    return l.filter.Load()
}

// internal/sandbox/layers/landlock.go
package layers

type LandlockLayer struct {
    config LandlockConfig
}

func (l *LandlockLayer) Apply() error {
    if !landlockSupported() {
        return nil  // Silent skip
    }
    
    ll, err := sandbox.NewLandlockSandbox()
    if err != nil {
        return err
    }
    
    // Pfade erlauben
    for _, path := range l.config.Paths.Read {
        ll.AllowPath(path, unix.LANDLOCK_ACCESS_FS_READ_FILE|unix.LANDLOCK_ACCESS_FS_READ_DIR)
    }
    for _, path := range l.config.Paths.Write {
        ll.AllowPath(path, unix.LANDLOCK_ACCESS_FS_WRITE_FILE|unix.LANDLOCK_ACCESS_FS_MAKE_DIR|unix.LANDLOCK_ACCESS_FS_REMOVE_FILE)
    }
    
    return ll.Enforce()
}

// internal/sandbox/layers/apparmor.go
package layers

type AppArmorLayer struct {
    profile string
}

func (l *AppArmorLayer) Setup(config interface{}) error {
    // AppArmor wird via aa-exec vor dem Binary gesetzt
    // Kein Setup nötig, nur Validierung
    return nil
}

func (l *AppArmorLayer) Apply() error {
    // AppArmor wird via Exec übernommen
    return nil
}
```

---

## 6. Session-Management

### 6.1 Persistente Shell-Session

```go
// internal/sandbox/session_manager.go
package sandbox

// SessionManager verwaltet aktive Sandbox-Sessions
type SessionManager struct {
    sessions map[string]*ShellSession
    mu       sync.RWMutex
    config   *OSSandboxConfig
}

// GetOrCreate gibt existierende Session zurück oder erstellt neue
func (m *SessionManager) GetOrCreate(workDir string) (*ShellSession, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Prüfe ob Session existiert
    if session, ok := m.sessions[workDir]; ok {
        if session.IsAlive() {
            session.Touch()  // Reset idle timer
            return session, nil
        }
        // Session tot, entfernen
        delete(m.sessions, workDir)
    }
    
    // Neue Session erstellen
    session, err := m.createSession(workDir)
    if err != nil {
        return nil, err
    }
    
    m.sessions[workDir] = session
    return session, nil
}

// Cleanup beendet abgelaufene Sessions
func (m *SessionManager) Cleanup() {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    for id, session := range m.sessions {
        if session.IsExpired() {
            session.Close()
            delete(m.sessions, id)
        }
    }
}
```

---

## 7. Auto-Detection

```go
// internal/sandbox/detector.go
package sandbox

// DetectionResult enthält verfügbare Features
type DetectionResult struct {
    KernelVersion       float64
    
    Namespaces          map[string]bool  // mount, pid, net, ipc, uts, user, cgroup
    Seccomp             bool
    SeccompFilter       bool  // BPF-Filter möglich
    Landlock            bool
    LandlockABI         int
    AppArmor            bool
    AppArmorProfiles    []string
    SELinux             bool
    CgroupsV2           bool
    UserNSUnprivileged  bool
    
    RecommendedStack    []string
}

// Detect untersucht das System
func Detect() *DetectionResult {
    r := &DetectionResult{
        Namespaces: make(map[string]bool),
    }
    
    // Kernel-Version
    r.KernelVersion = getKernelVersion()
    
    // Namespaces prüfen
    for _, ns := range []string{"mnt", "pid", "net", "ipc", "uts", "user", "cgroup"} {
        r.Namespaces[ns] = namespaceAvailable(ns)
    }
    
    // Seccomp
    r.Seccomp = fileExists("/proc/sys/kernel/seccomp")
    r.SeccompFilter = r.Seccomp  // Wenn seccomp da, ist Filter meist auch da
    
    // Landlock
    if r.KernelVersion >= 5.13 {
        abi, err := unix.LandlockCreateRuleset(nil, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
        if err == nil && abi >= 1 {
            r.Landlock = true
            r.LandlockABI = int(abi)
        }
    }
    
    // AppArmor
    r.AppArmor = fileExists("/sys/kernel/security/apparmor")
    if r.AppArmor {
        r.AppArmorProfiles = listAppArmorProfiles()
    }
    
    // cgroups
    r.CgroupsV2 = fileExists("/sys/fs/cgroup/cgroup.controllers")
    
    // Unprivileged User NS
    r.UserNSUnprivileged = userNSUnprivilegedAllowed()
    
    // Empfehlung erstellen
    r.RecommendedStack = r.buildRecommendation()
    
    return r
}

func (r *DetectionResult) buildRecommendation() []string {
    var stack []string
    
    // Basis: Namespaces
    if r.Namespaces["mnt"] && r.Namespaces["pid"] {
        stack = append(stack, "namespaces")
    }
    
    // Seccomp hinzufügen
    if r.Seccomp {
        stack = append(stack, "seccomp")
    }
    
    // Landlock wenn verfügbar
    if r.Landlock {
        stack = append(stack, "landlock")
    }
    
    // AppArmor als zusätzliche Schicht
    if r.AppArmor {
        stack = append(stack, "apparmor")
    }
    
    // cgroups für Ressourcen
    if r.CgroupsV2 {
        stack = append(stack, "cgroups")
    }
    
    return stack
}
```

---

## 8. UI-Integration

### 8.1 Status-Endpoint

```json
GET /api/sandbox/status

{
  "enabled": true,
  "mode": "custom",
  "platform": "linux",
  "detection": {
    "kernel_version": 6.5,
    "features": {
      "namespaces": ["mount", "pid", "net", "ipc", "uts", "user"],
      "seccomp": true,
      "landlock": true,
      "apparmor": true,
      "cgroups_v2": true
    },
    "recommended_stack": ["namespaces", "seccomp", "landlock", "cgroups"]
  },
  "active_stack": ["namespaces", "seccomp", "landlock"],
  "session": {
    "active": true,
    "uptime_seconds": 120,
    "executed_commands": 5
  },
  "validation": {
    "errors": [],
    "warnings": [
      {
        "field": "sandbox.os.custom.apparmor.enabled",
        "message": "AppArmor Profil nicht gefunden",
        "suggestion": "Profil mit 'sudo apparmor_parser -r /etc/apparmor.d/aurago-sandbox' laden"
      }
    ]
  }
}
```

---

## 9. Roadmap (Linux Only)

### Phase 1: Foundation (Woche 1)
- [ ] Layer-Interface
- [ ] Namespace-Layer
- [ ] Detection-System
- [ ] Config-Validierung

### Phase 2: Security Layer (Woche 2)
- [ ] Seccomp-Layer
- [ ] Capabilities-Layer
- [ ] Landlock-Layer
- [ ] Konflikterkennung

### Phase 3: Session & Integration (Woche 3)
- [ ] Session-Manager
- [ ] Shell-Tool Integration
- [ ] Validation-Endpunkte
- [ ] UI-Integration

### Phase 4: Optional Features (Woche 4)
- [ ] AppArmor-Layer
- [ ] cgroups-Layer
- [ ] Performance-Optimierung
- [ ] Dokumentation

---

## 10. Zusammenfassung

### Was ist anders?

| Aspekt | Vorher | Nachher |
|--------|--------|---------|
| **Plattformen** | Linux, macOS, Windows | **Nur Linux** |
| **Docker** | Als Option | **Nicht vorhanden** (hat ja llm-sandbox) |
| **Konfiguration** | Einfach | **Erweitert + Validierbar** |
| **Layer** | Fest | **Wählbar & Kombinierbar** |
| **Konflikte** | Ignoriert | **Erkannt & Gemeldet** |

### Hauptmerkmale

1. **6 isolierbare Layer:** Namespaces, Capabilities, Seccomp, Landlock, AppArmor, cgroups
2. **Auto-Detection:** Erkennt Kernel-Fähigkeiten automatisch
3. **Validierung:** Prüft auf Widersprüche (sudo + sandbox, etc.)
4. **Session-Modus:** Persistente Shell-Sessions in Sandbox
5. **3 Modi:** Auto (Beste Wahl), Strict (Maximum), Custom (Vollständige Kontrolle)

### Konflikte die erkannt werden

- ✅ `sudo_enabled` + `sandbox.enabled` = Fehler
- ✅ `allow_network_requests` + `net=isolated` = Warnung
- ✅ `seccomp.enabled` + Capabilities-Widerspruch = Fehler
- ✅ `landlock.enabled` + Kernel < 5.13 = Fehler
- ✅ AppArmor-Profil nicht geladen = Fehler

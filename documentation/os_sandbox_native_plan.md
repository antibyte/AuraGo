# Native OS Sandbox - Überarbeitetes Konzept

**Ziel:** Shell-Befehle nativ isoliert ausführen (ohne Docker)  
**Philosophie:** Leichtgewichtig, schnell, plattformspezifisch  
**Nicht-Ziel:** Kein Docker-Backend (bereits durch llm-sandbox abgedeckt)

---

## 1. Warum keine Docker-Sandbox für Shell?

```
Problem: Docker-Stacking
┌─────────────────────────────────────┐
│  Host OS                            │
│  ┌───────────────────────────────┐  │
│  │  AuraGo Binary                │  │
│  │  ┌─────────────────────────┐  │  │
│  │  │  llm-sandbox (Docker)   │  │  │  ← Bereits vorhanden
│  │  │  ┌───────────────────┐  │  │  │
│  │  │  │  Python Code      │  │  │  │
│  │  │  └───────────────────┘  │  │  │
│  │  └─────────────────────────┘  │  │
│  │                               │  │
│  │  ┌─────────────────────────┐  │  │  ← NEU: OS-Sandbox
│  │  │  Shell-Sandbox (Docker) │  │  │     ❌ Nicht gut!
│  │  │  ┌───────────────────┐  │  │  │     → Docker-in-Docker
│  │  │  │  Shell Befehle    │  │  │  │     → Overhead
│  │  │  └───────────────────┘  │  │  │     → Komplexität
│  │  └─────────────────────────┘  │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘

Stattdessen: Native Isolation
┌─────────────────────────────────────┐
│  Host OS                            │
│  ┌───────────────────────────────┐  │
│  │  AuraGo Binary                │  │
│  │  ┌─────────────────────────┐  │  │
│  │  │  llm-sandbox (Docker)   │  │  │  ← Für Python bleibt
│  │  └─────────────────────────┘  │  │
│  │                               │  │
│  │  ┌─────────────────────────┐  │  │  ← NEU: Native OS-Sandbox
│  │  │  Namespace + Seccomp    │  │  │     ✅ Schnell (~10ms)
│  │  │  ↓                     │  │  │     ✅ Kein Docker
│  │  │  Shell Prozess          │  │  │     ✅ Echte Isolation
│  │  └─────────────────────────┘  │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
```

---

## 2. Neue Architektur

### 2.1 Design-Prinzipien

1. **Kein Docker** - Nutzt native OS-Features
2. **Prozess-Isolation** - Jeder Befehl in frischem Namespace
3. **Schnell** - < 50ms Overhead pro Befehl
4. **Session-fähig** - Shell-Session in persistenter Sandbox möglich

### 2.2 Backend-Matrix (ohne Docker)

| Plattform | Backend | Technologie | Isolation | Geschwindigkeit |
|-----------|---------|-------------|-----------|-----------------|
| **Linux** | `namespace_seccomp` | PID/Mount/UTS/IPC NS + Seccomp-BPF | Hoch | ~10-20ms |
| **Linux** | `chroot` | chroot + User NS (Fallback) | Mittel | ~5ms |
| **macOS** | `seatbelt` | sandbox-exec (deprecated aber funktional) | Mittel | ~10ms |
| **macOS** | `chroot` | chroot (eingeschränkt) | Niedrig | ~5ms |
| **Windows** | `jobobject` | Job Objects + Integrity Level | Mittel | ~10ms |
| **Windows** | `appcontainer` | AppContainer (Win 10+) | Hoch | ~20ms |

### 2.3 Config (Minimal)

```yaml
sandbox:
  # Bereits existierend: Python llm-sandbox
  enabled: true
  backend: "docker"
  
  # NEU: Native OS-Sandbox für Shell
  shell:
    enabled: false                    # Toggle
    backend: "auto"                   # "auto" | "namespace" | "seatbelt" | "jobobject" | ...
    
    # Linux-spezifisch
    linux:
      use_seccomp: true               # Seccomp-BPF Filter
      use_landlock: true              # Landlock LSM (Kernel 5.13+)
      use_apparmor: false             # Optional, wenn verfügbar
      
      # Ressourcen-Limits
      max_memory_mb: 512
      max_cpu_percent: 50
      max_processes: 50
      
      # Netzwerk
      network_mode: "host"            # "host" | "none" | "restricted"
      
    # Session-Modus (optional)
    session:
      enabled: true                   # Persistente Shell-Session
      timeout_minutes: 30             # Auto-cleanup
```

---

## 3. Linux-Implementierung (Priorität 1)

### 3.1 Namespace + Seccomp Sandbox

```go
// internal/sandbox/shell_linux.go
package sandbox

import (
    "context"
    "os/exec"
    "syscall"
    "golang.org/x/sys/unix"
)

// ShellSandbox führt Shell-Befehle isoliert aus
type ShellSandbox struct {
    config *ShellSandboxConfig
    seccomp *SeccompFilter
}

// Execute führt einen Befehl in isolierter Umgebung aus
func (s *ShellSandbox) Execute(ctx context.Context, command string, args []string) (*Result, error) {
    cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
    
    // Setup Namespaces
    cmd.SysProcAttr = &syscall.SysProcAttr{
        // Neue Namespaces erstellen
        Cloneflags: syscall.CLONE_NEWNS |      // Mount namespace (Filesystem-Isolation)
                    syscall.CLONE_NEWPID |      // PID namespace (Prozess-Isolation)
                    syscall.CLONE_NEWUTS |      // UTS namespace (Hostname)
                    syscall.CLONE_NEWIPC |      // IPC namespace (Shared Memory, Semaphoren)
                    syscall.CLONE_NEWNET,       // Network namespace (optional)
        
        // Private mounts - verhindert Propagation zum Host
        Unshareflags: syscall.CLONE_NEWNS,
        
        // UID/GID Mapping für User Namespace
        UidMappings: []syscall.SysProcIDMap{
            {ContainerID: 0, HostID: os.Getuid(), Size: 1},
        },
        GidMappings: []syscall.SysProcIDMap{
            {ContainerID: 0, HostID: os.Getgid(), Size: 1},
        },
        
        // Keine Privilegien-Eskalation
        NoNewPrivileges: true,
    }
    
    // Pre-Exec Hook für Seccomp
    if s.config.UseSeccomp {
        cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
        // Seccomp wird über cgo oder external binary geladen
    }
    
    // Mounts setup (im Child-Prozess vor exec)
    // Siehe unten: setupMounts
    
    return cmd.CombinedOutput()
}
```

### 3.2 Mount-Setup (Pivot Root)

```go
// setupMounts bereitet das Filesystem im Container vor
func setupMounts(rootfs string, workDir string) error {
    // 1. Temporäres RootFS erstellen (Overlay oder tmpfs)
    tmpRoot, err := os.MkdirTemp("", "aurago-sandbox-*")
    if err != nil {
        return err
    }
    defer os.RemoveAll(tmpRoot)
    
    // 2. Essentialle Verzeichnisse mounten
    mounts := []struct {
        source string
        target string
        fstype string
        flags  uintptr
        opts   string
    }{
        // Proc
        {"proc", filepath.Join(tmpRoot, "proc"), "proc", unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV, ""},
        // Sys (read-only)
        {"sysfs", filepath.Join(tmpRoot, "sys"), "sysfs", unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV | unix.MS_RDONLY, ""},
        // Tmp
        {"tmpfs", filepath.Join(tmpRoot, "tmp"), "tmpfs", unix.MS_NOSUID | unix.MS_NODEV, "size=100m"},
        // Workspace (Host-Pfad gemountet)
        {workDir, filepath.Join(tmpRoot, "workspace"), "none", unix.MS_BIND | unix.MS_REC, ""},
    }
    
    for _, m := range mounts {
        target := m.target
        if err := os.MkdirAll(target, 0755); err != nil {
            return err
        }
        if err := unix.Mount(m.source, target, m.fstype, m.flags, m.opts); err != nil {
            return fmt.Errorf("mount %s failed: %w", m.target, err)
        }
    }
    
    // 3. Pivot Root
    // Wechseln in neue Root
    if err := unix.PivotRoot(tmpRoot, filepath.Join(tmpRoot, ".old_root")); err != nil {
        return fmt.Errorf("pivot_root failed: %w", err)
    }
    
    // Altes Root unmounten
    unix.Unmount("/.old_root", unix.MNT_DETACH)
    
    return nil
}
```

### 3.3 Seccomp-BPF Filter

```go
// internal/sandbox/seccomp_linux.go
package sandbox

// #cgo LDFLAGS: -lseccomp
// #include <seccomp.h>
// #include <stdlib.h>
import "C"

// SeccompFilter definiert erlaubte Syscalls
type SeccompFilter struct {
    ctx C.scmp_filter_ctx
}

// NewSeccompFilter erstellt einen Homelab-Filter
func NewSeccompFilter() (*SeccompFilter, error) {
    // Default: Blockieren
    ctx := C.seccomp_init(C.SCMP_ACT_ERRNO(C.EPERM))
    if ctx == nil {
        return nil, fmt.Errorf("seccomp_init failed")
    }
    
    f := &SeccompFilter{ctx: ctx}
    
    // Erlaube essentielle Syscalls
    essential := []C.int{
        C.SCMP_SYS(read),
        C.SCMP_SYS(write),
        C.SCMP_SYS(open),
        C.SCMP_SYS(openat),
        C.SCMP_SYS(close),
        C.SCMP_SYS(stat),
        C.SCMP_SYS(fstat),
        C.SCMP_SYS(lstat),
        C.SCMP_SYS(poll),
        C.SCMP_SYS(lseek),
        C.SCMP_SYS(mmap),
        C.SCMP_SYS(mprotect),
        C.SCMP_SYS(munmap),
        C.SCMP_SYS(brk),
        C.SCMP_SYS(rt_sigaction),
        C.SCMP_SYS(rt_sigprocmask),
        C.SCMP_SYS(ioctl),
        C.SCMP_SYS(pread64),
        C.SCMP_SYS(pwrite64),
        C.SCMP_SYS(readv),
        C.SCMP_SYS(writev),
        C.SCMP_SYS(access),
        C.SCMP_SYS(pipe),
        C.SCMP_SYS(pipe2),
        C.SCMP_SYS(select),
        C.SCMP_SYS(dup),
        C.SCMP_SYS(dup2),
        C.SCMP_SYS(dup3),
        C.SCMP_SYS(fcntl),
        C.SCMP_SYS(flock),
        C.SCMP_SYS(fsync),
        C.SCMP_SYS(fdatasync),
        C.SCMP_SYS(truncate),
        C.SCMP_SYS(ftruncate),
        C.SCMP_SYS(getdents),
        C.SCMP_SYS(getcwd),
        C.SCMP_SYS(chdir),
        C.SCMP_SYS(fchdir),
        C.SCMP_SYS(rename),
        C.SCMP_SYS(mkdir),
        C.SCMP_SYS(rmdir),
        C.SCMP_SYS(creat),
        C.SCMP_SYS(link),
        C.SCMP_SYS(unlink),
        C.SCMP_SYS(symlink),
        C.SCMP_SYS(readlink),
        C.SCMP_SYS(chmod),
        C.SCMP_SYS(fchmod),
        C.SCMP_SYS(chown),
        C.SCMP_SYS(fchown),
        C.SCMP_SYS(lchown),
        C.SCMP_SYS(umask),
        C.SCMP_SYS(gettimeofday),
        C.SCMP_SYS(clock_gettime),
        C.SCMP_SYS(exit),
        C.SCMP_SYS(exit_group),
        C.SCMP_SYS(wait4),
        C.SCMP_SYS(kill),
        C.SCMP_SYS(tgkill),
        C.SCMP_SYS(clone),      // Für Threads
        C.SCMP_SYS(fork),
        C.SCMP_SYS(vfork),
        C.SCMP_SYS(execve),
        C.SCMP_SYS(execveat),
        C.SCMP_SYS(getpid),
        C.SCMP_SYS(getppid),
        C.SCMP_SYS(getpgrp),
        C.SCMP_SYS(setsid),
        C.SCMP_SYS(setpgid),
        C.SCMP_SYS(getuid),
        C.SCMP_SYS(geteuid),
        C.SCMP_SYS(getgid),
        C.SCMP_SYS(getegid),
        C.SCMP_SYS(gettid),
        C.SCMP_SYS(sysinfo),
        C.SCMP_SYS(uname),
        C.SCMP_SYS(prctl),      // Für seccomp selbst
        C.SCMP_SYS(arch_prctl),
        C.SCMP_SYS(getrlimit),
        C.SCMP_SYS(setrlimit),
        C.SCMP_SYS(prlimit64),
        C.SCMP_SYS(ptrace),     // ❌ Achtung: Gefährlich, evtl. blockieren
        // Netzwerk (optional, je nach Config)
        C.SCMP_SYS(socket),
        C.SCMP_SYS(socketpair),
        C.SCMP_SYS(connect),
        C.SCMP_SYS(accept),
        C.SCMP_SYS(bind),
        C.SCMP_SYS(listen),
        C.SCMP_SYS(sendto),
        C.SCMP_SYS(recvfrom),
        C.SCMP_SYS(sendmsg),
        C.SCMP_SYS(recvmsg),
        C.SCMP_SYS(shutdown),
        C.SCMP_SYS(getsockname),
        C.SCMP_SYS(getpeername),
        C.SCMP_SYS(getsockopt),
        C.SCMP_SYS(setsockopt),
    }
    
    for _, syscall := range essential {
        C.seccomp_rule_add(ctx, C.SCMP_ACT_ALLOW, syscall, 0)
    }
    
    // Blockiere gefährliche Syscalls explizit
    dangerous := []C.int{
        C.SCMP_SYS(mount),
        C.SCMP_SYS(umount2),
        C.SCMP_SYS(pivot_root),
        C.SCMP_SYS(chroot),
        C.SCMP_SYS(setuid),
        C.SCMP_SYS(setgid),
        C.SCMP_SYS(setreuid),
        C.SCMP_SYS(setregid),
        C.SCMP_SYS(setresuid),
        C.SCMP_SYS(setresgid),
        C.SCMP_SYS(capset),
        C.SCMP_SYS(capget),
        C.SCMP_SYS(open_by_handle_at),
        C.SCMP_SYS(iopl),
        C.SCMP_SYS(ioperm),
        C.SCMP_SYS(swapon),
        C.SCMP_SYS(swapoff),
        C.SCMP_SYS(reboot),
        C.SCMP_SYS(kexec_load),
        C.SCMP_SYS(kexec_file_load),
    }
    
    for _, syscall := range dangerous {
        C.seccomp_rule_add(ctx, C.SCMP_ACT_KILL, syscall, 0)
    }
    
    return f, nil
}

func (f *SeccompFilter) Load() error {
    if ret := C.seccomp_load(f.ctx); ret != 0 {
        return fmt.Errorf("seccomp_load failed: %d", ret)
    }
    return nil
}

func (f *SeccompFilter) Release() {
    C.seccomp_release(f.ctx)
}
```

### 3.4 Landlock (Optional, Linux 5.13+)

```go
// internal/sandbox/landlock_linux.go
package sandbox

// Landlock restrictiert Dateisystem-Zugriffe
type LandlockSandbox struct {
    rulesetFd int
}

func NewLandlockSandbox() (*LandlockSandbox, error) {
    // Prüfe Landlock-Version
    abi, err := unix.LandlockCreateRuleset(nil, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
    if err != nil {
        return nil, fmt.Errorf("landlock not supported: %w", err)
    }
    if abi < 1 {
        return nil, fmt.Errorf("landlock ABI too old: %d", abi)
    }
    
    // Erstelle Ruleset
    attr := unix.LandlockRulesetAttr{
        HandledAccessFs: unix.LANDLOCK_ACCESS_FS_READ_FILE |
            unix.LANDLOCK_ACCESS_FS_WRITE_FILE |
            unix.LANDLOCK_ACCESS_FS_READ_DIR |
            unix.LANDLOCK_ACCESS_FS_MAKE_DIR |
            unix.LANDLOCK_ACCESS_FS_REMOVE_FILE |
            unix.LANDLOCK_ACCESS_FS_MAKE_CHAR |
            unix.LANDLOCK_ACCESS_FS_MAKE_BLOCK |
            unix.LANDLOCK_ACCESS_FS_MAKE_SOCK |
            unix.LANDLOCK_ACCESS_FS_MAKE_FIFO |
            unix.LANDLOCK_ACCESS_FS_MAKE_SYM |
            unix.LANDLOCK_ACCESS_FS_REFER,
    }
    
    fd, err := unix.LandlockCreateRuleset(&attr, int(unsafe.Sizeof(attr)), 0)
    if err != nil {
        return nil, err
    }
    
    return &LandlockSandbox{rulesetFd: fd}, nil
}

func (l *LandlockSandbox) AllowPath(path string, access uint64) error {
    fd, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC, 0)
    if err != nil {
        return err
    }
    defer unix.Close(fd)
    
    rule := unix.LandlockPathBeneathAttr{
        AllowedAccess: access,
        ParentFd:      uint64(fd),
    }
    
    return unix.LandlockAddRule(l.rulesetFd, unix.LANDLOCK_RULE_PATH_BENEATH, &rule, 0)
}

func (l *LandlockSandbox) Enforce() error {
    // Restrict self
    return unix.LandlockRestrictSelf(l.rulesetFd, 0)
}
```

---

## 4. Session-Modus (Wichtig!)

Ein häufiges Problem: Der Agent führt mehrere Shell-Befehle aus, die voneinander abhängen (z.B. `cd`, dann `ls`, dann `cat`). Mit isolierten Prozessen geht das nicht.

### 4.1 Persistente Shell-Session

```go
// internal/sandbox/session.go
package sandbox

// ShellSession repräsentiert eine persistente Shell in Sandbox
type ShellSession struct {
    id       string
    cmd      *exec.Cmd
    stdin    io.WriteCloser
    stdout   io.ReadCloser
    stderr   io.ReadCloser
    workDir  string
    mu       sync.Mutex
    
    // Sandbox-Config
    sandbox  *ShellSandbox
}

// NewSession erstellt neue Sandbox-Session
func NewSession(workDir string, cfg *ShellSandboxConfig) (*ShellSession, error) {
    session := &ShellSession{
        id:      generateID(),
        workDir: workDir,
        sandbox: &ShellSandbox{config: cfg},
    }
    
    // Starte Shell-Prozess in Sandbox
    // Nutzt Namespaces wie bei Execute, aber interaktiv
    cmd := exec.Command("/bin/sh", "-i")  // Interactive
    
    // Setup Namespaces (wie oben)
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Cloneflags: syscall.CLONE_NEWNS | syscall.CLONE_NEWPID | 
                    syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC,
        // ...
    }
    
    // Pipes für Interaktion
    stdin, _ := cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()
    stderr, _ := cmd.StderrPipe()
    
    session.cmd = cmd
    session.stdin = stdin
    session.stdout = stdout
    session.stderr = stderr
    
    if err := cmd.Start(); err != nil {
        return nil, err
    }
    
    // Cleanup-Timer
    go session.autoCleanup(cfg.SessionTimeout)
    
    return session, nil
}

// Execute führt Befehl in dieser Session aus
func (s *ShellSession) Execute(command string) (string, string, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // Sende Befehl an Shell
    // Verwende Marker für Ende-Erkennung
    marker := fmt.Sprintf("___END_%d___", time.Now().UnixNano())
    
    fmt.Fprintf(s.stdin, "%s; echo %s; echo %s >&2\n", 
        command, marker, marker)
    
    // Lese Output bis Marker
    var stdoutBuf, stderrBuf strings.Builder
    // ... Parsing-Logik
    
    return stdoutBuf.String(), stderrBuf.String(), nil
}

// Cleanup
func (s *ShellSession) Close() error {
    if s.cmd != nil && s.cmd.Process != nil {
        s.cmd.Process.Kill()
    }
    return nil
}
```

---

## 5. macOS-Implementierung

### 5.1 Seatbelt (trotz deprecated - funktioniert noch)

```go
// internal/sandbox/shell_darwin.go
package sandbox

import (
    "os"
    "os/exec"
)

// SeatbeltSandbox nutzt sandbox-exec
type SeatbeltSandbox struct {
    profilePath string
}

func NewSeatbeltSandbox(workDir string) (*SeatbeltSandbox, error) {
    // Generiere Profil
    profile := generateSeatbeltProfile(workDir)
    
    tmpFile, err := os.CreateTemp("", "aurago-sandbox-*.sb")
    if err != nil {
        return nil, err
    }
    
    if _, err := tmpFile.WriteString(profile); err != nil {
        tmpFile.Close()
        os.Remove(tmpFile.Name())
        return nil, err
    }
    tmpFile.Close()
    
    return &SeatbeltSandbox{profilePath: tmpFile.Name()}, nil
}

func (s *SeatbeltSandbox) Execute(ctx context.Context, command string) (string, error) {
    cmd := exec.CommandContext(ctx, "sandbox-exec", 
        "-f", s.profilePath,
        "/bin/sh", "-c", command)
    
    output, err := cmd.CombinedOutput()
    return string(output), err
}

func (s *SeatbeltSandbox) Cleanup() {
    os.Remove(s.profilePath)
}

func generateSeatbeltProfile(workDir string) string {
    return fmt.Sprintf(`(version 1)
(debug deny)

; Standard-Rechte
(allow default)

; Prozess-Ausführung
(allow process-exec (regex "^/bin/.*" "^/usr/bin/.*" "^/sbin/.*" "^/usr/sbin/.*" "^/usr/local/bin/.*"))

; Workspace-Zugriff
(allow file-read* (regex "^%s/.*$" "^/usr/.*" "^/bin/.*" "^/System/.*" "^/Library/.*"))
(allow file-write* (regex "^%s/.*$" "^/tmp/.*" "^/var/tmp/.*"))
(allow file-read-metadata (regex ".*"))

; Netzwerk (optional)
(allow network-outbound)
(allow network-inbound)

; Verboten
(deny file-read* (regex "^/Users/.*/\\.ssh/.*" "^/Users/.*/\\.gnupg/.*" "^/private/etc/shadow" "^/private/etc/master.passwd"))
(deny file-write* (regex "^/System/.*" "^/usr/.*" "^/bin/.*" "^/sbin/.*"))
(deny process-exec (regex "^/usr/bin/sudo$" "^/bin/su$"))
`, regexp.QuoteMeta(workDir), regexp.QuoteMeta(workDir))
}
```

---

## 6. Windows-Implementierung

### 6.1 Job Objects + Integrity Level

```go
// internal/sandbox/shell_windows.go
package sandbox

import (
    "golang.org/x/sys/windows"
)

type JobObjectSandbox struct {
    maxMemory    uint64
    maxProcesses uint32
    jobHandle    windows.Handle
}

func NewJobObjectSandbox() (*JobObjectSandbox, error) {
    // Erstelle Job Object
    jobHandle, err := windows.CreateJobObject(nil, nil)
    if err != nil {
        return nil, err
    }
    
    // Setze Limits
    limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
        BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
            LimitFlags: windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS |
                       windows.JOB_OBJECT_LIMIT_PROCESS_MEMORY |
                       windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
            ActiveProcessLimit: 10,
        },
        ProcessMemoryLimit: 512 * 1024 * 1024, // 512MB
    }
    
    err = windows.SetInformationJobObject(
        jobHandle,
        windows.JobObjectExtendedLimitInformation,
        unsafe.Pointer(&limits),
        uint32(unsafe.Sizeof(limits)),
    )
    if err != nil {
        windows.CloseHandle(jobHandle)
        return nil, err
    }
    
    // UI Restrictions
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
    
    return &JobObjectSandbox{
        jobHandle:    jobHandle,
        maxMemory:    512 * 1024 * 1024,
        maxProcesses: 10,
    }, nil
}

func (j *JobObjectSandbox) Execute(command string) (string, error) {
    // Starte cmd.exe in niedrigem Integritätslevel
    cmd := exec.Command("cmd", "/c", command)
    
    // Setze niedrigen Integrity Level
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Token: createLowIntegrityToken(), // Siehe unten
    }
    
    // Starte suspendiert
    cmd.SysProcAttr.CreationFlags = windows.CREATE_SUSPENDED | windows.CREATE_NEW_PROCESS_GROUP
    
    if err := cmd.Start(); err != nil {
        return "", err
    }
    
    // Weise Job zu
    processHandle, _ := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(cmd.Process.Pid))
    defer windows.CloseHandle(processHandle)
    
    windows.AssignProcessToJobObject(j.jobHandle, processHandle)
    
    // Resume
    threadHandle, _ := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, uint32(cmd.Process.Pid))
    windows.ResumeThread(threadHandle)
    windows.CloseHandle(threadHandle)
    
    // Warte
    output, _ := cmd.CombinedOutput()
    return string(output), nil
}
```

---

## 7. Integration mit Tools

### 7.1 Shell-Tool Modifikation

```go
// internal/tools/shell.go

var (
    shellSession     *sandbox.ShellSession
    shellSessionMu   sync.Mutex
)

func ExecuteShell(command, workspaceDir string) (string, string, error) {
    // Prüfe ob Shell-Sandbox aktiviert
    if config.Get().Sandbox.Shell.Enabled {
        return executeInSandbox(command, workspaceDir)
    }
    
    // Direkte Ausführung (bestehend)
    return executeDirect(command, workspaceDir)
}

func executeInSandbox(command, workspaceDir string) (string, string, error) {
    shellSessionMu.Lock()
    defer shellSessionMu.Unlock()
    
    // Session erstellen falls nicht vorhanden
    if shellSession == nil {
        var err error
        cfg := &sandbox.ShellSandboxConfig{
            UseSeccomp:  true,
            UseLandlock: true,
            MaxMemoryMB: 512,
            NetworkMode: "host",
        }
        shellSession, err = sandbox.NewSession(workspaceDir, cfg)
        if err != nil {
            // Fallback zu direkter Ausführung
            slog.Warn("Sandbox-Session failed, falling back", "error", err)
            return executeDirect(command, workspaceDir)
        }
    }
    
    // Führe in Session aus
    stdout, stderr, err := shellSession.Execute(command)
    return stdout, stderr, err
}

// Cleanup beim Shutdown
func CleanupShellSession() {
    shellSessionMu.Lock()
    defer shellSessionMu.Unlock()
    if shellSession != nil {
        shellSession.Close()
        shellSession = nil
    }
}
```

---

## 8. Zusammenfassung

### Was ist neu?

1. **Kein Docker** - Nutzt native OS-Features (Namespaces, Seccomp, etc.)
2. **Session-Modus** - Persistente Shell-Session für aufeinanderfolgende Befehle
3. **Schnell** - ~10-20ms Overhead statt 100ms+ bei Docker
4. **Echte Isolation** - PID, Mount, Netzwerk Namespaces + Seccomp

### Backend-Präferenz

```go
Linux:   namespace_seccomp > chroot
macOS:   seatbelt > chroot  
Windows: jobobject
```

### Config

```yaml
sandbox:
  shell:
    enabled: false          # Toggle
    backend: "auto"         # Oder "namespace", "seatbelt", "jobobject"
    session:
      enabled: true         # Persistente Session
      timeout_minutes: 30
```

### Vorteile gegenüber Docker-Ansatz

- ✅ Kein Docker-in-Docker Problem
- ✅ Schneller (10-20ms vs 100ms)
- ✅ Weniger Ressourcenverbrauch
- ✅ Einfacher (keine Container-Images)
- ✅ Funktioniert auf Systemen ohne Docker

### Nachteile

- ❌ Plattformspezifischer Code (Linux/macOS/Windows)
- ❌ Linux: Benötigt seccomp/libseccomp
- ❌ Weniger isoliert als VM/Container (geteilter Kernel)
- ❌ Komplexeres Setup auf macOS (Seatbelt deprecated)

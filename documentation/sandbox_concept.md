# AuraGo Sandbox Architecture Concept

**Version:** 1.0  
**Date:** 2026-03-17  
**Scope:** Multi-tier sandbox system for AI agent execution  
**Target:** Homelab environments (Linux primary, cross-platform support)

---

## Executive Summary

This concept describes a **layered sandbox architecture** for AuraGo that replaces direct OS access with configurable, isolated execution environments. The system supports multiple isolation levels from lightweight (seccomp) to heavy (VMs), allowing users to choose security vs. convenience based on their threat model.

### Key Principles
1. **Defense in Depth** - Multiple sandbox layers can be combined
2. **Homelab-First** - Works on consumer hardware, no enterprise requirements
3. **Progressive Hardening** - Start permissive, tighten based on needs
4. **Zero-Trust by Default** - Agent doesn't trust the host, host doesn't trust the agent

---

## 1. Sandbox Tiers Overview

```
┌─────────────────────────────────────────────────────────────────┐
│ TIER 4: Virtual Machine (Firecracker/gVisor)                   │
│ • Complete kernel isolation                                    │
│ • ~100ms startup time                                          │
│ • Recommended for: Untrusted internet-facing agents            │
├─────────────────────────────────────────────────────────────────┤
│ TIER 3: Container (Docker/Podman + gVisor)                     │
│ • Process namespace isolation                                  │
│ • Filesystem overlay                                           │
│ • Recommended for: Standard tool execution                     │
├─────────────────────────────────────────────────────────────────┤
│ TIER 2: System Call Filtering (seccomp-bpf + Landlock)         │
│ • Kernel-enforced syscall allowlist                            │
│ • Filesystem sandboxing                                        │
│ • Recommended for: Direct Python/shell execution               │
├─────────────────────────────────────────────────────────────────┤
│ TIER 1: Application Sandbox (AppArmor/SELinux)                 │
│ • MAC (Mandatory Access Control)                               │
│ • Resource limits (cgroups v2)                                 │
│ • Recommended for: Baseline protection                         │
├─────────────────────────────────────────────────────────────────┤
│ TIER 0: Capability Drop + Namespaces                           │
│ • Minimal overhead                                             │
│ • Linux namespaces only                                        │
│ • Recommended for: Trusted environments                        │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Detailed Tier Specifications

### 2.1 Tier 0: Lightweight Namespaces (Baseline)

**Use Case:** Trusted homelab, quick scripts, minimal overhead

**Implementation:**
```go
// internal/sandbox/tier0_linux.go
package sandbox

import (
    "os/exec"
    "syscall"
)

type Tier0Sandbox struct {
    WorkDir string
    Env     map[string]string
}

func (s *Tier0Sandbox) Execute(command string, args ...string) (*exec.Cmd, error) {
    cmd := exec.Command(command, args...)
    
    // Linux namespaces
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Cloneflags: syscall.CLONE_NEWNS |      // Mount namespace
                    syscall.CLONE_NEWPID |      // PID namespace
                    syscall.CLONE_NEWNET |      // Network namespace (optional)
                    syscall.CLONE_NEWIPC,       // IPC namespace
        
        // Private mounts - prevent propagation to host
        Unshareflags: syscall.CLONE_NEWNS,
    }
    
    // Drop capabilities (keep only CAP_NET_BIND_SERVICE if needed)
    cmd.SysProcAttr.AmbientCaps = []uintptr{}
    cmd.SysProcAttr.InheritCaps = []uintptr{}
    
    // Pivot root to workdir
    cmd.Dir = s.WorkDir
    
    return cmd, nil
}
```

**Security Profile:**
- ✅ Process isolation (PID namespace)
- ✅ Filesystem isolation (Mount namespace)
- ✅ No root escalation (Capability drop)
- ❌ Shared kernel (vulnerable to kernel exploits)
- ❌ No syscall filtering

**Homelab Suitability:** Perfect for low-resource devices (Raspberry Pi, old NUCs)

---

### 2.2 Tier 1: MAC + cgroups (Recommended Baseline)

**Use Case:** Standard homelab, Docker/Podman users, moderate security

**Implementation:**

#### Linux: AppArmor Profile
```apparmor
# /etc/apparmor.d/aurago-sandbox
#include <tunables/global>

profile aurago-sandbox flags=(enforce, complain) {
  #include <abstractions/base>
  
  # Deny dangerous capabilities
  deny capability sys_admin,
  deny capability sys_ptrace,
  deny capability sys_module,
  deny capability dac_read_search,
  
  # Workspace access (read-write)
  /opt/aurago/agent_workspace/workdir/** rwk,
  
  # Tools directory (read-only)
  /opt/aurago/agent_workspace/tools/** r,
  /opt/aurago/agent_workspace/skills/** r,
  
  # Temp directory
  /tmp/** rw,
  
  # Allow specific binaries
  /usr/bin/python3 ix,
  /usr/bin/pip3 ix,
  /usr/bin/bash ix,
  /usr/bin/curl ix,
  /usr/bin/wget ix,
  /usr/bin/ssh ix,
  
  # Deny access to sensitive host files
  deny /etc/shadow r,
  deny /etc/ssh/ssh_host_* r,
  deny /root/** r,
  deny /home/*/.ssh/** r,
  deny /proc/sys/** w,
  deny /sys/** w,
  
  # Network (customizable)
  network inet stream,
  network inet6 stream,
  deny network raw,
  deny network packet,
}
```

#### Linux: SELinux Policy (Alternative)
```te
# aurago-sandbox.te
policy_module(aurago-sandbox, 1.0)

require {
    type httpd_t;
    type user_home_t;
    type ssh_home_t;
    class file { read write execute };
    class dir { read search };
}

# Confined domain for agent
type aurago_agent_t;
domain_type(aurago_agent_t);

# Allow execution of Python/Shell
corecmd_exec_bin(aurago_agent_t)
corecmd_exec_shell(aurago_agent_t)

# Restrict to workspace
type aurago_workspace_t;
files_type(aurago_workspace_t);
allow aurago_agent_t aurago_workspace_t:dir { read search write };
allow aurago_agent_t aurago_workspace_t:file { read write execute };

# Deny access to user homes
ever dontaudit aurago_agent_t user_home_t:dir search;
ever dontaudit aurago_agent_t ssh_home_t:file read;
```

#### Go Integration
```go
// internal/sandbox/tier1_linux.go
package sandbox

import (
    "os/exec"
    "path/filepath"
)

type Tier1Sandbox struct {
    WorkDir       string
    AppArmorProfile string
    MaxMemoryMB   int
    MaxCPUPercent int
}

func (s *Tier1Sandbox) Execute(command string, args ...string) (*exec.Cmd, error) {
    // Check if AppArmor is available
    if s.isAppArmorAvailable() {
        // Wrap with aa-exec
        wrappedArgs := append([]string{"-p", s.AppArmorProfile, command}, args...)
        cmd := exec.Command("aa-exec", wrappedArgs...)
        return s.applyLimits(cmd), nil
    }
    
    // Fallback to SELinux or Tier 0
    return s.fallbackExecute(command, args...)
}

func (s *Tier1Sandbox) applyLimits(cmd *exec.Cmd) *exec.Cmd {
    // cgroups v2 resource limits
    if s.MaxMemoryMB > 0 {
        cmd.Env = append(cmd.Env, 
            "MEMORY_MAX="+fmt.Sprintf("%dm", s.MaxMemoryMB),
        )
    }
    
    // Use systemd-run for cgroup management
    wrapped := exec.Command("systemd-run",
        "--user",
        "--scope",
        "--property", fmt.Sprintf("MemoryMax=%dM", s.MaxMemoryMB),
        "--property", fmt.Sprintf("CPUQuota=%d%%", s.MaxCPUPercent),
        "--collect",
        cmd.Path,
    )
    wrapped.Args = append(wrapped.Args, cmd.Args[1:]...)
    
    return wrapped
}
```

**Configuration:**
```yaml
# config.yaml
sandbox:
  tier: 1
  apparmor:
    profile: "aurago-sandbox"
    enforce: true
  resources:
    max_memory_mb: 512
    max_cpu_percent: 50
    max_disk_mb: 1024
  network:
    mode: "restricted"  # none, restricted, full
    allowed_hosts:
      - "api.openai.com"
      - "github.com"
```

**Homelab Suitability:** Perfect - works on any modern Linux, minimal setup

---

### 2.3 Tier 2: Seccomp + Landlock (Fine-grained)

**Use Case:** Security-conscious users, multi-tenant environments

**Implementation:**

```go
// internal/sandbox/tier2_linux.go
package sandbox

/*
#cgo LDFLAGS: -lseccomp
#include <seccomp.h>
#include <stdlib.h>
*/
import "C"
import (
    "os"
    "os/exec"
    "syscall"
    "unsafe"
)

// SeccompFilter defines allowed syscalls
type SeccompFilter struct {
    DefaultAction Action
    Rules         []SyscallRule
}

type SyscallRule struct {
    Number int
    Action Action
    Args   []ArgCondition
}

type Action int

const (
    ActionAllow Action = iota
    ActionErrno
    ActionKill
    ActionTrace
    ActionLog
)

// DefaultHomelabFilter allows common operations but blocks dangerous ones
func DefaultHomelabFilter() *SeccompFilter {
    return &SeccompFilter{
        DefaultAction: ActionErrno,
        Rules: []SyscallRule{
            // File operations
            {Number: syscall.SYS_READ, Action: ActionAllow},
            {Number: syscall.SYS_WRITE, Action: ActionAllow},
            {Number: syscall.SYS_OPENAT, Action: ActionAllow},
            {Number: syscall.SYS_CLOSE, Action: ActionAllow},
            {Number: syscall.SYS_STAT, Action: ActionAllow},
            {Number: syscall.SYS_FSTAT, Action: ActionAllow},
            
            // Process management
            {Number: syscall.SYS_EXIT, Action: ActionAllow},
            {Number: syscall.SYS_EXIT_GROUP, Action: ActionAllow},
            {Number: syscall.SYS_CLONE, Action: ActionAllow}, // With flags checking
            {Number: syscall.SYS_WAIT4, Action: ActionAllow},
            {Number: syscall.SYS_WAITPID, Action: ActionAllow},
            
            // Memory
            {Number: syscall.SYS_BRK, Action: ActionAllow},
            {Number: syscall.SYS_MMAP, Action: ActionAllow},
            {Number: syscall.SYS_MUNMAP, Action: ActionAllow},
            
            // Network
            {Number: syscall.SYS_SOCKET, Action: ActionAllow},
            {Number: syscall.SYS_CONNECT, Action: ActionAllow},
            {Number: syscall.SYS_SENDTO, Action: ActionAllow},
            {Number: syscall.SYS_RECVFROM, Action: ActionAllow},
            
            // Time
            {Number: syscall.SYS_GETTIMEOFDAY, Action: ActionAllow},
            {Number: syscall.SYS_CLOCK_GETTIME, Action: ActionAllow},
        },
    }
}

// Tier2Sandbox uses seccomp-bpf for syscall filtering
type Tier2Sandbox struct {
    WorkDir    string
    Seccomp    *SeccompFilter
    Landlock   *LandlockRules
}

// LandlockRules defines filesystem access
type LandlockRules struct {
    ReadPaths  []string
    WritePaths []string
    ExecutePaths []string
}

func (s *Tier2Sandbox) Execute(command string, args ...string) (*exec.Cmd, error) {
    cmd := exec.Command(command, args...)
    
    // Setup namespaces
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Cloneflags: syscall.CLONE_NEWNS |
                    syscall.CLONE_NEWPID |
                    syscall.CLONE_NEWNET |
                    syscall.CLONE_NEWIPC,
    }
    
    // Pre-exec function to setup seccomp
    cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
    
    // Create seccomp context
    ctx, err := s.createSeccompContext()
    if err != nil {
        return nil, err
    }
    defer s.destroySeccompContext(ctx)
    
    // Setup Landlock (Linux 5.13+)
    if s.isLandlockAvailable() {
        if err := s.setupLandlock(); err != nil {
            return nil, err
        }
    }
    
    return cmd, nil
}

func (s *Tier2Sandbox) createSeccompContext() (*C.scmp_filter_ctx, error) {
    ctx := C.seccomp_init(C.SCMP_ACT_ERRNO(C.EACCES))
    if ctx == nil {
        return nil, fmt.Errorf("failed to initialize seccomp")
    }
    
    // Add rules
    for _, rule := range s.Seccomp.Rules {
        action := C.uint32_t(C.SCMP_ACT_ALLOW)
        if rule.Action == ActionErrno {
            action = C.SCMP_ACT_ERRNO(C.EACCES)
        }
        
        C.seccomp_rule_add(ctx, action, C.int(rule.Number), 0)
    }
    
    C.seccomp_load(ctx)
    return ctx, nil
}

func (s *Tier2Sandbox) setupLandlock() error {
    // Landlock ABI version check
    abi := C.landlock_create_ruleset(nil, 0, C.LANDLOCK_CREATE_RULESET_VERSION)
    if abi < 1 {
        return fmt.Errorf("landlock not supported")
    }
    
    // Create ruleset
    rulesetAttr := C.struct_landlock_ruleset_attr{
        handled_access_fs: C.LANDLOCK_ACCESS_FS_READ_FILE |
                           C.LANDLOCK_ACCESS_FS_WRITE_FILE |
                           C.LANDLOCK_ACCESS_FS_READ_DIR |
                           C.LANDLOCK_ACCESS_FS_MAKE_DIR |
                           C.LANDLOCK_ACCESS_FS_REMOVE_FILE |
                           C.LANDLOCK_ACCESS_FS_EXECUTE,
    }
    
    ruleset, _, errno := syscall.Syscall(
        C.SYS_landlock_create_ruleset,
        uintptr(unsafe.Pointer(&rulesetAttr)),
        unsafe.Sizeof(rulesetAttr),
        C.LANDLOCK_CREATE_RULESET_VERSION,
    )
    
    if errno != 0 {
        return fmt.Errorf("landlock_create_ruleset failed: %v", errno)
    }
    
    // Add path rules
    for _, path := range s.Landlock.ReadPaths {
        s.addPathRule(int(ruleset), path, C.LANDLOCK_ACCESS_FS_READ_FILE|C.LANDLOCK_ACCESS_FS_READ_DIR)
    }
    
    for _, path := range s.Landlock.WritePaths {
        s.addPathRule(int(ruleset), path, C.LANDLOCK_ACCESS_FS_WRITE_FILE|C.LANDLOCK_ACCESS_FS_MAKE_DIR|C.LANDLOCK_ACCESS_FS_REMOVE_FILE)
    }
    
    for _, path := range s.Landlock.ExecutePaths {
        s.addPathRule(int(ruleset), path, C.LANDLOCK_ACCESS_FS_EXECUTE)
    }
    
    // Enforce
    _, _, errno = syscall.Syscall(C.SYS_landlock_restrict_self, ruleset, 0, 0)
    if errno != 0 {
        return fmt.Errorf("landlock_restrict_self failed: %v", errno)
    }
    
    return nil
}
```

**Configuration:**
```yaml
sandbox:
  tier: 2
  seccomp:
    mode: "homelab"  # homelab, strict, custom
    custom_profile: "/etc/aurago/seccomp-agent.json"
  landlock:
    enabled: true
    paths:
      read:
        - /opt/aurago/agent_workspace/tools
        - /usr/lib/python3
      write:
        - /opt/aurago/agent_workspace/workdir
        - /tmp
      execute:
        - /usr/bin/python3
        - /usr/bin/bash
  network:
    mode: "restricted"
    dns_allowlist:
      - "*.openai.com"
      - "*.github.com"
```

**Homelab Suitability:** Good - requires Linux 5.13+ for Landlock, but seccomp works on older kernels

---

### 2.4 Tier 3: Container (gVisor)

**Use Case:** High security, untrusted code execution

**Implementation:**

```go
// internal/sandbox/tier3.go
package sandbox

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    
    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/client"
)

// Tier3Sandbox uses containers (Docker/Podman) with optional gVisor
type Tier3Sandbox struct {
    WorkDir       string
    Runtime       string  // "runc", "runsc" (gVisor), "kata"
    Image         string
    Resources     container.Resources
    NetworkMode   string
    CapDrop       []string
    CapAdd        []string
}

func NewTier3Sandbox(workDir string) *Tier3Sandbox {
    return &Tier3Sandbox{
        WorkDir: workDir,
        Runtime: "runsc",  // gVisor by default
        Image:   "aurago/sandbox:latest",
        Resources: container.Resources{
            Memory:     512 * 1024 * 1024,  // 512MB
            MemorySwap: 512 * 1024 * 1024,  // No swap
            CpuQuota:   50000,              // 50% of 1 CPU
            CpuPeriod:  100000,
            PidsLimit:  100,                // Max 100 processes
        },
        NetworkMode: "sandbox-network",  // Custom network with restrictions
        CapDrop: []string{
            "ALL",
        },
        CapAdd: []string{
            "NET_BIND_SERVICE",  // Allow binding to low ports if needed
        },
    }
}

func (s *Tier3Sandbox) Execute(command string, args ...string) error {
    cli, err := client.NewClientWithOpts(client.FromEnv)
    if err != nil {
        return fmt.Errorf("failed to create docker client: %w", err)
    }
    
    ctx := context.Background()
    
    // Create container config
    config := &container.Config{
        Image: s.Image,
        Cmd:   append([]string{command}, args...),
        WorkingDir: "/workspace",
        User:       "1000:1000",  // Non-root user
        Env: []string{
            "HOME=/workspace",
            "PATH=/usr/local/bin:/usr/bin:/bin",
        },
        // Security options
        StopSignal: "SIGTERM",
        StopTimeout: 30,
    }
    
    // Host config with security options
    hostConfig := &container.HostConfig{
        Runtime: s.Runtime,
        Binds: []string{
            fmt.Sprintf("%s:/workspace:rw", s.WorkDir),
            // Mount tools as read-only
            fmt.Sprintf("%s:/opt/aurago/tools:ro", filepath.Join(s.WorkDir, "../tools")),
        },
        Resources: s.Resources,
        NetworkMode: container.NetworkMode(s.NetworkMode),
        CapDrop: s.CapDrop,
        CapAdd: s.CapAdd,
        SecurityOpt: []string{
            "no-new-privileges:true",
            "seccomp:unconfined",  // We use our own seccomp profile
        },
        ReadonlyRootfs: true,  // Root filesystem is read-only
        Tmpfs: map[string]string{
            "/tmp": "rw,noexec,nosuid,size=100m",
        },
        LogConfig: container.LogConfig{
            Type: "json-file",
            Config: map[string]string{
                "max-size": "10m",
                "max-file": "3",
            },
        },
    }
    
    // Create container
    resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
    if err != nil {
        return fmt.Errorf("failed to create container: %w", err)
    }
    
    // Start container
    if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
        return fmt.Errorf("failed to start container: %w", err)
    }
    
    // Wait for completion
    statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
    select {
    case err := <-errCh:
        if err != nil {
            return fmt.Errorf("container wait error: %w", err)
        }
    case status := <-statusCh:
        if status.StatusCode != 0 {
            return fmt.Errorf("container exited with code %d", status.StatusCode)
        }
    }
    
    // Cleanup
    cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{})
    
    return nil
}
```

**Dockerfile (Sandbox Image):**
```dockerfile
# Dockerfile.sandbox
FROM python:3.11-slim-bookworm

# Security: Non-root user
RUN groupadd -r agent && useradd -r -g agent agent

# Install minimal tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    curl \
    jq \
    git \
    ssh-client \
    && rm -rf /var/lib/apt/lists/*

# Setup workspace
RUN mkdir -p /workspace /opt/aurago/tools && \
    chown -R agent:agent /workspace

# Copy Python requirements
COPY requirements.txt /tmp/
RUN pip install --no-cache-dir -r /tmp/requirements.txt

# Security hardening
RUN chmod 755 /workspace && \
    chmod 755 /opt/aurago/tools && \
    chmod 1777 /tmp

USER agent
WORKDIR /workspace

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD python3 -c "print('ok')" || exit 1
```

**gVisor Installation:**
```bash
# Install gVisor (runsc)
curl -fsSL https://gvisor.dev/archive/key | sudo gpg --dearmor -o /usr/share/keyrings/gvisor-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/gvisor-archive-keyring.gpg] https://storage.googleapis.com/gvisor/releases release main" | sudo tee /etc/apt/sources.list.d/gvisor.list
sudo apt-get update && sudo apt-get install -y runsc

# Configure Docker to use gVisor
sudo runsc install
sudo systemctl reload docker
```

**Homelab Suitability:** Good - requires Docker, ~100MB RAM overhead

---

### 2.5 Tier 4: Micro-VM (Firecracker)

**Use Case:** Maximum isolation, untrusted/multi-tenant

**Implementation:**

```go
// internal/sandbox/tier4.go
package sandbox

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "time"
)

// FirecrackerSandbox uses micro-VMs for maximum isolation
type FirecrackerSandbox struct {
    WorkDir      string
    VCPUs        int
    MemoryMB     int
    DiskSizeMB   int
    KernelPath   string
    RootfsPath   string
    FirecrackerBin string
}

func NewFirecrackerSandbox(workDir string) *FirecrackerSandbox {
    return &FirecrackerSandbox{
        WorkDir:        workDir,
        VCPUs:          2,
        MemoryMB:       256,
        DiskSizeMB:     512,
        FirecrackerBin: "/usr/local/bin/firecracker",
        KernelPath:     "/opt/aurago/vmlinux",
        RootfsPath:     "/opt/aurago/rootfs.ext4",
    }
}

func (s *FirecrackerSandbox) Execute(command string, args ...string) error {
    // Generate unique VM ID
    vmID := fmt.Sprintf("aurago-%d", time.Now().UnixNano())
    socketPath := filepath.Join("/tmp", vmID+".sock")
    
    // Create VM config
    config := s.createVMConfig(vmID, socketPath)
    
    // Start Firecracker
    fcCmd := exec.Command(s.FirecrackerBin,
        "--api-sock", socketPath,
        "--id", vmID,
        "--config-file", config,
    )
    
    if err := fcCmd.Start(); err != nil {
        return fmt.Errorf("failed to start firecracker: %w", err)
    }
    defer fcCmd.Process.Kill()
    
    // Wait for VM to boot
    time.Sleep(2 * time.Second)
    
    // Execute command via vsock or SSH
    if err := s.execInVM(socketPath, command, args...); err != nil {
        return fmt.Errorf("failed to execute in VM: %w", err)
    }
    
    return nil
}

func (s *FirecrackerSandbox) createVMConfig(vmID, socketPath string) string {
    // Create overlay disk for this execution
    overlayDisk := filepath.Join("/tmp", vmID+".ext4")
    
    // Use copy-on-write overlay
    exec.Command("qemu-img", "create", "-f", "qcow2", "-b", s.RootfsPath, overlayDisk, fmt.Sprintf("%dM", s.DiskSizeMB)).Run()
    
    config := fmt.Sprintf(`{
        "boot-source": {
            "kernel_image_path": "%s",
            "boot_args": "console=ttyS0 reboot=k panic=1 pci=off"
        },
        "drives": [
            {
                "drive_id": "rootfs",
                "path_on_host": "%s",
                "is_root_device": true,
                "is_read_only": false
            }
        ],
        "machine-config": {
            "vcpu_count": %d,
            "mem_size_mib": %d,
            "smt": false
        },
        "network-interfaces": [
            {
                "iface_id": "eth0",
                "guest_mac": "AA:FC:00:00:00:01",
                "host_dev_name": "tap0"
            }
        ]
    }`, s.KernelPath, overlayDisk, s.VCPUs, s.MemoryMB)
    
    configPath := filepath.Join("/tmp", vmID+"-config.json")
    os.WriteFile(configPath, []byte(config), 0644)
    
    return configPath
}

func (s *FirecrackerSandbox) execInVM(socketPath, command string, args ...string) error {
    // Use fcctl or direct HTTP API to exec command
    // This requires agent running inside VM
    
    fullCmd := append([]string{command}, args...)
    execReq := fmt.Sprintf(`{
        "command": %q,
        "timeout_ms": 300000
    }`, fullCmd)
    
    // HTTP request to Firecracker API
    // POST /actions with Execute request
    
    return nil
}
```

**Firecracker Setup:**
```bash
# Download Firecracker
curl -fsSL -o firecracker https://github.com/firecracker-microvm/firecracker/releases/latest/download/firecracker-$(uname -m)
sudo mv firecracker /usr/local/bin/
sudo chmod +x /usr/local/bin/firecracker

# Create rootfs
sudo mkdir -p /opt/aurago/microvm
cd /opt/aurago/microvm

# Download Alpine mini rootfs
wget https://dl-cdn.alpinelinux.org/alpine/v3.18/releases/x86_64/alpine-minirootfs-3.18.4-x86_64.tar.gz

# Create ext4 image
dd if=/dev/zero of=rootfs.ext4 bs=1M count=512
mkfs.ext4 rootfs.ext4
sudo mkdir -p /mnt/rootfs
sudo mount rootfs.ext4 /mnt/rootfs
sudo tar -xzf alpine-minirootfs-3.18.4-x86_64.tar.gz -C /mnt/rootfs
sudo umount /mnt/rootvm
```

**Homelab Suitability:** Moderate - Requires KVM support, ~256MB RAM per VM

---

## 3. Configuration System

### 3.1 Unified Sandbox Configuration

```yaml
# config.yaml - Sandbox Configuration
sandbox:
  # Tier selection: 0-4
  tier: 2
  
  # Fallback tiers if primary unavailable
  fallback_tiers: [1, 0]
  
  # Tier-specific settings
  tier0:
    namespaces:
      mount: true
      pid: true
      network: false  # Allow network in tier 0
      ipc: true
    capabilities:
      drop_all: true
      add: ["NET_BIND_SERVICE"]
  
  tier1:
    mac:
      type: "apparmor"  # apparmor, selinux
      profile: "aurago-sandbox"
      enforce: true
    resources:
      max_memory_mb: 512
      max_cpu_percent: 50
      max_disk_mb: 1024
      max_processes: 100
  
  tier2:
    seccomp:
      mode: "homelab"  # homelab, strict, custom
      custom_profile: "/etc/aurago/seccomp.json"
      default_action: "errno"  # errno, kill, trap
    landlock:
      enabled: true
      abi_version: 1
    
  tier3:
    container:
      runtime: "runsc"  # runc, runsc, kata
      image: "aurago/sandbox:latest"
      registry_mirror: "https://mirror.local"
      pull_policy: "if_not_present"  # always, never, if_not_present
    security:
      readonly_rootfs: true
      no_new_privileges: true
      seccomp: true
      apparmor: false  # Already handled by container
    resources:
      memory_limit: "512m"
      cpu_limit: "0.5"
      pids_limit: 100
      storage_limit: "1g"
  
  tier4:
    microvm:
      provider: "firecracker"  # firecracker, cloud-hypervisor
      vcpus: 2
      memory_mb: 256
      disk_mb: 512
      kernel: "/opt/aurago/vmlinux"
      rootfs: "/opt/aurago/rootfs.ext4"
      init_timeout: 30
    
  # Common settings
  workspace:
    base_path: "/opt/aurago/agent_workspace"
    mount_options:
      - "nodev"
      - "nosuid"
      - "noexec"  # Tier-dependent
  
  network:
    mode: "restricted"  # none, restricted, full, custom
    custom:
      # Allowlist approach
      allowed_hosts:
        - "api.openai.com"
        - "api.github.com"
        - "*.docker.io"
      allowed_ports:
        - 443
        - 80
      dns_servers:
        - "1.1.1.1"
        - "8.8.8.8"
  
  logging:
    level: "info"  # debug, info, warn, error
    audit_syscalls: true
    log_violations: true
    max_log_size_mb: 100
    max_log_files: 5
  
  # Emergency settings
  emergency:
    # Actions when sandbox escape detected
    on_violation: "log"  # log, warn, kill, panic
    on_resource_exhaustion: "kill"
    auto_kill_after_violations: 3
```

### 3.2 Per-Tool Sandbox Overrides

```yaml
# tools.yaml - Per-tool sandbox settings
tools:
  execute_shell:
    sandbox:
      tier: 2  # Force tier 2 for shell commands
      seccomp:
        allow:
          - "execve"
          - "fork"
          - "clone"
  
  execute_python:
    sandbox:
      tier: 3  # Container for Python
      container:
        image: "aurago/python-sandbox:latest"
        additional_packages:
          - "numpy"
          - "pandas"
          - "requests"
  
  filesystem:
    sandbox:
      tier: 1
      landlock:
        paths:
          read:
            - "/opt/aurago/agent_workspace"
            - "/home/user/documents"
          write:
            - "/opt/aurago/agent_workspace/workdir"
  
  remote_execution:
    sandbox:
      tier: 4  # Micro-VM for SSH connections
      microvm:
        memory_mb: 128
        network_mode: "host"  # Need host network for SSH
  
  docker:
    sandbox:
      tier: 0  # Already containerized, use minimal sandbox
      warning: "Executing Docker inside sandbox may lead to container escape"
```

---

## 4. Cross-Platform Support

### 4.1 Platform Matrix

| Tier | Linux | macOS | Windows | Notes |
|------|-------|-------|---------|-------|
| 0 | ✅ Full | ✅ Partial | ✅ Partial | Namespaces only on Linux |
| 1 | ✅ Full | ⚠️ Seatbelt | ⚠️ Windows Defender | macOS uses Seatbelt |
| 2 | ✅ Full | ❌ No | ❌ No | Seccomp is Linux-specific |
| 3 | ✅ Full | ✅ Docker | ✅ Docker | Use Docker Desktop |
| 4 | ✅ KVM | ❌ No | ❌ No | Requires hardware virtualization |

### 4.2 macOS Implementation (Tier 1)

```go
// internal/sandbox/tier1_darwin.go
package sandbox

import (
    "os/exec"
)

// DarwinSandbox uses Seatbelt (macOS sandbox)
type DarwinSandbox struct {
    WorkDir string
    Profile string
}

func (s *DarwinSandbox) Execute(command string, args ...string) (*exec.Cmd, error) {
    // Seatbelt profile
    profile := `(version 1)
(debug deny)
(allow default)
(allow process-exec (regex "/usr/bin/*"))
(allow file-read* (regex "/opt/aurago/agent_workspace/*"))
(allow file-write* (regex "/opt/aurago/agent_workspace/workdir/*"))
(deny file-read* (regex "/Users/*/\.ssh/*"))
(deny file-read* (regex "/etc/passwd"))
`
    
    // Write profile to temp file
    profilePath := "/tmp/aurago-seatbelt.sb"
    os.WriteFile(profilePath, []byte(profile), 0644)
    
    // Wrap with sandbox-exec
    cmd := exec.Command("sandbox-exec", "-f", profilePath, command)
    cmd.Args = append(cmd.Args, args...)
    
    return cmd, nil
}
```

### 4.3 Windows Implementation (Tier 1)

```go
// internal/sandbox/tier1_windows.go
package sandbox

import (
    "os/exec"
    "syscall"
)

// WindowsSandbox uses Windows Defender Application Control (WDAC) or AppLocker
type WindowsSandbox struct {
    WorkDir string
}

func (s *WindowsSandbox) Execute(command string, args ...string) (*exec.Cmd, error) {
    cmd := exec.Command(command, args...)
    
    // Windows Job Objects for resource limits
    cmd.SysProcAttr = &syscall.SysProcAttr{
        // Create in new job object
        CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
    }
    
    // Set Windows integrity level
    // Low integrity for sandboxed processes
    
    return cmd, nil
}
```

---

## 5. Homelab-Specific Considerations

### 5.1 Hardware Requirements

| Tier | CPU | RAM | Storage | Special |
|------|-----|-----|---------|---------|
| 0 | 1 core | 50MB | 10MB | None |
| 1 | 1 core | 100MB | 50MB | None |
| 2 | 1 core | 100MB | 50MB | Linux 5.13+ for Landlock |
| 3 | 1 core | 600MB | 1GB | Docker installed |
| 4 | 2 cores | 512MB | 1GB | KVM support |

### 5.2 Raspberry Pi / ARM Support

```yaml
# arm64 specific configuration
sandbox:
  tier: 2  # Tier 3/4 may be slow on RPi
  
  tier3:
    container:
      image: "aurago/sandbox:latest-arm64"
      platform: "linux/arm64"
  
  tier4:
    microvm:
      # Firecracker supports ARM64
      kernel: "/opt/aurago/vmlinux-arm64"
      rootfs: "/opt/aurago/rootfs-arm64.ext4"
```

### 5.3 NAS / Resource-Constrained Environments

```yaml
# Minimal resource configuration
sandbox:
  tier: 1  # Use MAC instead of containers for low overhead
  
  tier1:
    resources:
      max_memory_mb: 128
      max_cpu_percent: 25
      max_processes: 50
  
  # Disable heavy features
  logging:
    audit_syscalls: false  # Saves CPU
    level: "warn"  # Reduce log volume
```

### 5.4 Integration with Homelab Tools

#### Proxmox VE
```yaml
# Use Proxmox containers (LXC) for Tier 3
sandbox:
  tier: 3
  container:
    runtime: "lxc"  # Use Proxmox LXC instead of Docker
    template: "local:vztmpl/debian-12-standard_12_amd64.tar.zst"
    proxmox:
      node: "pve"
      storage: "local-zfs"
```

#### TrueNAS / Storage Systems
```yaml
# Sandbox with ZFS dataset restrictions
sandbox:
  workspace:
    base_path: "/mnt/tank/aurago/workspace"
    zfs_dataset: "tank/aurago/sandbox"
    quota: "10G"
    compression: "lz4"
```

---

## 6. Implementation Roadmap

### Phase 1: Foundation (Week 1-2)
- [ ] Tier 0 implementation (Linux namespaces)
- [ ] Tier 1 AppArmor profiles
- [ ] Configuration system
- [ ] Basic resource limits (cgroups v2)

### Phase 2: Security Hardening (Week 3-4)
- [ ] Tier 2 seccomp profiles
- [ ] Landlock integration
- [ ] Syscall auditing
- [ ] Per-tool sandbox overrides

### Phase 3: Container Support (Week 5-6)
- [ ] Tier 3 Docker/Podman integration
- [ ] gVisor support
- [ ] Custom sandbox images
- [ ] Registry mirror support

### Phase 4: Micro-VM (Week 7-8)
- [ ] Firecracker integration
- [ ] VM image management
- [ ] Fast boot optimization
- [ ] vsock communication

### Phase 5: Cross-Platform (Week 9-10)
- [ ] macOS Seatbelt support
- [ ] Windows Defender integration
- [ ] Platform detection
- [ ] Graceful degradation

---

## 7. Example Configurations

### 7.1 Secure Homelab (Recommended)
```yaml
sandbox:
  tier: 2
  fallback_tiers: [1]
  
  tier2:
    seccomp:
      mode: "homelab"
    landlock:
      enabled: true
  
  network:
    mode: "restricted"
    allowed_hosts:
      - "api.openai.com"
      - "github.com"
```

### 7.2 Minimal Resource (NAS/RPi)
```yaml
sandbox:
  tier: 1
  
  tier1:
    resources:
      max_memory_mb: 128
      max_cpu_percent: 25
```

### 7.3 Maximum Security (Untrusted Internet)
```yaml
sandbox:
  tier: 4
  
  tier4:
    microvm:
      vcpus: 2
      memory_mb: 256
  
  network:
    mode: "none"  # Air-gapped
```

### 7.4 Development/Debug
```yaml
sandbox:
  tier: 0
  
  tier0:
    namespaces:
      network: false  # Full network access
      mount: true
  
  logging:
    level: "debug"
    audit_syscalls: true
```

---

## 8. Monitoring & Alerting

```yaml
# monitoring.yaml
sandbox_monitoring:
  metrics:
    - name: "sandbox_violations_total"
      help: "Total number of sandbox policy violations"
      labels: ["tier", "tool", "violation_type"]
    
    - name: "sandbox_execution_duration"
      help: "Execution time in sandbox"
      labels: ["tier", "tool"]
    
    - name: "sandbox_resource_usage"
      help: "Resource usage per sandbox"
      labels: ["resource_type"]  # memory, cpu, disk, network
  
  alerts:
    - name: "SandboxEscapeAttempt"
      condition: "sandbox_violations_total > 0"
      severity: "critical"
      action: "kill_process"
    
    - name: "HighResourceUsage"
      condition: "sandbox_resource_usage > 90"
      severity: "warning"
      action: "notify"
    
    - name: "UnknownSyscall"
      condition: "rate(sandbox_violations_total{violation_type='unknown_syscall'}[5m]) > 10"
      severity: "warning"
```

---

## Appendix A: Comparison Matrix

| Feature | Tier 0 | Tier 1 | Tier 2 | Tier 3 | Tier 4 |
|---------|--------|--------|--------|--------|--------|
| **Overhead** | ~5ms | ~10ms | ~15ms | ~100ms | ~500ms |
| **Memory** | +10MB | +20MB | +30MB | +100MB | +256MB |
| **Kernel Protection** | ❌ | ⚠️ | ✅ | ✅ | ✅ |
| **Filesystem Isolation** | Partial | Partial | Full | Full | Full |
| **Network Isolation** | Optional | Optional | Optional | Full | Full |
| **Works on RPi** | ✅ | ✅ | ⚠️ | ⚠️ | ❌ |
| **Zero-Trust** | ❌ | ⚠️ | ✅ | ✅ | ✅ |

---

## Appendix B: Troubleshooting

### Issue: Sandbox too slow on RPi
**Solution:** Use Tier 1 (AppArmor) instead of Tier 3 (Containers)

### Issue: Seccomp breaks legitimate tool
**Solution:** Add specific syscall to allowlist or downgrade to Tier 1

### Issue: Docker not available
**Solution:** Use Tier 2 (seccomp) or install Podman

### Issue: VM fails to start (Tier 4)
**Solution:** Check KVM support: `kvm-ok` and ensure user is in `kvm` group

---

*Document Version: 1.0*  
*Target Release: AuraGo v2.0*  
*Classification: Design Specification*

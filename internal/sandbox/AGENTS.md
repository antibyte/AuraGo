# Shell sandbox

Linux helpers own Landlock enforcement and process resource limits for shell,
direct executable and MCP startup paths.

- Mask directory-only Landlock rights for non-directory file descriptors,
  including devices and Unix sockets. Inspect the opened descriptor; preserve
  final-symlink rejection and required-path failures.
- Bind Landlock setup and exec to one OS thread. Prepare argv/environment before
  resource limits, apply the address-space limit last, then exec without Go heap
  allocation. The configured cap belongs to the target, not Go's reserved heap.
- Never bypass failed isolation or relax configured limits to make startup work.
- Keep distribution-owned `/usr/share/python-wheels`, `/etc/debian_version` and
  `/etc/mime.types` readable, never writable, so Debian/Ubuntu `ensurepip` and
  pip's metadata probes can bootstrap a private venv under Landlock. All stay optional.
- Verify `TestLandlockRulesAcceptDirectoryFileDeviceAndSocket` and
  `TestSandboxHelperEnforcesIsolationAndLowMemoryLimit` and
  `TestSandboxHelperBootstrapsDistributionPip` on an actual Linux kernel,
  plus `go test ./internal/sandbox` on supported build platforms.

package virtualcomputers

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

const (
	workspacePatchVersion        = "aurago-workspace-patches-v2"
	workspaceRootfsLayoutVersion = "aurago-workspace-rootfs-v2"
	// These hashes cover the LF-normalized files stored by Git at the pinned
	// upstream revision. Do not calculate them from a Windows CRLF checkout.
	workspaceServerSHA256        = "1119d90cc3932a7beba47592ccdba336b519b91e87a594e01e1f34a0b8badf14"
	workspaceTemplateSHA256      = "b48d9702ec3ea5b05db5b241af37b07a3e2a2a2d641234cfbdcf90bf69b38d92"
	workspaceMachineVolumeSHA256 = "6a6a9e58cd5cebb8def27a25722245ed546530a3eb3770bed81bcf8048551e15"
	workspaceMachineSHA256       = "eb294a8e665cfe7d9e1855afb5edc4b829ca6ab7cbc149ca38f33d915fab22f9"
	workspaceSnapshotSHA256      = "d8dc45aeba926903b4321001a56a2714dab035510aa63ce6c936e4a957dbb51d"
	workspaceFirecrackerSHA256   = "80723bfd39819fb2c7a8d1b136533345a400e2c13b97b162f71681b352c0cf3d"
)

// The guest sources compile in the AuraGo module for tests. module.txt and
// sum.txt become the pinned standalone module files on the KVM build host.
//
//go:embed guest_workspace_agent/* patches/*.patch
var workspaceAssets embed.FS

func WorkspaceAssetFingerprint() string {
	return workspaceRuntimeAssetFingerprint(workspaceAssets)
}

func workspaceRuntimeAssetFingerprint(assets fs.FS) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(PinnedUpstreamRevision + "\n" + workspacePatchVersion + "\n" + workspaceRootfsLayoutVersion + "\n" + WorkspaceProtocolVersion + "\n"))
	for _, directory := range []string{"patches", "guest_workspace_agent"} {
		entries, _ := fs.ReadDir(assets, directory)
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if entry.IsDir() || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			name := directory + "/" + entry.Name()
			data, _ := fs.ReadFile(assets, name)
			_, _ = hash.Write([]byte(name + "\n"))
			_, _ = hash.Write(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
			_, _ = hash.Write([]byte("\n"))
		}
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func compatibleWorkspaceAssetFingerprint(current, reported string) bool {
	if current == reported {
		return true
	}
	// These two legacy stamps include tests, but their patches, guest runtime,
	// dependencies, protocol and rootfs layout are byte-identical after CRLF
	// normalization. Bind the migration to that exact runtime: a future runtime
	// edit must never inherit this compatibility allowance.
	if current != "ff9258da6a93b45bae13eadec9db9f4a4053dfc41e5c5ca1a8202d78585813b5" {
		return false
	}
	return reported == "ffb4211a9f999e7c97ee34ff9e89f348c2e5a6417e087d0dd7f9dbab868ad018" ||
		reported == "e98a3f8d79bd236c0830883d671b68adfd8b56500d161a4f1be48c594be15786"
}

func workspacePatchInstallSnippet() string {
	var script strings.Builder
	script.WriteString(`log "verifying and applying AuraGo boringd workspace patches"
PATCH_DIR="${INSTALL_DIR}/aurago-workspace-patches"
rm -rf "${PATCH_DIR}"
install -d -m0755 "${PATCH_DIR}"
`)
	entries, _ := fs.ReadDir(workspaceAssets, "patches")
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".patch") {
			continue
		}
		data, _ := workspaceAssets.ReadFile("patches/" + entry.Name())
		fmt.Fprintf(&script, "printf '%%s' '%s' | base64 -d > \"${PATCH_DIR}/%s\"\n", base64.StdEncoding.EncodeToString(data), entry.Name())
	}
	script.WriteString(`git -C "${REPO_DIR}" reset --hard "${BORING_REVISION}"
# checkout/reset intentionally preserves untracked files. Remove only files
# created by this AuraGo patch series before applying it again.
rm -f \
  "${REPO_DIR}/boringd/workspace.go" \
  "${REPO_DIR}/boringd/workspace_network.go" \
  "${REPO_DIR}/.aurago-workspace-patches"
printf '%s  %s\n' \
    '` + workspaceServerSHA256 + `' "${REPO_DIR}/boringd/server.go" \
    '` + workspaceTemplateSHA256 + `' "${REPO_DIR}/boringd/templates.go" \
    '` + workspaceMachineVolumeSHA256 + `' "${REPO_DIR}/boringd/machinevolume.go" \
    '` + workspaceMachineSHA256 + `' "${REPO_DIR}/boringd/machine.go" \
    '` + workspaceSnapshotSHA256 + `' "${REPO_DIR}/infra/latitude/build-template.sh" \
    '` + workspaceFirecrackerSHA256 + `' "${REPO_DIR}/boringd/firecracker.go" | sha256sum -c -
for patch in "${PATCH_DIR}"/*.patch; do
  git -C "${REPO_DIR}" apply --unidiff-zero --check "${patch}"
  git -C "${REPO_DIR}" apply --unidiff-zero "${patch}"
done
`)
	return script.String()
}

// workspaceGuestInstallSnippet materializes and builds the pinned guest agent,
// then defines the rootfs injection helper. The setup script decides when to
// inject so the Python agent is already running when its fast-start snapshot is
// captured.
func workspaceGuestInstallSnippet() string {
	var script strings.Builder
	script.WriteString(`log "building and installing aurago-workspace-agent"
WORKSPACE_AGENT_SRC="${INSTALL_DIR}/aurago-workspace-agent-src"
rm -rf "${WORKSPACE_AGENT_SRC}"
install -d -m0755 "${WORKSPACE_AGENT_SRC}"
`)
	entries, _ := fs.ReadDir(workspaceAssets, "guest_workspace_agent")
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, _ := workspaceAssets.ReadFile("guest_workspace_agent/" + entry.Name())
		name := entry.Name()
		if name == "module.txt" {
			name = "go.mod"
		} else if name == "sum.txt" {
			name = "go.sum"
		}
		fmt.Fprintf(&script, "printf '%%s' '%s' | base64 -d > \"${WORKSPACE_AGENT_SRC}/%s\"\n", base64.StdEncoding.EncodeToString(data), name)
	}
	script.WriteString(`/usr/local/go/bin/go -C "${WORKSPACE_AGENT_SRC}" mod verify
CGO_ENABLED=0 GOOS=linux GOARCH="${GOARCH}" /usr/local/go/bin/go -C "${WORKSPACE_AGENT_SRC}" build -trimpath -ldflags='-s -w' -o /opt/boring/bin/aurago-workspace-agent .

inject_workspace_agent() {
  image="$1"
  flavor="$2"
  [ -s "${image}" ] || return 0
  mount_dir="$(mktemp -d /tmp/aurago-workspace-rootfs.XXXXXX)"
  mount -o loop "${image}" "${mount_dir}"
  status=0
  (
  set -e
  install -D -m0755 /opt/boring/bin/aurago-workspace-agent "${mount_dir}/usr/local/bin/aurago-workspace-agent"
  install -d -m0700 "${mount_dir}/run/aurago"
  install -d -m0755 "${mount_dir}/workspace"
  if [ "${flavor}" = "desktop" ]; then
	# Remove the upstream unmanaged Chromium launch. The only browser left on
	# DISPLAY=:0 is the /run-profile instance controlled by the guest agent, so
	# VNC and structured browser actions always refer to the same browser.
	awk '
	  /^CHROMIUM_BIN=\/usr\/lib\/chromium\/chromium;/ { skip=1; next }
	  skip && /\/var\/log\/chromium\.log 2>\&1 \&$/ { skip=0; next }
	  !skip { print }
	' "${mount_dir}/sbin/boring-init" > "${mount_dir}/sbin/boring-init.aurago"
	install -m0755 "${mount_dir}/sbin/boring-init.aurago" "${mount_dir}/sbin/boring-init"
	rm -f "${mount_dir}/sbin/boring-init.aurago"
    # Replace our previous launch as well, so reinjection enables the browser
    # without adding a second agent or restoring the unmanaged Chromium process.
    sed -i '\|^[[:space:]]*\(DISPLAY=:0 \)\?/usr/local/bin/aurago-workspace-agent |d' "${mount_dir}/sbin/boring-init"
    sed -i '/^echo BORING_READY/i DISPLAY=:0 /usr/local/bin/aurago-workspace-agent --desktop-browser >>/var/log/aurago-workspace-agent.log 2>\&1 \&' "${mount_dir}/sbin/boring-init"
  else
    # BusyBox runs sysinit before respawn: the upstream ready marker races the
    # agent and snapshots a guest that cannot accept workspace connections yet.
    sed -i '/^::sysinit:.*echo BORING_READY/d; \|^::respawn:/usr/local/bin/aurago-workspace-agent|d' "${mount_dir}/etc/inittab"
    printf '%s\n' '::respawn:/usr/local/bin/aurago-workspace-agent --boot-ready' >> "${mount_dir}/etc/inittab"
  fi
  ) || status=$?
  {
    sync
    umount "${mount_dir}" 2>/dev/null || umount -l "${mount_dir}" 2>/dev/null || true
    rmdir "${mount_dir}" 2>/dev/null || true
  }
  return "${status}"
}

`)
	return script.String()
}

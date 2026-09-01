package virtualcomputers

import (
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
	workspacePatchVersion        = "aurago-workspace-patches-v1"
	workspaceRootfsLayoutVersion = "aurago-workspace-rootfs-v2"
	// These hashes cover the LF-normalized files stored by Git at the pinned
	// upstream revision. Do not calculate them from a Windows CRLF checkout.
	workspaceServerSHA256        = "1119d90cc3932a7beba47592ccdba336b519b91e87a594e01e1f34a0b8badf14"
	workspaceTemplateSHA256      = "b48d9702ec3ea5b05db5b241af37b07a3e2a2a2d641234cfbdcf90bf69b38d92"
	workspaceMachineVolumeSHA256 = "6a6a9e58cd5cebb8def27a25722245ed546530a3eb3770bed81bcf8048551e15"
	workspaceMachineSHA256       = "eb294a8e665cfe7d9e1855afb5edc4b829ca6ab7cbc149ca38f33d915fab22f9"
)

// The guest sources compile in the AuraGo module for tests. module.txt and
// sum.txt become the pinned standalone module files on the KVM build host.
//
//go:embed guest_workspace_agent/* patches/*.patch
var workspaceAssets embed.FS

func WorkspaceAssetFingerprint() string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(PinnedUpstreamRevision + "\n" + workspacePatchVersion + "\n" + workspaceRootfsLayoutVersion + "\n" + WorkspaceProtocolVersion + "\n"))
	for _, directory := range []string{"patches", "guest_workspace_agent"} {
		entries, _ := fs.ReadDir(workspaceAssets, directory)
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := directory + "/" + entry.Name()
			data, _ := workspaceAssets.ReadFile(name)
			_, _ = hash.Write([]byte(name + "\n"))
			_, _ = hash.Write(data)
			_, _ = hash.Write([]byte("\n"))
		}
	}
	return hex.EncodeToString(hash.Sum(nil))
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
    '` + workspaceMachineSHA256 + `' "${REPO_DIR}/boringd/machine.go" | sha256sum -c -
for patch in "${PATCH_DIR}"/*.patch; do
  git -C "${REPO_DIR}" apply --check "${patch}"
  git -C "${REPO_DIR}" apply "${patch}"
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
    if ! grep -q 'aurago-workspace-agent' "${mount_dir}/sbin/boring-init"; then
      sed -i '/^echo BORING_READY/i /usr/local/bin/aurago-workspace-agent >>/var/log/aurago-workspace-agent.log 2>\&1 \&' "${mount_dir}/sbin/boring-init"
    fi
  else
    if ! grep -q 'aurago-workspace-agent' "${mount_dir}/etc/inittab"; then
      printf '%s\n' '::respawn:/usr/local/bin/aurago-workspace-agent' >> "${mount_dir}/etc/inittab"
    fi
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

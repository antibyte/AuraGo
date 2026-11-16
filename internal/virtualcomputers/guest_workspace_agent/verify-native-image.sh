#!/bin/bash
# Before activation, boot a disposable copy of the candidate with no network.
set -euo pipefail
image=$1
work=$(mktemp -d /tmp/aurago-native-verify.XXXXXX)
trap 'rm -rf "$work"' EXIT
cp --reflink=auto --sparse=always "$image" "$work/rootfs.ext4"
cat > "$work/config.json" <<EOF
{"boot-source":{"kernel_image_path":"/opt/boring/kernel/vmlinux","boot_args":"console=ttyS0 reboot=k panic=1 pci=off root=/dev/vda rw init=/sbin/boring-init aurago.native_smoke=1"},"drives":[{"drive_id":"rootfs","path_on_host":"$work/rootfs.ext4","is_root_device":true,"is_read_only":false}],"machine-config":{"vcpu_count":2,"mem_size_mib":1024}}
EOF
status=0
timeout 75s /opt/boring/bin/firecracker --no-api --config-file "$work/config.json" > "$work/serial.log" 2>&1 || status=$?
# A guest power-off may report exit 1 on Firecracker without ACPI.
if [ "$status" -gt 1 ] || ! tr -d '\r' < "$work/serial.log" | grep -qx AURAGO_NATIVE_DESKTOP_OK; then
  tail -n 30 "$work/serial.log" >&2
  echo 'Native desktop candidate failed its isolated guest test' >&2
  exit 1
fi

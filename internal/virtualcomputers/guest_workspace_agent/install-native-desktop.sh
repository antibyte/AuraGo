#!/bin/sh
# Image-build only. Never called from the running guest.
set -eu
version=1.3.1
case "$(dpkg --print-architecture)" in
  amd64) target=x86_64-unknown-linux-gnu; size=3537152; hash=ca77b6b75034eeb12e537dd8c0a4f2a3424119253f38b7e6bd79ba0d2fef1264 ;;
  arm64) target=aarch64-unknown-linux-gnu; size=3407558; hash=438990a703fbeb19775596c341c3701435e80adbfa1b2f64b82ff17567d5573b ;;
  *) echo 'Unsupported native desktop architecture' >&2; exit 1 ;;
esac
dpkg --compare-versions "$(dpkg-query -W -f='${Version}' at-spi2-core)" ge 2.52
work=$(mktemp -d /tmp/aurago-native-install.XXXXXX)
trap 'rm -rf "$work"' EXIT HUP INT TERM
curl --fail --location --proto '=https' --tlsv1.2 --max-time 180 \
  "https://github.com/vercel-labs/agent-desktop/releases/download/v${version}/agent-desktop-v${version}-${target}.tar.gz" -o "$work/archive.tar.gz"
test "$(stat -c %s "$work/archive.tar.gz")" = "$size"
printf '%s  %s\n' "$hash" "$work/archive.tar.gz" | sha256sum -c -
tar -xzf "$work/archive.tar.gz" -C "$work"
for binary in agent-desktop agent-desktop-linux-clipboard-owner; do
  source=$(find "$work" -type f -name "$binary")
  test -n "$source"
  install -m0755 "$source" "/usr/local/bin/$binary"
done
/usr/local/bin/agent-desktop --version | grep -F '1.3.1'

install -D -m0644 /usr/local/share/agent-desktop-LICENSE.txt /usr/share/doc/agent-desktop/LICENSE

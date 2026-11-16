#!/bin/sh
# One D-Bus session owns the window manager, apps and both automation paths.
set -eu
export DISPLAY=:0 HOME=/root NO_AT_BRIDGE=0
# GTK3 has a built-in bridge; forcing GTK2 modules can crash native dialogs.
unset GTK_MODULES
export XDG_RUNTIME_DIR=/run/aurago/session
export AGENT_DESKTOP_HOME=/run/aurago/native-desktop
install -d -m0700 "$XDG_RUNTIME_DIR" "$AGENT_DESKTOP_HOME"
openbox >>/var/log/openbox.log 2>&1 &
xterm -fa 'DejaVu Sans Mono' -fs 10 -geometry 108x13+16+648 \
  -title 'AuraGo Agent Workspace' >>/var/log/xterm.log 2>&1 &
xcalc -geometry 300x400+956+20 >>/var/log/xcalc.log 2>&1 &
if grep -q "aurago.native_smoke=1" /proc/cmdline; then
  /usr/local/bin/aurago-workspace-agent --native-smoke >/dev/ttyS0 2>&1
  sync
  echo o > /proc/sysrq-trigger
  exit 0
fi
exec /usr/local/bin/aurago-workspace-agent --desktop-browser

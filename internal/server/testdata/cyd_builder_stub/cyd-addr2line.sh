#!/bin/sh
# Test stub of cyd-addr2line: one frame per address, JSON array on stdout.
set -eu
elf=""
chip=""
while [ $# -gt 0 ]; do
  case "$1" in
    --elf) elf="$2"; shift 2 ;;
    --chip) chip="$2"; shift 2 ;;
    *) break ;;
  esac
done
if [ ! -f "$elf" ]; then
  echo "missing elf $elf" >&2
  exit 2
fi
out="["
sep=""
for a in "$@"; do
  out="$out$sep{\"address\":\"$a\",\"function\":\"stub_$chip()\",\"file\":\"src/app/app_main.cpp\",\"line\":42}"
  sep=","
done
echo "$out]"

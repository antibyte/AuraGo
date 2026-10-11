#!/bin/sh
# Test stub of cyd-build (the real one ships in the plan 1b toolchain image).
# Reads <work>/stub.json {"exit":N,"lines":K,"sleep":S} (all optional) and
# writes the contract outputs into <work>/out/.
set -eu
work=""
env=""
build=""
cache=""
while [ $# -gt 0 ]; do
  case "$1" in
    --work) work="$2"; shift 2 ;;
    --env) env="$2"; shift 2 ;;
    --build-id) build="$2"; shift 2 ;;
    --cache-key) cache="$2"; shift 2 ;;
    *) echo "unknown argument $1" >&2; exit 2 ;;
  esac
done
num() {
  if [ -f "$work/stub.json" ]; then
    sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p" "$work/stub.json"
  fi
}
code=$(num exit)
lines=$(num lines)
pause=$(num sleep)
code=${code:-0}
lines=${lines:-0}
pause=${pause:-0}
echo "stub cyd-build env=$env build=$build cache=$cache"
echo "uid=$(id -u) gid=$(id -g)"
if touch /cyd-rootfs-probe 2>/dev/null; then echo "rootfs writable"; else echo "rootfs read-only"; fi
echo "net: $(ls /sys/class/net | tr '\n' ' ')"
if mkdir -p "/cache/$cache"; then echo "cache ok"; fi
i=0
while [ "$i" -lt "$lines" ]; do
  echo "line $i"
  i=$((i + 1))
done
echo "src/app/app_main.cpp:3:1: warning: stub warning" >&2
if [ "$pause" -gt 0 ]; then sleep "$pause"; fi
mkdir -p "$work/out"
if [ "$code" = 0 ]; then
  for f in merged.bin firmware.bin firmware.elf; do
    printf '%s-%s' "$f" "$build" > "$work/out/$f"
  done
fi
sha() { sha256sum "$work/out/$1" | cut -d' ' -f1; }
size() { wc -c < "$work/out/$1" | tr -d ' '; }
if [ "$code" = 0 ] || [ "$code" = 3 ]; then
  files=""
  ok=false
  if [ "$code" = 0 ]; then
    ok=true
    for f in merged.bin firmware.bin firmware.elf; do
      [ -n "$files" ] && files="$files,"
      files="$files\"$f\":{\"size\":$(size "$f"),\"sha256\":\"$(sha "$f")\"}"
    done
  fi
  printf '{"ok":%s,"exit":%s,"env":"%s","chip":"esp32","duration_ms":5,"warnings":1,"flash":{"used":1000,"total":1966080},"ram":{"used":10,"total":327680},"files":{%s}}\n' \
    "$ok" "$code" "$env" "$files" > "$work/out/result.json"
fi
exit "$code"

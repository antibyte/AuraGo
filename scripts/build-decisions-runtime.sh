#!/usr/bin/env bash
set -euo pipefail
backend=
target="$(go env GOOS)-$(go env GOARCH)"
build_type=Release
while (($#)); do
  case "$1" in
    --backend)
      if (($# < 2)); then echo 'usage: build-decisions-runtime.sh --backend {vulkan|cuda|metal|cpu} [--target TARGET] [--build-type TYPE]' >&2; exit 2; fi
      backend="$2"; shift 2 ;;
    --target) target="$2"; shift 2 ;;
    --build-type) build_type="$2"; shift 2 ;;
    *) echo "unknown option" >&2; exit 2 ;;
  esac
done
case "$backend" in cpu|cuda|vulkan|metal) ;; *) echo 'usage: build-decisions-runtime.sh --backend {vulkan|cuda|metal|cpu} [--target TARGET] [--build-type TYPE]' >&2; exit 2 ;; esac
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source="$repo/decisions_runtime"
runtime_dir="$repo/bin/decisions-runtime/$target/$backend"
build_dir="$repo/bin/decisions-runtime/build-$target-$backend"
mkdir -p "$runtime_dir" "$build_dir"
args=(-S "$source" -B "$build_dir" -DAURAGO_RUNTIME_BACKEND="$backend" -DAURAGO_RUNTIME_OUTPUT_DIR="$runtime_dir" -DCMAKE_BUILD_TYPE="$build_type" -DCMAKE_TRY_COMPILE_CONFIGURATION=Release)
local_source="$repo/bin/decisions-runtime/llama-src"
if [[ -d "$local_source/.git" ]]; then args+=(-DFETCHCONTENT_SOURCE_DIR_LLAMA_CPP="$local_source"); fi
cmake "${args[@]}"
cmake --build "$build_dir" --config "$build_type" --parallel "${CMAKE_BUILD_PARALLEL_LEVEL:-4}"
name=aurago-decisions-runtime
[[ "$target" == windows-* ]] && name=aurago-decisions-runtime.exe
binary="$runtime_dir/$name"
"$binary" --self-test-protocol
"$binary" --discover | python -c 'import json,sys; x=json.load(sys.stdin); assert x["protocol"]=="aurago.decisions.v1" and x["build"]["llama_cpp_commit"]=="d81235049384534c167caea52b85a694f6103d14" and isinstance(x["devices"], list)'
python "$repo/scripts/write-decisions-runtime-manifest.py" --binary "$binary" --target "$target" --backend "$backend"
printf 'Built and locally verified %s\n' "$binary"

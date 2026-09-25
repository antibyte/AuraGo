#!/usr/bin/env bash
set -euo pipefail

# Run manually only after the later start instruction, a verified data pack,
# and a separate active watchdog. The network volume must already be mounted.
cd /workspace/aurago-needle3
test -f lease.json
test -f training/needle3/generated/compiled/manifest.json
# The independent guard owns its own environment and keeps its control key.
unset RUNPOD_API_KEY OPENROUTER_API_KEY
export DEBIAN_FRONTEND=noninteractive
timeout 600 apt-get update
timeout 600 apt-get install -y --no-install-recommends python3.12 python3.12-venv ca-certificates git curl
python3.12 - <<'PY'
import json, time
lease = json.load(open('lease.json'))
assert lease['watchdog_armed'] and time.time() - lease['created_at_unix'] < 1800
assert time.time() - lease['watchdog_heartbeat_unix'] < 60
PY
python3.12 -m venv /workspace/needle-bootstrap
/workspace/needle-bootstrap/bin/pip install 'uv==0.11.15'
export UV_CACHE_DIR=/workspace/uv-cache
export HF_HUB_DISABLE_TELEMETRY=1 DO_NOT_TRACK=1 XLA_PYTHON_CLIENT_PREALLOCATE=false
timeout 900 /workspace/needle-bootstrap/bin/uv sync --project training/needle3 --locked --python python3.12 --group train
exec training/needle3/.venv/bin/python -m training.needle3.controller \
  --pack training/needle3/generated/compiled --lease lease.json --out /workspace/needle-results

#!/usr/bin/env bash
# Regenerate the Local Wikipedia test fixtures (dev tooling only; needs Docker).
#
#   bash scripts/localwiki/fixtures/generate.sh
#   REAL_ZIM=/path/to/wikipedia_en_climate_change_mini_2024-06.zim bash scripts/localwiki/fixtures/generate.sh
#
# Step 1 (python:3.12-slim-trixie + hash-pinned libzim wheel) writes
#   internal/zim/testdata/fixture_{de,en,pl,ja,bulk}.zim and, in the ignored
#   work dir disposable/_localwiki_fixtures/, the extracted Xapian blobs and
#   libzim's own search/suggestion results.
# Step 2 (debian:trixie-slim + xapian-tools, python3-xapian, python3-icu)
#   writes the golden JSON into internal/zim/xapian/testdata/.
# Step 3 (only with REAL_ZIM) records libzim's results on the real-world
#   fixture into internal/zim/xapian/testdata/real_fixture_libzim.json.
# On Windows run it from Git Bash; MSYS_NO_PATHCONV=1 keeps the -v mounts intact.
set -euo pipefail

PYTHON_IMAGE="python:3.12-slim-trixie@sha256:05cda9777409a9c3ffddd94a4c476b79f0769a0b4857f0c7ed9226b6800b0d6f"
DEBIAN_IMAGE="debian:trixie-slim@sha256:a29215f6a35e51e22adffa17f89e9d2ef06214e64a2bad10d765c46aea49f11f"
WORK="disposable/_localwiki_fixtures"
PIP_INSTALL='pip install --quiet --no-cache-dir --root-user-action=ignore --disable-pip-version-check --only-binary=:all: --require-hashes -r scripts/localwiki/fixtures/requirements.txt'

cd "$(dirname "$0")/../../.."
ROOT="$(pwd -W 2>/dev/null || pwd)"
export MSYS_NO_PATHCONV=1

rm -rf "$WORK"
mkdir -p "$WORK" internal/zim/testdata internal/zim/xapian/testdata

docker run --rm -v "$ROOT:/repo" -w /repo "$PYTHON_IMAGE" sh -euc "
  $PIP_INSTALL
  python scripts/localwiki/fixtures/make_zims.py --zim-dir internal/zim/testdata --work $WORK >/dev/null
"

docker run --rm -v "$ROOT:/repo" -w /repo "$DEBIAN_IMAGE" sh -euc '
  apt-get update -qq >/dev/null
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends \
      xapian-tools python3-xapian python3-icu >/dev/null
  dpkg-query -W -f="\${Package} \${Version}\n" xapian-tools python3-xapian python3-icu
  python3 scripts/localwiki/fixtures/make_goldens.py --work '"$WORK"' --out internal/zim/xapian/testdata
'

if [ -n "${REAL_ZIM:-}" ]; then
  REAL_DIR="$(cd "$(dirname "$REAL_ZIM")" && (pwd -W 2>/dev/null || pwd))"
  REAL_NAME="$(basename "$REAL_ZIM")"
  docker run --rm -v "$ROOT:/repo" -v "$REAL_DIR:/real:ro" -w /repo "$PYTHON_IMAGE" sh -euc "
    $PIP_INSTALL
    python scripts/localwiki/fixtures/real_fixture_queries.py --zim /real/$REAL_NAME \
        --out internal/zim/xapian/testdata/real_fixture_libzim.json
  "
fi

ls -l internal/zim/testdata/fixture_*.zim internal/zim/xapian/testdata/*.json

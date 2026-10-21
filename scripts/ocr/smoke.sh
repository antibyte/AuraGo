#!/usr/bin/env bash
set -euo pipefail
: "${AURAGO_OCR_SMOKE_IMAGE:?set the immutable local image ID}"
smoke_dir=$(mktemp -d)
trap 'rm -rf -- "$smoke_dir"' EXIT
chmod 777 "$smoke_dir"
docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges \
  --tmpfs /tmp:size=67108864 "$AURAGO_OCR_SMOKE_IMAGE" python -I /app/service.py --verify
docker run --rm --network none --read-only --cap-drop ALL \
  --mount "type=bind,source=$PWD/scripts/ocr,target=/scripts,readonly" \
  --mount "type=bind,source=$PWD/ui/fonts,target=/fonts,readonly" \
  --mount "type=bind,source=$smoke_dir,target=/fixtures" \
  "$AURAGO_OCR_SMOKE_IMAGE" python /scripts/smoke_fixtures.py /fixtures /fonts/RobotoCondensed-Variable.ttf
export AURAGO_OCR_SMOKE_DIR="$smoke_dir"
go test ./internal/ocr -run TestOCRDockerSmoke -count=1 -v
CGO_ENABLED=0 go test -c ./internal/ocr -o "$smoke_dir/ocr.test"
docker run --rm --network bridge --user 0 \
  --mount type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock \
  --mount "type=bind,source=$smoke_dir,target=/fixtures,readonly" \
  -e AURAGO_OCR_SMOKE_IMAGE -e AURAGO_OCR_SMOKE_IN_DOCKER=1 -e AURAGO_OCR_SMOKE_DIR=/fixtures \
  -e DOCKER_HOST=unix:///var/run/docker.sock \
  "$AURAGO_OCR_SMOKE_IMAGE" /fixtures/ocr.test -test.run TestOCRDockerSmoke -test.v

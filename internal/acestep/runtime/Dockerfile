# syntax=docker/dockerfile:1
FROM python:3.11.13-slim-bookworm@sha256:86adf8dbadc3d6e82ee5dd2c74bec2e1c2467cdad47886280501df722372d2e1 AS python
FROM ubuntu:24.04@sha256:4fbb8e6a8395de5a7550b33509421a2bafbc0aab6c06ba2cef9ebffbc7092d90
COPY --from=python /usr/local /usr/local
ARG BACKEND=cpu
ARG TORCH_INDEX=cpu
ARG UPSTREAM=ca1e85fe9430179831e6bc6be790c332190a3866
LABEL org.opencontainers.image.source="https://github.com/antibyte/AuraGo"
ENV DEBIAN_FRONTEND=noninteractive PIP_NO_CACHE_DIR=1 PYTHONUNBUFFERED=1 \
    PYTHONPATH=/opt/aurago:/app AURAGO_BACKEND=${BACKEND} HOME=/app/.cache USER=aurago \
    MPLCONFIGDIR=/app/.cache/matplotlib TORCHINDUCTOR_CACHE_DIR=/app/.cache/torchinductor \
    HF_HUB_OFFLINE=1 TRANSFORMERS_OFFLINE=1 HF_HUB_DISABLE_TELEMETRY=1 \
    ACESTEP_QUEUE_WORKERS=1 ACESTEP_API_WORKERS=1 ACESTEP_QUEUE_MAXSIZE=1 \
    ACESTEP_SAVE_MEMORY=1 TOKENIZERS_PARALLELISM=false
RUN apt-get -o Acquire::Retries=3 -o Acquire::ForceIPv4=true update && apt-get -o Acquire::Retries=3 -o Acquire::ForceIPv4=true install -y --no-install-recommends ca-certificates libssl3t64 libffi8 libexpat1 zlib1g libbz2-1.0 liblzma5 libsqlite3-0 git ffmpeg libsndfile1 libgomp1 libnuma1 libdrm2 libze1 build-essential \
    && rm -rf /var/lib/apt/lists/*
RUN pip install uv==0.7.22
WORKDIR /app
RUN git init . && git remote add origin https://github.com/ace-step/ACE-Step-1.5.git \
    && git fetch --depth=1 origin ${UPSTREAM} && git checkout --detach FETCH_HEAD \
    && test "$(git rev-parse HEAD)" = "${UPSTREAM}" && rm -rf .git
# Export the upstream lock; substitute only hardware-specific wheel families.
RUN uv export --frozen --no-dev --no-hashes --no-emit-project > /tmp/locked.txt \
    && python -c "from pathlib import Path; p=Path('/tmp/locked.txt'); p.write_text(''.join(l for l in p.read_text().splitlines(True) if not l.lstrip().startswith(('torch==','torchvision==','torchaudio==','torchcodec==','triton==','pytorch-triton','nvidia-','mlx','-e ','./','nano-vllm'))))" \
    && pip install --no-deps -r /tmp/locked.txt \
    && pip install torch==2.10.0 torchvision==0.25.0 torchaudio==2.10.0 --index-url https://download.pytorch.org/whl/${TORCH_INDEX} \
    && if [ "${BACKEND}" = "cuda" ]; then pip install --no-deps ./acestep/third_parts/nano-vllm; fi
COPY internal/acestep/runtime/ /opt/aurago/
COPY internal/acestep/release.json /opt/aurago/release.json
RUN python /opt/aurago/install_intel_runtime.py \
    && python /opt/aurago/patch_upstream.py /app \
    && mkdir -p /app/checkpoints /app/.cache/acestep/tmp/api_audio \
    && chown -R 65532:65532 /app/checkpoints /app/.cache \
    && python -X faulthandler -c "import torch, soundfile, acestep.api_server; print(torch.__version__)"
RUN AURAGO_BACKEND=cpu python /opt/aurago/test_runtime.py
USER 65532:65532
EXPOSE 8001
ENTRYPOINT ["python", "/opt/aurago/aurago_runtime.py"]
CMD ["serve"]

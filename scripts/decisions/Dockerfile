FROM ubuntu:24.04@sha256:4fbb8e6a8395de5a7550b33509421a2bafbc0aab6c06ba2cef9ebffbc7092d90
ARG BACKEND
ARG TARGETARCH
RUN if [ "$BACKEND" = vulkan ]; then apt-get update -qq && apt-get install -y --no-install-recommends mesa-vulkan-drivers && rm -rf /var/lib/apt/lists/*; fi
LABEL org.opencontainers.image.source="https://github.com/antibyte/AuraGo" \
      org.opencontainers.image.revision="d81235049384534c167caea52b85a694f6103d14" \
      org.aurago.decisions.llama_cpp_patch_sha256="a3e895bce9fafa23873a481ae29f31fdb98287e0f02939bdd1213c103271a790" \
      org.aurago.decisions.protocol="v1" \
      org.aurago.decisions.backend="${BACKEND}"
COPY linux-${TARGETARCH}-${BACKEND}/ /opt/decisions/
ENV LD_LIBRARY_PATH=/opt/decisions
ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=compute,utility
ENTRYPOINT ["/opt/decisions/aurago-decisions-runtime"]

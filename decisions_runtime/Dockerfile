ARG AURAGO_BUILD_IMAGE
FROM ${AURAGO_BUILD_IMAGE} AS build
ARG AURAGO_RUNTIME_BACKEND
ARG AURAGO_LLAMA_CPP_COMMIT=d81235049384534c167caea52b85a694f6103d14
ARG AURAGO_LLAMA_CPP_PATCH_SHA256=a3e895bce9fafa23873a481ae29f31fdb98287e0f02939bdd1213c103271a790
WORKDIR /src
COPY decisions_runtime/ decisions_runtime/
RUN test -n "$AURAGO_RUNTIME_BACKEND" \
    && cmake -S decisions_runtime -B /tmp/decisions-build \
      -DAURAGO_RUNTIME_BACKEND=${AURAGO_RUNTIME_BACKEND} \
      -DAURAGO_LLAMA_CPP_COMMIT=${AURAGO_LLAMA_CPP_COMMIT} \
      -DAURAGO_RUNTIME_OUTPUT_DIR=/runtime -DCMAKE_BUILD_TYPE=Release \
    && cmake --build /tmp/decisions-build --config Release --parallel 4 \
    && /runtime/aurago-decisions-runtime --self-test-protocol \
    && /runtime/aurago-decisions-runtime --discover >/tmp/discover.json

FROM ${AURAGO_BUILD_IMAGE} AS runtime
ARG AURAGO_RUNTIME_BACKEND
ARG AURAGO_LLAMA_CPP_COMMIT=d81235049384534c167caea52b85a694f6103d14
ARG AURAGO_LLAMA_CPP_PATCH_SHA256=a3e895bce9fafa23873a481ae29f31fdb98287e0f02939bdd1213c103271a790
LABEL org.opencontainers.image.title="AuraGo Decisions Runtime" \
      org.opencontainers.image.source="https://github.com/antibyte/AuraGo" \
      org.opencontainers.image.revision="${AURAGO_LLAMA_CPP_COMMIT}" \
      org.aurago.decisions.llama_cpp_patch_sha256="${AURAGO_LLAMA_CPP_PATCH_SHA256}" \
      org.aurago.decisions.protocol="v1" \
      org.aurago.decisions.backend="${AURAGO_RUNTIME_BACKEND}" \
      org.aurago.decisions.source="local-source-build"
COPY --from=build /runtime/aurago-decisions-runtime /usr/local/bin/aurago-decisions-runtime
ENTRYPOINT ["/usr/local/bin/aurago-decisions-runtime"]

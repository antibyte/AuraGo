FROM --platform=$BUILDPLATFORM rust:1.94-bookworm@sha256:6ae102bdbf528294bc79ad6e1fae682f6f7c2a6e6621506ba959f9685b308a55 AS build
ARG TARGETARCH
ARG BUILDARCH
RUN if [ "$TARGETARCH" != "$BUILDARCH" ]; then \
      test "$TARGETARCH" = arm64 && test "$BUILDARCH" = amd64 && \
      dpkg --add-architecture arm64 && apt-get update && \
      apt-get install -y --no-install-recommends gcc-aarch64-linux-gnu libc6-dev-arm64-cross libssl-dev:arm64 && \
      rm -rf /var/lib/apt/lists/*; fi
WORKDIR /source
COPY v2 v2
COPY vendor vendor
WORKDIR /source/v2
RUN if [ "$TARGETARCH" != "$BUILDARCH" ]; then \
      rustup target add --toolchain 1.89 aarch64-unknown-linux-gnu && \
      CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER=aarch64-linux-gnu-gcc \
      CC_aarch64_unknown_linux_gnu=aarch64-linux-gnu-gcc \
      PKG_CONFIG_ALLOW_CROSS=1 PKG_CONFIG_LIBDIR=/usr/lib/aarch64-linux-gnu/pkgconfig \
      cargo build --locked --release --target aarch64-unknown-linux-gnu --no-default-features -p wifi-densepose-sensing-server --bin sensing-server && \
      cp target/aarch64-unknown-linux-gnu/release/sensing-server /source/sensing-server; \
    else cargo build --locked --release --no-default-features -p wifi-densepose-sensing-server --bin sensing-server && \
      cp target/release/sensing-server /source/sensing-server; fi

FROM debian:bookworm-slim@sha256:7c7b2c966bc9ee8cedfeef67e0e279108992c77681fa595db4a9d65c06ccc587
COPY --from=build /source/sensing-server /app/sensing-server
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
LABEL org.opencontainers.image.source="https://github.com/ruvnet/RuView" org.opencontainers.image.revision="a71634ae8a606bba5463489450c69e8b42de114e"
WORKDIR /data
EXPOSE 3000/tcp 5005/udp
ENTRYPOINT ["/app/sensing-server"]
CMD ["--source", "esp32", "--data-dir", "/data", "--bind-addr", "0.0.0.0", "--no-edge-registry"]

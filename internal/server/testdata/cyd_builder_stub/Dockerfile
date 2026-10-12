# Test stub of the CYD Creator toolchain image for TestCYDBuilderStubMechanicsLive.
# It implements the cyd-build / cyd-addr2line contract without PlatformIO.
# Build it locally (never published):
#   docker build --label aurago.cyd-plan=1 -t aurago-cyd-builder-stub:test internal/server/testdata/cyd_builder_stub
FROM alpine:3.20
RUN adduser -D -u 1000 builder \
 && mkdir -p /work /cache \
 && chown 1000:1000 /work /cache
COPY cyd-build.sh /usr/local/bin/cyd-build
COPY cyd-addr2line.sh /usr/local/bin/cyd-addr2line
RUN chmod 0755 /usr/local/bin/cyd-build /usr/local/bin/cyd-addr2line
USER 1000:1000
WORKDIR /work

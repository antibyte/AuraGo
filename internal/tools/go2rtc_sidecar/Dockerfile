# Reviewed go2rtc v1.9.14 source + a single WebRTC lease-lifecycle fix.
ARG BASE_IMAGE
FROM golang:1.26.0-alpine@sha256:d4c4845f5d60c6a974c6000ce58ae079328d03ab7f721a0734277e69905473e5 AS build
ADD https://codeload.github.com/AlexxIT/go2rtc/tar.gz/b5948cfb25404cc5cb37b166ecaa2dca20b11d4b /source.tgz
RUN echo "78aa79bcedec8f155e4060a379613979b0b3ee48ff62ee5164bafc0ac6532386  /source.tgz" | sha256sum -c - && mkdir /src && tar xzf /source.tgz -C /src --strip-components=1
WORKDIR /src
COPY lifecycle.patch /lifecycle.patch
COPY lifecycle_test.go.txt /src/internal/webrtc/aurago_lifecycle_test.go
COPY aliases_test.go.txt /src/internal/streams/aurago_aliases_test.go
RUN patch -p1 < /lifecycle.patch && GOTOOLCHAIN=local go test -mod=readonly ./internal/webrtc ./internal/streams -run TestAuraGo -count=1 && CGO_ENABLED=0 GOTOOLCHAIN=local go build -mod=readonly -trimpath -ldflags "-s -w -X github.com/AlexxIT/go2rtc/internal/app.Version=1.9.14-aurago1" -o /aurago-go2rtc .
FROM ${BASE_IMAGE}
COPY --from=build /aurago-go2rtc /usr/local/bin/aurago-go2rtc
COPY --from=build /src/LICENSE /usr/share/licenses/aurago-go2rtc/LICENSE

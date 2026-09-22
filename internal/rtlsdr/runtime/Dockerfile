# Build from the repository root: docker build -f internal/rtlsdr/runtime/Dockerfile .
FROM ubuntu:24.04 AS native
ARG DEBIAN_FRONTEND=noninteractive
ARG SDRANGEL_REV=97e9e21fe9754b363ae70caac822802d908e8acd
ARG WELLE_REV=512558d1f8ac4c524d3c63e97510ea36c1bd7a70
ARG RTLSDR_REV=aed0ea19f3a273370a13c9009b96313c75d54c7b
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl git build-essential cmake pkg-config python3 python3-websocket python3-numpy \
    qtbase5-dev qtbase5-private-dev qtdeclarative5-dev qtmultimedia5-dev qttools5-dev libqt5websockets5-dev \
    libqt5serialport5-dev libqt5svg5-dev libqt5charts5-dev qtpositioning5-dev qtlocation5-dev \
    libqt5texttospeech5-dev libfftw3-dev libusb-1.0-0-dev libboost-all-dev libopus-dev \
    libfaad-dev libmpg123-dev libmp3lame-dev libflac++-dev libasound2-dev \
    pulseaudio pulseaudio-utils ffmpeg xxd && rm -rf /var/lib/apt/lists/*
WORKDIR /build
RUN curl -fsSL https://codeload.github.com/rtlsdrblog/rtl-sdr-blog/tar.gz/${RTLSDR_REV} | tar xz --strip-components=1 \
    && sed -i 's/set(VERSION "${GIT_DESCRIBE}")/set(VERSION "1.4.0")/' cmake/Modules/Version.cmake \
    && cmake -S . -B out -DDETACH_KERNEL_DRIVER=ON -DINSTALL_UDEV_RULES=OFF \
    && cmake --build out -j2 && cmake --install out && ldconfig
WORKDIR /build/sdrangel
RUN curl -fsSL https://codeload.github.com/f4exb/sdrangel/tar.gz/${SDRANGEL_REV} | tar xz --strip-components=1
COPY internal/rtlsdr/runtime/build_native.py /build/build_native.py
RUN python3 /build/build_native.py && cmake --build out -j2 && cmake --install out
WORKDIR /build/welle
RUN curl -fsSL https://codeload.github.com/AlbrechtL/welle.io/tar.gz/${WELLE_REV} | tar xz --strip-components=1 \
    && cmake -S . -B out -DBUILD_WELLE_IO=OFF -DRTLSDR=ON -DFLAC=ON -DAIRSPY=OFF -DSOAPYSDR=OFF -DCMAKE_INSTALL_PREFIX=/opt/welle \
    && cmake --build out -j2 && cmake --install out
RUN mkdir -p /opt/licenses/sdrangel /opt/licenses/welle /opt/licenses/rtl-sdr \
    && find /build/sdrangel -maxdepth 1 -type f \( -iname '*license*' -o -iname 'copying*' \) -exec cp '{}' /opt/licenses/sdrangel/ \; \
    && find /build/welle -maxdepth 1 -type f \( -iname '*license*' -o -iname 'copying*' \) -exec cp '{}' /opt/licenses/welle/ \; \
    && cp /build/COPYING /opt/licenses/rtl-sdr/ \
    && test -n "$(ls /opt/licenses/sdrangel)" && test -n "$(ls /opt/licenses/welle)"

FROM native AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends libqt5multimedia5-plugins \
    && rm -rf /var/lib/apt/lists/*
# The upstream AGC switch controls only digital gain. Include RF tuner AGC so
# the application's default automatic mode does not leave the tuner at 0 dB.
COPY internal/rtlsdr/runtime/tuner_agc.patch /build/tuner_agc.patch
RUN cd /build/sdrangel && git apply --check /build/tuner_agc.patch \
    && git apply /build/tuner_agc.patch \
    && cmake --build out --target inputrtlsdrsrv -j2 \
    && cmake --install out/pluginssrv/samplesource/rtlsdr
ENV PATH="/opt/sdrangel/bin:/opt/welle/bin:/usr/local/bin:${PATH}" \
    LD_LIBRARY_PATH="/opt/sdrangel/lib:/usr/local/lib" \
    HOME=/tmp/rtl-sdr XDG_RUNTIME_DIR=/tmp/rtl-sdr/run QT_QPA_PLATFORM=offscreen
COPY internal/rtlsdr/runtime/*.py /opt/aurago-rtl-sdr/
WORKDIR /opt/aurago-rtl-sdr
COPY internal/rtlsdr/runtime/tests/test_*.py tests/
RUN python3 -m unittest discover -s tests -p 'test_*.py'
USER 10001:10001
ENTRYPOINT ["python3", "worker.py"]

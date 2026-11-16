FROM ubuntu:24.04@sha256:73b999393d07f2bf65a40373778a44dc9a6d54ad7718d29247d8ff9413c4d6a8
ARG TARGETARCH
ENV DEBIAN_FRONTEND=noninteractive LANG=C.UTF-8 HOME=/tmp XDG_CONFIG_HOME=/tmp/config
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl gnupg python3 \
    && curl --fail --location --proto '=https' 'https://keyserver.ubuntu.com/pks/lookup?op=get&search=0xFDA854F61C4D0D9572BB95E5245D5502FAD7A805' -o /tmp/kicad.asc \
    && gpg --show-keys --with-colons /tmp/kicad.asc | grep -q '^fpr:::::::::FDA854F61C4D0D9572BB95E5245D5502FAD7A805:' \
    && gpg --dearmor --output /usr/share/keyrings/kicad.gpg /tmp/kicad.asc \
    && echo 'deb [signed-by=/usr/share/keyrings/kicad.gpg] https://ppa.launchpadcontent.net/kicad/kicad-10.0-releases/ubuntu noble main' > /etc/apt/sources.list.d/kicad.list \
    && apt-get update && cd /tmp && apt-get download 'kicad=10.0.8~ubuntu24.04.1' \
    && case "$TARGETARCH" in amd64) echo '38721bf8c584e9191e4f2738a4a7204f0062e4cc2c47e17c7614384d55b87e48  kicad_10.0.8~ubuntu24.04.1_amd64.deb' ;; arm64) echo '7bd560697fea055e334fdc123d9e52e95c3301801bcf3c8d04dcb5dc3a3d150f  kicad_10.0.8~ubuntu24.04.1_arm64.deb' ;; *) exit 1 ;; esac | sha256sum --check \
    && apt-get install -y --no-install-recommends ./kicad_*.deb kicad-symbols kicad-footprints kicad-templates \
    && test "$(kicad-cli version)" = '10.0.8' \
    && rm -rf /var/lib/apt/lists/* /tmp/kicad* \
    && apt-get clean
COPY export.py /opt/aurago/export.py
WORKDIR /job
CMD ["python3", "/opt/aurago/export.py"]

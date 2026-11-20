# Velxio

Velxio (davidmonterocrespo24, AGPL-3.0) is a browser simulator for Arduino,
ESP32 and Raspberry Pi Pico boards: write sketches, wire virtual circuits and
run them without hardware. AuraGo offers it in the Software Store as a Docker
app. **Connect to agent** lets AuraGo design circuits and compile sketches
through Velxio's own MCP server.

## Install

1. Open the Software Store on the Virtual Desktop and install **Velxio**.
2. The image is about 7 GB unpacked. Keep that much free space for Docker.
3. The first start downloads the Arduino cores. It needs internet access and
   takes several minutes on a slow link. Measured on Docker Desktop with fresh
   volumes: the web UI answered after 75 s and the agent endpoint after 79 s.
   The Store waits up to 15 minutes for the web UI.
4. On a very slow link, run `docker pull` for the pinned image before you
   install. The Store allows 30 minutes for the image pull and the first start
   together.
5. Open Velxio from the Desktop. It opens in its own browser tab, because the
   page does not allow embedding.

The Store pins the upstream image `ghcr.io/davidmonterocrespo24/velxio` by
digest (image tag `master`, built 2026-11-18; the last tagged release is
v3.0.5). The source is at
[github.com/davidmonterocrespo24/velxio](https://github.com/davidmonterocrespo24/velxio);
the image was built from commit `5ed52d7332b8bd87f4dcd71f1ed49d8ffad964aa`.
Velxio is AGPL-3.0; its author also sells a commercial licence, which a
self-hosted install for your own use does not need.

## Boards

The self-hosted edition covers Arduino AVR boards (including ATtiny85), the
Raspberry Pi Pico and the ESP32, ESP32-S3 and ESP32-C3 family, which is emulated
with QEMU. The first start installs the `arduino:avr`, `rp2040:rp2040` and
`ATTinyCore:avr` cores. ESP32 builds use the ESP-IDF bundled in the image.

STM32 and Raspberry Pi Linux boards exist only on velxio.dev. The in-editor AI
assistant of velxio.dev is not part of this edition.

ESP32 emulation has not been checked live.

## Access

- **Velxio has no login of its own.** Anyone who can open its web page can use
  it, including its compile feature. The default is local access on this
  machine. Do not choose LAN access, or share the page, unless the network is
  trusted.
- The page sends `X-Frame-Options: SAMEORIGIN`, so the Desktop cannot show it in
  a window. The Store opens it in its own tab.
- Apart from the agent port, the Store publishes only the web UI port. The
  agent port is never published on the LAN and never placed on Tailscale.

## Agent connection

**Connect to agent** needs no URL, account or token. Save the connection, then
run **Test**.

- **Read** gives five design tools: `create_circuit`, `update_circuit`,
  `import_wokwi_json`, `export_wokwi_json` and `generate_code_files`.
- **Execute** adds `compile_project` and `run_project`. In read-only Virtual
  Desktop mode the compile tools are withheld, even when Execute is saved.
- The bearer token is generated during install and kept in the AuraGo Vault
  under `desktop_store_velxio_mcp_token`. You never enter it, and the dialog
  never shows it.
- The MCP server listens inside the container on port 8002. The Store publishes
  that port on `127.0.0.1` only. It is never shown as an openable port, never
  previewed and never placed on Tailscale.
- AuraGo always derives the URL. Native AuraGo uses the published loopback
  port. Containerized AuraGo uses the container name on the shared Docker
  network that you select in the dialog; select that network and run a Store
  **Update**. A remote Docker Engine is not supported, because the loopback port
  of that host cannot be reached.
- The test reads the tool list and checks that the allowed tools exist. It does
  not compile anything.
- Tool results go to the agent. They do not open in a Velxio editor that is
  already open. Import the circuit or code in Velxio yourself.
- The launcher runs the MCP server in stateless HTTP mode.
- The MCP server starts a few seconds after the web UI, so the check right
  after a start, restart or update can fail. AuraGo then retries the connection
  in the background for up to 5 minutes.
- AuraGo stops a single MCP tool call after 60 seconds. A cold ESP32 build may
  take longer than that; this has not been measured. The `ccache` volume keeps
  compiler results, so a retry is faster.

## Security notes

- Velxio's own web API on port 80 has no login, and it can compile too. The
  agent token does not protect that path.
- The token protects only the agent endpoint. It is visible in `docker inspect`,
  and root processes in the container can read the launcher's environment. It
  is not a hard security boundary.
- The token also replaces the MCP server's localhost-only Host check, which
  would block AuraGo from reaching the server by container name. A neighbour
  container on the same shared network cannot call the endpoint without the
  token.
- The image runs nginx, the backend, arduino-cli and QEMU as root. The Store
  adds no extra hardening for it.

## Privacy

The Store sets `VELXIO_NEWS=off`. Velxio's README documents this as turning off
the news feed from velxio.dev and the remote media in its posts. AuraGo has not
checked whether the web UI loads other remote resources.

## Data, update and uninstall

Velxio keeps its data in five Docker volumes: `aurago_store_velxio_data`
(projects), `aurago_store_velxio_arduino15` (cores), `aurago_store_velxio_arduino`
(libraries), `aurago_store_velxio_ccache` (compiler cache) and
`aurago_store_velxio_build` (build output).

- **Update** pulls the pinned image and recreates the container. The volumes and
  the Vault token are kept.
- **Uninstall** keeps the volumes and the Vault token. A reinstall over them
  reuses the existing data and the downloaded cores.
- **Uninstall with Delete data** removes the five volumes, the Vault token and
  the agent connection.

The Store pins the image by digest. New Velxio versions arrive with an AuraGo
release that updates the pin.

## Live verification

Recorded on 2026-11-20 with Docker Desktop 29.9.3 (linux/amd64). The launcher
wrapper was run by hand against the pinned image. The run did not go through
the Store install.

- First start, fresh volumes: web UI `/health` answered after 75 s; the MCP
  server listened after 79 s.
- Restart, existing volumes: web UI after 17 s; MCP server after 24 s.
- The MCP server lists the 7 tools. A wrong or missing token gets 401. The
  right token gets an `initialize` answer 200.
- Compiling the blink sketch for `arduino:avr:uno` succeeded in about 1 s and
  returned a 2556-byte hex file.

**Still open:** the Store-path run. That covers the install through the Store,
the Connect to agent test, uninstall with Delete data, and containerized AuraGo
on a shared network. An agent chat that uses the tools has not been tested
either. ESP32 emulation and arm64 hosts have not been tested.

## Troubleshooting

- *Install stops at the readiness check:* the first start on a slow link can
  pass the wait. Run `docker pull` for the pinned image and install again.
- *Connect to agent says the MCP server is not reachable:* wait a minute after a
  start. AuraGo retries for up to 5 minutes. If it still fails, check
  `docker logs aurago-store-velxio`; the launcher prints lines that start with
  `aurago-velxio-mcp:`.
- *Connect to agent reports a missing token:* run a Store **Update**, or
  reinstall Velxio.
- *Containerized AuraGo cannot reach Velxio:* select the shared Docker network
  in **Connect to agent** and run a Store **Update**.
- *Compile tools are missing:* Execute is not saved, or the Virtual Desktop is
  read-only.
- *Not verified:* ESP32 emulation, arm64 hosts, containerized AuraGo on a shared
  network, and an agent chat that uses the tools.

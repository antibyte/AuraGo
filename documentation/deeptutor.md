# DeepTutor

DeepTutor (HKUDS, Apache-2.0) is an AI tutor with guided learning, knowledge
bases and chat. AuraGo offers it in the Software Store as a Docker app. It uses
the AuraGo LLM automatically: you do not enter a provider key in DeepTutor.

## Install

1. Open the Software Store on the Virtual Desktop and install **DeepTutor**.
2. The image is about 4 GB. The first start can take a minute or more; the
   Store waits up to five minutes for DeepTutor to report ready.
3. Open DeepTutor from the Desktop. Its model settings already list a profile
   named **AuraGo** and it is selected, unless you had set up another model
   before (then your choice stays).

The Store pins the official image `ghcr.io/hkuds/deeptutor` (1.7.2) by digest.
Data lives in the Docker volume `data` (mounted at `/app/data`).

## How the AuraGo LLM is used

DeepTutor never gets your provider key. AuraGo exposes a small OpenAI-compatible
relay at `/api/store-llm/deeptutor/v1` (`/models`, `/chat/completions`,
`/embeddings`). The Store gives the DeepTutor container a random relay token and
the relay address. AuraGo accepts that token only on this relay, only while
DeepTutor is installed, and forwards chat calls to the model AuraGo itself uses
(including its failover). The model name inside DeepTutor is the alias
`aurago`; the real model is whatever AuraGo currently uses.

At each container start a small script merges the **AuraGo** profile into
DeepTutor's `model_catalog.json`:

- It adds the profile and makes it active only if no other model profile is
  active. Your own profiles are never changed or removed.
- A restart does not repeat the merge unless the relay address or token changed.
  If you delete or replace the AuraGo profile in DeepTutor's settings, it does
  not come back by itself.
- After an AuraGo address change or token rotation, run **Update** in the Store;
  the Update recreates the container and refreshes the profile's address and
  token. Other fields of the profile you edited stay as they are.
- Vision support and the context window are taken from AuraGo when the profile
  is first created. Change them later in DeepTutor's settings if AuraGo's model
  changes.

Login cannot be disabled while DeepTutor is installed, including when stopped or when an installation is pending. Uninstall DeepTutor first. Rejected changes return `409 deeptutor_requires_login` and leave configuration and credentials unchanged. Manual invalid configuration prevents startup; a rejected reload retains the running valid settings. AuraGo never stops the container automatically to allow this change.

## Requirements

- **AuraGo login must be enabled** (Config > Auth, with a password set). The
  DeepTutor container can reach AuraGo's address, and without a login every
  AuraGo route would be open to it. The install stops with a clear message if
  login is off.
- **AuraGo must be reachable from Docker.** The container reaches AuraGo on the
  host through `host.docker.internal` (the Store adds the host-gateway mapping)
  over plain HTTP.
  - *Docker Desktop (Windows, macOS):* a loopback listener (`127.0.0.1`) is
    enough.
  - *Linux:* a stock install (`server.host` `127.0.0.1`, HTTPS on) is not
    reachable from containers. AuraGo needs a plain-HTTP listener on a
    non-loopback address, for example `AURAGO_SERVER_HOST=0.0.0.0` with HTTPS
    disabled for the relay port. Binding to all interfaces exposes AuraGo to the
    network, so keep login enabled and the firewall tight.

  If this is not the case, the install stops before downloading the image and
  tells you why. Fix the AuraGo server bind address or HTTPS setting and try
  again.
- **Embeddings** (knowledge bases) work only when AuraGo's embedding provider is
  an external OpenAI-compatible service. With a local or built-in embedding
  provider, DeepTutor gets no embedding profile (the relay answers 404
  `embeddings_unavailable`); configure an embedding model in DeepTutor's own
  settings if you want knowledge bases. If AuraGo later stops offering
  embeddings, remove the stale AuraGo embedding profile in DeepTutor.

## Security notes

- **DeepTutor has no login of its own.** Anyone who can open its web page can
  use it, and through it spend your AuraGo LLM. The default is local access on
  this machine only. Do not choose LAN access, or share the page, unless the
  network is trusted.
- The relay token is the only credential inside the container. It is visible in
  `docker inspect` and in the catalog file in the data volume. It opens nothing
  but the relay, and AuraGo refuses it as soon as DeepTutor is uninstalled. The
  real provider key never enters the container.
- The container starts as root and drops to DeepTutor's own user (an upstream
  design); the Store adds no extra hardening for this image.

## Rotate the token

Uninstall DeepTutor with **Delete data** (this removes the volume, including
your DeepTutor notebooks and knowledge bases) and install it again; a new token
is generated. An uninstall that keeps the data keeps the same token in the
AuraGo vault, but the relay rejects it while DeepTutor is not installed.

## Troubleshooting

- *Install fails with a relay message:* the relay address Docker can reach could
  not be determined; see Requirements above.
- *Chat fails in DeepTutor:* check that AuraGo is running and that its main LLM
  works; the relay returns the upstream error without provider keys.
- *The AuraGo profile is missing:* the container log (`docker logs
  aurago-store-deeptutor`) prints one `aurago-deeptutor:` line per start. A
  damaged `model_catalog.json` is kept as `model_catalog.json.corrupt-<time>`.
- *Not verified:* arm64 hosts, Linux host-gateway behavior and real provider
  traffic were not tested.

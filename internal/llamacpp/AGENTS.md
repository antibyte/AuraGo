# Shared llama.cpp build

## Purpose

The pinned llama.cpp build (`Build`, b9994) that local embeddings and the Chibi Sensei test chat both run: the release asset table, the Docker image digests, `llama-server` discovery, the loopback port and API key helpers, the GPU evidence check and hardware detection.

## Ownership

- `build.go` (`Build`, the Docker image constants, `DockerImage`), `manifest.go` (`AssetSpec`, `WindowsCUDARuntimeID`, the runtime manifest, `Runtimes`, `RuntimeAsset`, `SupplementalAssets`), `server.go` (`FindServer`, `GPUVerified`, `LoopbackPort`, `RandomAPIKey`), `hardware.go` (`HardwareInfo`, `DetectHardware`, `DetectHardwareWith`) and `vulkan_loader_{windows,other}.go`.
- Consumers: `internal/embeddings` (native and Docker llama.cpp; its wrappers, the Docker sidecar and `selectDockerModelMount` stay there) and `internal/chibisensei/chatrt` (the Chibi Sensei test chat runtime, its own install and process supervision).
- The package imports only the standard library and `internal/onnxrt`.
- Not here: the chat runtime's install, process and HTTP code (`internal/chibisensei/chatrt`), the embeddings sidecar, and the trainer's llama.cpp converter pin (`internal/chibisensei/trainer/requirements.lock`, checked against `Build` by a test).

## Local Contracts

- A pin change updates `Build`, every asset URL, size and SHA-256, the three Docker image digests, the trainer lock (`llama_cpp_tag`, `llama_cpp_url`, `llama_cpp_sha256`; `TestRuntimePinMatchesTrainerLock` in `internal/chibisensei`) and the llama.cpp licence rows in one change.
- Every asset passes `onnxrt.ValidateAssetSpec`, every URL carries the build tag (`/b9994/`), and `Runtimes()` returns a copy of the manifest.
- `DetectHardwareWith` puts the augment line (embeddings passes its Docker probe) before the `os=` and `arch=` lines. The hardware fingerprint is the SHA-256 of these evidence lines, and embeddings keys its cached selection on it: changing the evidence strings or their order invalidates every stored selection.
- `WindowsCUDARuntimeID` names the Windows CUDA runtime archive. `SupplementalAssets` returns it for windows/amd64/cuda only. Embeddings still looks the same ID up in its own manifest copy (`internal/embeddings/manager.go`), so renaming the ID breaks both paths.
- `GPUVerified(backend, logs)` needs the backend's name in the lower-cased log (for `metal`, the word `metal`) and one of `offloaded`, `using device` or `gpu layers`. Changing these words changes which GPU starts count as verified.
- `FindServer` returns the first `llama-server` (`llama-server.exe` on Windows, matched case-insensitively) below a root.
- `RandomAPIKey` returns 32 random bytes, URL-safe and unpadded (43 characters). The package does not register the key; the caller does (`security.RegisterSensitive`, as `chatrt` does).
- `DockerImage` covers cpu, cuda and vulkan only; `metal` has no image.

## Work Guidance

None yet.

## Verification

```
go test ./internal/llamacpp/... ./internal/embeddings/... ./internal/config/... -count=1
GOOS=linux GOARCH=amd64 go vet ./internal/llamacpp/ ./internal/embeddings/
GOOS=darwin GOARCH=arm64 go vet ./internal/llamacpp/ ./internal/embeddings/
go test ./internal/chibisensei -run '^TestRuntimePinMatchesTrainerLock$' -count=1
```

Tests: `TestRuntimeManifestPinsBuild`, `TestRuntimeAssetAndSupplements`, `TestDockerImageDigestPinned`, `TestServerHelpers`, `TestDetectHardwareWithAugment`.

## Child DOX Index

None.

# Chibi Sensei chat runtime

## Purpose

Supervisor of the local llama.cpp runtime of the Chibi Sensei test chat: it installs the pinned `llama-server` (`llamacpp.Build`) into its own cache, starts at most `MaxInstances` (2) loopback child processes, streams chat completions from them and stops them. It never imports another Chibi Sensei package.

## Ownership

- `types.go`, `errors.go`, `launcher.go` (the `Launcher` seam, `LaunchSpec`, `Process`), `manager.go` (`NewManager`, `Start`, `Status`, `StopAll`, `Close`, the reaper), `acquire.go` (`Admit`, `Acquire`, `Instance`, `Release`), `install.go` (`Installed`, `Install`, `BeginInstall`), `backend.go` (backend resolution and launch assets), `messages.go` (`PrepareMessages`), `client.go` (loopback client, request body, `streamChat`), `sse.go` (stream parser, upstream error classification), `native_launcher.go` (`NativeLauncher`, install set, argv and environment), `native_process.go` (start, health wait, log, chat, stop), `process_windows.go`, `process_other.go`, `logbuf.go`.
- `chatrttest/`: the exported fake launcher for server tests and the browser smoke. It is test support in the production tree.
- `real_smoke_test.go` and `testdata/` (`stream_basic.txt`, `context_error.json`, `context_error_stream.txt` from the b9994 discovery run; `stream_tolerant.txt` is synthetic).
- The audit inventory row for `internal/chibisensei/chatrt/` in `internal/audit/audit_manifests.go` (`NetworkClientInventory`) belongs to this package's HTTP client.
- Not here: the test step's domain logic and the smoke seam (`internal/chibisensei`), and the routes and the Smoker adapter (`internal/server/chibi_sensei_*.go`).

## Local Contracts

- Nothing downloads before `BeginInstall`. `Status`, `Admit` and `Acquire` never install. `Installed` only checks the files on disk, with no network access.
- The cache root is `Config.CacheDir`. The Chibi Sensei server sets it to `<data>/chibi_sensei/runtime`, never to `<data>/embeddings`, because embeddings removes the GPU runtimes it does not use.
- The install set is `Assets(selected)`: the CPU archive first, then the selected backend's archive and its supplements, without duplicate IDs (darwin/arm64 CPU and Metal are one archive). A platform without a CPU archive is `unsupported`.
- Backend resolution: the hardware probe runs on the manager's lifetime context, never a request context, and a result found while the manager closes is not cached. `auto` picks `metal` on darwin/arm64, else `cuda` when NVIDIA is present and a CUDA archive exists, else `vulkan` when Vulkan is present and a Vulkan archive exists, else `cpu`. An explicit `cuda` or `vulkan` without an archive for the platform runs on `cpu`. Hardware is probed only for `auto`.
- A change of the configured backend drops the cached resolution, the install check and the sticky GPU fallback.
- Argv carries no secret. The key goes to the child in `LLAMA_API_KEY`, is registered with `security.RegisterSensitive` before the start, and is never logged. Every inherited `LLAMA_*` variable is removed from the child environment.
- The model file is checked with `os.Lstat` right before a start (`modelFileOK`): a regular file, no symbolic link, directory or device, whose size equals `ModelRef.Bytes` when that is above 0; anything else is `ErrModelMissing`.
- One API key per `nativeLauncher`: created on first use, registered once with `security.RegisterSensitive` and given to every child it starts, so the registry does not grow with each idle stop and restart.
- A model file comes from a training run of the user's own Kaggle account; AuraGo checks its size, the manifest hash at download and the GGUF header, and then hands it to `llama-server`, which parses it as native code without an operating-system sandbox.
- Child arguments: `--host 127.0.0.1`, `--port` (a fresh loopback port), `--alias chibi`, `--parallel 1`, `--jinja`, `--reasoning off`, `--no-webui`, `--no-slots`, `--ctx-size` from the config, `--threads` and `--threads-batch` at `instanceThreads(NumCPU)` (a quarter of the logical processors, 1 to 8: two instances together use about half of them), `--n-gpu-layers 0` on cpu and `999` otherwise.
- Every chat request sends `chat_template_kwargs: {"enable_thinking": false}` and `cache_prompt: true`, and asks for `stream_options.include_usage`. The caller prepares the messages with `PrepareMessages` (the `merge_into_first_user` rule of the trainer); `Chat` does not.
- The HTTP client is built by `newLoopbackClient`: no proxy, no redirects (the key cannot leave the loopback origin) and `DisableKeepAlives`, because llama-server closes idle connections and a reused one failed with EOF in the discovery run. Cancelling the context closes the request, and llama-server stops the generation at once (`stop: cancel task` in its log).
- Stream parsing is tolerant. `data:` with or without a space, comments and `event:`, `id:`, `retry:` lines are accepted or skipped; `[DONE]` ends the stream; up to 16 undecodable payloads are skipped; a line above 1 MiB or a stream above 8 MiB is an error; an answer that sends no byte for 2 minutes (`streamIdleTimeout`) is cancelled with `ErrStalled`; reasoning fields are never decoded. A context-size error (type containing `context`, or the message pattern) maps to `ErrContext`; every other upstream error maps to a short error (type and message scrubbed with `security.Scrub`, each cut to 200 characters) that the server replaces with a fixed text.
- Instances: at most `MaxInstances` at a time, one request per instance (the slot). An instance that is being stopped (reaper, `StopAll`, eviction) still counts until its process has stopped, so no more than `MaxInstances` children ever run. `Acquire` waits up to `busyWait` (30 s) for a busy instance of the same key, then answers `ErrBusy`. A new model when both slots are full evicts the least recently used idle instance (ready, free slot, no waiter); without one it answers `ErrBusy` at once. `Admit` gives the same answer without starting anything.
- The reaper runs every 30 s and stops each ready instance that has no request, no waiter and has been idle for `IdleMinutes`. With the runtime disabled it stops every instance.
- GPU fallback: a GPU start that fails or shows no GPU evidence in its log is stopped, and the model starts again on CPU. `Status.Fallback` stays set (sticky) until the next `BeginInstall` or a change of the backend, so later instances start on CPU directly.
- Lock rule: `Manager.mu` is never held across `Hardware`, `Verify`, `Install`, `Launch`, `Chat` or `Stop`. Values are copied under the lock first.
- Install: free disk must be at least 4 times the install set plus 64 MiB, otherwise the install ends `disk_full` without a download. Error codes: `disk_full`, `checksum_mismatch` (the error text names SHA-256, the content length or a received size), otherwise `download_failed`. The raw error reaches the log only through `security.Scrub`. An install runs at most 60 minutes.
- Start: `waitHealthy` polls `GET /health` every 250 ms without a key (2 s per request; 503 means loading) and gives up after 3 minutes. A child that exits with a port-in-use message in its log gets one retry on a new port. A failed start is not cached: the next `Acquire` starts again.
- On Linux and macOS the launcher recreates the archive's link entries in `<cache>/links/<asset id>/` (`ensureArchiveLinks`), validated and pointing only at regular files of the verified runtime, and puts them on the library path, because the shared extractor skips links and `llama-server` needs the short library names.
- Child lifetime: on Windows the child runs in a job object with `KILL_ON_JOB_CLOSE`, so it ends when AuraGo exits or crashes. Elsewhere a plain process is started and killed by handle; `Stop` waits up to 10 s.
- `Acquire` checks the model file: it must be a regular file of the recorded size, otherwise `ErrModelMissing`. The package never checks the GGUF header; the caller does (`chibisensei.RunModels`).
- `Instance.Chat` answers `ErrStopped` after `StopAll` or `Close`, the context error after a cancel, and wraps the cause when the process died (the instance is then dropped).

## Work Guidance

None yet.

## Verification

```
go build ./...
go vet ./internal/chibisensei/chatrt/...
GOOS=linux GOARCH=amd64 go vet ./internal/chibisensei/chatrt/...
GOOS=darwin GOARCH=arm64 go vet ./internal/chibisensei/chatrt/...
go test ./internal/chibisensei/chatrt/... -count=1
go test ./internal/audit/ -count=1 -run '(?i)(NetworkClientInventory|CredentialedHTTPClients|SubprocessLaunches)'
```

Gated real smoke (downloads the pinned CPU runtime, needs a chat GGUF, `-timeout 20m`):

```
AURAGO_RUN_LLAMA_SMOKE=1 CHIBI_SMOKE_GGUF=<path to .gguf> go test ./internal/chibisensei/chatrt -run '^TestRealLlamaServerChat$' -count=1 -v -timeout 20m
```

Optional variables: `AURAGO_LLAMA_SMOKE_CACHE` (cache directory, default a temp directory), `CHIBI_SMOKE_RECORD` (directory for the raw outputs), `CHIBI_SMOKE_THREADS`. Re-record the fixtures with this test when the pin changes.

Verified with llama.cpp b9994 and a Qwen3.5 GGUF on 2026-11-27, on windows/amd64 CPU and on linux/amd64 CPU (Ubuntu, glibc 2.43, with `ensureArchiveLinks`; start, chat, cancel and stop by handle): the flags, the stream framing, `--reasoning off` (answers without reasoning fields), the context-size answer (HTTP 400, `type` `exceed_context_size_error`, the same body with and without `stream`), and the cancel at the first delta.

Not verified yet (plain statements):
- A `kill -9` of a hand-started AuraGo on Linux or macOS leaves `llama-server` running on loopback; systemd and Docker end it with AuraGo. macOS is not tested at all (process handling, the archive's links).
- The shared libraries the Linux binaries need inside AuraGo's Debian bookworm image (the Ubuntu test server has them), and the Vulkan loader for the Vulkan archive.
- Vulkan and CUDA starts, and their GPU evidence text at b9994.
- Speed and memory of two instances side by side on a quiet machine (two instances were run together on Windows through the gated server tests; a single instance on the 12-thread Linux test server generated 7 to 10 tokens per second with a 4B Q4 model on 3 threads).
- The disk rule (4 times the archive bytes plus 64 MiB) against real extracted sizes.
- Running inside AuraGo's Docker container: the native binary there is not tested.

## Child DOX Index

None.

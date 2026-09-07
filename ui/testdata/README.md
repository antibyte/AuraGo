# TeeVee browser fixture

`teevee-fixture.js` supplies a deterministic German catalog to the real desktop
shell. It is loaded only by `TestDesktopTeeVeeBrowser` and local preview tooling.
The testdata directory is not included by `ui/embed.go`.

`teevee-test.mp4` and `teevee-test.ts` contain the same locally generated,
synthetic FFmpeg `testsrc2` color bars/moving pattern with a visible clock:
640×360, 30 fps, three seconds, H.264/yuv420p, no audio or third-party media.
The browser test encrypts the transport stream in memory with a synthetic
AES-128 fixture key to exercise the actual hls.js decoder.

Run from the repository root:

```powershell
$env:AURAGO_RUN_BROWSER_SMOKE='1'
$env:AURAGO_BROWSER_ARTIFACT_DIR='../reports/teevee-browser'
go test ./ui -run '^TestDesktopTeeVeeBrowser$' -count=1 -v
```

Set `AURAGO_TEEVEE_RECORD=1` to save 30 sampled browser frames and separate
native/CRT performance JSON. The optional `gl.finish()` timing is diagnostic
only and never runs in the shipped renderer. It includes CPU submission and
GPU completion; it is not an isolated GPU timer.

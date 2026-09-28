# Screensaver posters

The posters (`<scene>.webp`, 1920x1080) and settings thumbnails (`<scene>-thumb.webp`,
480x270) are frames captured from AuraGo's own screensaver renderers in
`ui/js/desktop/screensavers/`. They contain no third-party imagery; the Sternenstaub
poster shows the AuraGo logo assembled from particles.

Regenerate them with a real GPU:

```powershell
$env:AURAGO_SCREENSAVER_GPU='1'; $env:AURAGO_SCREENSAVER_POSTERS='../reports/screensaver-posters'
go test -count=1 ./ui -run TestDesktopScreensaverPosterCapture
```

Set `AURAGO_SCREENSAVER_FAKE_CLOCK` to a logo minute (minute % 3 == 1, second 14)
when capturing Sternenstaub so the poster shows the timeless logo instead of a clock.

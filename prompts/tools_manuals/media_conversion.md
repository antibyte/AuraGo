# media_conversion

Convert audio, video, and image files inside the workspace.

## What it does

- Converts audio with `ffmpeg`
- Converts video with `ffmpeg`
- Converts image formats with `ImageMagick`
- Reads media metadata with `info`

## Safety rules

- Only use files inside the AuraGo workspace
- Prefer `info` first when you need codec, duration, dimensions, channels, or sample rate
- Use explicit `output_file` or `output_format` so the result path is predictable
- Respect read-only mode: only `info` is allowed there
- `output_file` and `output_format` must agree: an `output_file` without an extension gets
  the extension of `output_format`, and a contradicting extension is rejected
- `audio_convert` always drops the video track, so the result is a pure audio file

## Supported output formats

| Operation | Formats |
| --- | --- |
| `audio_convert` | mp3, wav, aac, m4a, ogg, flac, opus, aiff, wma |
| `video_convert` | mp4, webm, mkv, mov, avi, m4v, mpeg, ts |
| `image_convert` | png, jpg, jpeg, webp, gif, tiff, tif, bmp |

A format outside the list is rejected with an error that names the supported ones.

## Operations

### `info`

Required:

- `file_path`

Returns media metadata such as:

- format
- duration
- width / height
- audio channels
- sample rate
- audio codec
- video codec

### `audio_convert`

Required:

- `file_path`
- `output_file` or `output_format`

Optional:

- `audio_codec`
- `audio_bitrate`
- `sample_rate`

Example:

```json
{
  "operation": "audio_convert",
  "file_path": "recordings/interview.wav",
  "output_format": "mp3",
  "audio_codec": "libmp3lame",
  "audio_bitrate": "192k"
}
```

### `video_convert`

Required:

- `file_path`
- `output_file` or `output_format`

Optional:

- `video_codec`
- `audio_codec`
- `video_bitrate`
- `audio_bitrate`
- `width`
- `height`
- `fps`

Example:

```json
{
  "operation": "video_convert",
  "file_path": "videos/demo.mov",
  "output_file": "videos/demo.mp4",
  "video_codec": "libx264",
  "audio_codec": "aac",
  "width": 1280,
  "height": 720
}
```

### `image_convert`

Required:

- `file_path`
- `output_file` or `output_format`

Optional:

- `width`
- `height`
- `quality_pct`

Example:

```json
{
  "operation": "image_convert",
  "file_path": "images/source.png",
  "output_format": "webp",
  "quality_pct": 85
}
```

## Result

A successful conversion returns:

- `file` — the resulting file inside the workspace
- `output_format` — the format that was actually produced
- `output_size_bytes` — the size of the resulting file
- `width` / `height` — the dimensions of the result, when known
- `duration_seconds` — the duration of the result, when known

The result is registered in the media registry, so it can be reused with the media
tools without registering it again.

A conversion that exceeds the configured timeout fails with a `timed out` message
instead of a generic process error.

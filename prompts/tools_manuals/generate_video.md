# generate_video

Use `generate_video` when the user asks AuraGo to create a short video clip from a text prompt or to animate an image.

## Capabilities

- Text-to-video from a detailed prompt.
- Image-to-video via `first_frame_image`.
- First/last-frame guidance via `first_frame_image` and `last_frame_image` when the provider supports it.
- Subject/reference guidance via `reference_images` when the provider supports it.
- Saves the generated MP4 under `/files/generated_videos/...` and registers it in the media registry.

## Providers

- MiniMax Hailuo through `video_generation.provider`; default API model is `MiniMax-Hailuo-2.3` with a 6-second clip. Resolution is sent separately, defaulting to `768P`.
- Google Veo through `video_generation.provider`; default fallback is `veo-3.1-generate-preview` if no model is configured.

The provider is chosen in Settings > Video Generation, not per tool call. Only set `model` when the user explicitly asks for a model that belongs to the configured provider; otherwise leave it empty.

## Arguments

- `prompt` (required): describe the subject, action, camera movement, style, lighting, and mood.
- `duration_seconds` (optional): clip length. If omitted, AuraGo uses `video_generation.default_duration_seconds`.
- `resolution` (optional): for example `768P`, `1080P`, or `720p`.
- `aspect_ratio` (optional): for example `16:9`, `9:16`, or `1:1`.
- `negative_prompt` (optional): unwanted elements to avoid, if supported by the provider.
- `first_frame_image` (optional): URL or base64 image to animate from.
- `last_frame_image` (optional): URL or base64 image for ending-frame guidance.
- `reference_images` (optional): array of URL/base64 images for subject consistency.
- `model` (optional): per-call model override.

## Notes

Video generation is asynchronous and can take several minutes. The tool polls provider status until completion, failure, or `video_generation.timeout_seconds`.

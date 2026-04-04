# Image Generation Tool — Detailed Manual

## Overview
The `generate_image` tool creates images from text descriptions using various AI image generation providers. It supports text-to-image and image-to-image generation with configurable size, quality, and style options.

**IMPORTANT:** `generate_image` is a **native tool** that must be called directly via function calling. Do NOT use `execute_skill` to invoke it — it is not a skill.

## Configuration
Configure in Settings > Image Generation:
- **Provider** — Select an existing provider entry (OpenRouter, OpenAI, Stability AI, Ideogram, Google)
- **Model** — Override model name (empty = provider default)
- **Defaults** — Size, quality, style applied when not specified per-call

## Parameters

### prompt (required)
The text description of the image to generate. Be as descriptive as possible.

Good: "A serene Japanese garden at sunset, with a wooden bridge over a koi pond, cherry blossoms falling, soft golden light, watercolor style"
Bad: "garden"

### size
Image dimensions. Common values:
- `256x256` — Small, fast
- `512x512` — Medium
- `1024x1024` — Standard (default)
- `1024x1792` — Portrait
- `1792x1024` — Landscape

Note: Not all sizes are supported by all providers. The backend will map to the closest supported option.

### quality
- `standard` — Default quality, faster generation
- `hd` — Higher detail, slower generation (not all providers support this)

### style
- `natural` — Realistic, photographic look (default)
- `vivid` — More artistic, dramatic colors and composition

### model
Override the configured model. Usually leave empty to use the provider's default:
- OpenAI: `dall-e-3` or `dall-e-2`
- Stability AI: `sd3-medium`, `sd3-large`
- Ideogram: `V_2`
- Google Imagen: `imagen-3.0-generate-002`
- OpenRouter: any supported image model

### source_image
Path to an existing image for image-to-image generation. The AI will use this as a starting point and modify it based on the prompt.

Supported paths:
- Filename in workspace: `photo.jpg`
- Path in data directory: `generated_images/img_20240101_abc123.png`
- Absolute path: `/path/to/image.png`

### enhance_prompt
Boolean override for the prompt enhancement setting:
- `true` — Force prompt enhancement (LLM rewrites the prompt for better results)
- `false` — Force using the original prompt as-is
- Omit to use the configured default

## Examples

### Simple generation
```json
{
  "prompt": "A cute robot sitting in a field of sunflowers, digital art"
}
```

### With specific settings
```json
{
  "prompt": "Professional headshot of a business person, studio lighting",
  "size": "1024x1024",
  "quality": "hd",
  "style": "natural"
}
```

### Portrait format
```json
{
  "prompt": "A tall lighthouse on a cliff during a storm, dramatic oil painting",
  "size": "1024x1792",
  "quality": "hd",
  "style": "vivid"
}
```

### Image-to-image
```json
{
  "prompt": "Transform this photo into a watercolor painting with warm autumn colors",
  "source_image": "photo.jpg"
}
```

### With prompt enhancement disabled
```json
{
  "prompt": "pixel art sword, 16x16, transparent background",
  "enhance_prompt": false
}
```

## Output
Returns JSON with:
- `web_path` — URL to view/embed the image
- `markdown` — Ready-to-use markdown image embed
- `prompt` — Original prompt
- `enhanced_prompt` — Enhanced prompt (if enhancement was used)
- `model` — Model used
- `provider` — Provider type
- `size` — Actual size
- `duration_ms` — Generation time in milliseconds

## Gallery
All generated images are saved to the Image Gallery, accessible at `/gallery`. The gallery shows thumbnails, metadata, and supports search/filter by provider and prompt text.

## Cost
Estimated costs per image (vary by provider and settings):
- OpenAI DALL-E 3: ~$0.04 (standard) to ~$0.08 (HD)
- OpenRouter: ~$0.02-$0.05
- Stability AI: ~$0.03-$0.06
- Ideogram: ~$0.05-$0.08
- Google Imagen: ~$0.02-$0.04

Monthly generation limits can be configured in settings.

---
id: "tools_image_generation"
tags: ["conditional"]
priority: 33
conditions: ["image_generation_enabled"]
---
### Image Generation
| Tool | Purpose |
|---|---|
| `generate_image` | Generate images from text prompts using AI models |

**IMPORTANT:** `generate_image` is a **native tool**, NOT a skill. Call it directly via function calling — do NOT use `execute_skill`.

**Parameters:**
- `prompt` (required) — Text description of the image to generate
- `size` — Image dimensions (e.g. `1024x1024`, `1024x1792`, `512x512`)
- `quality` — Generation quality: `standard` or `hd`
- `style` — Visual style: `natural` or `vivid`
- `model` — Override model (usually not needed, uses configured default)
- `source_image` — Path to an existing image for image-to-image generation
- `enhance_prompt` — Set to `true`/`false` to override the default prompt enhancement setting

**Supported providers:** OpenRouter, OpenAI (DALL-E), Stability AI, Ideogram, Google Imagen

**Tips:**
- Be descriptive: include subject, style, lighting, composition, colors
- For image-to-image: provide `source_image` (filename in workspace or generated_images)
- Generated images are saved and viewable in the Image Gallery (/gallery)
- Cost varies by provider and quality setting

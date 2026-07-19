## Tool: Vision / Image Analysis (`analyze_image`)

Analyze an image using the configured Vision LLM (e.g., Gemini, GPT-4o, Agnes AI). Use this to describe images, read text from screenshots, identify objects, or answer questions about visual content.

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `file_path` | string | conditional | Path to a local image file (JPEG, PNG, GIF, WebP, BMP) |
| `image_url` | string | conditional | Publicly reachable HTTP(S) image URL |
| `prompt` | string | no | Custom prompt for the analysis (default: general description) |

Provide exactly one of `file_path` or `image_url`.

### Examples

**Describe an image:**
```json
{"action": "analyze_image", "file_path": "agent_workspace/workdir/screenshot.png"}
```

**OCR / Read text from image:**
```json
{"action": "analyze_image", "file_path": "agent_workspace/workdir/document.jpg", "prompt": "Extract all visible text from this image. Return the text verbatim."}
```

**Custom analysis:**
```json
{"action": "analyze_image", "file_path": "agent_workspace/workdir/chart.png", "prompt": "Analyze this chart. What trends do you see? Provide the approximate values."}
```

**Analyze a public image URL:**
```json
{"action": "analyze_image", "image_url": "https://images.example.com/chart.png", "prompt": "Summarize the chart."}
```

### Notes
- The file must exist on the local filesystem within the workspace
- Supported formats: JPEG, PNG, GIF, WebP, BMP
- Uses the Vision API configured in `config.yaml` (vision section)
- Large images are base64-encoded and sent directly; keep file sizes reasonable
- Agnes AI accepts only publicly reachable HTTP(S) `image_url` values. Local files, private/internal URLs, localhost, and base64/data URLs are rejected.

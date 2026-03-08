## Tool: Vision / Image Analysis (`analyze_image`)

Analyze an image file using the configured Vision LLM (e.g., Gemini, GPT-4o). Use this to describe images, read text from screenshots, identify objects, or answer questions about visual content.

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `file_path` | string | yes | Path to the image file (JPEG, PNG, GIF, WebP, BMP) |
| `prompt` | string | no | Custom prompt for the analysis (default: general description) |

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

### Notes
- The file must exist on the local filesystem within the workspace
- Supported formats: JPEG, PNG, GIF, WebP, BMP
- Uses the Vision API configured in `config.yaml` (vision section)
- Large images are base64-encoded and sent directly; keep file sizes reasonable

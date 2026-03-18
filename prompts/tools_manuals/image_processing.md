---
tool: image_processing
version: 1
tags: ["always"]
---

# Image Processing Tool

Process and transform images locally. Supports PNG, JPEG, GIF, BMP, and TIFF formats.

## Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `resize` | Resize image (maintains aspect ratio if only width or height given) | `file_path`, `width` and/or `height` |
| `convert` | Convert between image formats | `file_path`, `output_format` or `output_file` |
| `compress` | Reduce file size (converts to JPEG with quality control) | `file_path`, optionally `quality_pct` |
| `crop` | Extract rectangular region | `file_path`, `crop_x`, `crop_y`, `crop_width`, `crop_height` |
| `rotate` | Rotate by 90°, 180°, or 270° | `file_path`, `angle` |
| `info` | Get image dimensions, format, and file size | `file_path` |

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | string | **Required.** Operation to perform |
| `file_path` | string | **Required.** Input image file path |
| `output_file` | string | Output file path (auto-generated if omitted) |
| `output_format` | string | Target format: `png`, `jpeg`, `gif`, `bmp`, `tiff` |
| `width` | integer | Target width in pixels |
| `height` | integer | Target height in pixels |
| `quality_pct` | integer | Quality percentage 1-100 (default: 85 for resize, 75 for compress) |
| `crop_x` | integer | Crop region start X coordinate |
| `crop_y` | integer | Crop region start Y coordinate |
| `crop_width` | integer | Crop region width |
| `crop_height` | integer | Crop region height |
| `angle` | integer | Rotation: 90, 180, or 270 degrees |

## Examples

Resize to 800px wide (auto-height):
```json
{"operation": "resize", "file_path": "photo.jpg", "width": 800}
```

Convert PNG to JPEG:
```json
{"operation": "convert", "file_path": "screenshot.png", "output_format": "jpeg"}
```

Compress with 60% quality:
```json
{"operation": "compress", "file_path": "photo.jpg", "quality_pct": 60}
```

Crop 200x200 from top-left corner:
```json
{"operation": "crop", "file_path": "image.png", "crop_x": 0, "crop_y": 0, "crop_width": 200, "crop_height": 200}
```

Rotate 90° clockwise:
```json
{"operation": "rotate", "file_path": "photo.jpg", "angle": 90}
```

Get image info:
```json
{"operation": "info", "file_path": "photo.jpg"}
```

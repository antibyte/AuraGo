"""Bounded PDF and image conversion; JSON lines are the private wire protocol."""
import base64
import json
import os
import sys

MAX_PAGES = 100
MAX_PIXELS = 25_000_000


def emit(page, index, preview=None):
    data = {
        "index": index, "text": page.text, "markdown": page.markdown,
        "source": page.source, "note": page.note,
        "lines": [{"box": [float(v) for v in line.box], "text": line.text,
                   "confidence": float(line.conf)} for line in page.lines],
        "blocks": [{"label": block["label"], "bbox": [float(v) for v in block["bbox"]],
                    "text": block["text"]} for block in page.blocks],
    }
    if preview is not None:
        import cv2
        height, width = preview.shape[:2]
        scale = min(1, 1200 / max(height, width))
        thumb = cv2.resize(preview, (max(1, round(width * scale)), max(1, round(height * scale))))
        ok, image = cv2.imencode(".jpg", thumb, [cv2.IMWRITE_JPEG_QUALITY, 75])
        if ok:
            data["preview"] = base64.b64encode(image).decode("ascii")
    print(json.dumps({"type": "page", "page": data}, ensure_ascii=False), flush=True)


def convert(path, mode):
    import cv2
    import indra_ocr
    import pypdfium2 as pdfium
    from PIL import Image
    cv2.setNumThreads(1)

    converter = indra_ocr.load(path="/models", threads=min(4, indra_ocr.usable_cores()), mode=mode, dpi=200)
    if path.lower().endswith(".pdf"):
        with pdfium.PdfDocument(path) as doc:
            if not 0 < len(doc) <= MAX_PAGES:
                raise ValueError("page_limit")
            for index in range(len(doc)):
                page = doc[index]
                try:
                    width, height = page.get_size()
                    if width <= 0 or height <= 0 or width * height * (200 / 72) ** 2 > MAX_PIXELS:
                        raise ValueError("pixel_limit")
                finally:
                    page.close()
        for index, page in converter.convert(path):
            emit(page, index)
    else:
        Image.MAX_IMAGE_PIXELS = MAX_PIXELS
        with Image.open(path) as image:
            count = getattr(image, "n_frames", 1)
            if not 0 < count <= MAX_PAGES:
                raise ValueError("page_limit")
            # Indra's image helper decodes every TIFF frame before applying max_pages.
            for index in range(count):
                image.seek(index)
                if image.width * image.height > MAX_PIXELS:
                    raise ValueError("pixel_limit")
            for index in range(count):
                image.seek(index)
                import numpy as np
                gray = np.asarray(image.convert("L"))
                emit(converter.ocr(gray), index, gray)
    print('{"type":"done"}', flush=True)


if __name__ == "__main__":
    try:
        convert(sys.argv[1], sys.argv[2])
    except Exception:
        print('{"type":"error","code":"processing_failed"}', flush=True)
        raise SystemExit(1)

"""Generate deterministic synthetic OCR inputs inside the pinned test image."""
from pathlib import Path
import sys
from PIL import Image, ImageDraw, ImageFont

root = Path(sys.argv[1])
root.mkdir(parents=True, exist_ok=True)
font_path = Path(sys.argv[2]) if len(sys.argv) > 2 else Path(__file__).resolve().parents[2] / "ui/fonts/RobotoCondensed-Variable.ttf"
font = ImageFont.truetype(str(font_path), size=40)
pages = []
for text in ("Deutscher Test: Grüße aus AuraGo. Rechnung 12345.", "Zweite Seite 67890. Lokale Texterkennung."):
    page = Image.new("RGB", (1400, 240), "white")
    ImageDraw.Draw(page).text((40, 70), text, font=font, fill="black")
    pages.append(page)
pages[0].save(root / "scan.png")
turned = Image.new("RGB", (1000, 1400), "white")
for index in range(16):
    ImageDraw.Draw(turned).text((40, 60 + index * 78),
        f"Rechnung 12345. Position {index + 1}: Grüße aus AuraGo.", font=font, fill="black")
turned.rotate(90, expand=True).save(root / "rotated.png")
pages[0].save(root / "pages.tiff", save_all=True, append_images=pages[1:])
pdf = bytearray(b"%PDF-1.4\n")
stream = b"BT /F1 18 Tf 50 750 Td (AuraGo digital document 24680) Tj ET"
objects = [
    b"<< /Type /Catalog /Pages 2 0 R >>",
    b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
    b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
    b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
    b"<< /Length %d >>\nstream\n" % len(stream) + stream + b"\nendstream",
]
offsets = []
for index, obj in enumerate(objects, 1):
    offsets.append(len(pdf))
    pdf += b"%d 0 obj\n" % index + obj + b"\nendobj\n"
xref = len(pdf)
pdf += b"xref\n0 6\n0000000000 65535 f \n"
for offset in offsets:
    pdf += b"%010d 00000 n \n" % offset
pdf += b"trailer << /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n" % xref
(root / "digital.pdf").write_bytes(pdf)
pages[0].save(root / "scanned.pdf", resolution=150)
Image.new("L", (8, 8), "white").save(root / "too-many.tiff", save_all=True,
    append_images=[Image.new("L", (8, 8), "white") for _ in range(100)])
import pypdfium2 as pdfium
with pdfium.PdfDocument.new() as mixed, pdfium.PdfDocument(root / "digital.pdf") as digital, pdfium.PdfDocument(root / "scanned.pdf") as scanned:
    mixed.import_pages(digital)
    mixed.import_pages(scanned)
    mixed.save(root / "mixed.pdf")

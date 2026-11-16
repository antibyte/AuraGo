# Stirling PDF (`stirling_pdf`)

Process saved Desktop PDF files through the connected Stirling API. Inputs are
workspace-relative paths, never URLs or arbitrary host paths. Originals remain
unchanged; each result gets a unique `Shared/Store/stirling-pdf/` path.

| Operation | Arguments | Result |
|---|---|---|
| `ocr` | one `files` entry, `languages` (1–8 installed Tesseract names) | PDF with OCR for pages without existing text |
| `split` | one `files` entry, `pages` (`all` or inclusive split points/ranges) | ZIP of PDFs, or a PDF when upstream returns one |
| `merge` | 2–20 `files` entries in desired order | Merged PDF |
| `cert_sign` | one `files` entry | PDF signed using the administrator-selected server certificate |

```json
{"action":"stirling_pdf","operation":"ocr","files":["Shared/scan.pdf"],"languages":["deu","eng"]}
```

```json
{"action":"stirling_pdf","operation":"cert_sign","files":["Shared/contract.pdf"]}
```

Processing and signing are separate grants; signing defaults off. The server's
public certificate must match the saved SHA-256 fingerprint before and after
signing. Unexpected changes withhold the result. Private keys, keystores, passwords
and certificate administration are never accepted by this tool. A server signature
identifies that certificate; it does not imply a qualified electronic signature.

All operations require file publication permission. Total input and output are
bounded by the Desktop file cap, at most 64 MiB. Missing OCR language packages are
reported by name. Network failure after submission can mean an unknown outcome;
never replay automatically. Editing or merging signed PDFs can invalidate their
existing signatures; retained originals are the authoritative source copies.

Setup: save a Stirling API key, grant processing, and test capabilities. For signing,
an administrator configures the server certificate in Stirling and enters its
SHA-256 fingerprint in AuraGo. Certificate inspection requires an API key authorized
for Stirling's public-certificate endpoint. No certificate is generated automatically.

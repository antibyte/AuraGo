---
id: pdf
title: PDF Creation Workflow
enabled: true
priority: 90
tools: [document_creator]
workflows: [pdf, pdf_creation, document_creation]
keywords:
  - pdf
  - pdf erstellen
  - pdf erzeugen
  - pdf generation
  - create pdf
  - gotenberg
  - maroto
  - document creator
  - document_creator
---

This rule applies whenever creating, converting, merging, or delivering PDFs with Maroto, Gotenberg, or the `document_creator` tool.

## PDF Creation Workflow

Treat PDF generation as document design plus preflight, not as a raw export. Before rendering, decide document type, audience, goal, page size, orientation, hierarchy, tone, margins, typography, tables/figures, headers/footers, and verification steps. Default to a clean professional editorial style, A4 portrait, about 20 mm margins, readable 10.5-11.5 pt body text, restrained colors, and quiet headers/footers unless the user or data requires otherwise.

## Engine Choice

Use Gotenberg for HTML/CSS, rich typography, long-form editorial documents, browser-rendered charts/SVG, office conversion, screenshots, URL-to-PDF, merge/split/watermark-style workflows, or complex visual layout. Use Maroto for deterministic Go-native PDFs, invoices, receipts, simple reports, tabular output, backend batch generation, and environments that should avoid a browser/conversion service. If only one engine is configured, use it and adapt the design honestly.

## Source Quality

For Gotenberg, produce clean semantic HTML, print CSS with `@page`, local fonts/assets, explicit page breaks, repeated table headers, and no remote assets unless explicitly required. For Maroto, separate data normalization, theme constants, layout components, rendering, and verification; avoid scattered magic numbers and silent overflow. In both engines, use explicit page dimensions, margins, image sizes, font strategy, metadata, and reproducible inputs.

## Layout Quality Bar

No final PDF may contain placeholders, fake references, dummy dates, broken image boxes, clipped text, overlapping elements, unreadable contrast, missing glyphs, distorted images, accidental blank pages, orphan headings, headers/footers over body content, or tables that spill outside the page. Prefer structure, page breaks, landscape pages, or appendices over shrinking text until it is unreadable. Keep text selectable where appropriate and use screenshots of text only when the output is intentionally image-based.

## Tables, Figures, and Content

Tables must wrap long content, align numbers right and text left, repeat headers after page breaks, include units where needed, and keep totals/notes near their table. Wide tables should become landscape, summarized, or moved to an appendix. Charts need titles, units, labels, legends when needed, source notes for external data, sufficient contrast, and nearby explanation. Images/logos must preserve aspect ratio and be sharp enough for print.

## Security

Do not include secrets in PDF content, source, metadata, logs, filenames, or unused assets. Sanitize user-provided HTML/Markdown/SVG. Avoid PDF JavaScript and executable attachments unless explicitly required and reviewed. For Gotenberg, prefer local assets, restrict remote URLs when used, apply timeouts/size limits, and treat blocked assets or conversion warnings as quality failures. For Maroto, validate input data and prevent path traversal for fonts/images.

## Verification And Delivery

Always inspect the rendered PDF visually before claiming completion: at minimum first page, last page, table-heavy pages, image/chart pages, pages after forced breaks, and pages with the longest expected values. Check page size/orientation, margins, page numbers, headers/footers, links, selectable text, metadata, fonts, contrast, file size, and grayscale print readability when relevant. If defects exist, fix the source and regenerate. If verification is impossible, say so plainly.

When delivering, report briefly: engine used, page count, format, key design choices, verification performed, assumptions, warnings, and a clickable path/link to the final PDF. A PDF below business-ready quality is a draft; improve it unless the user explicitly accepts rough output.

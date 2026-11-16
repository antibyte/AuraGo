# KiCad export worker

## Purpose

Versioned, isolated exports for saved Ziro Shared projects.

## Ownership

`Dockerfile` and `export.py` own the KiCad 10.0.8 protocol-1 image.
`../../../licences/KICAD_EXPORT.md` records image provenance and licensing.
Desktop owns snapshot/isolation/publication; server owns Store grants and Docker
mount translation. `../../../scripts/test-kicad-export.py` owns unit/live smokes.

## Local Contracts

- Fixed `netlist`, `manufacturing` and read-only `probe`; no arbitrary commands.
- Official KiCad release PPA, verified signing fingerprint and architecture-specific
  package SHA-256. Keep Ubuntu digest, KiCad version, runtime constant and smoke
  expectations aligned. Installed symbols/footprints use the same release PPA.
- Read `/job/request.json` and `/job/source`; write only job artifacts. Netlists
  use `kicadsexpr`; manufacturing ZIPs contain Gerber-X2, Excellon and a hash manifest.
- Reject missing or escaping relative libraries/sheets. No network, host project
  mounts, private credentials or unsaved editor state.
- Publish `kicad-10.0.8-1` only after both amd64/arm64 smokes pass, then record the
  verified resulting image digest in `desktop.ZiroExportImage`. Never invent a digest.

## Work Guidance

Build with this directory as context. Publication is a separate release action.

## Verification

`python scripts/test-kicad-export.py`; add `--image <image> --platform linux/<arch>`
for real isolated CLI checks. Both architectures gate the image publication job.

## Child DOX Index

None.

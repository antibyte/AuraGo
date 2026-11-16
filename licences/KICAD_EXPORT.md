# KiCad export image

The optional `aurago-kicad-export:kicad-10.0.8-1` image runs KiCad 10.0.8 as
a separate process; it is not linked into the AuraGo binary. Build inputs and
package checksums are in `deploy/docker/kicad-export/Dockerfile`.

- KiCad source/license: [10.0.8 source](https://gitlab.com/kicad/code/kicad/-/tree/10.0.8),
  [GPL version 3 license](https://raw.githubusercontent.com/KiCad/kicad-source-mirror/10.0.8/LICENSE).
- Official packages: [KiCad stable PPA](https://launchpad.net/~kicad/+archive/ubuntu/kicad-10.0-releases).
  Preserve installed `/usr/share/doc/*/copyright` and the package source records
  when distributing the image; publish corresponding source alongside the release.
- Symbols and footprints: KiCad community libraries, CC-BY-SA-4.0 with the
  [upstream electronic-design exception](https://github.com/KiCad/kicad-symbols/blob/master/LICENSE.md).
  Preserve library attribution and license documents in the image. The exception
  covers generated electronic designs, not redistribution of the library collection.
- Ubuntu and dependencies retain their package-specific licenses and copyright
  files. The image build does not remove `/usr/share/doc`.
- AuraGo's worker and smoke scripts retain the repository's MIT license.

The version tag is reserved for the reviewed build. Its final digest and complete
redistributed package inventory must be recorded after both architecture smokes,
before image publication; local protocol tests do not certify the built image.

# AuraGo patches to go-smb2

Upstream: `github.com/hirochachacha/go-smb2/v2` at
`v2.0.0-20260929042113-82213d0ccb22` (BSD-2-Clause, see `LICENSE`).
This directory is a copy of the module without `*_test.go` files and testdata,
used through `replace github.com/hirochachacha/go-smb2/v2 => ./third_party/go-smb2`
in the root `go.mod`.

## Change

`x/wire/fscc.go`, `FileIdBothDirectoryInformationDecoder.IsInvalid`: the check
`if shortLen > 0 && IsInvalidShortName(c[70:70+shortLen])` is removed. The length
bounds (`shortLen > 24 || shortLen&1 != 0` is invalid) stay.

## Why

Samba fills the 8.3 ShortName of the "." entry in FILE_ID_BOTH_DIR_INFORMATION
with bytes the library treats as invalid (ShortNameLength 24, mostly zero bytes
plus 0x80). The whole QUERY_DIRECTORY reply was rejected ("broken query directory
response format"), so every directory listing against Samba failed. AuraGo never
uses short names, which are optional legacy data.

## Removal

Drop this copy, the `replace` line, the Dockerfile `COPY third_party/go-smb2`
line and this note once upstream accepts such short names.

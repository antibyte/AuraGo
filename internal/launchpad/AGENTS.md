# Launchpad links and icons

## Purpose

Persist bookmark tiles and obtain their artwork without granting access to other application data.

## Ownership

This package owns link validation, SQLite storage and icon download/deletion. Server owns HTTP authentication; Desktop and Store callers retain their own policy.

## Local Contracts

- Create and update validate icon references. Empty references and HTTP(S) artwork URLs remain supported; remote URLs are never interpreted as local cleanup paths.
- Local icon references contain exactly the launchpad_icons directory and one supported image filename. Accept stored Windows separators for portability; reject traversal, nested paths, alternate streams and partial extension matches.
- Delete only regular files through an opened icon root beneath the configured data root. Reject symlinked icon directories and files; never delete other data files or prefix-sharing sibling directories.
- Downloaded and catalog artwork share `storeIcon` and `openIconRoot` with deletion.
  Create the icon directory through the data root, reject symlinked directories,
  and create unique files exclusively through the icon root. Never overwrite or
  follow an existing icon filename; failed writes remove only their own new file.
- Downloads retain public-target/redirect validation, size/content-type limits and cancellation. Catalog artwork and downloaded artwork share the exact extension allowlist.

## Work Guidance

- Enforce ownership in the shared package even when the HTTP caller is authenticated. Use rooted operations and preserve remote artwork references.

## Verification

- Run go test ./internal/launchpad and the server Launchpad tests. Keep icon path, symlink and partial-extension regressions.

## Child DOX Index

None.

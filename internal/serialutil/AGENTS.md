# Serial port leases

## Purpose

Shared native serial enumeration, device reservations, and open-port lifetime for Desktop Quick Connect and MeshCore.

## Ownership

`internal/serialutil` owns process-local port leases and cancellation-safe serial opening. Desktop and MeshCore keep their own source policy and serial settings.

## Local Contracts

- Quick Connect may open only an exact name returned by the native enumerator. MeshCore may open its trusted configured path, including aliases such as `/dev/serial/by-id`; canonicalize reservation keys so aliases cannot bypass the shared lease.
- Reserve before opening. If open finishes after cancellation, close that device before releasing its reservation. Keep the reservation through `Port.Close`.
- Serialize DTR, RTS, and timed Break calls with close. Do not lock ordinary reads or writes; close must be able to interrupt blocked I/O. A timed Break must finish and clear its signal before closing the underlying handle.
- Keep this package CGO-free and never log serial payload bytes.

## Verification

- `go test ./internal/serialutil`
- `go test ./internal/desktop -run '^TestSerialProxy'`
- `go test ./internal/meshcore -run '^TestUSBTransportKeepsConfiguredAliasAndCompanionSignals$'`

## Child DOX Index

None.

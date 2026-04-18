# Testing Notes

## Windows `unlinkat ... test.exe` cleanup locks

On Windows, `go test` can report a successful package run and still exit non-zero while removing the temporary `*.test.exe` binary:

```text
go: unlinkat ...\\go-build...\\package.test.exe: The process cannot access the file because it is being used by another process.
```

This is a host cleanup issue, not a failing package test. Treat it separately from real compilation or assertion failures:

- Check the package output first. If the package already printed `ok`, the tests themselves passed.
- Re-run the affected package once the file lock is released.
- Exclude antivirus or indexers from the Go build cache/temp directories if the lock happens repeatedly.

For AuraGo validation, prefer reading package-level `ok`/`FAIL` lines before interpreting a trailing Windows cleanup error as a regression.

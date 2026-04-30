# Quick Task Summary: Fix image gallery monthly count timezone

## Outcome

Fixed `TestImageGalleryMonthlyCount` and the underlying month-boundary bug in `ImageGalleryMonthlyCount`.

## Root Cause

SQLite `CURRENT_TIMESTAMP` stores `generated_images.created_at` in UTC. The monthly count used the local month prefix as a string lower bound, so records created shortly after local midnight on the first day of a month could still have the previous UTC date and were excluded.

## Changes

- `ImageGalleryMonthlyCount` now computes local calendar-month bounds and converts those bounds to UTC timestamp strings before querying SQLite.
- The monthly count query now uses both lower and upper bounds.
- The existing test now fails fast if `SaveGeneratedImage` returns an error.
- Added regression coverage for the local May 1 / UTC April 30 boundary.

## Verification

- `rtk go test ./internal/tools -run "ImageGalleryMonthlyCount|SaveAndGetGeneratedImage|ListGeneratedImages" -count=1` passed.
- `rtk go test ./internal/tools -run TestImageGalleryMonthlyCount -count=1 -v` passed.
- `rtk go vet ./internal/tools` passed.
- `rtk go test ./internal/tools -count=1` passed: 1215 tests.
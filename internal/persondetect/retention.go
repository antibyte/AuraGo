package persondetect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// staleFrameTemporaryAge is how old a .frame-*.tmp file must be before retention
// removes it. A frame write takes well under a second, so a temporary of this age
// was left by a crash between CreateTemp and the rename in writeImageOnce.
const staleFrameTemporaryAge = time.Hour

// linkLike are the types of a directory entry that may lead out of the image
// tree: symbolic links and, on Windows, junctions and other reparse points.
const linkLike = os.ModeSymlink | os.ModeIrregular

// RetentionResult reports one retention run.
type RetentionResult struct {
	DeletedEvents    int
	FreedBytes       int64
	TotalBytes       int64
	ArmLogDeleted    int64
	StaleTemporaries int // .frame-*.tmp files older than staleFrameTemporaryAge that were removed
}

// applyRetention deletes ended events older than retentionDays, then the oldest
// ended events until the stored bytes are at most StorageTargetRatio of limitBytes,
// then arm-log rows older than the retention window (never younger than
// ArmLogMinAge), and finally the stale temporary files of frame writes. Open
// events are never deleted.
//
// The event steps stop at their first error. The sweep of temporaries is the last
// step; its error (a temporary that could not be removed, or the end of ctx) comes
// back together with the fully filled result, so it never hides the deletions.
func applyRetention(ctx context.Context, st *EventStore, now time.Time, retentionDays int, limitBytes int64) (RetentionResult, error) {
	var res RetentionResult
	if retentionDays < 1 {
		retentionDays = DefaultRetentionDays
	}
	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour)
	for {
		ids, err := st.expiredEventIDs(ctx, cutoff, 200)
		if err != nil {
			return res, err
		}
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			if err := removeRetainedEvent(ctx, st, id, &res); err != nil {
				return res, err
			}
		}
	}
	total, err := st.TotalBytes(ctx)
	if err != nil {
		return res, err
	}
	if limitBytes > 0 && total > limitBytes {
		target := int64(float64(limitBytes) * StorageTargetRatio)
		for total > target {
			ids, err := st.oldestEndedEventIDs(ctx, 50)
			if err != nil {
				return res, err
			}
			if len(ids) == 0 {
				break
			}
			for _, id := range ids {
				if total <= target {
					break
				}
				before := res.FreedBytes
				if err := removeRetainedEvent(ctx, st, id, &res); err != nil {
					return res, err
				}
				total -= res.FreedBytes - before
			}
		}
	}
	res.TotalBytes = total
	armCutoff := cutoff
	if minAge := now.Add(-ArmLogMinAge); armCutoff.After(minAge) {
		armCutoff = minAge
	}
	deleted, err := st.pruneArmLog(ctx, armCutoff)
	if err != nil {
		return res, err
	}
	res.ArmLogDeleted = deleted
	res.StaleTemporaries, err = st.removeStaleFrameTemporaries(ctx, now)
	return res, err
}

func removeRetainedEvent(ctx context.Context, st *EventStore, id int64, res *RetentionResult) error {
	freed, err := st.RemoveEvent(ctx, id)
	if errors.Is(err, ErrUnknownEvent) {
		return nil
	}
	if err != nil {
		return err
	}
	res.DeletedEvents++
	res.FreedBytes += freed
	return nil
}

// removeStaleFrameTemporaries removes the .frame-*.tmp files below the image root
// that were last modified more than staleFrameTemporaryAge before now, and
// returns how many it removed. They are what a crash between CreateTemp and the
// rename of writeImageOnce leaves behind, and they keep removeFile from deleting
// the directories of the frames around them. The files go through removeFile, so
// the date and camera directories they leave empty are pruned the same way as
// after the deletion of an event.
//
// The walk reads directories (os.ReadDir) from the root down instead of using
// filepath.WalkDir(root): WalkDir looks at the root with Lstat and does not
// descend when it is a Windows junction, a supported way to move the images to
// another disk. Below the root it never enters and never deletes an entry that is
// a symbolic link, a junction or another reparse point, nor anything but a real
// directory or regular file (checkElement), so it cannot leave the tree. A
// directory that cannot be read, such as lost+found on a mounted root, is skipped
// without a word. A temporary that is still there after its removal is counted
// into the returned error; ctx ends the walk.
func (st *EventStore) removeStaleFrameTemporaries(ctx context.Context, now time.Time) (int, error) {
	root := filepath.Clean(st.root)
	cutoff := now.Add(-staleFrameTemporaryAge)
	removed, failed := 0, 0
	var firstFailure string
	var sweep func(dir string) error
	sweep = func(dir string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		for _, entry := range entries {
			if entry.Type()&linkLike != 0 {
				continue
			}
			p := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				if info, err := os.Lstat(p); err == nil && info.Mode()&linkLike == 0 && checkElement(p, info, true) == nil {
					if err := sweep(p); err != nil {
						return err
					}
				}
				continue
			}
			if match, _ := filepath.Match(frameTempGlob, entry.Name()); !match {
				continue
			}
			info, err := os.Lstat(p)
			if err != nil || info.Mode()&linkLike != 0 || checkElement(p, info, false) != nil || !info.ModTime().Before(cutoff) {
				continue
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				continue
			}
			st.removeFile(filepath.ToSlash(rel))
			if _, err := os.Lstat(p); errors.Is(err, os.ErrNotExist) {
				removed++
				continue
			}
			if failed++; failed == 1 {
				firstFailure = p
			}
		}
		return nil
	}
	if err := sweep(root); err != nil {
		return removed, err
	}
	if failed > 0 {
		return removed, fmt.Errorf("persondetect: %d stale frame temporaries could not be removed, the first is %s", failed, firstFailure)
	}
	return removed, nil
}

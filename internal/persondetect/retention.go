package persondetect

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// staleFrameTemporaryAge is how old a .frame-*.tmp file must be before retention
// removes it. A frame write takes well under a second, so a temporary of this age
// was left by a crash between CreateTemp and the rename in writeImageOnce.
const staleFrameTemporaryAge = time.Hour

// Steps reported to retentionProgress.
const (
	retentionEventRemoved   = "event removed"   // after each event applyRetention deleted (detail: the id)
	retentionBeforeTotal    = "before total"    // before the final read of the stored bytes
	retentionSweepDirectory = "sweep directory" // when the sweep of stale temporaries reaches a directory (detail: its path)
)

// retentionProgress is called at the steps above; tests replace it to cut a pass
// short at a chosen point or to change the store while it runs.
var retentionProgress = func(step, detail string) {}

// RetentionResult reports one retention run.
type RetentionResult struct {
	DeletedEvents int
	FreedBytes    int64
	TotalBytes    int64 // the stored bytes after the event steps, read from the store once they are done
	ArmLogDeleted int64

	// The sweep of stale temporary files is best effort and reported here, never as
	// the error of the pass: a file that cannot be deleted (antivirus, wrong owner,
	// a read-only remount) would otherwise hide the results above from a caller
	// that applies them only when the pass succeeded.
	StaleTemporaries       int   // .frame-*.tmp files older than staleFrameTemporaryAge that were removed
	StaleTemporaryFailures int   // stale temporaries that could not be removed
	StaleTemporaryError    error // the first of those failures with its cause (wraps the *fs.PathError); nil when there was none
}

// applyRetention deletes ended events older than retentionDays, then the oldest
// ended events until the stored bytes are at most StorageTargetRatio of limitBytes,
// then arm-log rows older than the retention window (never younger than
// ArmLogMinAge), and finally the stale temporary files of frame writes. Open
// events are never deleted.
//
// The event and arm-log steps stop at their first error, and so does ctx: both
// come back as the error together with the partial result. A stale temporary that
// cannot be removed does not: the sweep reports it in the result
// (StaleTemporaryFailures, StaleTemporaryError) and the pass succeeds. Only the
// end of ctx during the sweep is returned, with the event results filled.
//
// now drives the retention windows of the events. The age of a temporary file is
// compared with the wall clock instead, because file times come from the file
// system's clock: a service clock that is ahead of it (after a forward step, or
// in a test harness) would otherwise make the temporaries of writes that are
// running right now look stale and delete them.
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
	// The total above was computed by subtraction; frames added to open events
	// and events deleted by others while the pass ran are only in the store.
	res.TotalBytes = total
	retentionProgress(retentionBeforeTotal, "")
	stored, err := st.TotalBytes(ctx)
	if err != nil {
		return res, err
	}
	res.TotalBytes = stored
	armCutoff := cutoff
	if minAge := now.Add(-ArmLogMinAge); armCutoff.After(minAge) {
		armCutoff = minAge
	}
	deleted, err := st.pruneArmLog(ctx, armCutoff)
	if err != nil {
		return res, err
	}
	res.ArmLogDeleted = deleted
	sweep, err := st.removeStaleFrameTemporaries(ctx, time.Now())
	res.StaleTemporaries, res.StaleTemporaryFailures, res.StaleTemporaryError = sweep.Removed, sweep.Failures, sweep.First
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
	retentionProgress(retentionEventRemoved, fmt.Sprint(id))
	return nil
}

// isLinkLike reports whether a file mode is a link that could lead out of the
// image tree: a symbolic link, or a Windows junction or other reparse point,
// which Go reports as ModeIrregular without ModeDir. A reparse-point directory
// that is no link (OneDrive placeholder, tiering store) keeps ModeDir together
// with ModeIrregular and is not link-like, in step with checkElement.
func isLinkLike(mode fs.FileMode) bool {
	return mode&fs.ModeSymlink != 0 || (mode&fs.ModeIrregular != 0 && mode&fs.ModeDir == 0)
}

// sweepDescendsInto reports whether the sweep may enter the directory p (Lstat
// result info): a real directory, no link.
func sweepDescendsInto(p string, info fs.FileInfo) bool {
	return !isLinkLike(info.Mode()) && checkElement(p, info, true) == nil
}

// sweepRemovableType reports whether p (Lstat result info) is the kind of entry
// the sweep may delete: a regular file, no link or reparse point.
func sweepRemovableType(p string, info fs.FileInfo) bool {
	return !isLinkLike(info.Mode()) && checkElement(p, info, false) == nil
}

// staleSweep is the outcome of removeStaleFrameTemporaries.
type staleSweep struct {
	Removed  int   // temporaries removed
	Failures int   // temporaries that could not be removed
	First    error // the first of those failures, with its cause
}

// removeStaleFrameTemporaries removes the .frame-*.tmp files below the image root
// that were last modified more than staleFrameTemporaryAge before now. They are
// what a crash between CreateTemp and the rename of writeImageOnce leaves behind,
// and they keep removeFile from deleting the directories of the frames around
// them. The files go through removeFileErr, so the date and camera directories
// they leave empty are pruned the same way as after the deletion of an event.
// applyRetention passes the wall clock as now; tests pass their own.
//
// The walk reads directories (os.ReadDir) from the root down instead of using
// filepath.WalkDir(root): WalkDir looks at the root with Lstat and does not
// descend when it is a Windows junction, a supported way to move the images to
// another disk. Below the root it never enters and never deletes a symbolic
// link, junction or other reparse point (isLinkLike), and only enters real
// directories and deletes regular files (checkElement), so it cannot leave the
// tree. A directory that cannot be read, such as lost+found on a mounted root,
// is skipped without a word.
//
// A temporary that cannot be removed is counted and its first error, wrapped
// with the cause (EACCES, EROFS, a sharing violation), is kept in the result; the
// sweep goes on. The returned error is only the end of ctx.
func (st *EventStore) removeStaleFrameTemporaries(ctx context.Context, now time.Time) (staleSweep, error) {
	var out staleSweep
	root := filepath.Clean(st.root)
	cutoff := now.Add(-staleFrameTemporaryAge)
	var sweep func(dir string) error
	sweep = func(dir string) error {
		retentionProgress(retentionSweepDirectory, dir)
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		for _, entry := range entries {
			if isLinkLike(entry.Type()) {
				continue
			}
			p := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				if info, err := os.Lstat(p); err == nil && sweepDescendsInto(p, info) {
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
			if err != nil || !sweepRemovableType(p, info) || !info.ModTime().Before(cutoff) {
				continue
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				continue
			}
			if err := st.removeFileErr(filepath.ToSlash(rel)); err != nil {
				if out.Failures++; out.Failures == 1 {
					out.First = fmt.Errorf("persondetect: remove stale frame temporary: %w", err)
				}
				continue
			}
			out.Removed++
		}
		return nil
	}
	err := sweep(root)
	return out, err
}

package persondetect

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
)

// Events returns one page of events, newest first. A switched-off service without
// a database returns an empty page; an enabled one whose database is not open
// returns ErrUnavailable.
func (s *Service) Events(ctx context.Context, q EventQuery) (EventPage, error) {
	st, err := s.apiStore()
	if errors.Is(err, ErrDisabled) {
		return EventPage{Events: []EventSummary{}}, nil
	}
	if err != nil {
		return EventPage{}, err
	}
	return st.Events(ctx, q)
}

// Event returns one event with its frames.
func (s *Service) Event(ctx context.Context, id int64) (EventDetail, error) {
	st, err := s.apiStore()
	if errors.Is(err, ErrDisabled) {
		return EventDetail{}, fmt.Errorf("%w: %d", ErrUnknownEvent, id)
	}
	if err != nil {
		return EventDetail{}, err
	}
	return st.Event(ctx, id)
}

// FrameFile returns the absolute path of a stored frame or its thumbnail. The path
// is looked up by id and must resolve to a regular file inside the detections root.
func (s *Service) FrameFile(ctx context.Context, frameID int64, thumb bool) (string, error) {
	root, rel, err := s.FrameLocation(ctx, frameID, thumb)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, rel), nil
}

// FrameLocation is FrameFile split into the detections root and the frame's path
// below it (native separators), checked the same way: a regular file with no link
// below the root. The root itself may be a link or a mount point on another disk, so
// a reader that opens frames through os.Root opens it at root, not at the data
// directory.
func (s *Service) FrameLocation(ctx context.Context, frameID int64, thumb bool) (root, rel string, err error) {
	st, err := s.apiStore()
	if errors.Is(err, ErrDisabled) {
		return "", "", fmt.Errorf("%w: frame %d", ErrUnknownEvent, frameID)
	}
	if err != nil {
		return "", "", err
	}
	frameRel, thumbRel, err := st.Frame(ctx, frameID)
	if err != nil {
		return "", "", err
	}
	if thumb {
		frameRel = thumbRel
	}
	root = st.Root()
	if _, err := resolveInside(root, frameRel); err != nil {
		return "", "", fmt.Errorf("%w: frame %d: %v", ErrUnknownEvent, frameID, err)
	}
	return root, filepath.FromSlash(frameRel), nil
}

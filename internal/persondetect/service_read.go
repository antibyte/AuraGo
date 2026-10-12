package persondetect

import (
	"context"
	"errors"
	"fmt"
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
	st, err := s.apiStore()
	if errors.Is(err, ErrDisabled) {
		return "", fmt.Errorf("%w: frame %d", ErrUnknownEvent, frameID)
	}
	if err != nil {
		return "", err
	}
	rel, thumbRel, err := st.Frame(ctx, frameID)
	if err != nil {
		return "", err
	}
	if thumb {
		rel = thumbRel
	}
	path, err := resolveInside(st.Root(), rel)
	if err != nil {
		return "", fmt.Errorf("%w: frame %d: %v", ErrUnknownEvent, frameID, err)
	}
	return path, nil
}

package jellyfin

import (
	"context"
	"fmt"
	"net/url"
)

// GetSessions returns all active sessions.
func (c *Client) GetSessions(ctx context.Context) ([]Session, error) {
	var sessions []Session
	if err := c.Get(ctx, "/Sessions", &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// SendPlayCommand sends a playback command to a specific session.
// Valid commands: Play, Pause, Unpause, Stop, NextTrack, PreviousTrack
func (c *Client) SendPlayCommand(ctx context.Context, sessionID string, command string) error {
	endpoint := fmt.Sprintf("/Sessions/%s/Playing/%s",
		url.PathEscape(sessionID),
		url.PathEscape(command))
	return c.Post(ctx, endpoint, nil, nil)
}

// SendSeekCommand seeks to a specific position (in ticks) in a session.
func (c *Client) SendSeekCommand(ctx context.Context, sessionID string, positionTicks int64) error {
	endpoint := fmt.Sprintf("/Sessions/%s/Playing/Seek?SeekPositionTicks=%d",
		url.PathEscape(sessionID), positionTicks)
	return c.Post(ctx, endpoint, nil, nil)
}

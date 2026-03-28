package jellyfin

import "context"

// GetSystemInfo returns the Jellyfin server system information.
func (c *Client) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	var info SystemInfo
	if err := c.Get(ctx, "/System/Info", &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// Ping checks if the Jellyfin server is reachable.
func (c *Client) Ping(ctx context.Context) error {
	return c.Get(ctx, "/System/Ping", nil)
}

// GetActivityLog returns the most recent activity log entries.
func (c *Client) GetActivityLog(ctx context.Context, limit int) (*ActivityLogResponse, error) {
	if limit <= 0 {
		limit = 25
	}
	var resp ActivityLogResponse
	endpoint := "/System/ActivityLog/Entries?StartIndex=0&Limit=" + itoa(limit)
	if err := c.Get(ctx, endpoint, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func itoa(i int) string {
	if i < 0 {
		return "-" + uitoa(uint(-i))
	}
	return uitoa(uint(i))
}

func uitoa(u uint) string {
	if u == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for u > 0 {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	return string(buf[i:])
}

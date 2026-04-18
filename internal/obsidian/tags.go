package obsidian

import "context"

// ListTags returns all tags used in the vault with their usage counts.
func (c *Client) ListTags(ctx context.Context) ([]Tag, error) {
	var tags []Tag
	if err := c.getJSON(ctx, "/tags/", &tags); err != nil {
		return nil, err
	}
	return tags, nil
}

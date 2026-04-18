package obsidian

import "context"

// ListCommands returns all available Obsidian commands.
func (c *Client) ListCommands(ctx context.Context) ([]Command, error) {
	var commands []Command
	if err := c.getJSON(ctx, "/commands/", &commands); err != nil {
		return nil, err
	}
	return commands, nil
}

// ExecuteCommand executes an Obsidian command by ID.
func (c *Client) ExecuteCommand(ctx context.Context, commandID string) error {
	return c.postJSON(ctx, "/commands/"+encodePath(commandID)+"/", nil, nil)
}

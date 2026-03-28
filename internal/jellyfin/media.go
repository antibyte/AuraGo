package jellyfin

import (
	"context"
	"fmt"
	"net/url"
)

// GetLibraries returns all media libraries (virtual folders).
func (c *Client) GetLibraries(ctx context.Context) ([]Library, error) {
	var libs []Library
	if err := c.Get(ctx, "/Library/VirtualFolders", &libs); err != nil {
		return nil, err
	}
	return libs, nil
}

// RefreshLibrary triggers a library scan/refresh for the given library item ID.
func (c *Client) RefreshLibrary(ctx context.Context, itemID string) error {
	endpoint := fmt.Sprintf("/Items/%s/Refresh", url.PathEscape(itemID))
	return c.Post(ctx, endpoint, map[string]interface{}{
		"Recursive":        true,
		"ReplaceAllImages": false,
	}, nil)
}

// GetItems returns items from the library, optionally filtered.
func (c *Client) GetItems(ctx context.Context, parentID string, includeTypes string, limit int, startIndex int) (*ItemsResponse, error) {
	params := url.Values{}
	params.Set("Recursive", "true")
	params.Set("Fields", "Overview,Genres,Studios,CommunityRating,OfficialRating,ProductionYear,RunTimeTicks,Path,DateCreated,ChildCount")
	if parentID != "" {
		params.Set("ParentId", parentID)
	}
	if includeTypes != "" {
		params.Set("IncludeItemTypes", includeTypes)
	}
	if limit > 0 {
		params.Set("Limit", itoa(limit))
	}
	if startIndex > 0 {
		params.Set("StartIndex", itoa(startIndex))
	}
	params.Set("SortBy", "SortName")
	params.Set("SortOrder", "Ascending")

	var resp ItemsResponse
	if err := c.Get(ctx, "/Items?"+params.Encode(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SearchItems searches for items matching a query string.
func (c *Client) SearchItems(ctx context.Context, query string, includeTypes string, limit int) (*ItemsResponse, error) {
	params := url.Values{}
	params.Set("SearchTerm", query)
	params.Set("Recursive", "true")
	params.Set("Fields", "Overview,Genres,CommunityRating,ProductionYear,RunTimeTicks,Path")
	if includeTypes != "" {
		params.Set("IncludeItemTypes", includeTypes)
	}
	if limit > 0 {
		params.Set("Limit", itoa(limit))
	}

	var resp ItemsResponse
	if err := c.Get(ctx, "/Items?"+params.Encode(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetItem returns detailed information about a specific item.
func (c *Client) GetItem(ctx context.Context, itemID string) (*MediaItem, error) {
	var item MediaItem
	endpoint := fmt.Sprintf("/Items/%s", url.PathEscape(itemID))
	if err := c.Get(ctx, endpoint, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem permanently removes an item from the library.
func (c *Client) DeleteItem(ctx context.Context, itemID string) error {
	endpoint := fmt.Sprintf("/Items/%s", url.PathEscape(itemID))
	return c.Delete(ctx, endpoint)
}

// GetRecentItems returns recently added items.
func (c *Client) GetRecentItems(ctx context.Context, includeTypes string, limit int) (*ItemsResponse, error) {
	params := url.Values{}
	params.Set("Recursive", "true")
	params.Set("SortBy", "DateCreated")
	params.Set("SortOrder", "Descending")
	params.Set("Fields", "Overview,Genres,CommunityRating,ProductionYear,RunTimeTicks,Path,DateCreated")
	if includeTypes != "" {
		params.Set("IncludeItemTypes", includeTypes)
	}
	if limit > 0 {
		params.Set("Limit", itoa(limit))
	} else {
		params.Set("Limit", "20")
	}

	var resp ItemsResponse
	if err := c.Get(ctx, "/Items?"+params.Encode(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

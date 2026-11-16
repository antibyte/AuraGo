package desktop

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

var defaultAppExtensionPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_+.-]{0,31}$`)

func parseDefaultAppAssociations(value string) (map[string]string, error) {
	var associations map[string]string
	if len(value) > 65536 || json.Unmarshal([]byte(value), &associations) != nil || associations == nil || len(associations) > 128 {
		return nil, fmt.Errorf("invalid default application associations")
	}
	for extension, appID := range associations {
		if !defaultAppExtensionPattern.MatchString(extension) || len(appID) == 0 || len(appID) > 128 {
			return nil, fmt.Errorf("invalid default application association")
		}
	}
	return associations, nil
}

func (s *Service) validateDefaultAppSetting(ctx context.Context, values map[string]string) error {
	value, changed := values["files.default_apps"]
	if !changed {
		return nil
	}
	associations, err := parseDefaultAppAssociations(value)
	if err != nil {
		return err
	}
	var previousValue string
	err = s.getDB().QueryRowContext(ctx, `SELECT value FROM desktop_settings WHERE key = 'files.default_apps'`).Scan(&previousValue)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read default applications: %w", err)
	}
	previous, _ := parseDefaultAppAssociations(previousValue)
	for extension, appID := range associations {
		// Removing an app must not silently erase the user's existing association.
		if previous[extension] == appID {
			continue
		}
		_, exists, err := s.findApp(ctx, appID)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("default application is unavailable for %s", extension)
		}
	}
	return nil
}

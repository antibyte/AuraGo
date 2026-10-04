package config

import "encoding/json"

// UnmarshalJSON accepts the historical integration spelling while writers keep
// the canonical expiry field shared by all OAuth consumers.
func (t *OAuthToken) UnmarshalJSON(data []byte) error {
	type token OAuthToken
	var decoded struct {
		token
		LegacyExpiry string `json:"token_expiry"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*t = OAuthToken(decoded.token)
	if t.Expiry == "" {
		t.Expiry = decoded.LegacyExpiry
	}
	return nil
}

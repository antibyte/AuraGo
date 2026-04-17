package agent

type ldapArgs struct {
	Operation       string
	BaseDN          string
	Filter          string
	Username        string
	GroupName       string
	UserDN          string
	DN              string
	Password        string
	Attributes      []string
	EntryAttributes map[string][]string
	Changes         map[string][]string
}

func toolArgStringSliceMap(args map[string]interface{}, keys ...string) map[string][]string {
	for _, key := range keys {
		raw, ok := toolArgRaw(args, key)
		if !ok {
			continue
		}
		switch values := raw.(type) {
		case map[string][]string:
			result := make(map[string][]string, len(values))
			for attr, items := range values {
				result[attr] = append([]string(nil), items...)
			}
			return result
		case map[string]interface{}:
			result := make(map[string][]string, len(values))
			for attr, value := range values {
				result[attr] = toolArgStringsFromRaw(value)
			}
			if len(result) > 0 {
				return result
			}
		}
	}
	return nil
}

func decodeLDAPArgs(tc ToolCall) ldapArgs {
	return ldapArgs{
		Operation:       firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		BaseDN:          toolArgString(tc.Params, "base_dn"),
		Filter:          toolArgString(tc.Params, "filter"),
		Username:        firstNonEmptyToolString(tc.Username, toolArgString(tc.Params, "username")),
		GroupName:       toolArgString(tc.Params, "group_name"),
		UserDN:          toolArgString(tc.Params, "user_dn"),
		DN:              toolArgString(tc.Params, "dn"),
		Password:        firstNonEmptyToolString(tc.Password, toolArgString(tc.Params, "password")),
		Attributes:      toolArgStringSlice(tc.Params, "attributes"),
		EntryAttributes: toolArgStringSliceMap(tc.Params, "entry_attributes"),
		Changes:         toolArgStringSliceMap(tc.Params, "changes"),
	}
}

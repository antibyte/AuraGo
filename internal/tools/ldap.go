package tools

import (
	"aurago/internal/config"
	"aurago/internal/ldap"
	"aurago/internal/security"
	"encoding/json"
	"fmt"
	"log/slog"
)

type ldapClient interface {
	Connect() error
	ConnectAndBind() error
	Close()
	Search(baseDN, filter string, attributes []string) (*ldap.SearchResult, error)
	GetUser(username string) (*ldap.SearchEntry, error)
	ListUsers() (*ldap.SearchResult, error)
	GetGroup(groupName string) (*ldap.SearchEntry, error)
	ListGroups() (*ldap.SearchResult, error)
	Authenticate(userDN, password string) (bool, error)
	AddEntry(dn string, attributes map[string][]string) error
	ModifyEntry(dn string, changes map[string][]string) error
	DeleteEntry(dn string) error
	TestConnection() error
}

var newLDAPClient = func(cfg ldap.LDAPConfig) ldapClient {
	return ldap.NewClient(cfg)
}

func LDAP(cfg *config.Config, vault *security.Vault, operation string, args map[string]interface{}, logger *slog.Logger) string {
	clientCfg := ldap.LDAPConfig{
		Enabled:            cfg.LDAP.Enabled,
		ReadOnly:           cfg.LDAP.ReadOnly,
		Host:               cfg.LDAP.Host,
		Port:               cfg.LDAP.Port,
		UseTLS:             cfg.LDAP.UseTLS,
		InsecureSkipVerify: cfg.LDAP.InsecureSkipVerify,
		BaseDN:             cfg.LDAP.BaseDN,
		BindDN:             cfg.LDAP.BindDN,
		BindPassword:       cfg.LDAP.BindPassword,
		UserSearchBase:     cfg.LDAP.UserSearchBase,
		GroupSearchBase:    cfg.LDAP.GroupSearchBase,
		ConnectTimeout:     cfg.LDAP.ConnectTimeout,
		RequestTimeout:     cfg.LDAP.RequestTimeout,
	}

	if clientCfg.BindPassword == "" && vault != nil {
		if pwd, err := vault.ReadSecret("ldap_bind_password"); err == nil && pwd != "" {
			clientCfg.BindPassword = pwd
		}
	}

	security.RegisterSensitive(clientCfg.BindPassword)

	searchAttributes := stringSliceArg(args["attributes"])
	entryAttributes := stringSliceMapArg(args["entry_attributes"])
	changes := stringSliceMapArg(args["changes"])
	baseDN, _ := args["base_dn"].(string)
	filter, _ := args["filter"].(string)
	username, _ := args["username"].(string)
	groupName, _ := args["group_name"].(string)
	userDN, _ := args["user_dn"].(string)
	password, _ := args["password"].(string)
	dn, _ := args["dn"].(string)

	if userDN == "" {
		userDN = dn
	}

	if clientCfg.Host == "" {
		return ldapError("LDAP host is not configured.")
	}
	if clientCfg.BindDN == "" {
		return ldapError("LDAP bind_dn is not configured.")
	}

	switch operation {
	case "search":
		if clientCfg.BaseDN == "" && baseDN == "" {
			return ldapError("LDAP base_dn is not configured.")
		}
		if baseDN == "" {
			baseDN = clientCfg.BaseDN
		}
		if filter == "" {
			return ldapError("'filter' is required for search operation.")
		}

		client := newLDAPClient(clientCfg)
		if err := client.ConnectAndBind(); err != nil {
			return ldapError("Failed to connect to LDAP server: %s", security.Scrub(err.Error()))
		}
		defer client.Close()

		result, err := client.Search(baseDN, filter, searchAttributes)
		if err != nil {
			return ldapError("Search failed: %s", security.Scrub(err.Error()))
		}
		return ldapSuccess(map[string]interface{}{"entries": result.Entries})

	case "get_user":
		if clientCfg.BaseDN == "" && clientCfg.UserSearchBase == "" {
			return ldapError("LDAP base_dn is not configured.")
		}
		if username == "" {
			return ldapError("'username' is required for get_user operation.")
		}

		client := newLDAPClient(clientCfg)
		if err := client.ConnectAndBind(); err != nil {
			return ldapError("Failed to connect to LDAP server: %s", security.Scrub(err.Error()))
		}
		defer client.Close()

		entry, err := client.GetUser(username)
		if err != nil {
			return ldapError("Get user failed: %s", security.Scrub(err.Error()))
		}
		if entry == nil {
			return ldapSuccess(map[string]interface{}{"user": nil, "message": "User not found"})
		}
		return ldapSuccess(map[string]interface{}{"user": entry})

	case "list_users":
		if clientCfg.BaseDN == "" && clientCfg.UserSearchBase == "" {
			return ldapError("LDAP base_dn is not configured.")
		}

		client := newLDAPClient(clientCfg)
		if err := client.ConnectAndBind(); err != nil {
			return ldapError("Failed to connect to LDAP server: %s", security.Scrub(err.Error()))
		}
		defer client.Close()

		result, err := client.ListUsers()
		if err != nil {
			return ldapError("List users failed: %s", security.Scrub(err.Error()))
		}
		return ldapSuccess(map[string]interface{}{"users": result.Entries})

	case "get_group":
		if clientCfg.BaseDN == "" && clientCfg.GroupSearchBase == "" {
			return ldapError("LDAP base_dn is not configured.")
		}
		if groupName == "" {
			return ldapError("'group_name' is required for get_group operation.")
		}

		client := newLDAPClient(clientCfg)
		if err := client.ConnectAndBind(); err != nil {
			return ldapError("Failed to connect to LDAP server: %s", security.Scrub(err.Error()))
		}
		defer client.Close()

		entry, err := client.GetGroup(groupName)
		if err != nil {
			return ldapError("Get group failed: %s", security.Scrub(err.Error()))
		}
		if entry == nil {
			return ldapSuccess(map[string]interface{}{"group": nil, "message": "Group not found"})
		}
		return ldapSuccess(map[string]interface{}{"group": entry})

	case "list_groups":
		if clientCfg.BaseDN == "" && clientCfg.GroupSearchBase == "" {
			return ldapError("LDAP base_dn is not configured.")
		}

		client := newLDAPClient(clientCfg)
		if err := client.ConnectAndBind(); err != nil {
			return ldapError("Failed to connect to LDAP server: %s", security.Scrub(err.Error()))
		}
		defer client.Close()

		result, err := client.ListGroups()
		if err != nil {
			return ldapError("List groups failed: %s", security.Scrub(err.Error()))
		}
		return ldapSuccess(map[string]interface{}{"groups": result.Entries})

	case "authenticate":
		if userDN == "" || password == "" {
			return ldapError("'user_dn' and 'password' are required for authenticate operation.")
		}

		security.RegisterSensitive(password)

		client := newLDAPClient(clientCfg)
		if err := client.Connect(); err != nil {
			return ldapError("Failed to connect to LDAP server: %s", security.Scrub(err.Error()))
		}
		defer client.Close()

		ok, err := client.Authenticate(userDN, password)
		if err != nil {
			return ldapError("Authentication failed: %s", security.Scrub(err.Error()))
		}
		if !ok {
			return ldapSuccess(map[string]interface{}{"authenticated": false, "message": "Invalid credentials"})
		}
		return ldapSuccess(map[string]interface{}{"authenticated": true, "message": "Authentication successful"})

	case "test_connection":
		client := newLDAPClient(clientCfg)
		if err := client.TestConnection(); err != nil {
			return ldapError("Connection test failed: %s", security.Scrub(err.Error()))
		}
		return ldapSuccess(map[string]interface{}{"message": "Connection successful"})

	case "add_user", "add_group":
		if dn == "" {
			return ldapError("'dn' is required for %s operation.", operation)
		}
		if len(entryAttributes) == 0 {
			return ldapError("'entry_attributes' is required for %s operation.", operation)
		}
		client := newLDAPClient(clientCfg)
		if err := client.ConnectAndBind(); err != nil {
			return ldapError("Failed to connect to LDAP server: %s", security.Scrub(err.Error()))
		}
		defer client.Close()

		if err := client.AddEntry(dn, entryAttributes); err != nil {
			return ldapError("Add operation failed: %s", security.Scrub(err.Error()))
		}
		if logger != nil {
			logger.Info("LDAP entry added", "operation", operation, "dn", dn)
		}
		return ldapSuccess(map[string]interface{}{"message": "Entry created successfully", "dn": dn})

	case "update_user", "update_group":
		if dn == "" {
			return ldapError("'dn' is required for %s operation.", operation)
		}
		if len(changes) == 0 {
			return ldapError("'changes' is required for %s operation.", operation)
		}
		client := newLDAPClient(clientCfg)
		if err := client.ConnectAndBind(); err != nil {
			return ldapError("Failed to connect to LDAP server: %s", security.Scrub(err.Error()))
		}
		defer client.Close()

		if err := client.ModifyEntry(dn, changes); err != nil {
			return ldapError("Update operation failed: %s", security.Scrub(err.Error()))
		}
		if logger != nil {
			logger.Info("LDAP entry updated", "operation", operation, "dn", dn)
		}
		return ldapSuccess(map[string]interface{}{"message": "Entry updated successfully", "dn": dn})

	case "delete_user", "delete_group":
		if dn == "" {
			return ldapError("'dn' is required for %s operation.", operation)
		}
		client := newLDAPClient(clientCfg)
		if err := client.ConnectAndBind(); err != nil {
			return ldapError("Failed to connect to LDAP server: %s", security.Scrub(err.Error()))
		}
		defer client.Close()

		if err := client.DeleteEntry(dn); err != nil {
			return ldapError("Delete operation failed: %s", security.Scrub(err.Error()))
		}
		if logger != nil {
			logger.Info("LDAP entry deleted", "operation", operation, "dn", dn)
		}
		return ldapSuccess(map[string]interface{}{"message": "Entry deleted successfully", "dn": dn})

	default:
		return ldapError("Unknown LDAP operation: %s", operation)
	}
}

func ldapError(format string, args ...interface{}) string {
	return ldapJSON(map[string]interface{}{
		"status":  "error",
		"message": fmt.Sprintf(format, args...),
	})
}

func ldapSuccess(fields map[string]interface{}) string {
	payload := map[string]interface{}{"status": "success"}
	for key, value := range fields {
		payload[key] = value
	}
	return ldapJSON(payload)
}

func ldapJSON(payload map[string]interface{}) string {
	data, err := json.Marshal(payload)
	if err != nil {
		fallback, _ := json.Marshal(map[string]interface{}{
			"status":  "error",
			"message": "Failed to serialize LDAP response.",
		})
		return security.Scrub(string(fallback))
	}
	return security.Scrub(string(data))
}

func stringSliceArg(raw interface{}) []string {
	switch values := raw.(type) {
	case []string:
		return append([]string(nil), values...)
	case []interface{}:
		result := make([]string, 0, len(values))
		for _, value := range values {
			if s, ok := value.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		return result
	case string:
		if values != "" {
			return []string{values}
		}
	}
	return nil
}

func stringSliceMapArg(raw interface{}) map[string][]string {
	switch values := raw.(type) {
	case map[string][]string:
		result := make(map[string][]string, len(values))
		for key, slice := range values {
			result[key] = append([]string(nil), slice...)
		}
		return result
	case map[string]interface{}:
		result := make(map[string][]string, len(values))
		for key, value := range values {
			result[key] = stringSliceArg(value)
		}
		if len(result) > 0 {
			return result
		}
	}
	return nil
}

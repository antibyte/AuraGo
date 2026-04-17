package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/ldap"
)

type ldapTestClient interface {
	TestConnection() error
}

var newLDAPTestClient = func(cfg ldap.LDAPConfig) ldapTestClient {
	return ldap.NewClient(cfg)
}

func handleLDAPTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		cfg := s.Cfg.LDAP
		s.CfgMu.RUnlock()

		if !cfg.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "LDAP integration is disabled",
			})
			return
		}

		if cfg.Host == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "LDAP host is not configured",
			})
			return
		}
		if cfg.BindDN == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "LDAP bind_dn is not configured",
			})
			return
		}

		clientCfg := ldap.LDAPConfig{
			Enabled:            cfg.Enabled,
			ReadOnly:           cfg.ReadOnly,
			Host:               cfg.Host,
			Port:               cfg.Port,
			UseTLS:             cfg.UseTLS,
			InsecureSkipVerify: cfg.InsecureSkipVerify,
			BaseDN:             cfg.BaseDN,
			BindDN:             cfg.BindDN,
			BindPassword:       cfg.BindPassword,
			UserSearchBase:     cfg.UserSearchBase,
			GroupSearchBase:    cfg.GroupSearchBase,
			ConnectTimeout:     cfg.ConnectTimeout,
			RequestTimeout:     cfg.RequestTimeout,
		}

		if clientCfg.BindPassword == "" && s.Vault != nil {
			if pwd, err := s.Vault.ReadSecret("ldap_bind_password"); err == nil && pwd != "" {
				clientCfg.BindPassword = pwd
			}
		}

		client := newLDAPTestClient(clientCfg)
		if err := client.TestConnection(); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Connection failed: " + err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "Connection successful",
		})
	}
}

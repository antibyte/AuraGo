package server

import (
	"fmt"
	"net/http"
	"strings"

	"aurago/internal/config"
)

func safeTuyaAccounts(cfg config.Go2RTCConfig) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(cfg.TuyaAccounts))
	for _, a := range cfg.TuyaAccounts {
		item := go2RTCDetectionProjection(a)
		item["credentials_configured"] = true
		count := 0
		for _, stream := range cfg.Streams {
			if stream.Tuya != nil && stream.Tuya.AccountID == a.ID {
				count++
			}
		}
		item["camera_count"] = count
		result = append(result, item)
	}
	return result
}

func handleGo2RTCTuyaAccounts(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			writeGo2RTCJSON(w, map[string]interface{}{"accounts": safeTuyaAccounts(s.ConfigSnapshot().Go2RTC)})
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", 405)
			return
		}
		mutateTuyaAccount(s, w, r, "", true)
	}
}

func handleGo2RTCTuyaAccount(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch && r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", 405)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/go2rtc/tuya/accounts/")
		if config.Go2RTCTuyaAccountKey(id) == "" {
			jsonError(w, "Invalid Tuya account ID", 400)
			return
		}
		mutateTuyaAccount(s, w, r, id, false)
	}
}

func mutateTuyaAccount(s *Server, w http.ResponseWriter, r *http.Request, id string, create bool) {
	if !sameOriginOrNoOrigin(r) {
		jsonError(w, "Request origin does not match server host", 403)
		return
	}
	var request struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Method string `json:"method"`
		Region string `json:"region"`
		config.Go2RTCTuyaCredentials
	}
	if r.Method != http.MethodDelete {
		if err := decodeGo2RTCAppJSON(w, r, &request); err != nil {
			jsonError(w, "Invalid Tuya account request", 400)
			return
		}
	}
	if create {
		id = request.ID
	}
	if config.Go2RTCTuyaAccountKey(id) == "" {
		jsonError(w, "Invalid Tuya account ID", 400)
		return
	}
	c := request.Go2RTCTuyaCredentials
	inputs := go2RTCTuyaInputs{Accounts: map[string]map[string]string{id: {"email": c.Email, "password": c.Password, "client_id": c.ClientID, "client_secret": c.ClientSecret, "uid": c.UID}}}
	result, err := updateManagedGo2RTCConfigWithTuya(r.Context(), s, func(cfg *config.Go2RTCConfig) error {
		found := -1
		for i, a := range cfg.TuyaAccounts {
			if a.ID == id {
				found = i
				break
			}
		}
		if create && found >= 0 {
			return &go2RTCMutationError{Status: 409, Code: "tuya_account_exists", Message: "Tuya account already exists"}
		}
		if !create && found < 0 {
			return &go2RTCMutationError{Status: 404, Code: "tuya_account_not_found", Message: "Tuya account was not found"}
		}
		if r.Method == http.MethodDelete {
			var cameras []string
			for _, stream := range cfg.Streams {
				if stream.Tuya != nil && stream.Tuya.AccountID == id {
					cameras = append(cameras, stream.ID)
				}
			}
			if len(cameras) > 0 {
				return &go2RTCMutationError{Status: 409, Code: "tuya_account_in_use", Cameras: cameras, Message: fmt.Sprintf("Tuya account is used by cameras: %s", strings.Join(cameras, ", "))}
			}
			cfg.TuyaAccounts = append(cfg.TuyaAccounts[:found], cfg.TuyaAccounts[found+1:]...)
			return nil
		}
		a := config.Go2RTCTuyaAccount{ID: id}
		if found >= 0 {
			a = cfg.TuyaAccounts[found]
		}
		if request.Name != "" {
			a.Name = strings.TrimSpace(request.Name)
		}
		if request.Method != "" {
			a.Method = request.Method
		}
		if request.Region != "" {
			a.Region = request.Region
		}
		if found >= 0 {
			cfg.TuyaAccounts[found] = a
		} else {
			cfg.TuyaAccounts = append(cfg.TuyaAccounts, a)
		}
		return nil
	}, nil, false, inputs)
	if err != nil {
		writeGo2RTCMutationFailure(w, err)
		return
	}
	status := http.StatusOK
	if create {
		status = http.StatusCreated
	}
	writeGo2RTCMutationResult(w, status, result, map[string]interface{}{"accounts": safeTuyaAccounts(result.Config)})
}

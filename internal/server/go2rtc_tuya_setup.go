package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

type go2RTCTuyaDevice struct {
	ID   string
	Name string
}
type go2RTCTuyaSession struct {
	identity, account, revision string
	expires                     time.Time
	devices                     map[string]go2RTCTuyaDevice
	reservation                 string
}

type go2RTCTuyaCameraRequest struct {
	AccountID      string `json:"account_id"`
	MainResolution string `json:"main_resolution"`
	SubResolution  string `json:"sub_resolution"`
	DeviceID       string `json:"device_id"`
	SetupToken     string `json:"setup_token"`
	CandidateID    string `json:"candidate_id"`
}

type go2RTCTuyaReservation struct {
	s                                       *Server
	token, lease, account, revision, device string
	once                                    sync.Once
}

func (x *go2RTCTuyaReservation) finish(commit bool) {
	if x == nil {
		return
	}
	x.once.Do(func() {
		x.s.go2RTCTuyaMu.Lock()
		defer x.s.go2RTCTuyaMu.Unlock()
		session := x.s.go2RTCTuyaSessions[x.token]
		if session == nil || session.reservation != x.lease {
			return
		}
		if commit || !time.Now().Before(session.expires) {
			delete(x.s.go2RTCTuyaSessions, x.token)
		} else {
			session.reservation = ""
		}
	})
}
func (x *go2RTCTuyaReservation) check(cfg config.Go2RTCConfig) error {
	if x == nil {
		return nil
	}
	for _, a := range cfg.TuyaAccounts {
		if a.ID == x.account && a.Revision == x.revision {
			return nil
		}
	}
	return &go2RTCMutationError{Status: 409, Code: "tuya_setup_expired", Message: "Tuya account changed; repeat camera setup"}
}

func (s *Server) tuyaAccount(id string) (config.Go2RTCTuyaAccount, error) {
	if snapshot := s.ConfigSnapshot(); snapshot != nil {
		for _, a := range snapshot.Go2RTC.TuyaAccounts {
			if a.ID == id {
				return a, nil
			}
		}
	}
	return config.Go2RTCTuyaAccount{}, fmt.Errorf("Tuya account was not found")
}
func (s *Server) claimTuyaAccount(id string) (func(), error) {
	s.go2RTCTuyaMu.Lock()
	defer s.go2RTCTuyaMu.Unlock()
	if s.go2RTCTuyaBusy == nil {
		s.go2RTCTuyaBusy = map[string]bool{}
	}
	if s.go2RTCTuyaBusy[id] {
		return nil, fmt.Errorf("Tuya account already has an active search or test")
	}
	s.go2RTCTuyaBusy[id] = true
	return func() { s.go2RTCTuyaMu.Lock(); delete(s.go2RTCTuyaBusy, id); s.go2RTCTuyaMu.Unlock() }, nil
}

func (s *Server) newTuyaSession(r *http.Request, a config.Go2RTCTuyaAccount, devices []go2RTCTuyaDevice) (string, []map[string]string, error) {
	token, err := tuyaNewRevision()
	if err != nil {
		return "", nil, err
	}
	session := &go2RTCTuyaSession{identity: go2RTCTalkIdentity(r), account: a.ID, revision: a.Revision, expires: time.Now().Add(5 * time.Minute), devices: map[string]go2RTCTuyaDevice{}}
	items := make([]map[string]string, 0, len(devices))
	for _, device := range devices {
		if len(items) >= 128 {
			break
		}
		id, err := tuyaNewRevision()
		if err != nil {
			return "", nil, err
		}
		session.devices[id] = device
		items = append(items, map[string]string{"id": id, "name": device.Name})
	}
	s.go2RTCTuyaMu.Lock()
	defer s.go2RTCTuyaMu.Unlock()
	if s.go2RTCTuyaSessions == nil {
		s.go2RTCTuyaSessions = map[string]*go2RTCTuyaSession{}
	}
	for id, x := range s.go2RTCTuyaSessions {
		if !time.Now().Before(x.expires) && x.reservation == "" {
			delete(s.go2RTCTuyaSessions, id)
		}
	}
	if len(s.go2RTCTuyaSessions) >= 32 {
		return "", nil, fmt.Errorf("too many active Tuya setup sessions")
	}
	s.go2RTCTuyaSessions[token] = session
	time.AfterFunc(5*time.Minute, func() {
		s.go2RTCTuyaMu.Lock()
		defer s.go2RTCTuyaMu.Unlock()
		if x := s.go2RTCTuyaSessions[token]; x == session && x.reservation == "" {
			delete(s.go2RTCTuyaSessions, token)
		}
	})
	return token, items, nil
}

func (s *Server) tuyaDevice(r *http.Request, a config.Go2RTCTuyaAccount, input go2RTCTuyaCameraRequest, reserve bool) (string, *go2RTCTuyaReservation, error) {
	if input.SetupToken == "" {
		return strings.TrimSpace(input.DeviceID), nil, nil
	}
	if input.DeviceID != "" {
		return "", nil, fmt.Errorf("use a Tuya selection or device ID")
	}
	s.go2RTCTuyaMu.Lock()
	defer s.go2RTCTuyaMu.Unlock()
	x := s.go2RTCTuyaSessions[input.SetupToken]
	if x == nil || !time.Now().Before(x.expires) || x.account != a.ID || x.revision != a.Revision || x.identity != go2RTCTalkIdentity(r) || x.reservation != "" {
		return "", nil, fmt.Errorf("Tuya setup is missing, expired, changed or already reserved")
	}
	device, ok := x.devices[input.CandidateID]
	if !ok {
		return "", nil, fmt.Errorf("Tuya camera selection is unavailable")
	}
	if !reserve {
		return device.ID, nil, nil
	}
	lease, err := tuyaNewRevision()
	if err != nil {
		return "", nil, err
	}
	x.reservation = lease
	return device.ID, &go2RTCTuyaReservation{s: s, token: input.SetupToken, lease: lease, account: a.ID, revision: a.Revision, device: device.ID}, nil
}

func handleGo2RTCTuyaSetup(s *Server, discover bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", 405)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", 403)
			return
		}
		var input struct {
			go2RTCTuyaCameraRequest
			Resolution string `json:"resolution"`
		}
		if err := decodeGo2RTCAppJSON(w, r, &input); err != nil {
			jsonError(w, "Invalid Tuya setup request", 400)
			return
		}
		a, err := s.tuyaAccount(input.AccountID)
		if err != nil {
			jsonError(w, err.Error(), 404)
			return
		}
		c, err := readTuyaCredentials(s.Vault, a.ID)
		if err != nil {
			jsonError(w, "Tuya credentials are unavailable", 400)
			return
		}
		if discover && a.Method != "smart" {
			jsonError(w, "Tuya Cloud uses a device ID", 400)
			return
		}
		release, err := s.claimTuyaAccount(a.ID)
		if err != nil {
			jsonError(w, err.Error(), 409)
			return
		}
		defer release()
		if s.Go2RTC == nil {
			jsonError(w, "go2rtc is unavailable", 503)
			return
		}
		budget := 45 * time.Second
		if discover {
			budget = 20 * time.Second
		}
		ctx, cancel := context.WithTimeout(r.Context(), budget)
		defer cancel()
		var result tools.Go2RTCTuyaResult
		var device string
		if discover {
			result = s.Go2RTC.TuyaDiscover(ctx, a, c)
		} else {
			device, _, err = s.tuyaDevice(r, a, input.go2RTCTuyaCameraRequest, false)
			if err != nil {
				jsonError(w, err.Error(), 400)
				return
			}
			resolution := input.Resolution
			if resolution == "" {
				resolution = "hd"
			}
			source, err := config.Go2RTCTuyaSource(a, c, device, resolution)
			if err != nil {
				jsonError(w, "Invalid Tuya device or quality", 400)
				return
			}
			result = s.Go2RTC.TuyaTest(ctx, source, a.ID)
		}
		current, err := s.tuyaAccount(a.ID)
		if err != nil || current.Revision != a.Revision || r.Context().Err() != nil {
			jsonError(w, "Tuya account changed; repeat setup", 409)
			return
		}
		response := go2RTCDetectionProjection(result)
		if result.Status == "ok" {
			if !discover && input.SetupToken != "" {
				if _, _, err := s.tuyaDevice(r, a, input.go2RTCTuyaCameraRequest, false); err == nil {
					response["setup_token"] = input.SetupToken
					response["candidates"] = []map[string]string{{"id": input.CandidateID, "name": ""}}
					writeGo2RTCJSON(w, response)
					return
				}
			}
			devices := []go2RTCTuyaDevice{}
			if discover {
				for _, candidate := range result.Candidates {
					if _, err := config.Go2RTCTuyaSource(a, c, candidate.DeviceID, "hd"); err == nil {
						devices = append(devices, go2RTCTuyaDevice{ID: candidate.DeviceID, Name: candidate.Name})
					}
				}
			} else {
				devices = append(devices, go2RTCTuyaDevice{ID: device, Name: ""})
			}
			token, items, err := s.newTuyaSession(r, a, devices)
			if err != nil {
				jsonError(w, "Tuya setup capacity reached", 429)
				return
			}
			response["setup_token"], response["candidates"] = token, items
		}
		writeGo2RTCJSON(w, response)
	}
}

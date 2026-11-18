package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"aurago/internal/commands"
	"aurago/internal/i18n"
	"aurago/internal/tools"
	"aurago/internal/uid"
)

type missionWriteRequest struct {
	tools.MissionV2
	ConfirmOperatorCommand bool `json:"confirm_operator_command"`
}

type missionOperatorRunKey struct{}

func operatorMissionEligible(mission *tools.MissionV2) bool {
	return commands.IsOperatorCommand(mission.Prompt) && !strings.ContainsAny(mission.Prompt, "\r\n\x00") &&
		(mission.ExecutionType == tools.ExecutionManual || mission.ExecutionType == tools.ExecutionScheduled) &&
		(mission.RunnerType == "" || mission.RunnerType == tools.MissionRunnerLocal) &&
		!mission.SyncedFromMaster && mission.RemoteNestID == "" && len(mission.CheatsheetIDs) == 0 && !mission.AutoPrepare
}

func missionOperatorSignature(secret string, mission *tools.MissionV2) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("aurago/mission-operator/v1\x00"))
	data, _ := json.Marshal([]string{mission.ID, mission.Prompt, string(mission.ExecutionType), mission.Schedule})
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// Operator approvals require the owner's browser session on both mission surfaces.
func (s *Server) missionOperatorOwnerSecret(r *http.Request) string {
	if _, bearer := bearerCredential(r.Header.Get("Authorization")); bearer {
		return ""
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.Auth.Enabled || cfg.Auth.PasswordHash == "" || cfg.Auth.SessionSecret == "" ||
		!IsAuthenticated(r, cfg.Auth.SessionSecret) || !checkCSRFOriginWithPolicy(r, true) {
		return ""
	}
	return cfg.Auth.SessionSecret
}

func (s *Server) prepareMissionOperatorApproval(w http.ResponseWriter, r *http.Request, mission *tools.MissionV2, id string, confirmed bool) bool {
	mission.OperatorCommandApproval = ""
	if !commands.IsOperatorCommand(mission.Prompt) && !confirmed {
		return true
	}
	lang := ""
	if cfg := s.ConfigSnapshot(); cfg != nil {
		lang = cfg.Server.UILanguage
	}
	if !operatorMissionEligible(mission) {
		writeMissionOperatorError(w, http.StatusBadRequest, "mission_operator_invalid", lang)
		return false
	}
	secret := s.missionOperatorOwnerSecret(r)
	if secret == "" {
		writeMissionOperatorError(w, http.StatusForbidden, "mission_operator_owner_required", lang)
		return false
	}
	if !confirmed {
		writeMissionOperatorError(w, http.StatusConflict, "mission_operator_confirmation_required", lang)
		return false
	}
	if r.Method == http.MethodPost {
		// The approval binds a new mission ID, even if a client sends an ID to create.
		id = "m_" + uid.New()
	}
	mission.ID = id
	mission.OperatorCommandApproval = missionOperatorSignature(secret, mission)
	return true
}

func writeMissionOperatorError(w http.ResponseWriter, status int, code, lang string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "error": i18n.T(lang, "backend."+code)})
}

func (s *Server) missionRunAllowsOperator(id, input string) bool {
	manager := s.MissionManagerV2
	if manager == nil || id == "" || !commands.IsOperatorCommand(input) {
		return false
	}
	mission, ok := manager.Get(id)
	cfg := s.ConfigSnapshot()
	if !ok || !mission.Enabled || mission.Prompt != input || !operatorMissionEligible(mission) ||
		cfg == nil || !cfg.Auth.Enabled || cfg.Auth.PasswordHash == "" || cfg.Auth.SessionSecret == "" ||
		!hmac.Equal([]byte(mission.OperatorCommandApproval), []byte(missionOperatorSignature(cfg.Auth.SessionSecret, mission))) {
		return false
	}
	switch manager.ActiveMissionTrigger(id) {
	case "cron":
		return mission.ExecutionType == tools.ExecutionScheduled
	case "manual":
		owner, owned := manager.ActiveOwnerContext(id)
		return owned && owner != nil && owner.Err() == nil && owner.Value(missionOperatorRunKey{}) == mission.OperatorCommandApproval
	default:
		return false
	}
}

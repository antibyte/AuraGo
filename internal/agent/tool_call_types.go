package agent

import "encoding/json"

type executeSkillNativeEnvelope struct {
	Skill     string          `json:"skill"`
	SkillName string          `json:"skill_name"`
	Params    json.RawMessage `json:"params"`
	SkillArgs json.RawMessage `json:"skill_args"`
}

func decodeJSONObject(raw json.RawMessage) map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil || len(obj) == 0 {
		return nil
	}

	return obj
}

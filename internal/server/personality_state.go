package server

func (s *Server) buildPersonalityStatePayload() map[string]interface{} {
	if !s.Cfg.Personality.Engine {
		return map[string]interface{}{"enabled": false}
	}

	traits, err := s.ShortTermMem.GetTraits()
	if err != nil {
		s.Logger.Error("Failed to get personality traits", "error", err)
		return map[string]interface{}{"enabled": false}
	}

	mood := s.ShortTermMem.GetCurrentMood()
	trigger := s.ShortTermMem.GetLastMoodTrigger()

	response := map[string]interface{}{
		"enabled": true,
		"mood":    string(mood),
		"trigger": trigger,
		"traits":  traits,
	}

	if s.Cfg.Personality.EmotionSynthesizer.Enabled {
		if latest, err := s.ShortTermMem.GetLatestEmotion(); err == nil && latest != nil {
			response["current_emotion"] = latest.Description
			response["emotion_timestamp"] = latest.Timestamp
			response["current_emotion_state"] = latest
		}
	}

	return response
}

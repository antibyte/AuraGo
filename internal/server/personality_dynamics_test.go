package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/memory"
)

func TestPersonalityDynamicsFeedbackAndReset(t *testing.T) {
	s := newTestServerWithPersonalityState(t)
	if err := s.ShortTermMem.SetTrait(memory.TraitAffinity, .85); err != nil {
		t.Fatal(err)
	}
	post := func(path, payload string, handler http.HandlerFunc) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		handler(response, httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload)))
		if response.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, response.Code, response.Body.String())
		}
		return response
	}
	post("/api/personality/feedback", `{"type":"negative","event_id":"feedback1"}`, handlePersonalityFeedback(s))
	before, _ := s.ShortTermMem.GetPersonalitySnapshotAt(time.Now())
	if before.Dynamics.Familiarity < .8 || before.Dynamics.Friction <= 0 {
		t.Fatalf("criticism erased familiarity or omitted friction: %+v", before.Dynamics)
	}
	post("/api/personality/feedback", `{"type":"negative","event_id":"feedback1"}`, handlePersonalityFeedback(s))
	duplicate, _ := s.ShortTermMem.GetPersonalitySnapshotAt(time.Now())
	if duplicate.Dynamics.Revision != before.Dynamics.Revision {
		t.Fatal("replayed feedback applied twice")
	}
	if err := s.ShortTermMem.InsertEmotionHistory("A previous description", "cautious", "feedback"); err != nil {
		t.Fatal(err)
	}
	response := post("/api/personality/dynamics/reset", "", handlePersonalityDynamicsReset(s))
	after, _ := s.ShortTermMem.GetPersonalitySnapshotAt(time.Now())
	if !reflect.DeepEqual(after.Traits, before.Traits) || after.Dynamics.Load != 0 || after.Dynamics.Friction != 0 || after.Affect.Valence != 0 || after.Epoch <= before.Epoch {
		t.Fatalf("bad reset: %+v", after)
	}
	if strings.Contains(response.Body.String(), "A previous description") {
		t.Fatal("reset exposed pre-reset narration")
	}
	for i := 0; i < 4; i++ {
		s.buildPersonalityStatePayload()
	}
	read, _ := s.ShortTermMem.GetPersonalitySnapshotAt(time.Now())
	if read.Dynamics.Revision != after.Dynamics.Revision {
		t.Fatal("state polling changed revision")
	}
}

func TestPersonalityDynamicsResetMethodAndDisabled(t *testing.T) {
	s := newTestServerWithPersonalityState(t)
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		r := httptest.NewRecorder()
		handlePersonalityDynamicsReset(s)(r, httptest.NewRequest(method, "/api/personality/dynamics/reset", nil))
		if r.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s: %d", method, r.Code)
		}
	}
	s.Cfg.Personality.Engine = false
	r := httptest.NewRecorder()
	handlePersonalityDynamicsReset(s)(r, httptest.NewRequest(http.MethodPost, "/api/personality/dynamics/reset", nil))
	if r.Code != http.StatusBadRequest {
		t.Fatalf("disabled reset: %d", r.Code)
	}
}

func TestPersonalityDynamicsResetUsesPersonalityWriteProtection(t *testing.T) {
	s := newTestServerWithPersonalityState(t)
	s.Cfg.Auth.Enabled, s.Cfg.Auth.RequireOriginHeader = true, true
	s.Cfg.Auth.SessionSecret, s.Cfg.Auth.PasswordHash = "fixture-session-secret", "fixture-password-hash"
	handler := authMiddleware(s, handlePersonalityDynamicsReset(s))
	for _, tc := range []struct {
		authenticated bool
		origin        string
		status        int
	}{
		{false, "http://example.com", http.StatusUnauthorized},
		{true, "http://other.invalid", http.StatusForbidden},
		{true, "http://example.com", http.StatusOK},
	} {
		request := httptest.NewRequest(http.MethodPost, "http://example.com/api/personality/dynamics/reset", nil)
		request.Header.Set("Origin", tc.origin)
		if tc.authenticated {
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("authenticated=%v origin=%s: %d", tc.authenticated, tc.origin, response.Code)
		}
	}
}

func TestPersonalityConfigPublicationInvalidatesHelper(t *testing.T) {
	s := newTestServerWithPersonalityState(t)
	s.Cfg.Personality.CorePersonality = "friend"
	s.syncPersonalityConfig(s.Cfg)
	before, err := s.ShortTermMem.GetPersonalitySnapshotAt(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	next := *s.Cfg
	next.Personality.CorePersonality = "punk"
	s.replaceConfigSnapshot(&next)
	_, err = s.ShortTermMem.ApplyPersonalityObservation(memory.PersonalityObservation{Semantic: true, Basis: &before, Mood: memory.MoodPlayful})
	if !errors.Is(err, memory.ErrStalePersonalityObservation) {
		t.Fatalf("late helper after persona switch: %v", err)
	}
	before, _ = s.ShortTermMem.GetPersonalitySnapshotAt(time.Now())
	disabled := *s.Cfg
	disabled.Personality.Engine = false
	s.replaceConfigSnapshot(&disabled)
	_, err = s.ShortTermMem.ApplyPersonalityObservation(memory.PersonalityObservation{Semantic: true, Basis: &before, Mood: memory.MoodPlayful})
	if !errors.Is(err, memory.ErrStalePersonalityObservation) {
		t.Fatalf("late helper after disabling engine: %v", err)
	}
}

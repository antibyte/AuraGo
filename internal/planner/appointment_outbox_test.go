package planner

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestAppointmentInstantsNormalizeAndCompareOffsets(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	now := time.Now()
	past := now.Add(-time.Minute).In(time.FixedZone("east", 12*3600)).Format(time.RFC3339Nano)
	future := now.Add(time.Hour).In(time.FixedZone("west", -12*3600)).Format(time.RFC3339Nano)
	id, err := CreateAppointment(db, Appointment{Title: "due across offsets", DateTime: future, NotificationAt: past})
	if err != nil {
		t.Fatal(err)
	}
	apt, err := GetAppointment(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if apt.NotificationAt[len(apt.NotificationAt)-1:] != "Z" {
		t.Fatal("notification was not normalized to UTC")
	}
	due, err := GetDueNotifications(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].ID != id {
		t.Fatalf("offset due comparison: %v", due)
	}
	if err := AutoExpireAppointments(db); err != nil {
		t.Fatal(err)
	}
	apt, _ = GetAppointment(db, id)
	if apt.Status != "upcoming" {
		t.Fatal("future appointment expired because of its offset")
	}
}

func TestAppointmentOutboxRetriesOnlyFailedDestinationAndSurvivesRestart(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	now := time.Now().UTC()
	id, err := CreateAppointment(db, Appointment{Title: "durable", DateTime: now.Add(time.Hour).Format(time.RFC3339), NotificationAt: now.Add(-time.Minute).Format(time.RFC3339), WakeAgent: true})
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	n := NewNotifier(db, logger)
	missionCalls, agentCalls := 0, 0
	var eventID string
	mission := func(event string, _ Appointment) error { missionCalls++; eventID = event; return nil }
	n.SetDurableMissionTrigger(mission)
	n.SetDurableExecutor(func(event, prompt string) error {
		agentCalls++
		if event != eventID {
			t.Fatal("destinations have different revision identities")
		}
		return errors.New("not admitted")
	})
	n.checkDue()
	apt, _ := GetAppointment(db, id)
	if apt.Notified {
		t.Fatal("failed admission was acknowledged")
	}
	n = NewNotifier(db, logger)
	n.SetDurableMissionTrigger(mission)
	n.SetDurableExecutor(func(string, string) error { agentCalls++; return nil })
	n.checkDue()
	n.checkDue()
	apt, _ = GetAppointment(db, id)
	if !apt.Notified || missionCalls != 1 || agentCalls != 2 {
		t.Fatalf("ack=%v mission=%d agent=%d", apt.Notified, missionCalls, agentCalls)
	}
	apt.NotificationAt = now.Add(-2 * time.Minute).Format(time.RFC3339)
	if err := UpdateAppointment(db, *apt); err != nil {
		t.Fatal(err)
	}
	n.checkDue()
	if missionCalls != 2 {
		t.Fatal("new revision did not create another admission")
	}
}

func TestAppointmentOutboxUnknownNeverReplays(t *testing.T) {
	for _, panics := range []bool{false, true} {
		t.Run(map[bool]string{false: "unknown", true: "panic"}[panics], func(t *testing.T) {
			db := testDB(t)
			defer db.Close()
			now := time.Now().UTC()
			_, err := CreateAppointment(db, Appointment{Title: "unknown", DateTime: now.Add(time.Hour).Format(time.RFC3339), NotificationAt: now.Add(-time.Minute).Format(time.RFC3339), WakeAgent: true})
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			fn := func(string, string) error {
				calls++
				if panics {
					panic("private panic detail")
				}
				return ErrNotificationOutcomeUnknown
			}
			n := NewNotifier(db, logger)
			n.SetDurableExecutor(fn)
			n.checkDue()
			n = NewNotifier(db, logger)
			n.SetDurableExecutor(fn)
			n.checkDue()
			states, err := ListAppointmentNotificationOutbox(db)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || len(states) != 1 || states[0].State != "unknown" {
				t.Fatalf("calls=%d states=%v", calls, states)
			}
		})
	}
}

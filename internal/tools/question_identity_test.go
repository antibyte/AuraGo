package tools

import "testing"

func TestQuestionIdentityAndIDAreCheckedBeforeRemoval(t *testing.T) {
	const session = "owner-question-identity"
	q := &PendingQuestion{Question: "Continue?", Options: []QuestionOption{{Value: "yes"}}}
	ch := RegisterQuestion(session, q)
	t.Cleanup(func() { CancelQuestion(session) })
	for _, tc := range []struct {
		name     string
		id       string
		identity QuestionAnswerIdentity
		selected string
	}{
		{"missing ID", "", OwnerQuestionIdentity(QuestionSourceWeb), "yes"},
		{"stale ID", "previous", OwnerQuestionIdentity(QuestionSourceDesktop), "yes"},
		{"unknown identity", q.QuestionID, QuestionAnswerIdentity{}, "yes"},
		{"internal loopback", q.QuestionID, OwnerQuestionIdentity(QuestionSourceInternal), "yes"},
		{"other SMS sender", q.QuestionID, ConversationQuestionIdentity(QuestionSourceTelnyx, "telnyx:+15550000002"), "yes"},
		{"Telegram group", q.QuestionID, ConversationQuestionIdentity(QuestionSourceTelegram, "telegram:-10:1"), "yes"},
		{"unlisted option", q.QuestionID, OwnerQuestionIdentity(QuestionSourceWeb), "run arbitrary command"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if CompleteQuestion(session, QuestionResponse{QuestionID: tc.id, Selected: tc.selected, Source: QuestionSourceWeb}, tc.identity) {
				t.Fatal("unauthorized answer accepted")
			}
			if GetPendingQuestion(session) != q {
				t.Fatal("rejected answer removed question")
			}
			select {
			case <-ch:
				t.Fatal("rejected answer delivered")
			default:
			}
		})
	}
	if !CompleteQuestion(session, QuestionResponse{QuestionID: q.QuestionID, Selected: "yes", Source: QuestionSourceInternal}, OwnerQuestionIdentity(QuestionSourceWhatsApp)) {
		t.Fatal("owner could not answer shared question from another channel")
	}
	if got := <-ch; got.Source != QuestionSourceWhatsApp {
		t.Fatal("payload overrode trusted source")
	}
}

func TestQuestionReplacementRejectsOldAnswerAndCancellation(t *testing.T) {
	const session = "question-replacement"
	old := &PendingQuestion{Question: "Old", AllowFreeText: true}
	RegisterQuestion(session, old)
	oldAnswer, _ := ResolveQuestionReply(session, "old answer")
	current := &PendingQuestion{Question: "New", AllowFreeText: true}
	ch := RegisterQuestion(session, current)
	t.Cleanup(func() { CancelQuestion(session) })
	if old.QuestionID == current.QuestionID {
		t.Fatal("question IDs reused")
	}
	CancelQuestion(session, old.QuestionID)
	if CompleteQuestion(session, oldAnswer, OwnerQuestionIdentity(QuestionSourceWeb)) || GetPendingQuestion(session) != current {
		t.Fatal("old reply or cancellation removed replacement")
	}
	answer, ok := ResolveQuestionReply(session, "new answer")
	if !ok || answer.QuestionID != current.QuestionID || !CompleteQuestion(session, answer, ConversationQuestionIdentity(QuestionSourceDiscord, session)) {
		t.Fatal("scoped native answer failed")
	}
	if (<-ch).FreeText != "new answer" {
		t.Fatal("wrong answer delivered")
	}
}

func TestQuestionOwnersCanAnswerAcrossChannels(t *testing.T) {
	for _, source := range []QuestionSource{QuestionSourceWeb, QuestionSourceDesktop, QuestionSourceTelegram, QuestionSourceWhatsApp, QuestionSourceDiscord, QuestionSourceTelnyx} {
		t.Run(string(source), func(t *testing.T) {
			q := &PendingQuestion{AllowFreeText: true}
			RegisterQuestion(t.Name(), q)
			t.Cleanup(func() { CancelQuestion(t.Name()) })
			if !CompleteQuestion(t.Name(), QuestionResponse{QuestionID: q.QuestionID, FreeText: "answer"}, OwnerQuestionIdentity(source)) {
				t.Fatal("owner refused")
			}
		})
	}
}

package meshcore

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestChannelRepliesDiscloseAIAndRejectEchoes(t *testing.T) {
	for _, mode := range []string{"prefix", "questions"} {
		t.Run(mode, func(t *testing.T) {
			m, radio, _ := testManager(t, Hooks{
				Scan: func(context.Context, Message) Review { return Review{Decision: "safe"} },
				Run:  func(context.Context, Message, string) (string, error) { return "answer", nil },
			})
			m.cfg.Channels[0].Mode = mode
			msg := Message{ID: "channel-disclosure", Kind: "channel", Direction: "incoming", State: "pending", IdentityKey: deviceKey,
				Channel: 0, Binding: m.Status().Channels[0].Binding, Text: "Operator: !aura question?", ReceivedAt: time.Now().Unix()}
			if _, err := m.store.insert(msg); err != nil {
				t.Fatal(err)
			}
			m.process(context.Background(), msg)
			radio.mu.Lock()
			defer radio.mu.Unlock()
			if len(radio.sent) != 1 || radio.sent[0][0] != 3 || !bytes.Contains(radio.sent[0], []byte("[AuraGo KI] answer")) {
				t.Fatalf("channel reply missing AI disclosure: %q", radio.sent)
			}
			for _, marker := range []string{"[AuraGo KI]", "[AuraGo]"} {
				msg.Text = "Other node: " + marker + " question?"
				if admitted, _ := admit(msg, m.cfg, m.Status(), time.Now()); admitted != "" {
					t.Fatalf("answered own/legacy AI marker %s", marker)
				}
			}
		})
	}
}

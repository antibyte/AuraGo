package agent

import (
	"strings"
	"testing"

	"aurago/internal/config"
)

// TestReachableChatChannelsNameSendWhatsApp runs the three WhatsApp states through
// buildReachableChatChannelsContext. The sender hook is process-wide state, so the test is not
// parallel.
func TestReachableChatChannelsNameSendWhatsApp(t *testing.T) {
	cases := []struct {
		name     string
		enabled  bool
		readonly bool
		sender   *recordingWhatsAppSender // nil: nothing published
		want     string
		notWant  []string
	}{
		{
			name:    "writable with a ready sender",
			enabled: true,
			sender:  &recordingWhatsAppSender{ready: true},
			want:    "- WhatsApp: reachable via `send_whatsapp` (text and files) to the owner's WhatsApp chat, or `send_notification` with channel `whatsapp`.",
		},
		{
			name:     "readonly beats a ready sender",
			enabled:  true,
			readonly: true,
			sender:   &recordingWhatsAppSender{ready: true},
			want:     "proactive sends (`send_whatsapp`, notification channel `whatsapp`) are blocked by config.",
			notWant:  []string{"reachable via `send_whatsapp`"},
		},
		{
			name:    "not linked",
			enabled: true,
			sender:  &recordingWhatsAppSender{ready: false},
			want:    "- WhatsApp: enabled but not linked or not running; do not try to send there.",
			notWant: []string{"reachable via `send_whatsapp`"},
		},
		{
			name:    "no sender published",
			enabled: true,
			want:    "- WhatsApp: enabled but not linked or not running; do not try to send there.",
			notWant: []string{"reachable via `send_whatsapp`"},
		},
		{
			name:    "disabled names nothing",
			enabled: false,
			sender:  &recordingWhatsAppSender{ready: true},
			notWant: []string{"WhatsApp", "send_whatsapp"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.sender != nil {
				publishWhatsAppSender(t, tc.sender)
			}
			cfg := &config.Config{}
			cfg.WhatsApp.Enabled = tc.enabled
			cfg.WhatsApp.ReadOnly = tc.readonly

			got := buildReachableChatChannelsContext(RunConfig{Config: cfg})
			if tc.want != "" && !strings.Contains(got, tc.want) {
				t.Fatalf("context missing %q:\n%s", tc.want, got)
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(got, notWant) {
					t.Fatalf("context holds %q:\n%s", notWant, got)
				}
			}
		})
	}
}

// TestWhatsAppCountsAsChannelAndCapabilityCue pins "whatsapp" as a channel cue for the
// reachable-channels injection and the chat-channel tool check, and as a capability subject.
func TestWhatsAppCountsAsChannelAndCapabilityCue(t *testing.T) {
	if !shouldInjectReachableChatChannelsContext("schick mir das per whatsapp", "web_chat", nil, nil) {
		t.Fatal("a web chat request that names whatsapp must inject the reachable channels")
	}
	if shouldInjectReachableChatChannelsContext("schick mir das per signal", "web_chat", nil, nil) {
		t.Fatal("the control request names no known channel and must not inject")
	}
	if !isChatChannelTool("send_whatsapp") {
		t.Fatal("send_whatsapp must count as a chat channel tool")
	}
	if !exactCapabilityQuery(intentWords("do you support whatsapp")) {
		t.Fatal("a direct capability question about whatsapp must be recognized")
	}
}

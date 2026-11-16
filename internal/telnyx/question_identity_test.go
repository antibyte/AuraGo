package telnyx

import (
	"aurago/internal/config"
	"testing"
)

func TestIncomingConversationOnlyExplicitOwnerSharesDefault(t *testing.T) {
	cfg := &config.Config{}
	cfg.Telnyx.AllowedNumbers = []string{"+15550000001", "+15550000002"}
	for _, owner := range []string{"", "+15550000001"} {
		cfg.Telnyx.OwnerNumber = owner
		for _, from := range cfg.Telnyx.AllowedNumbers {
			id, isOwner := IncomingConversation(cfg, from)
			wantOwner := owner != "" && from == owner
			if isOwner != wantOwner || (id == "default") != wantOwner {
				t.Fatal("allowlist granted owner identity")
			}
			if !wantOwner && id != "telnyx:"+from {
				t.Fatal("SMS conversation not isolated")
			}
		}
	}
	cfg.Telnyx.AllowedNumbers = nil
	if _, owner := IncomingConversation(cfg, cfg.Telnyx.OwnerNumber); owner {
		t.Fatal("revoked owner still authorized")
	}
}

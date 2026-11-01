package whatsapp

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func TestMaskDigitsKeepsLastFour(t *testing.T) {
	for in, want := range map[string]string{
		"4917012345678@s.whatsapp.net": "***5678@s.whatsapp.net",
		"code 405 after 2026":          "code 405 after 2026",
		"123456789012345:12@lid":       "***2345:12@lid",
	} {
		if got := maskDigits(in); got != want {
			t.Errorf("maskDigits(%q) = %q, want %q", in, got, want)
		}
	}
	if got := maskJID(types.NewJID("4917012345678", types.DefaultUserServer)); got != "***5678" {
		t.Errorf("maskJID = %q, want ***5678", got)
	}
	if maskJID(types.EmptyJID) != "" {
		t.Error("an empty JID produced a masked value")
	}
}

func TestWALoggerMasksAndDropsDebug(t *testing.T) {
	var buf bytes.Buffer
	log := newWALogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), "Client")
	log.Debugf("Emitting QR code %s", "2@secret-qr-payload")
	log.Sub("Socket").Infof("Successfully paired %s", "4917012345678:3@s.whatsapp.net")
	out := buf.String()
	if strings.Contains(out, "secret-qr-payload") {
		t.Fatal("whatsmeow debug output reached the log")
	}
	if strings.Contains(out, "4917012345678") || !strings.Contains(out, "***5678") || !strings.Contains(out, "Client/Socket") {
		t.Fatalf("log = %q", out)
	}
}

func TestWALoggerNeverLogsNodeContent(t *testing.T) {
	node := waBinary.Node{
		Tag:     "iq",
		Content: []waBinary.Node{{Tag: "ref", Content: []byte("pairing-ref-bytes")}},
	}
	var buf bytes.Buffer
	log := newWALogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), "Client")
	log.Warnf("unexpected %s", &node)
	log.Errorf("unexpected %v", node)
	log.Infof("unexpected %s", &node)
	log.Debugf("unexpected %s", &node)
	out := buf.String()
	if strings.Contains(out, "pairing-ref-bytes") || strings.Contains(out, "ref") {
		t.Fatalf("node content reached the log: %q", out)
	}
	if strings.Count(out, "<node iq>") != 3 {
		t.Fatalf("want the node tag three times, got %q", out)
	}
}

package mailer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/agentmail"
)

func TestStickerDraftAndMIMERoundTrip(t *testing.T) {
	data, err := os.ReadFile("../../ui/img/stickers/animals-cat.png")
	if err != nil {
		t.Fatal(err)
	}
	f := newOutboxFixture(t, filepath.Join(t.TempDir(), "mailer.db"))
	ctx := context.Background()
	draft, err := f.service.SaveDraft(ctx, Draft{AccountID: f.account.ID, To: []string{"recipient@example.test"}, Text: "Hello"})
	if err != nil {
		t.Fatal(err)
	}
	draft, err = f.service.AddDraftAttachment(ctx, draft.ID, draft.Revision, "sticker-animals-cat.png", "image/png", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	draft.Attachments[0].Inline = true
	cid := draft.Attachments[0].ID + "@aurago"
	draft.Attachments[0].ContentID = cid
	draft.Text = "Hello ![Cat](cid:" + cid + ")"
	draft.HTML = `<p>Hello <img src="cid:` + cid + `" alt="Cat" width="192" height="192"></p>`
	draft, err = f.service.SaveDraft(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := f.service.GetDraft(ctx, draft.ID)
	if err != nil || len(reopened.Attachments) != 1 || !reopened.Attachments[0].Inline || reopened.Attachments[0].ContentID != cid {
		t.Fatalf("inline draft lost on reopen: %v", err)
	}
	_, stream, err := f.service.ReadAttachment(ctx, reopened.Attachments[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	reopened.Attachments[0].Data, err = io.ReadAll(stream)
	stream.Close()
	if err != nil {
		t.Fatal(err)
	}
	reopened.Attachments = append(reopened.Attachments, Attachment{Filename: "note.txt", ContentType: "text/plain", Data: []byte("normal attachment")})
	for name, build := range map[string]func(string, Draft, time.Time) ([]byte, error){"smtp": BuildMIME, "gmail": buildGmailMIME} {
		t.Run(name, func(t *testing.T) {
			raw, err := build("sender@example.test", reopened, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			_, _, _, _, _, html, attachments, _, err := ParseMIME(raw, 8<<20)
			if err != nil || !strings.Contains(html, `src="cid:`+cid+`"`) || !strings.Contains(html, `width="192"`) || len(attachments) != 2 || !bytes.Contains(raw, []byte("multipart/related")) {
				t.Fatalf("invalid inline MIME round trip: HTML=%q attachments=%d error=%v", html, len(attachments), err)
			}
			att := attachments[0]
			if !att.Inline || att.ContentID != cid || att.ContentType != "image/png" || !bytes.Equal(att.Data, data) {
				t.Fatal("sticker lost bytes, transparency or CID metadata")
			}
			if attachments[1].Inline || attachments[1].Filename != "note.txt" || string(attachments[1].Data) != "normal attachment" {
				t.Fatal("regular attachment lost beside inline sticker")
			}
		})
	}
}

func TestAgentMailStickerWireFormat(t *testing.T) {
	for _, reply := range []bool{false, true} {
		t.Run(map[bool]string{false: "send", true: "reply"}[reply], func(t *testing.T) {
			var writes int
			client := &http.Client{Transport: agentMailFixtureTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodGet {
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"message_id":"original","subject":"Hello"}`)), Header: make(http.Header)}, nil
				}
				writes++
				var body struct {
					HTML        string              `json:"html"`
					Attachments []map[string]string `json:"attachments"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if len(body.Attachments) != 1 {
					t.Fatal("missing sticker")
				}
				att := body.Attachments[0]
				if att["content"] != base64.StdEncoding.EncodeToString([]byte("png bytes")) || att["content_base64"] != "" || att["content_id"] != "sticker@aurago" || att["content_disposition"] != "inline" || !strings.Contains(body.HTML, `src="cid:sticker@aurago"`) {
					t.Fatal("incorrect provider inline attachment fields")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"message_id":"sent"}`)), Header: make(http.Header)}, nil
			})}
			adapter, err := newAgentMailAdapter(TransportConfig{ID: "fixture", Source: "agentmail", AgentMailInboxID: "fixture", AgentMailAPIKey: "fixture", AgentMailBaseURL: "https://agentmail.example.test", HTTPClient: client})
			if err != nil {
				t.Fatal(err)
			}
			draft := Draft{To: []string{"recipient@example.test"}, Subject: "Hello", HTML: `<p><img src="cid:sticker@aurago"></p>`, Attachments: []Attachment{{Filename: "sticker.png", ContentType: "image/png", ContentID: "sticker@aurago", Inline: true, Data: []byte("png bytes")}}}
			if reply {
				draft.ReplyRemoteID = "original"
				draft.Subject = "Re: Hello"
			}
			result, err := adapter.Send(context.Background(), Outgoing{Draft: draft})
			if err != nil || !result.Confirmed || writes != 1 {
				t.Fatalf("fixture send failed: %v", err)
			}
			draft.Attachments[0].ContentID = "injected\r\nHeader: bad"
			result, err = adapter.Send(context.Background(), Outgoing{Draft: draft})
			if err == nil || !result.Rejected || writes != 1 {
				t.Fatal("unsafe CID reached provider")
			}
		})
	}
	message := agentMailMessage(TransportConfig{}, agentmail.Message{Attachments: []agentmail.Attachment{{ID: "image", ContentID: "<sticker@aurago>", ContentDisposition: "inline"}}})
	if len(message.Attachments) != 1 || !message.Attachments[0].Inline || message.Attachments[0].ContentID != "sticker@aurago" {
		t.Fatal("received sticker metadata was lost")
	}
}

package flows

import (
	"reflect"
	"testing"
)

func notifyRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := NewRegistry()
	if err := registerNotifyNodes(reg, nil); err != nil {
		t.Fatal(err)
	}
	return reg
}

const sentOK = `Tool Output: {"status":"success","results":[{"channel":"telegram","status":"sent"}]}`

func TestTelegramNode(t *testing.T) {
	tools := &fakeTools{respond: toolReply(sentOK)}
	def := lookupDef(t, notifyRegistry(t), TypeTelegram)
	res, err := execDef(def, map[string]any{"message": "Hallo", "title": "News", "file": FileRef("/data/documents/ki.pdf", "", "", "", 0)}, &Services{Tools: tools})
	if err != nil || res.Output["sent"] != true {
		t.Fatalf("telegram = %#v, %v", res.Output, err)
	}
	want := map[string]any{"message": "Hallo", "title": "News", "file_path": "/data/documents/ki.pdf"}
	if args := tools.last(t).Args; !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v", args)
	}
	tools.respond = toolReply(`Tool Output: {"status":"success","results":[{"channel":"telegram","status":"error","detail":"chat not found"}]}`)
	if _, err := execDef(def, map[string]any{"message": "x"}, &Services{Tools: tools}); asNodeError(err).Code != "FLOW_NOTIFY_FAILED" || asNodeError(err).Message != "chat not found" {
		t.Fatalf("failed channel = %v", err)
	}
	if _, err := execDef(def, map[string]any{"message": " "}, &Services{Tools: tools}); asNodeError(err).Code != "FLOW_PARAM_INVALID" {
		t.Fatalf("empty message = %v", err)
	}
	if !reflect.DeepEqual(def.Effects, []Effect{EffectSendsMessage}) {
		t.Fatal("telegram sends messages")
	}
}

func TestEmailNode(t *testing.T) {
	tools := &fakeTools{respond: toolReply(`Tool Output: {"status":"success","message":"Email sent"}`)}
	def := lookupDef(t, notifyRegistry(t), TypeEmail)
	res, err := execDef(def, map[string]any{"to": "a@b.de", "subject": "S", "body": "B", "attachment": "docs/a.pdf", "account": "work"}, &Services{Tools: tools})
	if err != nil || res.Output["sent"] != true || res.Output["to"] != "a@b.de" {
		t.Fatalf("email = %#v, %v", res.Output, err)
	}
	want := map[string]any{"to": "a@b.de", "subject": "S", "body": "B", "account": "work", "attachments": []any{"docs/a.pdf"}}
	if args := tools.last(t).Args; !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v", args)
	}
	if _, err := execDef(def, map[string]any{"body": "B"}, &Services{Tools: tools}); asNodeError(err).Code != "FLOW_PARAM_INVALID" {
		t.Fatalf("missing recipient = %v", err)
	}
}

func TestPushAndDiscordNodes(t *testing.T) {
	tools := &fakeTools{respond: toolReply(`Tool Output: {"status":"success","results":[{"channel":"ntfy","status":"sent"},{"channel":"push","status":"error","detail":"no subscription"}]}`)}
	push := lookupDef(t, notifyRegistry(t), TypePush)
	res, err := execDef(push, map[string]any{"message": "Hi"}, &Services{Tools: tools})
	if err != nil || res.Output["sent"] != true || len(res.Output["results"].([]any)) != 2 {
		t.Fatalf("push = %#v, %v", res.Output, err)
	}
	if args := tools.last(t).Args; args["channel"] != "all" || args["priority"] != "normal" || args["message"] != "Hi" {
		t.Fatalf("push args = %#v", args)
	}
	tools.respond = toolReply(`Tool Output: {"status":"success","results":[{"channel":"push","status":"error","detail":"off"}]}`)
	if _, err := execDef(push, map[string]any{"message": "Hi"}, &Services{Tools: tools}); asNodeError(err).Code != "FLOW_NOTIFY_FAILED" {
		t.Fatalf("all channels failed = %v", err)
	}

	tools.respond = toolReply(`Tool Output: {"status":"success","message":"Message sent"}`)
	discord := lookupDef(t, notifyRegistry(t), TypeDiscord)
	if res, err := execDef(discord, map[string]any{"message": "Hi", "channel_id": "123"}, &Services{Tools: tools}); err != nil || res.Output["sent"] != true {
		t.Fatalf("discord = %#v, %v", res.Output, err)
	}
	if args := tools.last(t).Args; !reflect.DeepEqual(args, map[string]any{"message": "Hi", "channel_id": "123"}) {
		t.Fatalf("discord args = %#v", args)
	}
}

package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestDesktopMailerResponsiveAndReaderContracts(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/mailer.js")
	settings := readDesktopAssetText(t, "js/desktop/apps/mailer-settings.js")
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(app, "window.MailerSettings.create(inst") {
		t.Fatal("desktop Mailer must initialize its lazy settings module")
	}
	for _, marker := range []string{"window.MailerSettings = { create }", "async function saveAccountForm", "async function previewRule", "async function applyRule", "rule_error: 'reason_rule_error'", "reasonText: code => reasonText(inst, code)"} {
		if !strings.Contains(settings, marker) {
			t.Fatalf("desktop Mailer settings module is missing %q", marker)
		}
	}
	for _, marker := range []string{
		"account.capabilities?.includes('read')",
		"reasonText(inst, reasonCode)",
		"network_disabled: 'reason_network_disabled'",
		"missing_scope: 'reason_missing_scope'",
		"source.reauthorize_supported",
		"account.reauthorize_supported",
		"inst.api.startOAuth({ account_id: accountId })",
		"if (inst.state.readonly) return;",
	} {
		if !strings.Contains(settings, marker) {
			t.Fatalf("desktop Mailer account settings are missing authorization/status behavior %q", marker)
		}
	}
	settingsIndex := strings.Index(loader, "'/js/desktop/apps/mailer-settings.js'")
	appIndex := strings.Index(loader, "'/js/desktop/apps/mailer.js'")
	if settingsIndex < 0 || appIndex <= settingsIndex {
		t.Fatal("desktop Mailer settings module must load before its app entry point")
	}
	viewsIndex := strings.Index(loader, "'/js/desktop/apps/mailer-views.js'")
	if viewsIndex < 0 || appIndex <= viewsIndex {
		t.Fatal("desktop Mailer views module must load before its app entry point")
	}
	if !strings.Contains(loader, "'mailer': ['mailer', 'stickers']") {
		t.Fatal("desktop Mailer must load its lazy translation section before app scripts")
	}
	if !strings.Contains(app, "ctx.t('mailer.' + key, vars)") {
		t.Fatal("desktop Mailer app must use its lazy translation namespace")
	}
	for _, marker := range []string{
		"inst.mailerRoot = host.querySelector('[data-mailer-app]')",
		"inst.mailerRoot.classList.add('has-active-reader')",
		"inst.mailerRoot.classList.remove('has-active-reader')",
		"inst.api.read(threadId, true)",
		"canThreadCapability(inst, listThread, 'read_state')",
		"catch (_) { setStatus(inst, t(inst.ctx, 'action_failed'), true); }",
		"selectedHasCapability(inst, capability)",
		"canThreadCapability(inst, thread, 'archive')",
		"canThreadCapability(inst, thread, 'trash')",
		"canThreadCapability(inst, thread, 'restore')",
		"canThreadCapability(inst, thread, 'labels_write')",
		"canThreadCapability(inst, item, 'star')",
		"canThreadCapability(inst, thread, 'send')",
		"draft?.account_id || thread?.account_id || options.accountId || inst.accountId",
		"reply_remote_id: account?.source === 'agentmail' && ['reply', 'reply_all'].includes(options.replyType) ? last.remote_id || '' : ''",
		"if (!draft && !thread)",
		"const account = preferred || inst.state.accounts.find(item => canThreadCapability(inst, { account_id: item.id }, 'send'))",
		"!account.disabled",
		"account.protocol === 'imap'",
		"params.cursor = inst.nextCursor",
		"result?.next_cursor",
		"more.hidden = !inst.nextCursor",
		"vd-mailer-thread-account",
		"threadIdentity(inst, item)",
		"readerActionButton(inst, 'reply', 'reply', iconSVG('reply'), false, true)",
		"vd-mailer-reader-action-group",
		"vd-mailer-participant-chip",
		"participants_count",
		"params.remote = 'true'",
		"inst.remoteSearch = !inst.remoteSearch",
		"message.html_skipped",
		"message_unavailable",
		"providerSearchFolder(inst)",
		"outboxHasMore = !!result?.has_more && !!inst.outboxCursor",
		"inst.api.outbox(Object.assign({ limit: 50 }, append && inst.outboxCursor ? { cursor: inst.outboxCursor } : {}))",
		"loadRuleRuns(inst, threadId, thread.messages || [])",
		"change.type === 'account_sync'",
		"change.type === 'rule_run'",
		"loadInlineImages(inst, message, inst.readerAbort.signal)",
		"frame && result.images.size > 0",
		"image/png", "image/jpeg", "image/gif", "image/webp",
		"URL.revokeObjectURL(url)",
	} {
		if !strings.Contains(app, marker) {
			t.Fatalf("desktop Mailer is missing behavior marker %q", marker)
		}
	}
	if strings.Contains(app, "params.offset") {
		t.Fatal("desktop Mailer pagination must follow the server cursor rather than offsets")
	}
	if strings.Contains(app, "vd-mailer-unread-dot") {
		t.Fatal("unread indicator must not become an auto-placed grid child")
	}
	if strings.Contains(app, "(item.participants || []).slice(0, 2).join") {
		t.Fatal("the message list must not derive the sender from the participant list")
	}
	compose := readDesktopAssetText(t, "js/desktop/apps/mailer-compose.js")
	for _, marker := range []string{
		"copyMessageAttachment", "forward_attachment_copy_review", "else if (pending.length)", "options.assistContext",
		"reply_remote_id: String(inst.draft.reply_remote_id || '')", "if (nextAccountId !== inst.accountId) inst.draft.reply_remote_id = ''",
		"await Promise.all([inst.uploadTask, inst.forwardCopyPromise, inst.attachmentTask])", "inst.failedUploads.size > 0",
		"data-resolve-draft-conflict", "data-reload-latest-draft", "function resolveDraftConflict(inst)", "function reloadLatestDraft(inst)",
		"function retryFailedUpload(inst, key)", "function skipForwardAttachment(inst, attachmentId)",
	} {
		if !strings.Contains(compose, marker) {
			t.Fatalf("desktop Mailer composer is missing safety marker %q", marker)
		}
	}
	api := readDesktopAssetText(t, "js/desktop/apps/mailer-api.js")
	for _, marker := range []string{"outbox: params =>", "ruleRuns: messageId =>", "retryRuleRun: (messageId, ruleId) =>"} {
		if !strings.Contains(api, marker) {
			t.Fatalf("desktop Mailer API is missing %q", marker)
		}
	}

	css := readDesktopAssetText(t, "css/desktop-app-mailer.css")
	for _, marker := range []string{
		"container-name: mailer;",
		"@container mailer (max-width: 1099px)",
		"@container mailer (max-width: 719px)",
		".vd-mailer-workspace { display: flex; flex-direction: column; position: relative; }",
		".vd-mailer-folders { flex: 0 0 auto;",
		".vd-mailer-list-panel { position: relative; flex: 1 1 auto;",
		".vd-mailer-reader { position: absolute; inset: 0; z-index: 2; display: none;",
		".vd-mailer.has-active-reader .vd-mailer-folders, .vd-mailer.has-active-reader .vd-mailer-list-panel { display: none; }",
		"container-name: mailer-compose;",
		"@container mailer-compose (max-width: 719px)",
		".vd-mailer, .vd-mailer-compose {",
		".vd-mailer-reader-action-group + .vd-mailer-reader-action-group {",
		".vd-mailer-reader-action--icon {",
		".vd-mailer-participant-chip {",
		".vd-mailer-reader { display: flex; flex-direction: column;",
		".vd-mailer-message-stack { display: flex; flex-direction: column; flex: 1 1 auto; min-height: 0; }",
		".vd-mailer-message-body { display: flex; flex-direction: column; flex: 1 1 auto; min-height: 210px;",
		".vd-mailer-html-wrap iframe { position: absolute; inset: 0;",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop Mailer styles are missing responsive/theme marker %q", marker)
		}
	}
	if strings.Contains(css, "@media (max-width: 1099px)") || strings.Contains(css, "@media (max-width: 719px)") {
		t.Fatal("Mailer breakpoints must follow the app window width, not the browser viewport")
	}
}

func TestDesktopMailerLocaleCoverage(t *testing.T) {
	t.Parallel()

	english := readMailerLocale(t, "en")
	keys := make([]string, 0, 312)
	for key := range english {
		if strings.HasPrefix(key, "mailer.") {
			keys = append(keys, key)
		}
	}
	if len(keys) < 312 {
		t.Fatalf("expected complete Mailer translations, got only %d English keys", len(keys))
	}
	for _, key := range []string{
		"mailer.reauthorize",
		"mailer.reason_account_disabled",
		"mailer.reason_integration_disabled",
		"mailer.reason_desktop_access_required",
		"mailer.reason_mail_read_required",
		"mailer.reason_network_disabled",
		"mailer.reason_missing_scope",
		"mailer.reason_auth_required",
		"mailer.reason_permission_denied",
		"mailer.reason_offline",
		"mailer.reason_sync_failed",
		"mailer.reason_storage_full",
		"mailer.reason_unknown",
	} {
		if strings.TrimSpace(english[key]) == "" {
			t.Errorf("English Mailer locale is missing settings status text %s", key)
		}
	}
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		translations := readMailerLocale(t, locale)
		desktop := readDesktopLocale(t, locale)
		if desktop["desktop.app_mailer"] != "Mailer" {
			t.Errorf("locale %s must retain the proper-name app label Mailer", locale)
		}
		for key := range desktop {
			if strings.HasPrefix(key, "desktop.mailer_") {
				t.Errorf("locale %s still embeds lazy Mailer key %s in the Desktop section", locale, key)
			}
		}
		for _, key := range keys {
			value, ok := translations[key]
			if !ok || strings.TrimSpace(value) == "" || value == key || strings.Contains(value, "desktop.mailer_") {
				t.Errorf("locale %s is missing a usable value for %s", locale, key)
				continue
			}
			if got, want := mailerPlaceholders(value), mailerPlaceholders(english[key]); strings.Join(got, "|") != strings.Join(want, "|") {
				t.Errorf("locale %s key %s has placeholders %v; want %v", locale, key, got, want)
			}
		}
		for key := range translations {
			if strings.HasPrefix(key, "mailer.") {
				if _, ok := english[key]; !ok {
					t.Errorf("locale %s has unknown Mailer key %s", locale, key)
				}
			}
		}
	}
}

var mailerPlaceholderPattern = regexp.MustCompile(`\{[a-zA-Z][a-zA-Z0-9_]*\}`)

func mailerPlaceholders(value string) []string {
	placeholders := mailerPlaceholderPattern.FindAllString(value, -1)
	sort.Strings(placeholders)
	return placeholders
}

func readMailerLocale(t *testing.T, locale string) map[string]string {
	t.Helper()
	path := filepath.Join("lang", "mailer", locale+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Mailer locale %s: %v", locale, err)
	}
	var translations map[string]string
	if err := json.Unmarshal(data, &translations); err != nil {
		t.Fatalf("parse Mailer locale %s: %v", locale, err)
	}
	return translations
}

func readDesktopLocale(t *testing.T, locale string) map[string]string {
	t.Helper()
	path := filepath.Join("lang", "desktop", locale+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read locale %s: %v", locale, err)
	}
	var translations map[string]string
	if err := json.Unmarshal(data, &translations); err != nil {
		t.Fatalf("parse locale %s: %v", locale, err)
	}
	return translations
}

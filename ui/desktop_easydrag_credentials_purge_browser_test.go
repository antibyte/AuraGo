package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

// TestDesktopEasyDragCredentialsPurgeBrowser walks the hint card and the purge dialog against a stubbed backend.
func TestDesktopEasyDragCredentialsPurgeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-credentials-purge-fixture.js")
	p := s.page
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(p); err != nil {
		t.Fatal(err)
	}
	p.MustEval(`()=>fixtureTheme('standard')`)
	s.wait(`()=>matchMedia('(prefers-reduced-motion: reduce)').matches`)
	// The hint shows while plaintext remains.
	s.wait(`()=>{const c=document.querySelector('[data-ed-credentials-hint]');return !!c && !c.hidden && c.textContent.includes(t('easydrag.ui.cred_hint_title'))}`)
	s.shot("credentials-hint")
	p.MustElement(`[data-ed-credentials-hint] [data-ed-credentials-open]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal--credentials [data-ed-credentials-snapshot]')`)
	if got := p.MustEval(`()=>document.querySelectorAll('.ed-modal--credentials [data-ed-credentials-snapshot]:checked').length`).Int(); got != 2 {
		t.Fatalf("checked snapshots = %d, want both checked by default", got)
	}
	// One snapshot is unchecked: it must not be sent.
	p.MustElement(`.ed-modal--credentials [data-ed-credentials-snapshot=".flows-v8-backup-def456"]`).MustClick()
	s.shot("credentials-dialog")
	p.MustElement(`.ed-modal--credentials [data-ed-action="purge"]`).MustClick()
	s.wait(`()=>{const s=document.querySelector('.ed-modal--credentials [data-ed-credentials-status]');return !!s && s.textContent.includes(t('easydrag.ui.cred_purge_clean'))}`)
	s.shot("credentials-done")
	got := p.MustEval(`()=>JSON.stringify(edCredFixture.state.requests.filter(r=>r.route==='purge'&&r.method==='POST').map(r=>r.body.snapshots))`).Str()
	if got != `[[".flows-v7-backup-abc123"]]` {
		t.Fatalf("purge request snapshots = %s, want only the checked one", got)
	}
	p.MustElement(`.ed-modal--credentials [data-ed-action="close"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--credentials')&&document.querySelector('[data-ed-credentials-hint]').hidden`)
	s.noRawKeys("credentials purge dialog")
}

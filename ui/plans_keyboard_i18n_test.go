package ui

import (
	"strings"
	"testing"
)

func TestPlansKnowledgeKeyboardConfirmAndPDFModule(t *testing.T) {
	t.Parallel()

	knowledgeHTML := readEmbeddedText(t, "knowledge.html")
	if strings.Contains(knowledgeHTML, `<script src="/js/vendor/pdf.min.js`) {
		t.Fatal("pdf.min.js is an ES-module shim and must not be loaded as a classic script")
	}
	if strings.Contains(knowledgeHTML, `<label class="btn btn-primary kc-upload-btn"`) ||
		strings.Contains(knowledgeHTML, `<label class="btn btn-primary kc-empty-cta"`) {
		t.Fatal("knowledge upload controls must be real buttons, not labels around hidden inputs")
	}
	knowledgeJS := readEmbeddedText(t, "js/knowledge/main.js")
	if !strings.Contains(knowledgeJS, "import('/js/vendor/pdf.min.js')") {
		t.Fatal("knowledge preview must import pdf.min.js as a module")
	}
	if strings.Contains(knowledgeJS, "pdf.worker.min.js") {
		t.Fatal("knowledge must not override the shim's versioned workerSrc")
	}

	plansJS := readEmbeddedText(t, "js/plans/main.js")
	for _, want := range []string{
		`<button type="button" class="plan-row`,
		`action === 'archive' && !(await showConfirm(`,
		`action === 'cancelled' && !(await showConfirm(`,
	} {
		if !strings.Contains(plansJS, want) {
			t.Fatalf("plans/main.js missing %q", want)
		}
	}
}

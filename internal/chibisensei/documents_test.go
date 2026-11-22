package chibisensei

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func docKey(err error) string {
	var ve *ValidationError
	if errors.As(err, &ve) && len(ve.Issues) > 0 {
		return ve.Issues[0].MessageKey
	}
	return fmt.Sprint(err)
}

func TestAddDocumentRules(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, err := st.CreateProject(ctx, ProjectInput{Name: "Bot", Recipe: RecipeAssistant})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddDocument(ctx, p.ID, "Documents/a.pdf", []byte("x")); docKey(err) != "chibi_sensei.document.unsupported_file" {
		t.Fatalf("pdf: %v", err)
	}
	if _, err := st.AddDocument(ctx, p.ID, "Documents/a.txt", []byte{0xff}); docKey(err) != "chibi_sensei.document.not_utf8" {
		t.Fatalf("utf8: %v", err)
	}
	d, err := st.AddDocument(ctx, p.ID, "Documents/faq.md", append([]byte{0xEF, 0xBB, 0xBF}, []byte("héllo")...))
	if err != nil || d.Chars != 5 || d.Name != "faq.md" {
		t.Fatalf("add: %+v %v", d, err)
	}
	for i := 1; i < MaxDocuments; i++ {
		if _, err := st.AddDocument(ctx, p.ID, fmt.Sprintf("Documents/%d.txt", i), []byte("text")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.AddDocument(ctx, p.ID, "Documents/more.txt", []byte("text")); docKey(err) != "chibi_sensei.document.too_many_documents" {
		t.Fatalf("21st: %v", err)
	}
	list, err := st.ListDocuments(ctx, p.ID)
	if err != nil || len(list) != MaxDocuments || list[0].Text != "" {
		t.Fatalf("list: %d %v", len(list), err)
	}
	got, err := st.GetDocuments(ctx, p.ID, []string{d.ID})
	if err != nil || got[0].Text != "héllo" {
		t.Fatalf("get: %+v %v", got, err)
	}
	other, _ := st.CreateProject(ctx, ProjectInput{Name: "Other", Recipe: RecipeAssistant})
	if _, err := st.GetDocuments(ctx, other.ID, []string{d.ID}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign id: %v", err)
	}
	if err := st.DeleteDocument(ctx, p.ID, d.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, p.ID, d.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

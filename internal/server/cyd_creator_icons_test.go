package server

import (
	"aurago/internal/cydcreator"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestCYDCreatorIconAPI(t *testing.T) {
	f := newCYDTestServer(t)
	old := uiFiles
	uiFiles = os.DirFS("../../ui")
	t.Cleanup(func() { uiFiles = old })
	var catalog cydcreator.IconCatalog
	f.json(t, f.read, http.MethodGet, "icons", "", nil, http.StatusOK, &catalog)
	if len(catalog.Motifs) != 60 || len(catalog.Styles) != 4 {
		t.Fatal("incomplete catalog")
	}
	p := f.createProject(t, "Icons")
	path := "projects/" + p.ID + "/icons"
	body := `{"style":"modern","tone":"mono","motif":"wifi","size":32,"animated":true}`
	wantCYDError(t, f.do(f.admin, http.MethodPost, path, strings.NewReader(body), nil), http.StatusBadRequest, cydcreator.CodeBadRequest)
	var result cydcreator.IconImport
	f.json(t, f.admin, http.MethodPost, path, body, map[string]string{"If-Match": "1"}, http.StatusOK, &result)
	if result.Asset.Encoding != "a8" || len(result.Frames) != 6 || result.Version != 2 {
		t.Fatalf("bad import: %+v", result)
	}
	wantCYDError(t, f.do(f.admin, http.MethodPost, path, strings.NewReader(body), map[string]string{"If-Match": "1"}), http.StatusConflict, cydcreator.CodeProjectConflict)
	wantCYDError(t, f.do(f.admin, http.MethodPost, path, strings.NewReader(strings.Replace(body, `"size":32`, `"size":33`, 1)), map[string]string{"If-Match": "2"}), http.StatusBadRequest, cydcreator.CodeBadRequest)
}

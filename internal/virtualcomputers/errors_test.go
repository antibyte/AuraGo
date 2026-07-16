package virtualcomputers

import (
	"net/http"
	"strings"
	"testing"
)

func TestClassifyErrorExplainsUnregisteredVolumeRoutes(t *testing.T) {
	err := RESTError{
		Method:     http.MethodPost,
		Path:       "/v1/volumes",
		StatusCode: http.StatusNotFound,
		Body:       "404 page not found",
	}
	classified := ClassifyError(err)
	if classified.Code != "storage_unavailable" || classified.HTTPStatus != http.StatusServiceUnavailable {
		t.Fatalf("classified error = %+v", classified)
	}
	for _, want := range []string{"S3", "Install / Repair"} {
		if !strings.Contains(classified.Message, want) {
			t.Errorf("message %q missing %q", classified.Message, want)
		}
	}
	if strings.Contains(classified.Message, "404 page not found") {
		t.Fatalf("message still exposes upstream router response: %q", classified.Message)
	}
}

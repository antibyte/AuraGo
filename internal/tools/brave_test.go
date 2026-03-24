package tools

import "testing"

func TestBraveFormatAPIError_InvalidToken(t *testing.T) {
	body := []byte(`{"error":{"code":"SUBSCRIPTION_TOKEN_INVALID","detail":"The provided subscription token is invalid.","status":422},"type":"ErrorResponse"}`)
	got := braveFormatAPIError(422, body)
	want := "Brave Search API key is invalid. Update the Brave Search subscription token in the vault/settings."
	if got != want {
		t.Fatalf("braveFormatAPIError() = %q, want %q", got, want)
	}
}

func TestBraveFormatAPIError_GenericDetail(t *testing.T) {
	body := []byte(`{"error":{"code":"BAD_REQUEST","detail":"Unsupported language.","status":422},"type":"ErrorResponse"}`)
	got := braveFormatAPIError(422, body)
	want := "Brave Search error BAD_REQUEST: Unsupported language."
	if got != want {
		t.Fatalf("braveFormatAPIError() = %q, want %q", got, want)
	}
}

func TestBraveFormatAPIError_Fallback(t *testing.T) {
	got := braveFormatAPIError(500, []byte(`not json`))
	want := "Brave Search HTTP error 500"
	if got != want {
		t.Fatalf("braveFormatAPIError() = %q, want %q", got, want)
	}
}

func TestBraveNormalizeSearchLang(t *testing.T) {
	if got := braveNormalizeSearchLang("de"); got != "de" {
		t.Fatalf("braveNormalizeSearchLang(de) = %q, want de", got)
	}
	if got := braveNormalizeSearchLang("de-DE"); got != "de" {
		t.Fatalf("braveNormalizeSearchLang(de-DE) = %q, want de", got)
	}
	if got := braveNormalizeSearchLang("zh"); got != "zh-hans" {
		t.Fatalf("braveNormalizeSearchLang(zh) = %q, want zh-hans", got)
	}
	if got := braveNormalizeSearchLang("zh-TW"); got != "zh-hant" {
		t.Fatalf("braveNormalizeSearchLang(zh-TW) = %q, want zh-hant", got)
	}
	if got := braveNormalizeSearchLang("ja"); got != "" {
		t.Fatalf("braveNormalizeSearchLang(ja) = %q, want empty string", got)
	}
	if got := braveNormalizeSearchLang("pt"); got != "" {
		t.Fatalf("braveNormalizeSearchLang(pt) = %q, want empty string", got)
	}
}

func TestBraveNormalizeUILang(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "de", want: "de-DE"},
		{in: "en", want: "en-US"},
		{in: "de-DE", want: "de-DE"},
		{in: "pt", want: "pt-BR"},
		{in: "sv", want: "sv-SE"},
		{in: "cs", want: ""},
	}
	for _, tt := range tests {
		if got := braveNormalizeUILang(tt.in); got != tt.want {
			t.Fatalf("braveNormalizeUILang(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

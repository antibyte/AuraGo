package agent

import "testing"

func TestAnsibleExtraVariablesRejectMalformedBody(t *testing.T) {
	for _, body := range []string{"{", "[]", "null", "false", `"@/fixture/private"`} {
		if _, err := (ansibleArgs{Body: body}).extraVars(); err == nil {
			t.Fatalf("malformed extra variables silently dropped: %s", body)
		}
	}
	for _, body := range []string{"", "  ", `{}`, `{"release":"1.2"}`} {
		if _, err := (ansibleArgs{Body: body}).extraVars(); err != nil {
			t.Fatalf("valid application data rejected: %s: %v", body, err)
		}
	}
}

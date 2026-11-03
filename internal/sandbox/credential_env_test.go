package sandbox

import (
	"reflect"
	"testing"
)

func TestFilterEnvRemovesAWSAccessKeyID(t *testing.T) {
	input := []string{"AWS_ACCESS_KEY_ID=id-sentinel", "aws_access_key_id=lower-sentinel",
		"AWS_SECRET_ACCESS_KEY=secret-sentinel", "AWS_SESSION_TOKEN=session-sentinel",
		"USER_ID=1000", "AWS_REGION=eu-central-1", "PATH=/usr/bin", "LANG=C"}
	want := input[4:]
	if got := FilterEnv(input); !reflect.DeepEqual(got, want) {
		t.Fatal("FilterEnv did not remove the complete AWS credential tuple or changed safe values")
	}
}

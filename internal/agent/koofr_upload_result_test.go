package agent

import "testing"

func TestKoofrUploadResultMissingVerificationDetectsLegacySuccess(t *testing.T) {
	result := `Tool Output: {"status":"success"}`
	if !koofrUploadResultMissingVerification(result) {
		t.Fatal("legacy Koofr upload success should require redeploy/verification")
	}
}

func TestKoofrUploadResultMissingVerificationAllowsVerifiedSuccess(t *testing.T) {
	result := `Tool Output: {"status":"success","bytes":12,"remote_directory":"/aurgo/pictures","filename":"cat.jpeg"}`
	if koofrUploadResultMissingVerification(result) {
		t.Fatal("verified Koofr upload success should be accepted")
	}
}

package memory

import (
	"fmt"
	"regexp"

	chromem "github.com/philippgille/chromem-go"
)

// Property names are data, not an exemption from the credential policy. Drop
// sensitive fields from a copy before serialization, preserving canonical SQL.
var kgSensitiveProperty = regexp.MustCompile(`(?i)(?:^|[^a-z])(?:key|secret|password|passwd|passwort|pwd|pin|token|auth|credential|api_key|master_key|bot_token)(?:$|[^a-z])|(?:apiKey|accessToken|privateKey|clientSecret)$`)

func semanticSafeProperties(properties map[string]string) map[string]string {
	safe := make(map[string]string, len(properties))
	for key, value := range properties {
		if kgSensitiveProperty.MatchString(key) || HasDurableCredential(key+": "+value) {
			continue
		}
		safe[key] = value
	}
	return safe
}

// The final guard also covers labels, IDs, relationships and metadata. A
// letters-only password or short PIN is still a credential.
func kgSemanticDocumentAllowed(doc chromem.Document) error {
	if HasDurableCredential(doc.Content) {
		return fmt.Errorf("%w: credentials require encrypted Vault storage", ErrAutomaticMemorySkipped)
	}
	for key, value := range doc.Metadata {
		if HasDurableCredential(key + "=" + value) {
			return fmt.Errorf("%w: credentials require encrypted Vault storage", ErrAutomaticMemorySkipped)
		}
	}
	return nil
}

package memory

import (
	"errors"
	"fmt"
	"regexp"

	"aurago/internal/security"

	chromem "github.com/philippgille/chromem-go"
)

// ErrAutomaticMemorySkipped identifies content that must not be retried or restored.
var ErrAutomaticMemorySkipped = errors.New("automatic memory skipped")

var (
	// Generic JSON/Vault/S3 keys identify data, not a credential value. Explicit
	// credential fields and recognizable key formats remain checked below.
	durableObjectKey            = regexp.MustCompile(`(?i)\bkey(["']?\s*[:=])`)
	durableTokenOption          = regexp.MustCompile(`(?i)\binject_token(["']?\s*[:=]\s*["']?)(true|false)\b`)
	durableTemplateValue        = regexp.MustCompile(`(?i)\b([a-z][a-z0-9_]*)(["']?\s*[:=]\s*)(["']?)(your[-_][a-z0-9_-]+|\$\{[^}\r\n]+\}|<[^>\r\n]+>)(["']?)`)
	durableEnvironmentReference = regexp.MustCompile(`(?i)\b[a-z][a-z0-9_]*(["']?\s*[:=]\s*)(?:os\.(?:Getenv|LookupEnv|environ(?:\.get)?)|process\.env)\b`)
	durableKeyMaterial          = regexp.MustCompile(`(?:-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----|\bsk-(?:proj-)?[A-Za-z0-9_-]{16,}|\bgh[pousr]_[A-Za-z0-9]{20,}|\bgithub_pat_[A-Za-z0-9_]{20,}|\bAKIA[A-Z0-9]{16}|\btskey-(?:api|auth)-[A-Za-z0-9_-]{16,})`)
)

// HasDurableCredential detects credential values, independently of the more
// aggressive logger redactor. Registered secrets always win over placeholders.
func HasDurableCredential(text string) bool {
	if security.Scrub(text) != text || durableKeyMaterial.MatchString(text) {
		return true
	}
	candidate := durableObjectKey.ReplaceAllString(text, `_memory_object_key${1}`)
	candidate = durableTokenOption.ReplaceAllString(candidate, `_memory_option${1}${2}`)
	candidate = durableTemplateValue.ReplaceAllString(candidate, `${1}${2}${3}[redacted]${5}`)
	candidate = durableEnvironmentReference.ReplaceAllString(candidate, `_memory_reference${1}environment`)
	return security.RedactSensitiveInfo(candidate) != candidate
}

func validateDurableDocument(concept, content string) error {
	if HasDurableCredential(concept + "\n" + content) {
		return fmt.Errorf("%w: credentials require encrypted Vault storage", ErrAutomaticMemorySkipped)
	}
	return nil
}

func validateMemoryDocument(concept, content string) error {
	if len(content) > maxBatchItemBytes {
		return fmt.Errorf("memory content exceeds %d byte limit", maxBatchItemBytes)
	}
	return validateDurableDocument(concept, content)
}

func validateNewVectorDocuments(docs []chromem.Document) error {
	for _, doc := range docs {
		if err := validateDurableDocument("", doc.Content); err != nil {
			// Only byte-identical shipped guide chunks may contain reviewed credential
			// demonstrations. User overrides and arbitrary collections gain no grant.
			if security.Scrub(doc.Content) != doc.Content || !knownBundledToolGuideDocument(doc) {
				return err
			}
		}
		for key, value := range doc.Metadata {
			if err := validateDurableDocument("", key+"="+value); err != nil {
				return err
			}
		}
	}
	return nil
}

// CanDeleteWithoutEmbeddings exposes source-only deletion to privacy callers;
// other VectorDB implementations retain their existing readiness policy.
func (cv *ChromemVectorDB) CanDeleteWithoutEmbeddings() bool {
	return cv != nil && cv.corpus != nil && cv.corpus.db != nil && !cv.closed.Load()
}

func (cv *ChromemVectorDB) requireReadyForDelete() error {
	if cv.CanDeleteWithoutEmbeddings() {
		return nil
	}
	return cv.requireReadyForStore()
}

package decisions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

const (
	modelRepository = "jersobh/jeff-2b-gguf"
	modelRevision   = "263ac27c59688a360d2db255c4f7e57f665bcca4"
	modelID         = "jeff-2b-gguf"
)

// Artifact is one immutable public model artifact, pinned by revision, exact
// byte size, and SHA-256. Artifact URLs are derived only from the fixed repo
// and revision rather than accepted from configuration.
type Artifact struct {
	Name     string
	Filename string
	Revision string
	Size     int64
	SHA256   string
}

// Manifest pins both files required by the multimodal decision model.
type Manifest struct {
	Repository string
	Model      Artifact
	Projector  Artifact
}

// DefaultManifest returns the immutable JEFF model and projector pins.
func DefaultManifest() Manifest {
	return Manifest{
		Repository: modelRepository,
		Model: Artifact{
			Name: "model", Filename: "jeff-2B.Q4_K_M.gguf", Revision: modelRevision,
			Size: 1274396448, SHA256: "d4d9fbcf40345a1677dc92596c7c1e157fac5cba98e2a1b14ec92db94c67d46b",
		},
		Projector: Artifact{
			Name: "projector", Filename: "jeff-2B.mmproj.gguf", Revision: modelRevision,
			Size: 671372608, SHA256: "1601a51a07942318aacb7a7298847fe505f8beb592be37f538ab85577772bb68",
		},
	}
}

func (manifest Manifest) Validate() error {
	if manifest.Repository != modelRepository {
		return fmt.Errorf("unsupported model repository")
	}
	if err := validateArtifact(manifest.Model); err != nil {
		return fmt.Errorf("validate decision model pin: %w", err)
	}
	if err := validateArtifact(manifest.Projector); err != nil {
		return fmt.Errorf("validate decision projector pin: %w", err)
	}
	if manifest.Model.Revision != manifest.Projector.Revision {
		return fmt.Errorf("model and projector revisions differ")
	}
	return nil
}

func validateArtifact(artifact Artifact) error {
	if artifact.Name == "" || artifact.Filename == "" || artifact.Revision != modelRevision || artifact.Size <= 0 {
		return fmt.Errorf("incomplete or unsupported artifact pin")
	}
	if strings.ContainsAny(artifact.Filename, `/\\`) || artifact.Filename == "." || artifact.Filename == ".." {
		return fmt.Errorf("artifact filename is not a single path component")
	}
	hash, err := hex.DecodeString(artifact.SHA256)
	if err != nil || len(hash) != sha256.Size {
		return fmt.Errorf("artifact SHA-256 pin is invalid")
	}
	return nil
}

func artifactURL(manifest Manifest, artifact Artifact) string {
	return "https://huggingface.co/" + manifest.Repository + "/resolve/" + artifact.Revision + "/" + url.PathEscape(artifact.Filename)
}

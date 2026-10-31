package services

import (
	"context"
	"io"
	"os"
	"strings"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/decisions/consumers"
)

// IndexerDecisionService is the part of the Decisions hub the file indexer uses.
type IndexerDecisionService interface {
	Snapshot(cfg *config.Config, id consumers.ID) consumers.Snapshot
	RefreshImageCapability(ctx context.Context, cfg *config.Config, id consumers.ID) consumers.Snapshot
	PrepareImage(data []byte, maxSide int) (decisions.Image, error)
}

var indexerKindOptions = []decisions.Option{
	{ID: "photo", Label: "Photo of a real scene"},
	{ID: "screenshot", Label: "Screenshot of a screen or app"},
	{ID: "document", Label: "Scanned or photographed document"},
	{ID: "diagram", Label: "Diagram, chart or drawing"},
	{ID: "other", Label: "Something else"},
}

// indexerKindLabels are the German labels of the coarse Agnes entry.
var indexerKindLabels = map[string]string{
	"photo": "Foto", "screenshot": "Screenshot", "document": "Dokument",
	"diagram": "Diagramm oder Zeichnung", "other": "Sonstiges Bild",
}

const indexerDecisionMaxBytes = 32 << 20

// indexerImageClass is the optional Decisions verdict about one image.
type indexerImageClass struct {
	Kind    string // empty: no confident kind
	HasText string // "ja", "nein" or "" when unsure
}

// SetDecisionService attaches the optional Decisions service; nil detaches it.
func (fi *FileIndexer) SetDecisionService(service IndexerDecisionService) {
	fi.decisionMu.Lock()
	fi.decisions = service
	fi.decisionMu.Unlock()
}

func (fi *FileIndexer) decisionService() IndexerDecisionService {
	fi.decisionMu.RLock()
	defer fi.decisionMu.RUnlock()
	return fi.decisions
}

// decisionsImagesConfigured is the configuration-only check behind the fingerprint
// suffix and the Agnes keep-state rule. It must not depend on runtime state
// (catalog age, runtime readiness, budget), or the corpus would re-index whenever
// that state flips.
func (fi *FileIndexer) decisionsImagesConfigured(cfg *config.Config, allowMedia bool) bool {
	if fi.decisionService() == nil || !allowMedia || cfg == nil {
		return false
	}
	if !cfg.Decisions.Enabled || !cfg.DecisionConsumers.IndexerImages.Enabled {
		return false
	}
	return cfg.Decisions.Normalize().Backend == config.DecisionsBackendLocal || cfg.DecisionConsumers.IndexerImages.AllowExternalImages
}

// classifyImage asks Decisions about one image of a folder that allows media
// analysis. Any failure or missing capability returns the zero value, which keeps
// today's behaviour.
func (fi *FileIndexer) classifyImage(ctx context.Context, cfg *config.Config, path string, allowMedia bool) indexerImageClass {
	service := fi.decisionService()
	if service == nil || !allowMedia || cfg == nil {
		return indexerImageClass{}
	}
	snapshot := service.Snapshot(cfg, consumers.IndexerImages)
	if snapshot.Code == consumers.CodeImagesUnknown && snapshot.Backend == "provider" {
		snapshot = service.RefreshImageCapability(ctx, cfg, consumers.IndexerImages)
	}
	if !snapshot.Available || !snapshot.ImagesAllowed || snapshot.Evaluate == nil {
		return indexerImageClass{}
	}
	file, err := os.Open(path)
	if err != nil {
		return indexerImageClass{}
	}
	raw, err := io.ReadAll(io.LimitReader(file, indexerDecisionMaxBytes+1))
	file.Close()
	if err != nil || len(raw) == 0 || len(raw) > indexerDecisionMaxBytes {
		return indexerImageClass{}
	}
	image, err := service.PrepareImage(raw, 448)
	if err != nil {
		return indexerImageClass{}
	}
	ctx, cancel := context.WithTimeout(ctx, consumers.Registry[consumers.IndexerImages].Timeout)
	defer cancel()
	response, err := snapshot.Evaluate(ctx, decisions.Request{
		State: consumers.NewState().String(),
		Questions: []decisions.Question{
			{ID: "kind", Type: "choice", Instructions: "What kind of image is this? Use other when unsure.", Options: indexerKindOptions},
			{ID: "text", Type: "noul", Instructions: "Does the image contain readable text?"},
		},
		Images: []decisions.Image{image},
	})
	if err != nil {
		return indexerImageClass{}
	}
	var class indexerImageClass
	if kind, ok := consumers.Choice(response, "kind", indexerKindOptions, 0.50, 0.15); ok {
		class.Kind = kind
	}
	if p, ok := consumers.YesNo(response, "text"); ok {
		switch {
		case p >= 0.65:
			class.HasText = "ja"
		case p <= 0.35:
			class.HasText = "nein"
		}
	}
	return class
}

// imagePromptHint is the German sentence the kind adds to the vision prompt.
func imagePromptHint(kind string) string {
	switch kind {
	case "screenshot", "document":
		return " Gib sichtbaren Text so wörtlich wie möglich wieder."
	case "photo":
		return " Beschreibe dargestellte Personen, Orte und Gegenstände."
	}
	return ""
}

// coarseImageEntry is stored instead of removing an image the vision provider
// cannot read (Agnes needs a public URL); empty when Decisions had no verdict.
func coarseImageEntry(name, relPath string, class indexerImageClass) string {
	label, ok := indexerKindLabels[class.Kind]
	if !ok {
		return ""
	}
	hasText := class.HasText
	if hasText == "" {
		hasText = "unklar"
	}
	return "Bild " + strings.TrimSpace(name) + " (Pfad: " + relPath + "): " + label + ", enthält Text: " + hasText
}

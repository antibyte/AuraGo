package flows

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"aurago/internal/security"
)

const (
	LocalPackFormat        = "aurago.easydrag.local-pack"
	LocalPackVersion       = 1
	MaxLocalPackBytes      = 32 << 20
	maxLocalPackTotalBytes = MaxDocumentBytes + maxBlockBytes
)

var (
	ErrLocalPackTooLarge    = errors.New("local flow pack exceeds the size limit")
	ErrLocalPackInvalid     = errors.New("invalid local flow pack")
	ErrLocalPackUnsupported = errors.New("local flow pack contains unsupported content")
	ErrLocalPackConflict    = errors.New("local flow pack conflicts with current data")
)

// LocalPack is a versioned, machine-local export. Its hashes are integrity checks, not signatures.
type LocalPack struct {
	Format  string           `json:"format"`
	Version int              `json:"version"`
	Parent  LocalPackParent  `json:"parent"`
	Blocks  []LocalPackBlock `json:"blocks"`
}

type LocalPackParent struct {
	SourceRevision int    `json:"source_revision"`
	Hash           string `json:"hash"`
	Flow           *Flow  `json:"flow"`
}

type LocalPackBlock struct {
	SourceID       string `json:"source_id"`
	SourceRevision int    `json:"source_revision"`
	Hash           string `json:"hash"`
	Flow           *Flow  `json:"flow"`
}

type LocalPackBlockTarget struct {
	SourceID       string `json:"source_id"`
	SourceRevision int    `json:"source_revision"`
	LocalID        string `json:"local_id"`
	Hash           string `json:"hash"`
}

// LocalPackProposal contains only the identity and hash manifest submitted back to Import.
type LocalPackProposal struct {
	PackDigest string                 `json:"pack_digest"`
	Digest     string                 `json:"proposal_digest"`
	ParentID   string                 `json:"parent_id"`
	ParentHash string                 `json:"parent_hash"`
	Blocks     []LocalPackBlockTarget `json:"blocks"`
}

type LocalPackReviewItem struct {
	Name           string `json:"name"`
	SourceID       string `json:"source_id"`
	SourceRevision int    `json:"source_revision"`
}

type LocalPackReview struct {
	Parent            LocalPackReviewItem   `json:"parent"`
	Blocks            []LocalPackReviewItem `json:"blocks"`
	SampleDataCleared bool                  `json:"sample_data_cleared"`
	LocalOnly         bool                  `json:"local_only"`
}

type LocalPackPreview struct {
	Proposal LocalPackProposal `json:"proposal"`
	Review   LocalPackReview   `json:"review"`
}

type LocalPackImportRequest struct {
	Pack     LocalPack         `json:"pack"`
	Proposal LocalPackProposal `json:"proposal"`
}

type LocalPackReconcileRequest struct {
	Proposal LocalPackProposal `json:"proposal"`
}

type LocalPackImportResult struct {
	Status string            `json:"status"`
	Flow   *LocalPackRecord  `json:"flow"`
	Blocks []LocalPackRecord `json:"blocks"`
}

type LocalPackReconcileResult struct {
	Status string            `json:"status"`
	Flow   *LocalPackRecord  `json:"flow,omitempty"`
	Blocks []LocalPackRecord `json:"blocks"`
}

type LocalPackPin struct {
	BlockID  string `json:"block_id"`
	Revision int    `json:"revision"`
	Hash     string `json:"hash"`
}

// LocalPackRecord is a bounded acknowledgement; it never echoes a Flow document.
type LocalPackRecord struct {
	ID                     string         `json:"id"`
	Kind                   Kind           `json:"kind"`
	Name                   string         `json:"name"`
	MissionID              string         `json:"mission_id,omitempty"`
	DraftRevision          int            `json:"draft_revision"`
	LiveRevision           int            `json:"live_revision"`
	PublishedDraftRevision int            `json:"published_draft_revision"`
	Hash                   string         `json:"hash"`
	Pins                   []LocalPackPin `json:"pins"`
}

type localPackCleaner interface {
	DeleteCreatedFlowMission(missionID, expectedFlowID string) error
}

type localPackPrepared struct {
	digest    string
	parent    *Flow
	blocks    []resolvedBlock // source revisions, dependency-first
	byRef     map[blockRef]resolvedBlock
	parentRev int
}

type localPackMapped struct {
	parent *Flow
	blocks []resolvedBlock // local revision 1, dependency-first
	byRef  map[blockRef]resolvedBlock
}

func localPackHash(v any) (string, error) { return extractionHash(v) }

func localPackProposalDigest(p LocalPackProposal) (string, error) {
	p.Digest = ""
	return localPackHash(p)
}

func validLocalPackID(id string) bool { return extractionBlockID.MatchString(id) }

func newLocalPackID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "flow_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw[:])), nil
}

func localPackDocBytes(doc *Flow) ([]byte, *Flow, error) {
	if doc == nil {
		return nil, nil, ErrLocalPackInvalid
	}
	raw, err := doc.Marshal()
	if err != nil {
		return nil, nil, ErrLocalPackInvalid
	}
	if len(raw) > MaxDocumentBytes {
		return nil, nil, ErrLocalPackTooLarge
	}
	parsed, err := ParseFlow(raw)
	if err != nil {
		return nil, nil, ErrLocalPackInvalid
	}
	canonical, err := parsed.Marshal()
	if err != nil || !bytes.Equal(raw, canonical) {
		return nil, nil, ErrLocalPackInvalid
	}
	return raw, parsed, nil
}

func prepareLocalPack(ctx context.Context, pack LocalPack) (*localPackPrepared, error) {
	if pack.Format != LocalPackFormat || pack.Version != LocalPackVersion || pack.Parent.SourceRevision < 1 || pack.Parent.Flow == nil || pack.Blocks == nil || len(pack.Blocks) > maxBlockVersions {
		return nil, ErrLocalPackInvalid
	}
	parentBytes, parent, err := localPackDocBytes(pack.Parent.Flow)
	if err != nil {
		return nil, err
	}
	if parent.Schema != SchemaVersion || parent.ID == "" || parent.Kind != KindFlow || !validExtractionHash(pack.Parent.Hash) {
		return nil, ErrLocalPackInvalid
	}
	if !localPackJSONControlsAllowed(parent) {
		return nil, ErrLocalPackInvalid
	}
	parentHash, _ := localPackHash(parent)
	if parentHash != pack.Parent.Hash {
		return nil, ErrLocalPackConflict
	}
	total := len(parentBytes)
	byRef := make(map[blockRef]resolvedBlock, len(pack.Blocks))
	for _, item := range pack.Blocks {
		if item.Flow == nil || item.SourceID == "" || !blockFlowID.MatchString(item.SourceID) || item.SourceRevision < 1 || item.Flow.ID != item.SourceID || item.Flow.Kind != KindBlock || item.Flow.Schema != SchemaVersion || !validExtractionHash(item.Hash) {
			return nil, ErrLocalPackInvalid
		}
		raw, doc, err := localPackDocBytes(item.Flow)
		if err != nil {
			return nil, err
		}
		total += len(raw)
		if total > maxLocalPackTotalBytes {
			return nil, ErrLocalPackTooLarge
		}
		hash, _ := localPackHash(doc)
		if hash != item.Hash {
			return nil, ErrLocalPackConflict
		}
		if !localPackJSONControlsAllowed(doc) {
			return nil, ErrLocalPackInvalid
		}
		ref := blockRef{FlowID: item.SourceID, Revision: item.SourceRevision}
		if _, exists := byRef[ref]; exists {
			return nil, ErrLocalPackInvalid
		}
		if !blockDocumentAllowed(doc) {
			return nil, ErrLocalPackUnsupported
		}
		byRef[ref] = resolvedBlock{Ref: ref, Document: doc, Hash: sha256.Sum256(raw)}
	}
	packBytes, err := json.Marshal(pack)
	if err != nil {
		return nil, ErrLocalPackInvalid
	}
	if len(packBytes) > MaxLocalPackBytes {
		return nil, ErrLocalPackTooLarge
	}
	if total > maxLocalPackTotalBytes {
		return nil, ErrLocalPackTooLarge
	}
	if !localPackFlowAllowed(parent) {
		return nil, ErrLocalPackUnsupported
	}
	if err := localPackSecretExpressions(parent); err != nil {
		return nil, err
	}
	for _, block := range byRef {
		if err := localPackSecretExpressions(block.Document); err != nil {
			return nil, err
		}
	}
	if !manualSamplesCleared(parent) {
		return nil, ErrLocalPackUnsupported
	}
	loaded := func(_ context.Context, id string, revision int) (*Flow, error) {
		block, ok := byRef[blockRef{FlowID: id, Revision: revision}]
		if !ok {
			return nil, ErrBlockUnavailable
		}
		return block.Document, nil
	}
	closure, err := resolveBlockDependencies(ctx, parent, loaded)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, ErrLocalPackUnsupported
	}
	if len(closure) != len(byRef) {
		return nil, ErrLocalPackInvalid
	}
	for _, block := range closure {
		if _, ok := byRef[block.Ref]; !ok {
			return nil, ErrLocalPackInvalid
		}
		if issues := validateBlockDocument(block.Document); HasErrors(issues) {
			return nil, ErrLocalPackInvalid
		}
	}
	return &localPackPrepared{digest: extractionDigestForPack(packBytes), parent: parent, blocks: closure, byRef: byRef, parentRev: pack.Parent.SourceRevision}, nil
}

func extractionDigestForPack(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func localPackFlowAllowed(doc *Flow) bool {
	if doc == nil || doc.Kind != KindFlow || len(doc.Nodes) > MaxNodes || len(doc.Edges) > MaxEdges {
		return false
	}
	for _, node := range doc.Nodes {
		if node.TypeVersion != 1 {
			return false
		}
		switch node.Type {
		case TypeTriggerManual, TypeSet, TypeIf, TypeSwitch, TypeMerge, blockCallType:
		default:
			return false
		}
		if node.Type == blockCallType && node.Settings.Retry.Count != 0 {
			return false
		}
	}
	return true
}

func manualSamplesCleared(doc *Flow) bool {
	for _, node := range doc.Nodes {
		if node.Type != TypeTriggerManual {
			continue
		}
		if raw, ok := node.Params["data"]; ok && raw != nil {
			data, ok := raw.(map[string]any)
			if !ok || len(data) != 0 {
				return false
			}
		}
	}
	return true
}

func localPackSecretExpressions(doc *Flow) error {
	for i := range doc.Nodes {
		refs, problems := CollectTemplateRefs(doc.Nodes[i].Params)
		if len(problems) != 0 {
			return ErrLocalPackInvalid
		}
		for _, ref := range refs {
			if ref.Expr != nil && ref.Expr.Root == "secrets" {
				return ErrLocalPackUnsupported
			}
		}
	}
	return nil
}

func localPackJSONControlsAllowed(doc *Flow) bool {
	data, err := doc.Marshal()
	if err != nil {
		return false
	}
	var value any
	if json.Unmarshal(data, &value) != nil {
		return false
	}
	var safe func(any) bool
	safe = func(value any) bool {
		switch item := value.(type) {
		case string:
			for _, r := range item {
				if r < 0x20 && r != '\t' && r != '\n' && r != '\r' || r >= 0x7f && r <= 0x9f {
					return false
				}
			}
		case []any:
			for _, child := range item {
				if !safe(child) {
					return false
				}
			}
		case map[string]any:
			for key, child := range item {
				if !safe(key) || !safe(child) {
					return false
				}
			}
		}
		return true
	}
	return safe(value)
}

func scrubLocalPackValue(value any, depth int) (any, error) {
	if depth > maxParamDepth {
		return nil, ErrLocalPackInvalid
	}
	switch v := value.(type) {
	case string:
		return security.ScrubSecrets(v), nil
	case []any:
		out := make([]any, len(v))
		for i := range v {
			item, err := scrubLocalPackValue(v[i], depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = item
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, raw := range v {
			cleanKey := security.ScrubSecrets(key)
			if _, exists := out[cleanKey]; exists {
				return nil, ErrLocalPackInvalid
			}
			item, err := scrubLocalPackValue(raw, depth+1)
			if err != nil {
				return nil, err
			}
			out[cleanKey] = item
		}
		return out, nil
	default:
		return value, nil
	}
}

func sanitizeLocalPackFlow(source *Flow, clearSample bool) (*Flow, error) {
	redacted, err := RedactCredentials(source)
	if err != nil || redacted == nil {
		return nil, ErrLocalPackInvalid
	}
	for i := range redacted.Nodes {
		node := &redacted.Nodes[i]
		node.Label = security.ScrubSecrets(node.Label)
		node.Settings.Notes = security.ScrubSecrets(node.Settings.Notes)
		if clearSample && node.Type == TypeTriggerManual {
			node.Params["data"] = map[string]any{}
		}
		params := make(map[string]any, len(node.Params))
		for key, raw := range node.Params {
			if node.Type == blockCallType && (key == "flow_id" || key == "revision") {
				params[key] = raw
				continue // These typed references are structural identity.
			}
			cleanKey := security.ScrubSecrets(key)
			clean, err := scrubLocalPackValue(raw, 0)
			if err != nil {
				return nil, err
			}
			if _, exists := params[cleanKey]; exists {
				return nil, ErrLocalPackInvalid
			}
			params[cleanKey] = clean
		}
		node.Params = params
	}
	redacted.Name = security.ScrubSecrets(redacted.Name)
	redacted.Description = security.ScrubSecrets(redacted.Description)
	for i := range redacted.Notes {
		redacted.Notes[i].Text = security.ScrubSecrets(redacted.Notes[i].Text)
	}
	if redacted.Groups != nil {
		for i := range *redacted.Groups {
			(*redacted.Groups)[i].Title = security.ScrubSecrets((*redacted.Groups)[i].Title)
			(*redacted.Groups)[i].Note = security.ScrubSecrets((*redacted.Groups)[i].Note)
		}
	}
	return redacted, nil
}

func mustMarshalFlow(doc *Flow) []byte {
	b, _ := doc.Marshal()
	return b
}

func validateLocalPackMapped(ctx context.Context, parent *Flow, blocks []resolvedBlock, byRef map[blockRef]resolvedBlock) error {
	if !localPackFlowAllowed(parent) || !manualSamplesCleared(parent) {
		return ErrLocalPackUnsupported
	}
	for _, block := range blocks {
		if !blockDocumentAllowed(block.Document) {
			return ErrLocalPackUnsupported
		}
	}
	loaded := func(_ context.Context, id string, revision int) (*Flow, error) {
		block, ok := byRef[blockRef{FlowID: id, Revision: revision}]
		if !ok {
			return nil, ErrBlockUnavailable
		}
		return block.Document, nil
	}
	closure, err := resolveBlockDependencies(ctx, parent, loaded)
	if err != nil || len(closure) != len(blocks) {
		return ErrLocalPackInvalid
	}
	byNodePlan := buildLocalBlockPlan(parent, closure)
	if byNodePlan == nil {
		return ErrLocalPackInvalid
	}
	if err := measureBlockPlan(ctx, parent, byNodePlan); err != nil {
		return ErrLocalPackInvalid
	}
	return nil
}

func (s *Service) validateLocalPackSource(ctx context.Context, prepared *localPackPrepared) error {
	if prepared == nil || !localPackFlowAllowed(prepared.parent) || localPackSecretExpressions(prepared.parent) != nil {
		return ErrLocalPackUnsupported
	}
	if HasErrors(Validate(prepared.parent, s.reg, s.vc(ModeDraft))) {
		return ErrLocalPackInvalid
	}
	for _, block := range prepared.blocks {
		if issues := s.ValidateBlock(block.Document, ModePublish); HasErrors(issues) {
			return ErrLocalPackInvalid
		}
	}
	plan := buildLocalBlockPlan(prepared.parent, prepared.blocks)
	if plan == nil {
		return ErrLocalPackInvalid
	}
	if err := measureBlockPlan(ctx, prepared.parent, plan); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return ErrLocalPackInvalid
	}
	return nil
}

func (s *Service) validateLocalPackMapped(ctx context.Context, mapped *localPackMapped) error {
	if mapped == nil || mapped.parent == nil || localPackSecretExpressions(mapped.parent) != nil {
		return ErrLocalPackInvalid
	}
	if HasErrors(Validate(mapped.parent, s.reg, s.vc(ModeDraft))) {
		return ErrLocalPackInvalid
	}
	for _, block := range mapped.blocks {
		if issues := s.ValidateBlock(block.Document, ModePublish); HasErrors(issues) {
			return ErrLocalPackInvalid
		}
	}
	if err := validateLocalPackMapped(ctx, mapped.parent, mapped.blocks, mapped.byRef); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return ErrLocalPackInvalid
	}
	return nil
}

func buildLocalBlockPlan(root *Flow, blocks []resolvedBlock) *blockPlan {
	byRef := make(map[blockRef]resolvedBlock, len(blocks))
	for _, block := range blocks {
		byRef[block.Ref] = block
	}
	var frame func(*Flow) (*blockPlan, bool)
	frame = func(doc *Flow) (*blockPlan, bool) {
		plan := &blockPlan{byNode: make(map[string]resolvedBlock)}
		seen := make(map[blockRef]bool)
		for _, node := range doc.Nodes {
			if node.Type != blockCallType {
				continue
			}
			ref, err := blockReference(node)
			if err != nil {
				return nil, false
			}
			child, ok := byRef[ref]
			if !ok || checkBlockBindings(node, child.Contract) != nil {
				return nil, false
			}
			if child.plan == nil {
				childPlan, ok := frame(child.Document)
				if !ok {
					return nil, false
				}
				child.plan = childPlan
				byRef[ref] = child
			}
			plan.byNode[node.ID] = child
			if child.plan != nil {
				for _, descendant := range child.plan.blocks {
					if !seen[descendant.Ref] {
						seen[descendant.Ref] = true
						plan.blocks = append(plan.blocks, descendant)
					}
				}
			}
			if !seen[ref] {
				seen[ref] = true
				plan.blocks = append(plan.blocks, child)
			}
		}
		return plan, true
	}
	plan, ok := frame(root)
	if !ok || len(plan.blocks) != len(blocks) {
		return nil
	}
	return plan
}

func validateBlockDocument(doc *Flow) []Issue {
	if !blockDocumentAllowed(doc) {
		return []Issue{{Code: IssueNodeTypeUnknown, Severity: SeverityError}}
	}
	issues := Validate(doc, blockRegistry(), ValidateContext{Mode: ModePublish})
	if _, err := readBlockContract(doc); err != nil {
		issues = append(issues, Issue{Code: IssueParamInvalid, Severity: SeverityError})
	}
	return issues
}

func (s *Service) ExportLocalPack(ctx context.Context, flowID string) (LocalPack, error) {
	rec, err := s.requireFlow(ctx, flowID)
	if err != nil {
		return LocalPack{}, err
	}
	plan, err := prepareDraftBlockPlan(ctx, s.store, rec.Draft, rec.DraftRevision, s.services)
	if err != nil {
		return LocalPack{}, err
	}
	if !localPackFlowAllowed(rec.Draft) || localPackSecretExpressions(rec.Draft) != nil {
		return LocalPack{}, ErrLocalPackUnsupported
	}
	parent, err := sanitizeLocalPackFlow(rec.Draft, true)
	if err != nil {
		return LocalPack{}, err
	}
	pack := LocalPack{Format: LocalPackFormat, Version: LocalPackVersion,
		Parent: LocalPackParent{SourceRevision: rec.DraftRevision, Flow: parent}, Blocks: []LocalPackBlock{}}
	pack.Parent.Hash, err = localPackHash(parent)
	if err != nil {
		return LocalPack{}, ErrLocalPackInvalid
	}
	if plan != nil {
		for _, source := range plan.blocks {
			if err := localPackSecretExpressions(source.Document); err != nil {
				return LocalPack{}, err
			}
			doc, err := sanitizeLocalPackFlow(source.Document, false)
			if err != nil {
				return LocalPack{}, err
			}
			hash, err := localPackHash(doc)
			if err != nil {
				return LocalPack{}, ErrLocalPackInvalid
			}
			pack.Blocks = append(pack.Blocks, LocalPackBlock{SourceID: source.Ref.FlowID, SourceRevision: source.Ref.Revision, Hash: hash, Flow: doc})
		}
	}
	prepared, err := prepareLocalPack(ctx, pack)
	if err != nil {
		return LocalPack{}, err
	}
	if err := s.validateLocalPackSource(ctx, prepared); err != nil {
		return LocalPack{}, err
	}
	return pack, nil
}

func (s *Service) PreviewLocalPack(ctx context.Context, pack LocalPack) (LocalPackPreview, error) {
	prepared, err := prepareLocalPack(ctx, pack)
	if err != nil {
		return LocalPackPreview{}, err
	}
	parentID, err := newLocalPackID()
	if err != nil {
		return LocalPackPreview{}, err
	}
	blocks := make([]LocalPackBlockTarget, len(prepared.blocks))
	for i, block := range prepared.blocks {
		localID, err := newLocalPackID()
		if err != nil {
			return LocalPackPreview{}, err
		}
		blocks[i] = LocalPackBlockTarget{SourceID: block.Ref.FlowID, SourceRevision: block.Ref.Revision, LocalID: localID}
	}
	proposal := LocalPackProposal{PackDigest: prepared.digest, ParentID: parentID, Blocks: blocks}
	mapped, err := localPackMappedDocsWithHashes(prepared, proposal)
	if err != nil {
		return LocalPackPreview{}, err
	}
	proposal.ParentHash = mapped.parentHash
	proposal.Blocks = mapped.targets
	proposal.Digest, err = localPackProposalDigest(proposal)
	if err != nil {
		return LocalPackPreview{}, ErrLocalPackInvalid
	}
	if err := s.validateLocalPackMapped(ctx, mapped.docs); err != nil {
		return LocalPackPreview{}, err
	}
	review := LocalPackReview{Parent: LocalPackReviewItem{Name: security.ScrubSecrets(prepared.parent.Name), SourceID: prepared.parent.ID, SourceRevision: prepared.parentRev},
		Blocks: []LocalPackReviewItem{}, SampleDataCleared: true, LocalOnly: true}
	for _, block := range prepared.blocks {
		review.Blocks = append(review.Blocks, LocalPackReviewItem{Name: security.ScrubSecrets(block.Document.Name), SourceID: block.Ref.FlowID, SourceRevision: block.Ref.Revision})
	}
	return LocalPackPreview{Proposal: proposal, Review: review}, nil
}

type localPackMappedWithHashes struct {
	parentHash string
	targets    []LocalPackBlockTarget
	docs       *localPackMapped
}

func localPackMappedDocsWithHashes(prepared *localPackPrepared, proposal LocalPackProposal) (localPackMappedWithHashes, error) {
	if prepared == nil || prepared.parent == nil || !validLocalPackID(proposal.ParentID) || len(proposal.Blocks) != len(prepared.blocks) {
		return localPackMappedWithHashes{}, ErrLocalPackInvalid
	}
	sourceIDs := map[string]bool{prepared.parent.ID: true}
	for _, block := range prepared.blocks {
		sourceIDs[block.Ref.FlowID] = true
	}
	if sourceIDs[proposal.ParentID] {
		return localPackMappedWithHashes{}, ErrLocalPackInvalid
	}
	ids := make(map[blockRef]string, len(proposal.Blocks))
	seenIDs := map[string]bool{proposal.ParentID: true}
	for i := range proposal.Blocks {
		block := prepared.blocks[i]
		target := proposal.Blocks[i]
		if target.SourceID != block.Ref.FlowID || target.SourceRevision != block.Ref.Revision || !validLocalPackID(target.LocalID) || seenIDs[target.LocalID] || sourceIDs[target.LocalID] {
			return localPackMappedWithHashes{}, ErrLocalPackInvalid
		}
		ids[block.Ref], seenIDs[target.LocalID] = target.LocalID, true
	}
	remap := func(source *Flow, id string, kind Kind) (*Flow, error) {
		doc, err := sanitizeLocalPackFlow(source, false)
		if err != nil {
			return nil, err
		}
		doc.ID, doc.Kind = id, kind
		for i := range doc.Nodes {
			node := &doc.Nodes[i]
			if node.Type != blockCallType {
				continue
			}
			ref, err := blockReference(*node)
			if err != nil {
				return nil, ErrLocalPackInvalid
			}
			localID, ok := ids[ref]
			if !ok {
				return nil, ErrLocalPackInvalid
			}
			node.Params["flow_id"], node.Params["revision"] = localID, float64(1)
		}
		return doc, nil
	}
	parent, err := remap(prepared.parent, proposal.ParentID, KindFlow)
	if err != nil {
		return localPackMappedWithHashes{}, err
	}
	parentHash, err := localPackHash(parent)
	if err != nil {
		return localPackMappedWithHashes{}, ErrLocalPackInvalid
	}
	blocks := make([]resolvedBlock, len(prepared.blocks))
	byRef := make(map[blockRef]resolvedBlock, len(prepared.blocks))
	targets := make([]LocalPackBlockTarget, len(prepared.blocks))
	for i, source := range prepared.blocks {
		id := ids[source.Ref]
		doc, err := remap(source.Document, id, KindBlock)
		if err != nil {
			return localPackMappedWithHashes{}, err
		}
		hash, err := localPackHash(doc)
		if err != nil {
			return localPackMappedWithHashes{}, ErrLocalPackInvalid
		}
		ref := blockRef{FlowID: id, Revision: 1}
		contract, err := readBlockContract(doc)
		if err != nil {
			return localPackMappedWithHashes{}, ErrLocalPackInvalid
		}
		block := resolvedBlock{Ref: ref, Document: doc, Contract: contract, Hash: sha256.Sum256(mustMarshalFlow(doc))}
		blocks[i], byRef[ref] = block, block
		targets[i] = LocalPackBlockTarget{SourceID: source.Ref.FlowID, SourceRevision: source.Ref.Revision, LocalID: id, Hash: hash}
	}
	return localPackMappedWithHashes{parentHash: parentHash, targets: targets,
		docs: &localPackMapped{parent: parent, blocks: blocks, byRef: byRef}}, nil
}

func (s *Service) ImportLocalPack(ctx context.Context, req LocalPackImportRequest) (LocalPackImportResult, error) {
	prepared, err := prepareLocalPack(ctx, req.Pack)
	if err != nil {
		return LocalPackImportResult{}, err
	}
	mapped, err := localPackMappedDocs(prepared, req.Proposal)
	if err != nil {
		return LocalPackImportResult{}, err
	}
	if err := s.validateLocalPackMapped(ctx, mapped); err != nil {
		return LocalPackImportResult{}, err
	}
	cleaner, ok := s.bridge.(localPackCleaner)
	if !ok || s.bridge == nil {
		return LocalPackImportResult{}, ErrMissionControlUnavailable
	}
	missionID, err := s.bridge.CreateFlowMission(mapped.parent.ID, mapped.parent.Name)
	if err != nil {
		return LocalPackImportResult{}, err
	}
	cleanup := func(cause error) error {
		cleanupErr := cleaner.DeleteCreatedFlowMission(missionID, mapped.parent.ID)
		return errors.Join(cause, cleanupErr)
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return LocalPackImportResult{}, cleanup(err)
	}
	defer func() { _ = tx.Rollback() }()
	committed := false
	rollbackAndCleanup := func(cause error) error {
		if committed {
			return cause
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return errors.Join(cause, rollbackErr) // Keep the disabled owner when rollback is uncertain.
		}
		return cleanup(cause)
	}
	parentBytes, err := mapped.parent.Marshal()
	if err != nil {
		return LocalPackImportResult{}, rollbackAndCleanup(ErrLocalPackInvalid)
	}
	if err := insertLocalPackParent(ctx, tx, mapped.parent, missionID, string(parentBytes), s.now()); err != nil {
		return LocalPackImportResult{}, rollbackAndCleanup(err)
	}
	for _, block := range mapped.blocks {
		data, err := block.Document.Marshal()
		if err != nil || len(data) > MaxDocumentBytes {
			return LocalPackImportResult{}, rollbackAndCleanup(ErrLocalPackInvalid)
		}
		if err := insertLocalPackBlock(ctx, tx, block.Document, string(data), s.now()); err != nil {
			return LocalPackImportResult{}, rollbackAndCleanup(err)
		}
		closure, err := resolveBlockDependencies(ctx, block.Document, func(_ context.Context, id string, revision int) (*Flow, error) {
			child, ok := mapped.byRef[blockRef{FlowID: id, Revision: revision}]
			if !ok {
				return nil, ErrBlockUnavailable
			}
			return child.Document, nil
		})
		if err != nil {
			return LocalPackImportResult{}, rollbackAndCleanup(ErrLocalPackInvalid)
		}
		if err := pinBlockVersionTx(ctx, tx, block.Ref.FlowID, 1, closure); err != nil {
			return LocalPackImportResult{}, rollbackAndCleanup(err)
		}
	}
	if _, err := prepareBlockPlanTx(ctx, tx, mapped.parent, s.services); err != nil {
		return LocalPackImportResult{}, rollbackAndCleanup(ErrLocalPackInvalid)
	}
	if err := tx.Commit(); err != nil {
		confirmed, reconcileErr := s.reconcileLocalPack(ctx, req.Proposal)
		if reconcileErr == nil && confirmed.Status == "applied_state_confirmed" {
			return LocalPackImportResult{Status: "applied", Flow: confirmed.Flow, Blocks: confirmed.Blocks}, nil
		}
		return LocalPackImportResult{}, errors.Join(err, reconcileErr) // Unknown commit: retain the mission.
	}
	committed = true
	if s.store.afterLocalPackCommit != nil {
		s.store.afterLocalPackCommit()
	}
	confirmed, err := s.reconcileLocalPack(ctx, req.Proposal)
	if err != nil || confirmed.Status != "applied_state_confirmed" {
		return LocalPackImportResult{}, errors.Join(ErrLocalPackConflict, err)
	}
	return LocalPackImportResult{Status: "applied", Flow: confirmed.Flow, Blocks: confirmed.Blocks}, nil
}

func localPackMappedDocs(prepared *localPackPrepared, proposal LocalPackProposal) (*localPackMapped, error) {
	if proposal.PackDigest != prepared.digest || !validLocalPackID(proposal.ParentID) || !validExtractionHash(proposal.ParentHash) || len(proposal.Blocks) != len(prepared.blocks) {
		return nil, ErrLocalPackInvalid
	}
	preview, err := localPackMappedDocsWithHashes(prepared, proposal)
	if err != nil {
		return nil, err
	}
	if preview.parentHash != proposal.ParentHash {
		return nil, ErrLocalPackConflict
	}
	for i := range proposal.Blocks {
		if preview.targets[i] != proposal.Blocks[i] {
			return nil, ErrLocalPackConflict
		}
	}
	wantDigest, err := localPackProposalDigest(proposal)
	if err != nil || subtle.ConstantTimeCompare([]byte(wantDigest), []byte(proposal.Digest)) != 1 {
		return nil, ErrLocalPackInvalid
	}
	return preview.docs, nil
}

func insertLocalPackParent(ctx context.Context, tx *sql.Tx, parent *Flow, missionID, data string, now time.Time) error {
	stamp := formatTime(now)
	res, err := tx.ExecContext(ctx, `INSERT INTO flows(id,kind,name,description,mission_id,draft_json,draft_revision,live_json,live_revision,published_draft_revision,published_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,1,'',0,0,'',?,?) ON CONFLICT(id) DO NOTHING`, parent.ID, string(KindFlow), parent.Name, parent.Description, missionID, data, stamp, stamp)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return ErrLocalPackConflict
	}
	return nil
}

func insertLocalPackBlock(ctx context.Context, tx *sql.Tx, block *Flow, data string, now time.Time) error {
	stamp := formatTime(now)
	res, err := tx.ExecContext(ctx, `INSERT INTO flows(id,kind,name,description,mission_id,draft_json,draft_revision,live_json,live_revision,published_draft_revision,published_at,created_at,updated_at)
		VALUES(?,?,?,?, '', ?,1,?,1,1,?,?,?) ON CONFLICT(id) DO NOTHING`, block.ID, string(KindBlock), block.Name, block.Description, data, data, stamp, stamp, stamp)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return ErrLocalPackConflict
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO flow_versions(flow_id,revision,json,published_at) VALUES(?,1,?,?)`, block.ID, data, stamp); err != nil {
		return err
	}
	return nil
}

func (s *Service) ReconcileLocalPack(ctx context.Context, req LocalPackReconcileRequest) (LocalPackReconcileResult, error) {
	return s.reconcileLocalPack(ctx, req.Proposal)
}

func (s *Service) reconcileLocalPack(ctx context.Context, proposal LocalPackProposal) (LocalPackReconcileResult, error) {
	unknown := LocalPackReconcileResult{Status: "unknown"}
	if err := validateLocalPackProposal(proposal); err != nil {
		return unknown, nil
	}
	var flow *FlowRecord
	dbBlocks := make([]*FlowRecord, len(proposal.Blocks))
	blockPins := make([][]LocalPackPin, len(proposal.Blocks))
	blockDocs := make(map[blockRef]resolvedBlock, len(proposal.Blocks))
	readErr := blockReadSnapshot(ctx, s.store.db, func(tx *sql.Tx) error {
		var err error
		flow, err = scanFlow(tx.QueryRowContext(ctx, `SELECT `+flowColumns+` FROM flows WHERE id=?`, proposal.ParentID))
		if errors.Is(err, sql.ErrNoRows) {
			return errLocalPackUnknown
		}
		if err != nil {
			return err
		}
		if flow.Kind != KindFlow || flow.MissionID == "" || flow.DraftRevision != 1 || flow.Live != nil || flow.LiveRevision != 0 || flow.PublishedDraftRevision != 0 {
			return errLocalPackUnknown
		}
		parentHash, err := localPackHash(flow.Draft)
		if err != nil || parentHash != proposal.ParentHash || !localPackFlowAllowed(flow.Draft) || !manualSamplesCleared(flow.Draft) {
			return errLocalPackUnknown
		}
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM flow_versions WHERE flow_id=?`, proposal.ParentID).Scan(&count); err != nil || count != 0 {
			if err != nil {
				return err
			}
			return errLocalPackUnknown
		}
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM flow_block_version_pins WHERE parent_flow_id=?`, proposal.ParentID).Scan(&count); err != nil || count != 0 {
			if err != nil {
				return err
			}
			return errLocalPackUnknown
		}
		for i, target := range proposal.Blocks {
			block, err := scanFlow(tx.QueryRowContext(ctx, `SELECT `+flowColumns+` FROM flows WHERE id=?`, target.LocalID))
			if errors.Is(err, sql.ErrNoRows) {
				return errLocalPackUnknown
			}
			if err != nil {
				return err
			}
			if block.Kind != KindBlock || block.MissionID != "" || block.DraftRevision != 1 || block.LiveRevision != 1 || block.PublishedDraftRevision != 1 || block.Live == nil {
				return errLocalPackUnknown
			}
			draftHash, err := localPackHash(block.Draft)
			if err != nil || draftHash != target.Hash {
				return errLocalPackUnknown
			}
			liveHash, err := localPackHash(block.Live)
			if err != nil || liveHash != target.Hash {
				return errLocalPackUnknown
			}
			var raw string
			if err := tx.QueryRowContext(ctx, `SELECT json FROM flow_versions WHERE flow_id=? AND revision=1`, target.LocalID).Scan(&raw); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return errLocalPackUnknown
				}
				return err
			}
			version, err := ParseFlow([]byte(raw))
			if err != nil {
				return errLocalPackUnknown
			}
			versionHash, err := localPackHash(version)
			if err != nil || versionHash != target.Hash {
				return errLocalPackUnknown
			}
			var versions int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM flow_versions WHERE flow_id=?`, target.LocalID).Scan(&versions); err != nil || versions != 1 {
				if err != nil {
					return err
				}
				return errLocalPackUnknown
			}
			contract, err := readBlockContract(version)
			if err != nil {
				return errLocalPackUnknown
			}
			ref := blockRef{FlowID: target.LocalID, Revision: 1}
			blockDocs[ref] = resolvedBlock{Ref: ref, Document: version, Contract: contract, Hash: sha256.Sum256(mustMarshalFlow(version))}
			dbBlocks[i] = block
			blockPins[i], err = localPackPinsForFlowTx(ctx, tx, block.ID, 1)
			if err != nil {
				return err
			}
		}
		loaded := func(_ context.Context, id string, revision int) (*Flow, error) {
			block, ok := blockDocs[blockRef{FlowID: id, Revision: revision}]
			if !ok {
				return nil, ErrBlockUnavailable
			}
			return block.Document, nil
		}
		closure, err := resolveBlockDependencies(ctx, flow.Draft, loaded)
		if err != nil || len(closure) != len(blockDocs) {
			return errLocalPackUnknown
		}
		for _, imported := range dbBlocks {
			localID := imported.ID
			doc := blockDocs[blockRef{FlowID: localID, Revision: 1}].Document
			descendants, err := resolveBlockDependencies(ctx, doc, loaded)
			if err != nil {
				return errLocalPackUnknown
			}
			if err := verifyExistingBlockVersionPins(ctx, tx, localID, 1, descendants); err != nil {
				return errLocalPackUnknown
			}
		}
		return nil
	})
	if readErr != nil {
		if errors.Is(readErr, context.Canceled) || errors.Is(readErr, context.DeadlineExceeded) {
			return unknown, readErr
		}
		return unknown, nil
	}
	if s.bridge == nil || s.bridge.FlowMissionState(flow.MissionID, proposal.ParentID) != MissionStateDisabled {
		return unknown, nil
	}
	flowSummary, err := localPackRecord(flow, proposal.ParentHash, []LocalPackPin{})
	if err != nil {
		return unknown, nil
	}
	blockSummaries := make([]LocalPackRecord, len(dbBlocks))
	for i, block := range dbBlocks {
		blockSummaries[i], err = localPackRecord(block, proposal.Blocks[i].Hash, blockPins[i])
		if err != nil {
			return unknown, nil
		}
	}
	return LocalPackReconcileResult{Status: "applied_state_confirmed", Flow: &flowSummary, Blocks: blockSummaries}, nil
}

var errLocalPackUnknown = errors.New("local pack state is not confirmed")

func localPackPinsForFlowTx(ctx context.Context, tx *sql.Tx, parent string, revision int) ([]LocalPackPin, error) {
	rows, err := tx.QueryContext(ctx, `SELECT block_flow_id,block_revision,block_hash FROM flow_block_version_pins WHERE parent_flow_id=? AND parent_revision=? ORDER BY block_flow_id,block_revision`, parent, revision)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pins := []LocalPackPin{}
	for rows.Next() {
		var pin LocalPackPin
		var hash []byte
		if err := rows.Scan(&pin.BlockID, &pin.Revision, &hash); err != nil {
			return nil, err
		}
		if len(hash) != sha256.Size || pin.Revision < 1 {
			return nil, ErrLocalPackInvalid
		}
		pin.Hash = hex.EncodeToString(hash)
		pins = append(pins, pin)
	}
	return pins, rows.Err()
}

func localPackRecord(record *FlowRecord, hash string, pins []LocalPackPin) (LocalPackRecord, error) {
	if record == nil || !validExtractionHash(hash) {
		return LocalPackRecord{}, ErrLocalPackInvalid
	}
	return LocalPackRecord{ID: record.ID, Kind: record.Kind, Name: security.ScrubSecrets(record.Name), MissionID: record.MissionID,
		DraftRevision: record.DraftRevision, LiveRevision: record.LiveRevision,
		PublishedDraftRevision: record.PublishedDraftRevision, Hash: hash, Pins: pins}, nil
}

func validateLocalPackProposal(proposal LocalPackProposal) error {
	if !validLocalPackID(proposal.ParentID) || !validExtractionHash(proposal.PackDigest) || !validExtractionHash(proposal.Digest) || !validExtractionHash(proposal.ParentHash) || proposal.Blocks == nil || len(proposal.Blocks) > maxBlockVersions {
		return ErrLocalPackInvalid
	}
	seen := map[string]bool{proposal.ParentID: true}
	refs := make(map[blockRef]bool, len(proposal.Blocks))
	sourceIDs := make(map[string]bool, len(proposal.Blocks))
	for _, block := range proposal.Blocks {
		ref := blockRef{FlowID: block.SourceID, Revision: block.SourceRevision}
		if !blockFlowID.MatchString(block.SourceID) || block.SourceRevision < 1 || !validLocalPackID(block.LocalID) || !validExtractionHash(block.Hash) || seen[block.LocalID] || refs[ref] {
			return ErrLocalPackInvalid
		}
		seen[block.LocalID], refs[ref] = true, true
		sourceIDs[block.SourceID] = true
	}
	if sourceIDs[proposal.ParentID] {
		return ErrLocalPackInvalid
	}
	for _, block := range proposal.Blocks {
		if sourceIDs[block.LocalID] {
			return ErrLocalPackInvalid
		}
	}
	want, err := localPackProposalDigest(proposal)
	if err != nil || subtle.ConstantTimeCompare([]byte(want), []byte(proposal.Digest)) != 1 {
		return ErrLocalPackInvalid
	}
	return nil
}

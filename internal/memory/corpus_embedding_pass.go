package memory

import (
	"context"
	"errors"
	"fmt"

	chromem "github.com/philippgille/chromem-go"
)

// Blocked reasons of pending corpus documents. A pending document keeps its source
// content; its reason says why it has no vector and who may try again.
const (
	// blockedCredentialReason marks a document whose content or metadata holds a
	// credential. It is never sent to the embedding provider and never loaded into
	// the index. The runtime retry loop leaves it alone; the startup reconcile
	// re-evaluates it, and rewriting the document clears the block.
	blockedCredentialReason = "credential_policy"
	// blockedRejectedReason marks a document the provider refused for good (a 4xx
	// that concerns this input). The runtime retry loop does not send it again; the
	// startup reconcile does, once, because a changed configuration may accept it.
	blockedRejectedReason = "embedding_rejected"
)

// blockedRetryMaxConsecutiveFailures is how many provider-side failures in a row end
// a reconcile or retry pass. The provider is evidently unavailable, and every further
// document would only burn a full retry cycle before the next attempt.
const blockedRetryMaxConsecutiveFailures = 3

// blockedRetryMaxFailedAttempts is the hard cap of failed embedding attempts in one
// runtime retry pass, whatever their kind, so that a provider answering every
// document with an error is not asked about the whole backlog every interval.
const blockedRetryMaxFailedAttempts = 20

// corpusPassMaxReportedErrors bounds the errors a pass returns; the rest is only counted.
const corpusPassMaxReportedErrors = 5

type embeddingFailureKind int

const (
	// embeddingFailureProvider covers transient and unclassified failures. They say
	// nothing about the document, so they count towards the breaker.
	embeddingFailureProvider embeddingFailureKind = iota
	// embeddingFailureProviderWide covers refusals that hold for every document:
	// rejected credentials, missing payment, missing permission.
	embeddingFailureProviderWide
	// embeddingFailureDocument covers answers that blame this input only.
	embeddingFailureDocument
)

// classifyEmbeddingFailure sorts an embedding error by the HTTP status it carries.
// Statuses that point at the endpoint or the request shape (404, 405, 415, ...) are
// deliberately not document-level: they would reject the whole corpus for good.
func classifyEmbeddingFailure(err error) embeddingFailureKind {
	var statusErr *embeddingHTTPStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.statusCode {
		case 401, 402, 403:
			return embeddingFailureProviderWide
		case 400, 413, 422:
			return embeddingFailureDocument
		}
	}
	return embeddingFailureProvider
}

// embeddingFailureBreaker stops a pass that keeps hitting an unavailable provider.
type embeddingFailureBreaker struct {
	maxConsecutive int
	maxFailures    int // 0: no cap
	consecutive    int
	failures       int
	open           bool
}

func newEmbeddingFailureBreaker(maxFailures int) embeddingFailureBreaker {
	return embeddingFailureBreaker{maxConsecutive: blockedRetryMaxConsecutiveFailures, maxFailures: maxFailures}
}

func (b *embeddingFailureBreaker) succeeded() { b.consecutive = 0 }

// failed records a failed attempt and reports how it was classified. The breaker
// opens after maxConsecutive provider-side failures in a row, at once on a
// provider-wide refusal, and when the cap of failed attempts is reached. A document
// rejection proves the provider is answering, so it resets the consecutive count.
func (b *embeddingFailureBreaker) failed(err error) embeddingFailureKind {
	kind := classifyEmbeddingFailure(err)
	b.failures++
	switch kind {
	case embeddingFailureProviderWide:
		b.open = true
	case embeddingFailureDocument:
		b.consecutive = 0
	default:
		b.consecutive++
		if b.consecutive >= b.maxConsecutive {
			b.open = true
		}
	}
	if b.maxFailures > 0 && b.failures >= b.maxFailures {
		b.open = true
	}
	return kind
}

// corpusCredentialError applies the write-time credential policy to a stored
// document. The fingerprint is owned by the embedding pass, which replaces it, so
// it is not judged: fingerprints written before they were redacted still carry the
// query credential of the endpoint, and must not block the document they stamp.
func corpusCredentialError(doc chromem.Document) error {
	if _, ok := doc.Metadata["embedding_fingerprint"]; ok {
		meta := make(map[string]string, len(doc.Metadata))
		for key, value := range doc.Metadata {
			if key != "embedding_fingerprint" {
				meta[key] = value
			}
		}
		doc.Metadata = meta
	}
	return validateNewVectorDocuments([]chromem.Document{doc})
}

// corpusEmbeddingPass is the state shared by the pages of one reconcile or retry
// pass: the failure breaker, the outcome counters and the errors to report.
type corpusEmbeddingPass struct {
	cv      *ChromemVectorDB
	breaker embeddingFailureBreaker
	// fullReconcile passes also mark documents they no longer try after the breaker
	// opened, so that the retry loop picks them up. Retry passes only touch
	// documents that are blocked for a failure already.
	fullReconcile bool

	attempted, embedded                           int
	policyBlocked, rejected, failed, notAttempted int // notAttempted: skipped because the breaker was open
	errCount                                      int
	errs                                          error
}

func newCorpusEmbeddingPass(cv *ChromemVectorDB, fullReconcile bool, maxFailures int) *corpusEmbeddingPass {
	return &corpusEmbeddingPass{cv: cv, breaker: newEmbeddingFailureBreaker(maxFailures), fullReconcile: fullReconcile}
}

// corpusPageResult is what one page of snapshots turned into.
type corpusPageResult struct {
	candidates []corpusReconcileCandidate
	policy     []corpusDocumentSnapshot // credential policy: never embedded
	rejected   []corpusDocumentSnapshot // refused by the provider for good
	failed     []corpusDocumentSnapshot // provider-side failure or not tried
}

func (p *corpusEmbeddingPass) recordError(err error) {
	p.errCount++
	if p.errCount <= corpusPassMaxReportedErrors {
		p.errs = errors.Join(p.errs, err)
	}
}

// report returns the errors of the pass, noting how many were left out.
func (p *corpusEmbeddingPass) report() error {
	if p.errs == nil || p.errCount <= corpusPassMaxReportedErrors {
		return p.errs
	}
	return errors.Join(p.errs, fmt.Errorf("%d further document embeddings failed", p.errCount-corpusPassMaxReportedErrors))
}

// embedPage embeds the documents of one page. Every document first passes the
// credential policy, then, unless the breaker is open, the provider. An error is
// returned only when ctx ends; embedding failures are collected in the result.
func (p *corpusEmbeddingPass) embedPage(ctx context.Context, snapshots []corpusDocumentSnapshot) (corpusPageResult, error) {
	var res corpusPageResult
	for _, snapshot := range snapshots {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if corpusCredentialError(snapshot.document) != nil {
			res.policy = append(res.policy, snapshot)
			p.policyBlocked++
			continue
		}
		if p.breaker.open {
			if !p.fullReconcile {
				break
			}
			res.failed = append(res.failed, snapshot)
			p.notAttempted++
			continue
		}
		doc := snapshot.document
		doc.Embedding = nil
		embedded := []chromem.Document{doc}
		p.attempted++
		if err := p.cv.embedDocumentsOutsideLock(ctx, embedded); err != nil {
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			p.recordError(err)
			if p.breaker.failed(err) == embeddingFailureDocument {
				res.rejected = append(res.rejected, snapshot)
				p.rejected++
			} else {
				res.failed = append(res.failed, snapshot)
				p.failed++
			}
			continue
		}
		p.breaker.succeeded()
		p.embedded++
		doc = embedded[0]
		doc.Metadata = p.cv.addEmbeddingMetadata(doc.Metadata)
		res.candidates = append(res.candidates, corpusReconcileCandidate{snapshot: snapshot, document: doc})
	}
	return res, nil
}

// markPage records the blocked reasons of a page. Retry passes leave the documents
// that failed again alone: they carry their reason already.
func (p *corpusEmbeddingPass) markPage(ctx context.Context, collection string, res corpusPageResult) error {
	if err := p.cv.setCorpusBlockedSnapshots(ctx, collection, res.policy, blockedCredentialReason); err != nil {
		return err
	}
	if err := p.cv.setCorpusBlockedSnapshots(ctx, collection, res.rejected, blockedRejectedReason); err != nil {
		return err
	}
	if p.fullReconcile {
		return p.cv.setCorpusBlockedSnapshots(ctx, collection, res.failed, blockedEmbeddingReason)
	}
	return nil
}

// logSummary logs the pass once, with counts only; document content never appears.
func (p *corpusEmbeddingPass) logSummary(label string) {
	held := p.policyBlocked + p.rejected + p.failed + p.notAttempted
	if p.cv.logger == nil || (p.attempted == 0 && held == 0) {
		return
	}
	logf := p.cv.logger.Info
	if held > 0 {
		logf = p.cv.logger.Warn
	}
	logf("Corpus embedding pass finished", "pass", label, "attempted", p.attempted, "embedded", p.embedded,
		"credential_policy", p.policyBlocked, "embedding_rejected", p.rejected, "embedding_failed", p.failed,
		"not_attempted", p.notAttempted, "failed_attempts_total", p.breaker.failures, "breaker_open", p.breaker.open)
}

// setCorpusBlockedSnapshots blocks pending documents for a reason, but only those
// whose row still is what the pass read: a concurrent writer's newer version stays
// as it is, and the writer publishes it.
func (cv *ChromemVectorDB) setCorpusBlockedSnapshots(ctx context.Context, collection string, snapshots []corpusDocumentSnapshot, reason string) error {
	if len(snapshots) == 0 {
		return nil
	}
	tx, err := cv.corpus.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, snapshot := range snapshots {
		if _, err := tx.ExecContext(ctx, `UPDATE documents SET blocked_reason=? WHERE collection=? AND id=? AND pending<>0 AND content=? AND metadata=?`,
			reason, collection, snapshot.document.ID, snapshot.document.Content, snapshot.metadata); err != nil {
			return err
		}
	}
	return tx.Commit()
}

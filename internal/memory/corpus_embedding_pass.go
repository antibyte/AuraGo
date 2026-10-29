package memory

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

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

// corpusPassDocumentTimeout is the deadline of one document in a background pass,
// counted from the moment it holds the store-side embedding slot. Together with the
// single attempt a background context gets (withBackgroundEmbedding) it bounds how
// long a pass keeps foreground stores waiting behind one document.
const corpusPassDocumentTimeout = 60 * time.Second

// corpusDocumentMaxFailedPasses is how many passes in a row may count against a
// document (embeddingFailureBlamesDocument, with another document embedded after it
// in the same pass) before it is blocked with blockedRejectedReason.
const corpusDocumentMaxFailedPasses = 3

// corpusFailureMemoryMax bounds the documents whose failures are remembered; the
// memory starts over when it is full, which only delays a verdict.
const corpusFailureMemoryMax = 4096

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

// errEmptyEmbeddingVector is what the provider answered when it returned no vector.
var errEmptyEmbeddingVector = errors.New("empty vector")

// embeddingFailureBlamesDocument reports the only answers that can point at the
// input itself while the provider is otherwise alive: an HTTP 500, 502 or 504 for
// this request, or an empty vector. Deadlines, cancellations, transport errors,
// errors of the local embeddings runtime (no HTTP status), restarts and rate limits
// or overload (429, 503) say something about the provider, never about the document.
func embeddingFailureBlamesDocument(err error) bool {
	var statusErr *embeddingHTTPStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.statusCode {
		case 500, 502, 504:
			return true
		}
		return false
	}
	return errors.Is(err, errEmptyEmbeddingVector)
}

// documentEmbeddingError marks an error as coming from the embedding step of a
// store, so callers can tell it from policy, persistence and publication failures.
// The text is the one these errors always had.
type documentEmbeddingError struct {
	docID string
	err   error
}

func (e *documentEmbeddingError) Error() string {
	return "couldn't create embedding of document " + e.docID + ": " + e.err.Error()
}

func (e *documentEmbeddingError) Unwrap() error { return e.err }

// isProviderEmbeddingFailure reports whether err is a failure of the embedding step
// that blames the provider or the way to it (outage, timeout, refused credentials)
// and not the document.
func isProviderEmbeddingFailure(err error) bool {
	var embedErr *documentEmbeddingError
	return errors.As(err, &embedErr) && classifyEmbeddingFailure(embedErr.err) != embeddingFailureDocument
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
	return b.failedAs(classifyEmbeddingFailure(err))
}

// failedAs is failed for a caller that classified the failure itself.
func (b *embeddingFailureBreaker) failedAs(kind embeddingFailureKind) embeddingFailureKind {
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
	id      uint64 // numbers the passes of this process; see corpusPassMemory
	breaker embeddingFailureBreaker
	// fullReconcile passes also mark documents they no longer try after the breaker
	// opened, so that the retry loop picks them up. Retry passes only touch
	// documents that are blocked for a failure already.
	fullReconcile bool
	// last is the document the pass attempted most recently.
	last corpusCursor
	// suspects are the documents that failed in this pass with an answer that can
	// blame them and have not yet seen the provider embed another document after
	// that. The first embedding after them is the evidence that the provider was
	// working while they failed.
	suspects []corpusSuspect

	attempted, embedded                           int
	policyBlocked, rejected, failed, notAttempted int // notAttempted: skipped because the breaker was open
	repeatedFailures                              int // part of rejected: failed pass after pass while the provider embedded others after them
	errCount                                      int
	errs                                          error
}

type corpusSuspect struct {
	collection string
	snapshot   corpusDocumentSnapshot
}

func newCorpusEmbeddingPass(cv *ChromemVectorDB, fullReconcile bool, maxFailures int) *corpusEmbeddingPass {
	return &corpusEmbeddingPass{cv: cv, id: cv.corpusMemory.beginPass(), breaker: newEmbeddingFailureBreaker(maxFailures), fullReconcile: fullReconcile}
}

// corpusCursor names a document by its place in the order the passes walk: collection
// name, then ID. The zero value names no document.
type corpusCursor struct{ collection, id string }

type corpusDocumentKey struct{ collection, id string }

type corpusDocumentFailures struct {
	passes   int    // passes in a row that counted against the document
	lastPass uint64 // the latest of them
	content  [sha256.Size]byte
}

// corpusPassMemory is what the corpus embedding passes of one process remember
// between runs. It lives in memory only: a restart starts over, and the startup
// reconcile gives every pending document one more try anyway.
type corpusPassMemory struct {
	mu       sync.Mutex
	passes   uint64
	resume   corpusCursor
	failures map[corpusDocumentKey]corpusDocumentFailures
}

func (m *corpusPassMemory) beginPass() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.passes++
	return m.passes
}

// resumePoint is the document the latest retry pass attempted last; the next one
// starts behind it.
func (m *corpusPassMemory) resumePoint() corpusCursor {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resume
}

func (m *corpusPassMemory) setResumePoint(cursor corpusCursor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resume = cursor
}

// countFailedPass counts the given pass against doc and returns how many passes in
// a row now count against it. The caller counts a pass only when it can blame the
// document: an answer about this input, and proof in the same pass that the provider
// embedded another document afterwards. A document whose content changed starts over.
func (m *corpusPassMemory) countFailedPass(pass uint64, collection string, doc chromem.Document) int {
	key := corpusDocumentKey{collection, doc.ID}
	content := sha256.Sum256([]byte(doc.Content))
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, known := m.failures[key]
	if !known || entry.content != content {
		if m.failures == nil || len(m.failures) >= corpusFailureMemoryMax {
			m.failures = make(map[corpusDocumentKey]corpusDocumentFailures)
		}
		entry = corpusDocumentFailures{content: content}
	}
	if entry.lastPass != pass {
		entry.passes++
		entry.lastPass = pass
	}
	m.failures[key] = entry
	return entry.passes
}

// forgetFailures drops what is remembered about a document: it embedded, failed in a
// way that does not blame it, or got its verdict.
func (m *corpusPassMemory) forgetFailures(collection, id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.failures, corpusDocumentKey{collection, id})
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

// embedPage embeds the documents of one page of a collection. Every document first
// passes the credential policy, then, unless the breaker is open, the provider. An
// error is returned only when ctx ends; embedding failures are collected in the
// result.
//
// A pass is background work: the provider gets one attempt per document (the next
// pass is the retry) and a deadline of corpusPassDocumentTimeout that starts when the
// document holds the store-side embedding slot. A document that the provider answers
// with a 500, 502 or 504 (or an empty vector) in corpusDocumentMaxFailedPasses passes
// in a row, each time with another document embedded after it in the same pass, is
// blocked as rejected (settleSuspects). Any other failure only feeds the breaker.
func (p *corpusEmbeddingPass) embedPage(ctx context.Context, collection string, snapshots []corpusDocumentSnapshot) (corpusPageResult, error) {
	var res corpusPageResult
	background := withBackgroundEmbedding(ctx)
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
		p.last = corpusCursor{collection: collection, id: doc.ID}
		if err := p.cv.embedDocumentsWithin(background, embedded, corpusPassDocumentTimeout); err != nil {
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			p.recordError(err)
			kind := classifyEmbeddingFailure(err)
			if kind == embeddingFailureProvider {
				if embeddingFailureBlamesDocument(err) {
					p.suspects = append(p.suspects, corpusSuspect{collection: collection, snapshot: snapshot})
				} else {
					// Not an answer about this input (deadline, transport, restart, rate
					// limit ...): earlier failures no longer count towards a verdict.
					p.cv.corpusMemory.forgetFailures(collection, doc.ID)
				}
			}
			if p.breaker.failedAs(kind) == embeddingFailureDocument {
				res.rejected = append(res.rejected, snapshot)
				p.rejected++
			} else {
				res.failed = append(res.failed, snapshot)
				p.failed++
			}
			continue
		}
		p.breaker.succeeded()
		p.cv.corpusMemory.forgetFailures(collection, doc.ID)
		p.embedded++
		p.settleSuspects(ctx, collection, &res)
		doc = embedded[0]
		doc.Metadata = p.cv.addEmbeddingMetadata(doc.Metadata)
		res.candidates = append(res.candidates, corpusReconcileCandidate{snapshot: snapshot, document: doc})
	}
	return res, nil
}

// settleSuspects runs when the provider has just embedded a document: the documents
// that failed earlier in this pass with an answer that can blame them now have proof
// that the provider was working, so this pass counts against them. One that has been
// counted against in corpusDocumentMaxFailedPasses passes in a row is blocked as
// rejected, so the runtime loop stops sending it. res is the result of the page in
// progress; a suspect of an earlier page or collection is blocked directly.
func (p *corpusEmbeddingPass) settleSuspects(ctx context.Context, collection string, res *corpusPageResult) {
	suspects := p.suspects
	p.suspects = nil
	for _, suspect := range suspects {
		doc := suspect.snapshot.document
		if p.cv.corpusMemory.countFailedPass(p.id, suspect.collection, doc) < corpusDocumentMaxFailedPasses {
			continue
		}
		p.cv.corpusMemory.forgetFailures(suspect.collection, doc.ID)
		p.failed--
		p.rejected++
		p.repeatedFailures++
		if p.cv.logger != nil {
			p.cv.logger.Warn("Document keeps failing while the provider embeds others; the retry loop leaves it alone",
				"collection", suspect.collection, "id", doc.ID, "failed_passes", corpusDocumentMaxFailedPasses)
		}
		if suspect.collection == collection {
			if i := slices.IndexFunc(res.failed, func(s corpusDocumentSnapshot) bool { return s.document.ID == doc.ID }); i >= 0 {
				res.failed = slices.Delete(res.failed, i, i+1)
				res.rejected = append(res.rejected, suspect.snapshot)
				continue
			}
		}
		if err := p.cv.setCorpusBlockedSnapshots(ctx, suspect.collection, []corpusDocumentSnapshot{suspect.snapshot}, blockedRejectedReason); err != nil {
			p.recordError(err)
		}
	}
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
		"credential_policy", p.policyBlocked, "embedding_rejected", p.rejected, "rejected_after_repeated_failures", p.repeatedFailures,
		"embedding_failed", p.failed, "not_attempted", p.notAttempted, "failed_attempts_total", p.breaker.failures, "breaker_open", p.breaker.open)
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

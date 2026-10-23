package memory

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"aurago/internal/memory/kgsemantic"

	chromem "github.com/philippgille/chromem-go"
)

// blockedEmbeddingReason marks pending corpus documents whose embedding failed.
// Startup reconcile sets it; the retry loop below clears it on success.
const blockedEmbeddingReason = "embedding_failed"

// blockedRetryMaxConsecutiveFailures is how many provider-side failures in a row end
// a retry pass. The provider is evidently unavailable, and every further document
// would only burn a full retry cycle before the next interval tries again.
const blockedRetryMaxConsecutiveFailures = 3

// blockedRetryFlightKey keys the single-flight slot shared by every corpus
// reconcile and retry pass. The NUL prefix keeps it clear of the query strings
// that sfGroup deduplicates elsewhere.
const blockedRetryFlightKey = "\x00corpus-embedding-reconcile"

// blockedEmbeddingRetryInterval is the pause between retry passes: about ten
// minutes plus up to a minute of jitter, so instances do not retry in lockstep.
// Tests replace it.
var blockedEmbeddingRetryInterval = func() time.Duration {
	return 10*time.Minute + rand.N(time.Minute)
}

// startBlockedEmbeddingRetry runs the retry loop in the background. It is tracked
// like any other indexing operation, so Close waits for it, and it ends with ctx,
// which Close cancels. NewChromemVectorDB starts it once startup reconcile is done.
func (cv *ChromemVectorDB) startBlockedEmbeddingRetry(ctx context.Context) {
	if cv.corpus == nil {
		return
	}
	done, err := cv.beginTrackedOperation(&cv.indexingWg)
	if err != nil {
		return
	}
	go func() {
		defer done()
		cv.runBlockedEmbeddingRetryLoop(ctx)
	}()
}

func (cv *ChromemVectorDB) runBlockedEmbeddingRetryLoop(ctx context.Context) {
	timer := time.NewTimer(blockedEmbeddingRetryInterval())
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if err := cv.runBlockedEmbeddingRetryPass(ctx); err != nil && ctx.Err() == nil && cv.logger != nil {
			cv.logger.Warn("Blocked document embeddings are still failing; retrying later", "error", err)
		}
		timer.Reset(blockedEmbeddingRetryInterval())
	}
}

// reconcileCorpusSingleFlight runs the full corpus reconcile in the slot that the
// retry passes share, so the two never run side by side.
func (cv *ChromemVectorDB) reconcileCorpusSingleFlight(ctx context.Context) error {
	_, err, _ := cv.sfGroup.Do(blockedRetryFlightKey, func() (any, error) {
		return nil, cv.reconcileCorpus(ctx)
	})
	return err
}

// runBlockedEmbeddingRetryPass runs one retry pass. It does nothing, and costs one
// cheap query, while no document is blocked for a failed embedding.
func (cv *ChromemVectorDB) runBlockedEmbeddingRetryPass(ctx context.Context) error {
	if cv.corpus == nil || cv.requireReadyForStore() != nil {
		return nil
	}
	_, err, _ := cv.sfGroup.Do(blockedRetryFlightKey, func() (any, error) {
		return nil, cv.retryBlockedEmbeddings(ctx)
	})
	return err
}

// retryBlockedEmbeddings is the narrow counterpart of reconcileCorpus: it embeds
// only the documents blocked with blockedEmbeddingReason and publishes them through
// the same recheck-and-persist step, which clears the block. A document that fails
// again simply stays blocked.
func (cv *ChromemVectorDB) retryBlockedEmbeddings(ctx context.Context) error {
	var anyBlocked bool
	if err := cv.corpus.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM documents WHERE pending<>0 AND blocked_reason=? AND collection<>?)`, blockedEmbeddingReason, kgsemantic.CollectionName).Scan(&anyBlocked); err != nil {
		return err
	}
	if !anyBlocked {
		return nil
	}
	names, err := cv.corpusCollectionNames(ctx)
	if err != nil {
		return err
	}
	var passErr error
	var attempted, recovered int
	consecutiveFailures := 0
	defer func() {
		if attempted > 0 && cv.logger != nil {
			cv.logger.Info("Retried documents blocked by failed embeddings", "attempted", attempted, "recovered", recovered)
		}
	}()
	for _, name := range names {
		if name == kgsemantic.CollectionName {
			continue
		}
		col, err := cv.db.GetOrCreateCollection(name, nil, cv.embeddingFunc)
		if err != nil {
			return errors.Join(passErr, err)
		}
		afterID := ""
		for {
			page, err := cv.corpus.blockedDocumentPage(ctx, name, afterID, blockedEmbeddingReason)
			if err != nil {
				return errors.Join(passErr, err)
			}
			if len(page) == 0 {
				break
			}
			var candidates []corpusReconcileCandidate
			providerDown := false
			for _, snapshot := range page {
				if err := ctx.Err(); err != nil {
					return err
				}
				doc := snapshot.document
				doc.Embedding = nil
				embedded := []chromem.Document{doc}
				attempted++
				if err := cv.embedDocumentsOutsideLock(ctx, embedded); err != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					passErr = errors.Join(passErr, err)
					if shouldRetryEmbeddingError(ctx, err) {
						consecutiveFailures++
					} else {
						consecutiveFailures = 0
					}
					if consecutiveFailures >= blockedRetryMaxConsecutiveFailures {
						providerDown = true
						break
					}
					continue
				}
				consecutiveFailures = 0
				doc = embedded[0]
				doc.Metadata = cv.addEmbeddingMetadata(doc.Metadata)
				candidates = append(candidates, corpusReconcileCandidate{snapshot: snapshot, document: doc})
			}
			// Publish what was embedded before the pass gives up.
			if err := cv.persistReconcileCandidates(ctx, name, col, candidates); err != nil {
				return errors.Join(passErr, err)
			}
			recovered += len(candidates)
			if providerDown {
				return passErr
			}
			afterID = page[len(page)-1].document.ID
			if len(page) < corpusPageSize {
				break
			}
		}
	}
	return passErr
}

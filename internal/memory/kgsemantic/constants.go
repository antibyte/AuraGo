package kgsemantic

import "time"

const CollectionName = "kg_embeddings"
const QueryTimeout = 60 * time.Second
const EdgeMinSimilarity = 0.35

const QueryCacheTTL = 5 * time.Minute
const EdgeMaxResults = 50
const QueryCacheMaxSize = 100

const RetryMaxAttempts = 3
const RetryBackoffBase = 250 * time.Millisecond
const ConsistencyCheckSampleSize = 200
const ReindexDocumentBatchSize = 25
const EdgeReindexBatchSize = 100
const ContentCacheMaxSize = 5000

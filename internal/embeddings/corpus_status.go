package embeddings

// CorpusStatus reports durable storage independently from embedding readiness.
// VectorPayloadBytes estimates float32 payload only, excluding all index,
// document, map and allocator overhead. It is not measured process RSS.
type CorpusStatus struct {
	Available          bool   `json:"available"`
	Documents          int64  `json:"documents"`
	Dimensions         []int  `json:"dimensions"`
	DatabaseBytes      int64  `json:"database_bytes"`
	WALBytes           int64  `json:"wal_bytes"`
	VectorPayloadBytes int64  `json:"vector_payload_bytes"`
	Error              string `json:"error,omitempty"`
}

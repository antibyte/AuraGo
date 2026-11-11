package embeddings

// CorpusStatus reports durable storage independently from embedding readiness.
// VectorPayloadBytes estimates float32 payload only, excluding all index,
// document, map and allocator overhead. It is not measured process RSS.
type CorpusStatus struct {
	Available          bool    `json:"available"`
	Documents          int64   `json:"documents"`
	Dimensions         []int   `json:"dimensions"`
	DatabaseBytes      int64   `json:"database_bytes"`
	WALBytes           int64   `json:"wal_bytes"`
	VectorPayloadBytes int64   `json:"vector_payload_bytes"`
	Error              string  `json:"error,omitempty"`
	LimitBytes         int64   `json:"limit_bytes"`
	UsedBytes          int64   `json:"used_bytes"`
	UtilizationPercent float64 `json:"utilization_percent"`
	DiskFreeBytes      *int64  `json:"disk_free_bytes"`
	DiskReserveBytes   int64   `json:"disk_reserve_bytes"`
	CapacityWarning    bool    `json:"capacity_warning"`
	PublicationPaused  bool    `json:"publication_paused"`
	IndexPaused        bool    `json:"index_paused"`
	PauseReason        string  `json:"pause_reason,omitempty"`
}

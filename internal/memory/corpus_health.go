package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"aurago/internal/embeddings"
)

// CorpusStatus reads aggregate SQL metadata without decoding vectors or content.
// The retained legacy KG snapshot counts as durable storage but is never loaded
// into the current memory index, so it is excluded from the payload estimate.
func (cv *ChromemVectorDB) CorpusStatus(ctx context.Context) embeddings.CorpusStatus {
	status := embeddings.CorpusStatus{Dimensions: []int{}}
	if cv == nil || cv.corpus == nil {
		return status
	}
	rows, err := cv.corpus.db.QueryContext(ctx, `SELECT dimension,count(*),
		COALESCE(sum(CASE WHEN collection<>'kg_embeddings' AND dimension>0 THEN dimension*4 ELSE 0 END),0)
		FROM documents GROUP BY dimension ORDER BY dimension`)
	if err != nil {
		status.Error = "corpus_status_unavailable"
		return status
	}
	for rows.Next() {
		var dimension int
		var count, payload int64
		if err := rows.Scan(&dimension, &count, &payload); err != nil {
			rows.Close()
			status.Error = "corpus_status_unavailable"
			return status
		}
		status.Documents += count
		status.VectorPayloadBytes += payload
		if dimension > 0 {
			status.Dimensions = append(status.Dimensions, dimension)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil || ctx.Err() != nil {
		status.Error = "corpus_status_unavailable"
		return status
	}
	path := filepath.Join(cv.dataDir, "documents.sqlite")
	status.DatabaseBytes, err = corpusFileBytes(path, false)
	if err == nil {
		status.WALBytes, err = corpusFileBytes(path+"-wal", true)
	}
	if err != nil {
		status.Error = "corpus_size_unavailable"
		return status
	}
	status.LimitBytes = cv.corpus.limitBytes
	status.UsedBytes, err = cv.corpus.usedBytes(ctx)
	if err != nil {
		status.Error = "corpus_size_unavailable"
		return status
	}
	status.UtilizationPercent = float64(status.UsedBytes) * 100 / float64(status.LimitBytes)
	status.CapacityWarning = status.UtilizationPercent >= 80
	status.DiskReserveBytes = CorpusDiskReserveBytes
	status.IndexPaused = cv.corpus.indexPaused
	free, diskErr := cv.corpus.freeDisk(cv.corpus.path)
	if diskErr == nil {
		status.DiskFreeBytes = &free
	}
	switch {
	case status.UsedBytes > status.LimitBytes:
		status.PauseReason = "corpus_limit_exceeded"
	case status.IndexPaused:
		status.PauseReason = cv.corpus.pauseReason
	case diskErr != nil:
		status.PauseReason = "corpus_disk_status_unavailable"
	case free < CorpusDiskReserveBytes+65536:
		status.PauseReason = "corpus_disk_reserve"
	}
	status.PublicationPaused = status.PauseReason != ""
	status.Available = true
	return status
}

func corpusFileBytes(path string, optional bool) (int64, error) {
	info, err := os.Stat(path)
	if optional && os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("stat corpus storage: %w", err)
	}
	return info.Size(), nil
}

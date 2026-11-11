package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	chromem "github.com/philippgille/chromem-go"
)

const CorpusDiskReserveBytes int64 = 256 << 20

var ErrCorpusCapacity = errors.New("document corpus capacity unavailable")

type corpusCapacityError string

func (e corpusCapacityError) Error() string { return ErrCorpusCapacity.Error() + ": " + string(e) }
func (e corpusCapacityError) Unwrap() error { return ErrCorpusCapacity }

func corpusWriteError(err error) error {
	var coded interface{ Code() int }
	if errors.As(err, &coded) && coded.Code()&255 == 13 {
		return errors.Join(corpusCapacityError("corpus_storage_full"), err)
	}
	return err
}

func (c *documentCorpus) usedBytes(ctx context.Context) (int64, error) {
	var pages, size int64
	if err := c.db.QueryRowContext(ctx, `SELECT page_count,page_size FROM pragma_page_count(),pragma_page_size()`).Scan(&pages, &size); err != nil {
		return 0, err
	}
	used := pages * size
	info, err := os.Stat(c.path)
	if err != nil {
		return 0, err
	}
	return max(used, info.Size()), nil
}

func estimatedCorpusBytes(docs []chromem.Document, dimension int) int64 {
	var bytes int64
	for _, doc := range docs {
		metadata, _ := json.Marshal(doc.Metadata)
		vectorDimension := len(doc.Embedding)
		if vectorDimension == 0 {
			vectorDimension = max(dimension, 1024)
		}
		bytes += int64(len(doc.Content) + len(metadata) + len(doc.ID) + vectorDimension*16 + 4096)
	}
	return bytes
}

func (c *documentCorpus) checkCapacity(ctx context.Context, additional int64) error {
	if c == nil {
		return nil
	}
	used, err := c.usedBytes(ctx)
	if err != nil {
		return fmt.Errorf("read corpus capacity: %w", err)
	}
	if used > c.limitBytes {
		return corpusCapacityError("corpus_limit_exceeded")
	}
	var freePages, pageSize int64
	if err := c.db.QueryRowContext(ctx, `SELECT freelist_count,page_size FROM pragma_freelist_count(),pragma_page_size()`).Scan(&freePages, &pageSize); err != nil {
		return err
	}
	if additional > 0 && used-freePages*pageSize+additional > c.limitBytes {
		return corpusCapacityError("corpus_limit_reached")
	}
	free, err := c.freeDisk(c.path)
	if err != nil {
		return corpusCapacityError("corpus_disk_status_unavailable")
	}
	// Reserve room for the new database pages, WAL and transaction bookkeeping.
	needed := CorpusDiskReserveBytes + additional*3 + 65536
	if free < needed {
		return corpusCapacityError("corpus_disk_reserve")
	}
	return nil
}

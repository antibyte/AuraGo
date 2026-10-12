package layerling

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"testing"
)

func TestArchiveBounds(t *testing.T) {
	archive := func(name string) []byte {
		var b bytes.Buffer
		z := zip.NewWriter(&b)
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte("model"))
		if err = z.Close(); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	good := archive("3D/model.model")
	if err := ValidateArchive(context.Background(), "part.3mf", good); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../escape", "C:/drive", "dir\\escape"} {
		if ValidateArchive(context.Background(), "part.3mf", archive(name)) == nil {
			t.Fatal("accepted path", name)
		}
	}
	oversized := append([]byte(nil), good...)
	offset := bytes.Index(oversized, []byte{'P', 'K', 1, 2})
	binary.LittleEndian.PutUint32(oversized[offset+24:], 257<<20)
	if ValidateArchive(context.Background(), "part.3mf", oversized) == nil {
		t.Fatal("accepted expansion bomb")
	}
	if ValidateArchive(context.Background(), "part.3mf", good[:len(good)-12]) == nil {
		t.Fatal("accepted truncated archive")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if ValidateArchive(ctx, "part.3mf", good) == nil {
		t.Fatal("ignored cancellation")
	}
}

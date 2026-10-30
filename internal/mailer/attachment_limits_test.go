package mailer

import "testing"

func TestAttachmentLimitsRespectUploadAndOutgoingBudgets(t *testing.T) {
	for _, size := range []int64{1 << 20, 25 << 20, 32 << 20} {
		s := &Service{blobs: &BlobStore{maxFile: size}}
		got := s.AttachmentLimits()
		if got.MaxFileBytes != min(size, 25<<20) || got.MaxMessageBytes != 25<<20 {
			t.Fatalf("upload limit %d: unexpected limits %+v", size, got)
		}
	}
}

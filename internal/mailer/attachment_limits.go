package mailer

// AttachmentLimits describes the existing upload and outgoing-message budgets.
type AttachmentLimits struct {
	MaxFileBytes    int64 `json:"max_file_bytes"`
	MaxMessageBytes int64 `json:"max_message_bytes"`
}

func (s *Service) AttachmentLimits() AttachmentLimits {
	return AttachmentLimits{
		MaxFileBytes:    min(s.blobs.maxFile, maxOutgoingAttachmentBytes),
		MaxMessageBytes: maxOutgoingAttachmentBytes,
	}
}

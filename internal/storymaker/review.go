package storymaker

import (
	"context"
	"io"
)

// ReviewSheetName is the file name of the contact sheet of a project's last agent check; its folder is review/.
const ReviewSheetName = "contact-sheet.png"

// AgentReview is the review evidence of an agent job: the sheet and how many scenes it shows.
type AgentReview struct {
	Sheet  string `json:"sheet"`
	Scenes int    `json:"scenes"`
}

// SaveReviewSheet stores the contact sheet of the latest agent check. It replaces the previous one.
func (p *ProjectStore) SaveReviewSheet(ctx context.Context, id string, data []byte) error {
	if err := p.checkID(id); err != nil {
		return err
	}
	_, err := p.st.WriteFile(ctx, p.file(id, "review", ReviewSheetName), data, "")
	return err
}

// OpenReviewSheet opens the stored contact sheet. A missing sheet wraps fs.ErrNotExist.
func (p *ProjectStore) OpenReviewSheet(ctx context.Context, id string) (io.ReadSeekCloser, FileInfo, error) {
	if err := p.checkID(id); err != nil {
		return nil, FileInfo{}, err
	}
	return p.st.Open(ctx, p.file(id, "review", ReviewSheetName))
}

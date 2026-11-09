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

// SaveReviewSheet stores the contact sheet of the latest agent check. It replaces the previous one. A missing project wraps
// fs.ErrNotExist: the store creates the folders of a file it writes, so a check that finishes after its project was deleted
// would otherwise bring back <root>/<id>/review/ with nothing else in it. The check narrows that window; it does not lock
// against a delete in the same instant.
func (p *ProjectStore) SaveReviewSheet(ctx context.Context, id string, data []byte) error {
	if _, err := p.Get(ctx, id); err != nil {
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

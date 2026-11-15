package gamemaker

import "context"

func (s *Service) jobContext(job Job, project Project) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), s.opts.JobTimeout)
}

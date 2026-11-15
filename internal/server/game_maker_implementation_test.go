package server

import (
	"aurago/internal/gamemaker"
	"context"
)

type implementationTestRunner func(context.Context, gamemaker.JobRun) error

func (f implementationTestRunner) RunGameMakerJob(ctx context.Context, run gamemaker.JobRun) error {
	return f(ctx, run)
}

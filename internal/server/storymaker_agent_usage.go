package server

import "aurago/internal/storymaker/render"

// storymakerCodeDailyLimit refuses a job over storymaker.agent.max_daily_jobs, and a job whose count
// cannot be read.
const storymakerCodeDailyLimit = "daily_limit_reached"

// storymakerAgentUsage counts the jobs of one local date in storymaker/agent/usage.json: every create
// and revise, from the app and from chat, plus chat render. It reuses the narration counter, so the
// file has the same form (its "chars" field holds jobs) and it fails closed the same way.
type storymakerAgentUsage struct {
	count *storymakerNarrationUsage
}

func newStorymakerAgentUsage(dir string) *storymakerAgentUsage {
	return &storymakerAgentUsage{count: newStorymakerNarrationUsage(dir)}
}

// reserve counts one job for today, or returns a daily_limit_reached job error.
func (u *storymakerAgentUsage) reserve(maxDaily int) error {
	if err := u.count.reserve(1, maxDaily); err != nil {
		return render.Fail(storymakerCodeDailyLimit, "The daily limit of %d Storymaker agent jobs is reached, or its count could not be read.", maxDaily)
	}
	return nil
}

// release gives one job back after a reservation whose job was never queued.
func (u *storymakerAgentUsage) release(maxDaily int) {
	u.count.release(1, maxDaily)
}

// today returns the jobs counted for the current local date.
func (u *storymakerAgentUsage) today(maxDaily int) (int, error) {
	return u.count.today(maxDaily)
}

package server

import (
	"aurago/internal/retrogames"
	"aurago/internal/tools"
)

// publishRetroGamesTool hands the server-owned service to the retro_games tool.
func publishRetroGamesTool(svc *retrogames.Service) {
	tools.SetRetroGamesService(svc)
}

// withdrawRetroGamesTool withdraws the service before shutdown, but only while
// it is still the one this server published.
func withdrawRetroGamesTool(svc *retrogames.Service) {
	tools.WithdrawRetroGamesService(svc)
}

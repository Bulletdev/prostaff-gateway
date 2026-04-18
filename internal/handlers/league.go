package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	"prostaff-riot-gateway/internal/cache"
	"prostaff-riot-gateway/internal/riot"
)

// LeagueHandler handles league/ranked endpoints.
type LeagueHandler struct {
	deps
}

func NewLeagueHandler(rc *riot.Client, l1 *cache.Memory, l2 *cache.Redis, logger *slog.Logger) *LeagueHandler {
	return &LeagueHandler{deps{riot: rc, l1: l1, l2: l2, logger: logger}}
}

// BySummoner handles GET /riot/league/{region}/by-summoner/{summonerId}
func (h *LeagueHandler) BySummoner(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	region, summonerID := vars["region"], vars["summonerId"]
	if !validateRegion(region, w) {
		return
	}
	h.fetch(w, r,
		buildKey("league", "summoner", region, summonerID),
		"league-summoner",
		region,
		fmt.Sprintf("/lol/league/v4/entries/by-summoner/%s", summonerID),
	)
}

// ByPUUID handles GET /riot/league/{region}/by-puuid/{puuid}
func (h *LeagueHandler) ByPUUID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	region, puuid := vars["region"], vars["puuid"]
	if !validateRegion(region, w) {
		return
	}
	h.fetch(w, r,
		buildKey("league", "puuid", region, puuid),
		"league-puuid",
		region,
		fmt.Sprintf("/lol/league/v4/entries/by-puuid/%s", puuid),
	)
}

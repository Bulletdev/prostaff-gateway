package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	"prostaff-riot-gateway/internal/cache"
	"prostaff-riot-gateway/internal/riot"
)

// SummonerHandler handles summoner and account endpoints.
type SummonerHandler struct {
	deps
}

func NewSummonerHandler(rc *riot.Client, l1 *cache.Memory, l2 *cache.Redis, logger *slog.Logger) *SummonerHandler {
	return &SummonerHandler{deps{riot: rc, l1: l1, l2: l2, logger: logger}}
}

// ByPUUID handles GET /riot/summoner/{region}/by-puuid/{puuid}
func (h *SummonerHandler) ByPUUID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	region, puuid := vars["region"], vars["puuid"]
	if !validateRegion(region, w) {
		return
	}
	h.fetch(w, r,
		buildKey("summoner", "by-puuid", region, puuid),
		"summoner-by-puuid",
		region,
		fmt.Sprintf("/lol/summoner/v4/summoners/by-puuid/%s", puuid),
	)
}

// ByName handles GET /riot/summoner/{region}/by-name/{name}
func (h *SummonerHandler) ByName(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	region, name := vars["region"], vars["name"]
	if !validateRegion(region, w) {
		return
	}
	h.fetch(w, r,
		buildKey("summoner", "by-name", region, name),
		"summoner-by-name",
		region,
		fmt.Sprintf("/lol/summoner/v4/summoners/by-name/%s", name),
	)
}

// AccountByRiotID handles GET /riot/account/{region}/{riotId}/{tagline}
func (h *SummonerHandler) AccountByRiotID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	region, riotID, tagline := vars["region"], vars["riotId"], vars["tagline"]
	if !validateRegion(region, w) {
		return
	}
	h.fetch(w, r,
		buildKey("account", region, riotID, tagline),
		"account",
		region,
		fmt.Sprintf("/riot/account/v1/accounts/by-riot-id/%s/%s", riotID, tagline),
	)
}

// AccountByPUUID handles GET /riot/account/{region}/by-puuid/{puuid}
func (h *SummonerHandler) AccountByPUUID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	region, puuid := vars["region"], vars["puuid"]
	if !validateRegion(region, w) {
		return
	}
	h.fetch(w, r,
		buildKey("account", "by-puuid", region, puuid),
		"account",
		region,
		fmt.Sprintf("/riot/account/v1/accounts/by-puuid/%s", puuid),
	)
}

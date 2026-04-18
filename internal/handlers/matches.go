package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	"prostaff-riot-gateway/internal/cache"
	"prostaff-riot-gateway/internal/ratelimit"
	"prostaff-riot-gateway/internal/riot"
	"prostaff-riot-gateway/internal/webutils"
)

// MatchesHandler handles Match-V5 endpoints (uses regional routing).
type MatchesHandler struct {
	deps
}

func NewMatchesHandler(rc *riot.Client, l1 *cache.Memory, l2 *cache.Redis, logger *slog.Logger) *MatchesHandler {
	return &MatchesHandler{deps{riot: rc, l1: l1, l2: l2, logger: logger}}
}

// IDs handles GET /riot/matches/{region}/{puuid}/ids
func (h *MatchesHandler) IDs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	region, puuid := vars["region"], vars["puuid"]
	if !validateRegion(region, w) {
		return
	}

	routing, err := resolveRouting(region)
	if err != nil {
		webutils.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	q := r.URL.Query()
	count := q.Get("count")
	queue := q.Get("queue")
	start := q.Get("start")
	if count == "" {
		count = "20"
	}

	riotPath := fmt.Sprintf("/lol/match/v5/matches/by-puuid/%s/ids?count=%s", puuid, count)
	if queue != "" {
		riotPath += "&queue=" + queue
	}
	if start != "" {
		riotPath += "&start=" + start
	}

	cacheKey := buildKey("match-ids", routing, puuid,
		fmt.Sprintf("count=%s&queue=%s&start=%s", count, queue, start))

	h.fetch(w, r, cacheKey, "match-ids", routing, riotPath)
}

// Detail handles GET /riot/match/{region}/{matchId}
func (h *MatchesHandler) Detail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	region, matchID := vars["region"], vars["matchId"]
	if !validateRegion(region, w) {
		return
	}

	routing, err := resolveRouting(region)
	if err != nil {
		webutils.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	h.fetch(w, r,
		buildKey("match-detail", routing, matchID),
		"match-detail",
		routing,
		fmt.Sprintf("/lol/match/v5/matches/%s", matchID),
	)
}

func resolveRouting(region string) (string, error) {
	// If already a routing region, return as-is
	routingRegions := map[string]bool{"americas": true, "europe": true, "asia": true, "sea": true}
	if routingRegions[region] {
		return region, nil
	}
	routing, ok := ratelimit.ServerToRouting[region]
	if !ok {
		return "", fmt.Errorf("cannot resolve routing for region: %s", region)
	}
	return routing, nil
}

package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	"prostaff-riot-gateway/internal/cache"
	"prostaff-riot-gateway/internal/riot"
)

// MasteryHandler handles champion mastery endpoints.
type MasteryHandler struct {
	deps
}

func NewMasteryHandler(rc *riot.Client, l1 *cache.Memory, l2 *cache.Redis, logger *slog.Logger) *MasteryHandler {
	return &MasteryHandler{deps{riot: rc, l1: l1, l2: l2, logger: logger}}
}

// Top handles GET /riot/mastery/{region}/{puuid}/top
func (h *MasteryHandler) Top(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	region, puuid := vars["region"], vars["puuid"]
	if !validateRegion(region, w) {
		return
	}

	count := r.URL.Query().Get("count")
	if count == "" {
		count = "10"
	}

	h.fetch(w, r,
		buildKey("mastery-top", region, puuid, fmt.Sprintf("count=%s", count)),
		"mastery-top",
		region,
		fmt.Sprintf("/lol/champion-mastery/v4/champion-masteries/by-puuid/%s/top?count=%s", puuid, count),
	)
}

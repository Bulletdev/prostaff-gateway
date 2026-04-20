package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"prostaff-riot-gateway/internal/cache"
	"prostaff-riot-gateway/internal/ratelimit"
	"prostaff-riot-gateway/internal/riot"
	"prostaff-riot-gateway/internal/webutils"
)

type SummonerHandler struct {
	deps
}

func NewSummonerHandler(rc *riot.Client, l1 *cache.Memory, l2 *cache.Redis, logger *slog.Logger) *SummonerHandler {
	return &SummonerHandler{deps{riot: rc, l1: l1, l2: l2, logger: logger}}
}

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
// Deprecated: Riot removed this endpoint in 2024.
// Use ByRiotID (/riot/summoner/{region}/by-riot-id/{gameName}/{tagLine}) instead.
func (h *SummonerHandler) ByName(w http.ResponseWriter, r *http.Request) {
	webutils.ErrorJSON(w,
		fmt.Errorf("endpoint removed: use /riot/summoner/{region}/by-riot-id/{gameName}/{tagLine}"),
		http.StatusGone,
	)
}

// ByRiotID resolves gameName#tagLine in two steps:
//  1. Account-V1: gameName + tagLine → PUUID (routing region)
//  2. Summoner-V4: PUUID → summoner data (server region)
func (h *SummonerHandler) ByRiotID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	region, gameName, tagLine := vars["region"], vars["gameName"], vars["tagLine"]

	if !validateRegion(region, w) {
		return
	}

	compositeKey := buildKey("summoner", "by-riot-id", region, gameName, tagLine)

	if h.l1.IsNegative(compositeKey) {
		h.logger.Debug("cache L1 negative hit", "key", compositeKey)
		webutils.ErrorJSON(w, fmt.Errorf("not found"), http.StatusNotFound)
		return
	}

	if data, ok := h.l1.Get(compositeKey); ok {
		h.logger.Debug("cache L1 hit", "key", compositeKey)
		webutils.RawJSON(w, http.StatusOK, data)
		return
	}

	if data, err := h.l2.Get(r.Context(), compositeKey); err == nil {
		ttl := cache.TTLs["summoner-by-riot-id"]
		h.l1.Set(compositeKey, data, ttl.L1)
		h.logger.Debug("cache L2 hit", "key", compositeKey)
		webutils.RawJSON(w, http.StatusOK, data)
		return
	}

	// Account-V1 requires routing region (americas/europe/asia/sea), not the server region.
	routingRegion := ratelimit.ServerToRouting[region]
	if routingRegion == "" {
		routingRegion = region
	}

	accountData, accountStatus, err := h.riot.Do(
		r.Context(),
		routingRegion,
		fmt.Sprintf("/riot/account/v1/accounts/by-riot-id/%s/%s", gameName, tagLine),
	)
	if err != nil {
		h.logger.Warn("account lookup failed", "gameName", gameName, "tagLine", tagLine, "error", err)

		var rlErr *riot.RateLimitError
		if errors.As(err, &rlErr) && rlErr.RetryAfter != "" {
			w.Header().Set("Retry-After", rlErr.RetryAfter)
		}

		if riot.IsNotFound(err) {
			h.l1.SetNegative(compositeKey, cache.NegativeTTLs["summoner-by-riot-id"])
		}

		webutils.ErrorJSON(w, err, accountStatus)
		return
	}

	var account struct {
		PUUID string `json:"puuid"`
	}
	if err := json.Unmarshal(accountData, &account); err != nil || account.PUUID == "" {
		webutils.ErrorJSON(w, fmt.Errorf("invalid account response"), http.StatusInternalServerError)
		return
	}

	summonerData, summonerStatus, err := h.riot.Do(
		r.Context(),
		region,
		fmt.Sprintf("/lol/summoner/v4/summoners/by-puuid/%s", account.PUUID),
	)
	if err != nil {
		h.logger.Warn("summoner lookup failed", "puuid", account.PUUID, "error", err)

		var rlErr *riot.RateLimitError
		if errors.As(err, &rlErr) && rlErr.RetryAfter != "" {
			w.Header().Set("Retry-After", rlErr.RetryAfter)
		}

		webutils.ErrorJSON(w, err, summonerStatus)
		return
	}

	var summoner map[string]json.RawMessage
	if err := json.Unmarshal(summonerData, &summoner); err != nil {
		webutils.ErrorJSON(w, fmt.Errorf("invalid summoner response"), http.StatusInternalServerError)
		return
	}

	gnBytes, _ := json.Marshal(gameName)
	tlBytes, _ := json.Marshal(tagLine)
	summoner["gameName"] = gnBytes
	summoner["tagLine"] = tlBytes

	merged, err := json.Marshal(summoner)
	if err != nil {
		webutils.ErrorJSON(w, fmt.Errorf("failed to encode response"), http.StatusInternalServerError)
		return
	}

	ttl := cache.TTLs["summoner-by-riot-id"]
	h.l1.Set(compositeKey, merged, ttl.L1)

	cacheCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	h.l2.Set(cacheCtx, compositeKey, merged, ttl.L2)

	h.logger.Debug("by-riot-id resolved", "gameName", gameName, "tagLine", tagLine, "puuid", account.PUUID)
	webutils.RawJSON(w, http.StatusOK, merged)
}

// AccountByRiotID handles GET /riot/account/{region}/by-riot-id/{riotId}/{tagline}
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

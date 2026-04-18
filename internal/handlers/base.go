package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"prostaff-riot-gateway/internal/cache"
	"prostaff-riot-gateway/internal/riot"
	"prostaff-riot-gateway/internal/webutils"
)

// deps bundles shared dependencies for all handlers.
type deps struct {
	riot   *riot.Client
	l1     *cache.Memory
	l2     *cache.Redis
	logger *slog.Logger
}

// fetch executes the full cache-lookup → riot-call → cache-set pipeline.
func (d *deps) fetch(
	w http.ResponseWriter,
	r *http.Request,
	cacheKey string,
	ttlKey string,
	region string,
	riotPath string,
) {
	// L1 hit
	if data, ok := d.l1.Get(cacheKey); ok {
		d.logger.Debug("cache L1 hit", "key", cacheKey)
		webutils.RawJSON(w, http.StatusOK, data)
		return
	}

	// L2 hit
	if data, err := d.l2.Get(r.Context(), cacheKey); err == nil {
		ttl := cache.TTLs[ttlKey]
		d.l1.Set(cacheKey, data, ttl.L1)
		d.logger.Debug("cache L2 hit", "key", cacheKey)
		webutils.RawJSON(w, http.StatusOK, data)
		return
	}

	// Riot API call
	data, status, err := d.riot.Do(r.Context(), region, riotPath)
	if err != nil {
		d.logger.Warn("riot call failed", "region", region, "path", riotPath, "status", status, "error", err)
		var rlErr *riot.RateLimitError
		if errors.As(err, &rlErr) && rlErr.RetryAfter != "" {
			w.Header().Set("Retry-After", rlErr.RetryAfter)
		}
		webutils.ErrorJSON(w, err, status)
		return
	}

	ttl := cache.TTLs[ttlKey]
	d.l1.Set(cacheKey, data, ttl.L1)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	d.l2.Set(ctx, cacheKey, data, ttl.L2)

	d.logger.Debug("riot call success, cached", "key", cacheKey)
	webutils.RawJSON(w, http.StatusOK, data)
}

func buildKey(parts ...string) string {
	key := "riot-gw"
	for _, p := range parts {
		key += ":" + p
	}
	return key
}

func validateRegion(region string, w http.ResponseWriter) bool {
	allowed := []string{
		"br1", "na1", "euw1", "eun1", "kr", "jp1", "oc1", "tr1", "ru", "la1", "la2",
		"americas", "europe", "asia", "sea",
	}
	for _, r := range allowed {
		if region == r {
			return true
		}
	}
	webutils.ErrorJSON(w, fmt.Errorf("invalid region: %s", region), http.StatusBadRequest)
	return false
}

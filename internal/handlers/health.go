package handlers

import (
	"context"
	"net/http"
	"time"

	"prostaff-riot-gateway/internal/circuit"
	"prostaff-riot-gateway/internal/webutils"
)

type pinger interface {
	Ping(ctx context.Context) error
	Enabled() bool
}

// HealthHandler handles GET /health.
type HealthHandler struct {
	breakers *circuit.RegionBreakers
	redis    pinger
}

func NewHealthHandler(breakers *circuit.RegionBreakers, redis pinger) *HealthHandler {
	return &HealthHandler{breakers: breakers, redis: redis}
}

func (h *HealthHandler) Handle(w http.ResponseWriter, r *http.Request) {
	redisStatus := "ok"
	if h.redis.Enabled() {
		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
		defer cancel()
		if err := h.redis.Ping(ctx); err != nil {
			redisStatus = "unreachable"
		}
	} else {
		redisStatus = "disabled"
	}

	webutils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "ok",
		"redis":            redisStatus,
		"circuit_breakers": h.breakers.States(),
	})
}

package ratelimit

import (
	"context"

	"golang.org/x/time/rate"
)

// AllowedRegions maps Riot server regions to their base API hostnames.
var AllowedRegions = map[string]string{
	"br1":      "https://br1.api.riotgames.com",
	"na1":      "https://na1.api.riotgames.com",
	"euw1":     "https://euw1.api.riotgames.com",
	"eun1":     "https://eun1.api.riotgames.com",
	"kr":       "https://kr.api.riotgames.com",
	"jp1":      "https://jp1.api.riotgames.com",
	"oc1":      "https://oc1.api.riotgames.com",
	"tr1":      "https://tr1.api.riotgames.com",
	"ru":       "https://ru.api.riotgames.com",
	"la1":      "https://la1.api.riotgames.com",
	"la2":      "https://la2.api.riotgames.com",
	"americas": "https://americas.api.riotgames.com",
	"europe":   "https://europe.api.riotgames.com",
	"asia":     "https://asia.api.riotgames.com",
	"sea":      "https://sea.api.riotgames.com",
}

// ServerToRouting maps server regions to their Match-V5 routing region.
var ServerToRouting = map[string]string{
	"br1":  "americas",
	"na1":  "americas",
	"la1":  "americas",
	"la2":  "americas",
	"euw1": "europe",
	"eun1": "europe",
	"tr1":  "europe",
	"ru":   "europe",
	"kr":   "asia",
	"jp1":  "asia",
	"oc1":  "sea",
}

// AppLimiter enforces the Riot app-level rate limit globally across all regions.
// It combines two token buckets to cover both the per-second and per-2-minute windows
// that Riot enforces on every API key.
type AppLimiter struct {
	perSecond *rate.Limiter
	per2Min   *rate.Limiter
}

// NewAppLimiter creates an AppLimiter with independent per-second and per-2-minute buckets.
// rps and burst configure the 1-second window; per2Min configures the 2-minute window.
func NewAppLimiter(rps float64, burst int, per2Min int) *AppLimiter {
	return &AppLimiter{
		perSecond: rate.NewLimiter(rate.Limit(rps), burst),
		// Riot's 2-min limit modelled as a token bucket: refill rate = per2Min/120s, burst = per2Min.
		// This is a safe approximation: it prevents bursting past the 2-min cap while
		// allowing the full burst at startup (matching Riot's sliding-window behaviour).
		per2Min: rate.NewLimiter(rate.Limit(float64(per2Min)/120.0), per2Min),
	}
}

// Wait blocks until both rate windows have capacity, or ctx is cancelled.
func (al *AppLimiter) Wait(ctx context.Context) error {
	if err := al.perSecond.Wait(ctx); err != nil {
		return err
	}
	return al.per2Min.Wait(ctx)
}

// ValidRegion returns true if the region is in the allowed list.
func ValidRegion(region string) bool {
	_, ok := AllowedRegions[region]
	return ok
}

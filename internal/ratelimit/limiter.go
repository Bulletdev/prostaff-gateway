package ratelimit

import (
	"context"
	"sync"

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

// RegionLimiter holds one token bucket per Riot region.
type RegionLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
	rps      float64
	burst    int
}

func NewRegionLimiter(rps float64, burst int) *RegionLimiter {
	return &RegionLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rps,
		burst:    burst,
	}
}

// Wait blocks until a token is available for the given region, or ctx is cancelled.
func (rl *RegionLimiter) Wait(ctx context.Context, region string) error {
	return rl.get(region).Wait(ctx)
}

func (rl *RegionLimiter) get(region string) *rate.Limiter {
	rl.mu.RLock()
	l, ok := rl.limiters[region]
	rl.mu.RUnlock()
	if ok {
		return l
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()
	if l, ok = rl.limiters[region]; ok {
		return l
	}
	l = rate.NewLimiter(rate.Limit(rl.rps), rl.burst)
	rl.limiters[region] = l
	return l
}

// ValidRegion returns true if the region is in the allowed list.
func ValidRegion(region string) bool {
	_, ok := AllowedRegions[region]
	return ok
}

package cache

import "time"

type TTL struct {
	L1 time.Duration
	L2 time.Duration
}

// TTLs defines cache durations for each Riot resource type.
var TTLs = map[string]TTL{
	"summoner-by-riot-id": {L1: 10 * time.Minute, L2: 10 * time.Minute},
	"summoner-by-puuid":   {L1: 10 * time.Minute, L2: 10 * time.Minute},
	"summoner-by-name":    {L1: 5 * time.Minute, L2: 5 * time.Minute},
	"account":             {L1: time.Hour, L2: time.Hour},
	"league-summoner":     {L1: 5 * time.Minute, L2: 5 * time.Minute},
	"league-puuid":        {L1: 5 * time.Minute, L2: 5 * time.Minute},
	"match-ids":           {L1: 5 * time.Minute, L2: 5 * time.Minute},
	"match-detail":        {L1: time.Hour, L2: 24 * time.Hour},
	"mastery-top":         {L1: 30 * time.Minute, L2: time.Hour},
}

// NegativeTTLs defines how long a confirmed 404 is cached in L1 per resource type.
// Only cached in L1 (not L2) since these are short-lived and service-specific.
var NegativeTTLs = map[string]time.Duration{
	"summoner-by-riot-id": 30 * time.Second,
	"summoner-by-puuid":   2 * time.Minute,
	"summoner-by-name":    30 * time.Second,
	"account":             2 * time.Minute,
	"league-summoner":     30 * time.Second,
	"league-puuid":        30 * time.Second,
	"match-ids":           30 * time.Second,
	"match-detail":        5 * time.Minute,
	"mastery-top":         30 * time.Second,
}

package cache

import "time"

type TTL struct {
	L1 time.Duration
	L2 time.Duration
}

// TTLs defines cache durations for each Riot resource type.
var TTLs = map[string]TTL{
	"summoner-by-puuid": {L1: 10 * time.Minute, L2: 10 * time.Minute},
	"summoner-by-name":  {L1: 5 * time.Minute, L2: 5 * time.Minute},
	"account":           {L1: time.Hour, L2: time.Hour},
	"league-summoner":   {L1: 5 * time.Minute, L2: 5 * time.Minute},
	"league-puuid":      {L1: 5 * time.Minute, L2: 5 * time.Minute},
	"match-ids":         {L1: 5 * time.Minute, L2: 5 * time.Minute},
	"match-detail":      {L1: time.Hour, L2: 24 * time.Hour},
	"mastery-top":       {L1: 30 * time.Minute, L2: time.Hour},
}

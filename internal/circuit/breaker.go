package circuit

import (
	"sync"
	"time"
)

type State int

const (
	StateClosed   State = iota // normal operation
	StateOpen                  // rejecting requests
	StateHalfOpen              // probing recovery
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

type Breaker struct {
	mu          sync.Mutex
	threshold   int
	cbTimeout   time.Duration
	cooldown    time.Duration
	failures    int
	windowStart time.Time
	openedAt    time.Time
	state       State
}

func New(threshold int, cbTimeout, cooldown time.Duration) *Breaker {
	return &Breaker{
		threshold: threshold,
		cbTimeout: cbTimeout,
		cooldown:  cooldown,
		state:     StateClosed,
	}
}

func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(b.openedAt) >= b.cooldown {
			b.state = StateHalfOpen
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.state = StateClosed
}

func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if b.windowStart.IsZero() || now.Sub(b.windowStart) > b.cbTimeout {
		b.failures = 0
		b.windowStart = now
	}

	b.failures++
	if b.failures >= b.threshold {
		b.state = StateOpen
		b.openedAt = now
	}
}

func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// RegionBreakers manages one Breaker per (region, endpoint) composite key.
// This prevents a failure in one endpoint group from tripping the circuit for
// healthy endpoints in the same region.
type RegionBreakers struct {
	mu        sync.RWMutex
	breakers  map[string]*Breaker
	threshold int
	cbTimeout time.Duration
	cooldown  time.Duration
}

func NewRegionBreakers(threshold int, cbTimeout, cooldown time.Duration) *RegionBreakers {
	return &RegionBreakers{
		breakers:  make(map[string]*Breaker),
		threshold: threshold,
		cbTimeout: cbTimeout,
		cooldown:  cooldown,
	}
}

// Get returns the breaker for (region, endpoint). Key: "region:endpoint" e.g. "br1:summoner".
func (rb *RegionBreakers) Get(region, endpoint string) *Breaker {
	key := region + ":" + endpoint
	rb.mu.RLock()
	b, ok := rb.breakers[key]
	rb.mu.RUnlock()
	if ok {
		return b
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()
	if b, ok = rb.breakers[key]; ok {
		return b
	}
	b = New(rb.threshold, rb.cbTimeout, rb.cooldown)
	rb.breakers[key] = b
	return b
}

func (rb *RegionBreakers) States() map[string]string {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	out := make(map[string]string, len(rb.breakers))
	for key, b := range rb.breakers {
		out[key] = b.State().String()
	}
	return out
}

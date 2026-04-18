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
	mu         sync.Mutex
	threshold  int
	cbTimeout  time.Duration
	cooldown   time.Duration
	failures   int
	windowStart time.Time
	openedAt   time.Time
	state      State
}

func New(threshold int, cbTimeout, cooldown time.Duration) *Breaker {
	return &Breaker{
		threshold: threshold,
		cbTimeout: cbTimeout,
		cooldown:  cooldown,
		state:     StateClosed,
	}
}

// Allow returns false when the circuit is open and the cooldown has not elapsed.
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

// RecordSuccess resets the breaker on a successful call.
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.state = StateClosed
}

// RecordFailure increments the failure count and opens the circuit if threshold reached.
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

// State returns the current circuit state (safe for external reads).
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// RegionBreakers manages one Breaker per Riot region.
type RegionBreakers struct {
	mu       sync.RWMutex
	breakers map[string]*Breaker
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

func (rb *RegionBreakers) Get(region string) *Breaker {
	rb.mu.RLock()
	b, ok := rb.breakers[region]
	rb.mu.RUnlock()
	if ok {
		return b
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()
	if b, ok = rb.breakers[region]; ok {
		return b
	}
	b = New(rb.threshold, rb.cbTimeout, rb.cooldown)
	rb.breakers[region] = b
	return b
}

// States returns a snapshot of every known region's state (for /health).
func (rb *RegionBreakers) States() map[string]string {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	out := make(map[string]string, len(rb.breakers))
	for region, b := range rb.breakers {
		out[region] = b.State().String()
	}
	return out
}

package riot

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"prostaff-riot-gateway/internal/circuit"
	"prostaff-riot-gateway/internal/ratelimit"
)

// RateLimitError is returned when Riot responds with 429. It carries the Retry-After
// value from the Riot header so callers can propagate it to their own clients.
type RateLimitError struct {
	RetryAfter string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("riot api rate limited, retry after %s seconds", e.RetryAfter)
}

// Client calls the Riot Games API with rate limiting and circuit breaking.
type Client struct {
	http     *http.Client
	apiKey   string
	limiter  *ratelimit.AppLimiter
	breakers *circuit.RegionBreakers
	logger   *slog.Logger
}

func NewClient(
	timeout time.Duration,
	apiKey string,
	limiter *ratelimit.AppLimiter,
	breakers *circuit.RegionBreakers,
	logger *slog.Logger,
) *Client {
	return &Client{
		http:     &http.Client{Timeout: timeout},
		apiKey:   apiKey,
		limiter:  limiter,
		breakers: breakers,
		logger:   logger,
	}
}

// Do sends a GET request to Riot API for the given region and path.
// Returns the response body, HTTP status code, and any error.
func (c *Client) Do(ctx context.Context, region, path string) ([]byte, int, error) {
	baseURL, ok := ratelimit.AllowedRegions[region]
	if !ok {
		return nil, http.StatusBadRequest, fmt.Errorf("unsupported region: %s", region)
	}

	breaker := c.breakers.Get(region)
	if !breaker.Allow() {
		c.logger.Warn("circuit open", "region", region)
		return nil, http.StatusServiceUnavailable, fmt.Errorf("riot api circuit open for region %s", region)
	}

	if err := c.limiter.Wait(ctx); err != nil {
		return nil, http.StatusGatewayTimeout, fmt.Errorf("rate limit wait cancelled: %w", err)
	}

	url := baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	req.Header.Set("X-Riot-Token", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		breaker.RecordFailure()
		c.logger.Error("riot api request failed", "region", region, "path", path, "error", err)
		return nil, http.StatusBadGateway, fmt.Errorf("riot api error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		breaker.RecordFailure()
		return nil, http.StatusBadGateway, fmt.Errorf("failed to read riot response: %w", err)
	}

	// 429 is a rate limit signal, not an infrastructure failure — do not trip the circuit breaker.
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get("Retry-After")
		c.logger.Warn("riot api rate limited", "region", region, "retry_after", retryAfter)
		return nil, http.StatusTooManyRequests, &RateLimitError{RetryAfter: retryAfter}
	}

	if resp.StatusCode >= 500 {
		breaker.RecordFailure()
		c.logger.Warn("riot api server error", "region", region, "riot_status", resp.StatusCode)
		return nil, http.StatusBadGateway, fmt.Errorf("riot api returned %d", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, http.StatusNotFound, fmt.Errorf("not found")
	}

	breaker.RecordSuccess()
	return body, http.StatusOK, nil
}


package riot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"prostaff-riot-gateway/internal/circuit"
	"prostaff-riot-gateway/internal/ratelimit"
)

const maxRetries = 3

var retryDelays = []time.Duration{0, 100 * time.Millisecond, 500 * time.Millisecond}

// RateLimitError carries the Retry-After header from a Riot 429 so callers can propagate it.
type RateLimitError struct {
	RetryAfter string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("riot api rate limited, retry after %s seconds", e.RetryAfter)
}

// NotFoundError signals a Riot 404; callers use errors.As to apply negative caching.
type NotFoundError struct{}

func (e *NotFoundError) Error() string { return "not found" }

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

// Do retries 5xx responses up to maxRetries times before tripping the circuit breaker.
func (c *Client) Do(ctx context.Context, region, path string) ([]byte, int, error) {
	baseURL, ok := ratelimit.AllowedRegions[region]
	if !ok {
		return nil, http.StatusBadRequest, fmt.Errorf("unsupported region: %s", region)
	}

	breaker := c.breakers.Get(region, endpointGroup(path))
	if !breaker.Allow() {
		c.logger.Warn("circuit open", "region", region, "endpoint", endpointGroup(path))
		return nil, http.StatusServiceUnavailable, fmt.Errorf("riot api circuit open for region %s", region)
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, http.StatusGatewayTimeout, ctx.Err()
			case <-time.After(retryDelays[attempt]):
			}
			c.logger.Info("retrying riot request", "attempt", attempt+1, "region", region, "path", path)
		}

		if err := c.limiter.Wait(ctx); err != nil {
			return nil, http.StatusGatewayTimeout, fmt.Errorf("rate limit wait cancelled: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		req.Header.Set("X-Riot-Token", c.apiKey)

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("riot api error: %w", err)
			c.logger.Warn("riot request failed", "attempt", attempt+1, "region", region, "path", path, "error", err)
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("failed to read riot response: %w", readErr)
			continue
		}

		// 429 does not retry and does not trip the circuit breaker.
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := resp.Header.Get("Retry-After")
			c.logger.Warn("riot api rate limited", "region", region, "retry_after", retryAfter)
			return nil, http.StatusTooManyRequests, &RateLimitError{RetryAfter: retryAfter}
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("riot api returned %d", resp.StatusCode)
			c.logger.Warn("riot api 5xx, will retry", "attempt", attempt+1, "region", region, "status", resp.StatusCode)
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			return nil, http.StatusNotFound, &NotFoundError{}
		}

		if resp.StatusCode >= 400 {
			return nil, resp.StatusCode, fmt.Errorf("riot api returned %d", resp.StatusCode)
		}

		breaker.RecordSuccess()
		return body, http.StatusOK, nil
	}

	breaker.RecordFailure()
	c.logger.Warn("riot api failed after all retries", "region", region, "path", path, "error", lastErr)
	return nil, http.StatusBadGateway, lastErr
}

func endpointGroup(path string) string {
	switch {
	case strings.HasPrefix(path, "/lol/summoner"):
		return "summoner"
	case strings.HasPrefix(path, "/riot/account"):
		return "account"
	case strings.HasPrefix(path, "/lol/league"):
		return "league"
	case strings.HasPrefix(path, "/lol/match"):
		return "match"
	case strings.HasPrefix(path, "/lol/champion-mastery"):
		return "mastery"
	default:
		return "other"
	}
}

func IsNotFound(err error) bool {
	var nf *NotFoundError
	return errors.As(err, &nf)
}

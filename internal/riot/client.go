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

// Client calls the Riot Games API with rate limiting and circuit breaking.
type Client struct {
	http     *http.Client
	apiKey   string
	limiter  *ratelimit.RegionLimiter
	breakers *circuit.RegionBreakers
	logger   *slog.Logger
}

func NewClient(
	timeout time.Duration,
	apiKey string,
	limiter *ratelimit.RegionLimiter,
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

	if err := c.limiter.Wait(ctx, region); err != nil {
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

	status := mapRiotStatus(resp.StatusCode)

	if resp.StatusCode >= 500 || resp.StatusCode == 429 {
		breaker.RecordFailure()
		c.logger.Warn("riot api returned error", "region", region, "riot_status", resp.StatusCode)
		return nil, status, fmt.Errorf("riot api returned %d", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, http.StatusNotFound, fmt.Errorf("not found")
	}

	breaker.RecordSuccess()
	return body, http.StatusOK, nil
}

func mapRiotStatus(riotStatus int) int {
	switch {
	case riotStatus == http.StatusOK:
		return http.StatusOK
	case riotStatus == http.StatusNotFound:
		return http.StatusNotFound
	case riotStatus == 429:
		return 429
	case riotStatus >= 500:
		return http.StatusBadGateway
	default:
		return riotStatus
	}
}

// Package wheretheiss is the library behind the wheretheiss command line:
// the HTTP client, request shaping, and the typed data models for the
// Where The ISS At API (api.wheretheiss.at).
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests, and retries transient failures (429, 5xx).
package wheretheiss

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Host is the API hostname this client talks to.
const Host = "api.wheretheiss.at"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// ISSID is the NORAD catalog number for the International Space Station.
const ISSID = 25544

// Config holds tuneable settings for the client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns production-ready defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		Rate:      500 * time.Millisecond,
		Timeout:   15 * time.Second,
		Retries:   3,
		UserAgent: "wheretheiss-cli/0.1 (tamnd87@gmail.com)",
	}
}

// Client talks to the Where The ISS At API over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	BaseURL   string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with sensible defaults.
func NewClient() *Client {
	cfg := DefaultConfig()
	return &Client{
		HTTP:      &http.Client{Timeout: cfg.Timeout},
		UserAgent: cfg.UserAgent,
		BaseURL:   cfg.BaseURL,
		Rate:      cfg.Rate,
		Retries:   cfg.Retries,
	}
}

// Position is a single ISS position record, as returned by the API.
type Position struct {
	Name       string  `json:"name"       kit:"id"`
	ID         int     `json:"id"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	Altitude   float64 `json:"altitude"`
	Velocity   float64 `json:"velocity"`
	Visibility string  `json:"visibility"`
	Timestamp  int64   `json:"timestamp"`
	Units      string  `json:"units"`
}

// CurrentPosition fetches the live ISS position.
func (c *Client) CurrentPosition(ctx context.Context) (*Position, error) {
	url := fmt.Sprintf("%s/v1/satellites/%d", c.BaseURL, ISSID)
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var p Position
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("decode position: %w", err)
	}
	return &p, nil
}

// HistoricalPositions fetches ISS positions at the given Unix timestamps.
func (c *Client) HistoricalPositions(ctx context.Context, timestamps []int64) ([]*Position, error) {
	if len(timestamps) == 0 {
		return nil, fmt.Errorf("at least one timestamp required")
	}
	parts := make([]string, len(timestamps))
	for i, ts := range timestamps {
		parts[i] = strconv.FormatInt(ts, 10)
	}
	url := fmt.Sprintf("%s/v1/satellites/%d/positions?timestamps=%s",
		c.BaseURL, ISSID, strings.Join(parts, ","))
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var positions []*Position
	if err := json.Unmarshal(body, &positions); err != nil {
		return nil, fmt.Errorf("decode positions: %w", err)
	}
	return positions, nil
}

// Get fetches the URL and returns the response body. It paces and retries
// according to the client's settings.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

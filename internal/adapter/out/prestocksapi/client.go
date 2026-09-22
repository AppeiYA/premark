package prestocksapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.MarketDataSource = (*Client)(nil)

type tokenDTO struct {
	Name             string  `json:"name"`
	Symbol           string  `json:"symbol"`
	Description      string  `json:"description"`
	Image            string  `json:"image"`
	ExternalURL      string  `json:"external_url"`
	ContractAddress  string  `json:"contract_address"`
	MarkPrice        float64 `json:"markPrice"`
	MarkValuation    float64 `json:"markValuation"`
	TokenPrice       float64 `json:"tokenPrice"`
	ImpliedValuation float64 `json:"impliedValuation"`
	Supply           float64 `json:"supply"`
}

type Client struct {
	baseURL  string
	hc       *http.Client
	clock    ports.Clock
	cacheTTL time.Duration
	mu       sync.Mutex
	cached   []domain.MarketSnapshot
	cachedAt time.Time
}

func New(baseURL string, hc *http.Client, clock ports.Clock, cacheTTL time.Duration) *Client {
	if baseURL == "" {
		baseURL = "https://prestocks.com/api/prestocks"
	}
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{
		baseURL:  baseURL,
		hc:       hc,
		clock:    clock,
		cacheTTL: cacheTTL,
	}
}

func (c *Client) FetchAll(ctx context.Context) ([]domain.MarketSnapshot, error) {
	c.mu.Lock()
	if c.cacheTTL > 0 && len(c.cached) > 0 && c.clock.Now().Sub(c.cachedAt) < c.cacheTTL {
		res := make([]domain.MarketSnapshot, len(c.cached))
		copy(res, c.cached)
		c.mu.Unlock()
		return res, nil
	}
	c.mu.Unlock()

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, c.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrUpstream, err)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrUpstream, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: prestocks API status %d", domain.ErrUpstream, resp.StatusCode)
	}

	var dtos []tokenDTO
	if err := json.NewDecoder(resp.Body).Decode(&dtos); err != nil {
		return nil, fmt.Errorf("%w: json decode failed: %w", domain.ErrUpstream, err)
	}

	now := c.clock.Now()
	snapshots := make([]domain.MarketSnapshot, 0, len(dtos))
	for _, dto := range dtos {
		sym, err := domain.ParseSymbol(dto.Symbol)
		if err != nil {
			// Skip entries whose symbol fails ParseSymbol
			continue
		}
		snapshots = append(snapshots, domain.MarketSnapshot{
			Token: domain.Token{
				Symbol:      sym,
				Name:        dto.Name,
				Description: dto.Description,
				Mint:        dto.ContractAddress,
			},
			TokenPrice:       dto.TokenPrice,
			MarkPrice:        dto.MarkPrice,
			Supply:           dto.Supply,
			MarkValuation:    dto.MarkValuation,
			ImpliedValuation: dto.ImpliedValuation,
			ObservedAt:       now,
		})
	}

	if c.cacheTTL > 0 {
		c.mu.Lock()
		c.cached = snapshots
		c.cachedAt = now
		c.mu.Unlock()
	}

	res := make([]domain.MarketSnapshot, len(snapshots))
	copy(res, snapshots)
	return res, nil
}

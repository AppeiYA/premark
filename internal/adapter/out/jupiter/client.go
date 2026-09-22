package jupiter

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.QuoteProvider = (*Client)(nil)

type Config struct {
	BaseURL          string // default https://lite-api.jup.ag/swap/v1
	APIKey           string // optional, sent as header x-api-key when non-empty
	USDCMint         string // default EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v
	SlippageBps      int    // default 50
	ImpactIsFraction bool   // default true, see note
}

// MintInfoReader is defined INSIDE the jupiter package (adapters must not import each other).
type MintInfoReader interface {
	MintInfo(ctx context.Context, mint string) (MintInfo, error)
}

type MintInfo struct {
	Decimals   int
	Multiplier float64 // ScaledUiAmount multiplier currently in effect; 1 if the mint has no such extension
}

type Client struct {
	cfg   Config
	hc    *http.Client
	clock ports.Clock
	mints MintInfoReader
}

func New(cfg Config, hc *http.Client, clock ports.Clock, mints MintInfoReader) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://lite-api.jup.ag/swap/v1"
	}
	if cfg.USDCMint == "" {
		cfg.USDCMint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	}
	if cfg.SlippageBps == 0 {
		cfg.SlippageBps = 50
	}
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{
		cfg:   cfg,
		hc:    hc,
		clock: clock,
		mints: mints,
	}
}

type quoteResponse struct {
	InAmount       string `json:"inAmount"`
	OutAmount      string `json:"outAmount"`
	PriceImpactPct string `json:"priceImpactPct"`
}

func (c *Client) QuoteBuy(ctx context.Context, token domain.Token, budget domain.USDC) (domain.Quote, error) {
	urlStr := fmt.Sprintf("%s/quote?inputMint=%s&outputMint=%s&amount=%d&slippageBps=%d",
		c.cfg.BaseURL, c.cfg.USDCMint, token.Mint, int64(budget), c.cfg.SlippageBps)

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, urlStr, nil)
	if err != nil {
		return domain.Quote{}, fmt.Errorf("%w: %w", domain.ErrUpstream, err)
	}

	if c.cfg.APIKey != "" {
		req.Header.Set("x-api-key", c.cfg.APIKey)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return domain.Quote{}, fmt.Errorf("%w: %w", domain.ErrUpstream, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return domain.Quote{}, fmt.Errorf("%w: jupiter server error status %d", domain.ErrUpstream, resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return domain.Quote{}, fmt.Errorf("%w: jupiter client error status %d", domain.ErrQuoteUnavailable, resp.StatusCode)
	}

	var dto quoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&dto); err != nil {
		return domain.Quote{}, fmt.Errorf("%w: decode quote response: %w", domain.ErrQuoteUnavailable, err)
	}

	outRaw, err := strconv.ParseFloat(dto.OutAmount, 64)
	if err != nil || outRaw <= 0 || math.IsNaN(outRaw) || math.IsInf(outRaw, 0) {
		return domain.Quote{}, fmt.Errorf("%w: invalid outAmount %q", domain.ErrQuoteUnavailable, dto.OutAmount)
	}

	impactVal, err := strconv.ParseFloat(dto.PriceImpactPct, 64)
	if err != nil || math.IsNaN(impactVal) || math.IsInf(impactVal, 0) {
		return domain.Quote{}, fmt.Errorf("%w: invalid priceImpactPct %q", domain.ErrQuoteUnavailable, dto.PriceImpactPct)
	}

	var impactBps float64
	if c.cfg.ImpactIsFraction {
		impactBps = impactVal * 10000.0
	} else {
		impactBps = impactVal * 100.0
	}

	mintInfo, err := c.mints.MintInfo(ctx, token.Mint)
	if err != nil {
		return domain.Quote{}, fmt.Errorf("%w: mint info failed: %w", domain.ErrUpstream, err)
	}

	outTokens := (outRaw / math.Pow10(mintInfo.Decimals)) * mintInfo.Multiplier
	if outTokens <= 0 || math.IsNaN(outTokens) || math.IsInf(outTokens, 0) {
		return domain.Quote{}, fmt.Errorf("%w: invalid outTokens calculated: %v", domain.ErrQuoteUnavailable, outTokens)
	}

	swapURL := fmt.Sprintf("https://jup.ag/swap/%s-%s", c.cfg.USDCMint, token.Mint)

	return domain.Quote{
		Symbol:         token.Symbol,
		InputMint:      c.cfg.USDCMint,
		OutputMint:     token.Mint,
		InAmount:       budget,
		OutTokens:      outTokens,
		PriceImpactBps: impactBps,
		SwapURL:        swapURL,
		QuotedAt:       c.clock.Now(),
	}, nil
}

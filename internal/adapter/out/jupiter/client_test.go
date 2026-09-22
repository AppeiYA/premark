package jupiter_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"premark/internal/adapter/out/jupiter"
	"premark/internal/domain"
	"premark/internal/testsupport"
)

type fakeMintReader struct {
	decimals   int
	multiplier float64
	err        error
}

func (f *fakeMintReader) MintInfo(ctx context.Context, mint string) (jupiter.MintInfo, error) {
	if f.err != nil {
		return jupiter.MintInfo{}, f.err
	}
	return jupiter.MintInfo{
		Decimals:   f.decimals,
		Multiplier: f.multiplier,
	}, nil
}

func TestJupiter_Client(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	clock := testsupport.NewFixedClock(now)

	token := domain.Token{
		Symbol: "ANTHROPIC",
		Mint:   "MINT_ANTHROPIC",
	}

	t.Run("successful quote with ImpactIsFraction=true", func(t *testing.T) {
		var receivedKey string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedKey = r.Header.Get("x-api-key")
			w.Header().Set("Content-Type", "application/json")
			// 9574980 raw units, priceImpactPct = 0.0065837
			fmt.Fprintf(w, `{"inAmount":"10000000","outAmount":"1000000000","priceImpactPct":"0.0065837"}`)
		}))
		defer ts.Close()

		mints := &fakeMintReader{decimals: 9, multiplier: 1.5}

		cfg := jupiter.Config{
			BaseURL:          ts.URL,
			APIKey:           "secret-key",
			USDCMint:         "USDC_MINT",
			SlippageBps:      50,
			ImpactIsFraction: true,
		}

		client := jupiter.New(cfg, ts.Client(), clock, mints)
		quote, err := client.QuoteBuy(ctx, token, 10*domain.USDCUnit)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if receivedKey != "secret-key" {
			t.Errorf("expected x-api-key header 'secret-key', got %q", receivedKey)
		}

		// 1000000000 raw / 10^9 * 1.5 = 1.5 tokens
		if math.Abs(quote.OutTokens-1.5) > 1e-9 {
			t.Errorf("expected OutTokens=1.5, got %v", quote.OutTokens)
		}

		// 0.0065837 * 10000 = 65.837 bps
		if math.Abs(quote.PriceImpactBps-65.837) > 1e-3 {
			t.Errorf("expected PriceImpactBps=65.837, got %v", quote.PriceImpactBps)
		}

		expectedSwapURL := "https://jup.ag/swap/USDC_MINT-MINT_ANTHROPIC"
		if quote.SwapURL != expectedSwapURL {
			t.Errorf("expected SwapURL=%s, got %s", expectedSwapURL, quote.SwapURL)
		}
		if !quote.QuotedAt.Equal(now) {
			t.Errorf("expected QuotedAt=%v, got %v", now, quote.QuotedAt)
		}
	})

	t.Run("ImpactIsFraction=false multiplies by 100", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"inAmount":"10000000","outAmount":"1000000000","priceImpactPct":"0.65837"}`)
		}))
		defer ts.Close()

		mints := &fakeMintReader{decimals: 9, multiplier: 1.0}

		cfg := jupiter.Config{
			BaseURL:          ts.URL,
			ImpactIsFraction: false,
		}

		client := jupiter.New(cfg, ts.Client(), clock, mints)
		quote, err := client.QuoteBuy(ctx, token, 10*domain.USDCUnit)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// 0.65837 * 100 = 65.837 bps
		if math.Abs(quote.PriceImpactBps-65.837) > 1e-3 {
			t.Errorf("expected PriceImpactBps=65.837, got %v", quote.PriceImpactBps)
		}
	})

	t.Run("4xx response returns ErrQuoteUnavailable", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer ts.Close()

		client := jupiter.New(jupiter.Config{BaseURL: ts.URL}, ts.Client(), clock, &fakeMintReader{decimals: 9, multiplier: 1.0})
		_, err := client.QuoteBuy(ctx, token, 10*domain.USDCUnit)
		if !errors.Is(err, domain.ErrQuoteUnavailable) {
			t.Errorf("expected ErrQuoteUnavailable for 400 status, got %v", err)
		}
	})

	t.Run("5xx response returns ErrUpstream", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		client := jupiter.New(jupiter.Config{BaseURL: ts.URL}, ts.Client(), clock, &fakeMintReader{decimals: 9, multiplier: 1.0})
		_, err := client.QuoteBuy(ctx, token, 10*domain.USDCUnit)
		if !errors.Is(err, domain.ErrUpstream) {
			t.Errorf("expected ErrUpstream for 500 status, got %v", err)
		}
	})

	t.Run("invalid outAmount returns ErrQuoteUnavailable", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `{"inAmount":"10000000","outAmount":"0","priceImpactPct":"0"}`)
		}))
		defer ts.Close()

		client := jupiter.New(jupiter.Config{BaseURL: ts.URL}, ts.Client(), clock, &fakeMintReader{decimals: 9, multiplier: 1.0})
		_, err := client.QuoteBuy(ctx, token, 10*domain.USDCUnit)
		if !errors.Is(err, domain.ErrQuoteUnavailable) {
			t.Errorf("expected ErrQuoteUnavailable for outAmount=0, got %v", err)
		}
	})

	t.Run("mint reader failure returns ErrUpstream", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `{"inAmount":"10000000","outAmount":"1000000","priceImpactPct":"0"}`)
		}))
		defer ts.Close()

		mints := &fakeMintReader{err: errors.New("solana rpc error")}
		client := jupiter.New(jupiter.Config{BaseURL: ts.URL}, ts.Client(), clock, mints)
		_, err := client.QuoteBuy(ctx, token, 10*domain.USDCUnit)
		if !errors.Is(err, domain.ErrUpstream) {
			t.Errorf("expected ErrUpstream when mint reader fails, got %v", err)
		}
	})
}

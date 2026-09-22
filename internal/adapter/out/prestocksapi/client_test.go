package prestocksapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"premark/internal/adapter/out/prestocksapi"
	"premark/internal/domain"
	"premark/internal/testsupport"
)

func TestPreStocksAPI_Client(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	clock := testsupport.NewFixedClock(now)

	sampleJSON := `[
		{
			"name": "Anthropic PreStocks",
			"symbol": "ANTHROPIC",
			"description": "Anthropic shares",
			"contract_address": "MINT_ANTHROPIC",
			"markPrice": 1050.40,
			"markValuation": 10504000.0,
			"tokenPrice": 1046.93,
			"impliedValuation": 10469300.0,
			"supply": 10000.0
		},
		{
			"name": "Invalid Symbol",
			"symbol": "!",
			"description": "Bad symbol",
			"contract_address": "MINT_BAD",
			"markPrice": 100,
			"tokenPrice": 100
		}
	]`

	t.Run("successful fetch and skip invalid symbol", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(sampleJSON))
		}))
		defer ts.Close()

		client := prestocksapi.New(ts.URL, ts.Client(), clock, 0)
		snaps, err := client.FetchAll(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(snaps) != 1 {
			t.Fatalf("expected 1 valid snapshot (1 invalid skipped), got %d", len(snaps))
		}

		snap := snaps[0]
		if snap.Token.Symbol != "ANTHROPIC" {
			t.Errorf("expected ANTHROPIC, got %s", snap.Token.Symbol)
		}
		if snap.Token.Mint != "MINT_ANTHROPIC" {
			t.Errorf("expected MINT_ANTHROPIC, got %s", snap.Token.Mint)
		}
		if snap.TokenPrice != 1046.93 || snap.MarkPrice != 1050.40 {
			t.Errorf("unexpected prices: token=%v mark=%v", snap.TokenPrice, snap.MarkPrice)
		}
		if !snap.ObservedAt.Equal(now) {
			t.Errorf("expected ObservedAt=%v, got %v", now, snap.ObservedAt)
		}
	})

	t.Run("caching with cacheTTL", func(t *testing.T) {
		var hitCount int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hitCount, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(sampleJSON))
		}))
		defer ts.Close()

		client := prestocksapi.New(ts.URL, ts.Client(), clock, 10*time.Second)

		// First call
		snaps1, err := client.FetchAll(ctx)
		if err != nil || len(snaps1) != 1 {
			t.Fatalf("call 1 failed: %v", err)
		}
		if atomic.LoadInt32(&hitCount) != 1 {
			t.Fatalf("expected 1 server hit, got %d", atomic.LoadInt32(&hitCount))
		}

		// Second call at +5s (within TTL) should not hit HTTP server
		clock.Advance(5 * time.Second)
		snaps2, err := client.FetchAll(ctx)
		if err != nil || len(snaps2) != 1 {
			t.Fatalf("call 2 failed: %v", err)
		}
		if atomic.LoadInt32(&hitCount) != 1 {
			t.Errorf("expected still 1 server hit due to caching, got %d", atomic.LoadInt32(&hitCount))
		}

		// Third call at +15s (after TTL) hits server again
		clock.Advance(10 * time.Second)
		snaps3, err := client.FetchAll(ctx)
		if err != nil || len(snaps3) != 1 {
			t.Fatalf("call 3 failed: %v", err)
		}
		if atomic.LoadInt32(&hitCount) != 2 {
			t.Errorf("expected 2 server hits after TTL expiration, got %d", atomic.LoadInt32(&hitCount))
		}
	})

	t.Run("non-200 HTTP status returns wrapped ErrUpstream", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer ts.Close()

		client := prestocksapi.New(ts.URL, ts.Client(), clock, 0)
		_, err := client.FetchAll(ctx)
		if !errors.Is(err, domain.ErrUpstream) {
			t.Errorf("expected ErrUpstream for 503, got %v", err)
		}
	})

	t.Run("malformed json returns wrapped ErrUpstream", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("not valid json"))
		}))
		defer ts.Close()

		client := prestocksapi.New(ts.URL, ts.Client(), clock, 0)
		_, err := client.FetchAll(ctx)
		if !errors.Is(err, domain.ErrUpstream) {
			t.Errorf("expected ErrUpstream for bad json, got %v", err)
		}
	})
}

package solanarpc_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"premark/internal/adapter/out/solanarpc"
	"premark/internal/domain"
	"premark/internal/testsupport"
)

func TestSolanaRPC_Client(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	clock := testsupport.NewFixedClock(now)

	t.Run("standard mint without scaledUiAmount extension", func(t *testing.T) {
		respJSON := `{
			"jsonrpc": "2.0",
			"id": 1,
			"result": {
				"value": {
					"data": {
						"parsed": {
							"info": {
								"decimals": 9,
								"extensions": []
							}
						}
					}
				}
			}
		}`

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(respJSON))
		}))
		defer ts.Close()

		client := solanarpc.New(ts.URL, ts.Client(), clock, 60*time.Second)
		info, err := client.MintInfo(ctx, "MINT_STANDARD")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.Decimals != 9 {
			t.Errorf("expected Decimals=9, got %d", info.Decimals)
		}
		if info.Multiplier != 1.0 {
			t.Errorf("expected Multiplier=1.0, got %v", info.Multiplier)
		}
	})

	t.Run("mint with scaledUiAmountConfig multiplier", func(t *testing.T) {
		respJSON := `{
			"jsonrpc": "2.0",
			"id": 1,
			"result": {
				"value": {
					"data": {
						"parsed": {
							"info": {
								"decimals": 9,
								"extensions": [
									{
										"extension": "scaledUiAmountConfig",
										"state": {
											"multiplier": "1.4861347"
										}
									}
								]
							}
						}
					}
				}
			}
		}`

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(respJSON))
		}))
		defer ts.Close()

		client := solanarpc.New(ts.URL, ts.Client(), clock, 60*time.Second)
		info, err := client.MintInfo(ctx, "MINT_SCALED")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.Multiplier != 1.4861347 {
			t.Errorf("expected Multiplier=1.4861347, got %v", info.Multiplier)
		}
	})

	t.Run("mint with future and active newMultiplier timestamp", func(t *testing.T) {
		futureTs := now.Unix() + 3600 // 1 hour in future
		respJSONFuture := `{
			"jsonrpc": "2.0",
			"id": 1,
			"result": {
				"value": {
					"data": {
						"parsed": {
							"info": {
								"decimals": 9,
								"extensions": [
									{
										"extension": "scaledUiAmountConfig",
										"state": {
											"multiplier": "2.0",
											"newMultiplier": "5.0",
											"newMultiplierEffectiveTimestamp": ` + string(rune(futureTs)) + `
										}
									}
								]
							}
						}
					}
				}
			}
		}`
		// Let's format timestamp properly
		respJSONFuture = `{
			"jsonrpc": "2.0",
			"id": 1,
			"result": {
				"value": {
					"data": {
						"parsed": {
							"info": {
								"decimals": 9,
								"extensions": [
									{
										"extension": "scaledUiAmountConfig",
										"state": {
											"multiplier": 2.0,
											"newMultiplier": 5.0,
											"newMultiplierEffectiveTimestamp": 1789999999
										}
									}
								]
							}
						}
					}
				}
			}
		}`

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(respJSONFuture))
		}))
		defer ts.Close()

		// Case 1: clock is before timestamp 1789999999 -> uses multiplier 2.0
		clock.Set(time.Unix(1780000000, 0))
		client := solanarpc.New(ts.URL, ts.Client(), clock, 1*time.Second)
		info1, err := client.MintInfo(ctx, "MINT_TS")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info1.Multiplier != 2.0 {
			t.Errorf("expected Multiplier=2.0 before effective timestamp, got %v", info1.Multiplier)
		}

		// Case 2: clock is at or after timestamp 1789999999 -> uses newMultiplier 5.0
		clock.Set(time.Unix(1790000000, 0))
		info2, err := client.MintInfo(ctx, "MINT_TS")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info2.Multiplier != 5.0 {
			t.Errorf("expected Multiplier=5.0 after effective timestamp, got %v", info2.Multiplier)
		}
	})

	t.Run("caching avoids repeated rpc calls", func(t *testing.T) {
		var hitCount int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hitCount, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"value":{"data":{"parsed":{"info":{"decimals":6}}}}}}`))
		}))
		defer ts.Close()

		client := solanarpc.New(ts.URL, ts.Client(), clock, 60*time.Second)

		_, _ = client.MintInfo(ctx, "MINT_CACHE")
		_, _ = client.MintInfo(ctx, "MINT_CACHE")

		if atomic.LoadInt32(&hitCount) != 1 {
			t.Errorf("expected 1 hit due to caching, got %d", atomic.LoadInt32(&hitCount))
		}
	})

	t.Run("account not found (null value) returns ErrUpstream", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"value":null}}`))
		}))
		defer ts.Close()

		client := solanarpc.New(ts.URL, ts.Client(), clock, 60*time.Second)
		_, err := client.MintInfo(ctx, "MINT_UNKNOWN")
		if !errors.Is(err, domain.ErrUpstream) {
			t.Errorf("expected ErrUpstream for missing account, got %v", err)
		}
	})

	t.Run("rpc error response returns ErrUpstream", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid Request"}}`))
		}))
		defer ts.Close()

		client := solanarpc.New(ts.URL, ts.Client(), clock, 60*time.Second)
		_, err := client.MintInfo(ctx, "MINT_ERR")
		if !errors.Is(err, domain.ErrUpstream) {
			t.Errorf("expected ErrUpstream for RPC error, got %v", err)
		}
	})

	t.Run("http status != 200 returns ErrUpstream", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer ts.Close()

		client := solanarpc.New(ts.URL, ts.Client(), clock, 60*time.Second)
		_, err := client.MintInfo(ctx, "MINT_HTTP_ERR")
		if !errors.Is(err, domain.ErrUpstream) {
			t.Errorf("expected ErrUpstream for HTTP 502, got %v", err)
		}
	})
}

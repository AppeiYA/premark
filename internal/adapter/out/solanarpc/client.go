package solanarpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"premark/internal/domain"
	"premark/internal/ports"
)

type MintInfo struct {
	Decimals   int
	Multiplier float64
}

type cacheEntry struct {
	info     MintInfo
	cachedAt time.Time
}

type Client struct {
	rpcURL   string
	hc       *http.Client
	clock    ports.Clock
	cacheTTL time.Duration
	mu       sync.RWMutex
	cache    map[string]cacheEntry
}

func New(rpcURL string, hc *http.Client, clock ports.Clock, cacheTTL time.Duration) *Client {
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}
	if hc == nil {
		hc = http.DefaultClient
	}
	if cacheTTL <= 0 {
		cacheTTL = 60 * time.Second
	}
	return &Client{
		rpcURL:   rpcURL,
		hc:       hc,
		clock:    clock,
		cacheTTL: cacheTTL,
		cache:    make(map[string]cacheEntry),
	}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type rpcResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		Value *struct {
			Data struct {
				Parsed struct {
					Info struct {
						Decimals   int `json:"decimals"`
						Extensions []struct {
							Extension string         `json:"extension"`
							State     map[string]any `json:"state"`
						} `json:"extensions"`
					} `json:"info"`
				} `json:"parsed"`
			} `json:"data"`
		} `json:"value"`
	} `json:"result"`
	Error any `json:"error"`
}

func parseFloatFlexible(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}

func parseIntFlexible(v any) (int64, bool) {
	switch val := v.(type) {
	case float64:
		return int64(val), true
	case int:
		return int64(val), true
	case int64:
		return val, true
	case string:
		i, err := strconv.ParseInt(val, 10, 64)
		if err == nil {
			return i, true
		}
	}
	return 0, false
}

func (c *Client) MintInfo(ctx context.Context, mint string) (MintInfo, error) {
	c.mu.RLock()
	entry, ok := c.cache[mint]
	if ok && c.clock.Now().Sub(entry.cachedAt) < c.cacheTTL {
		c.mu.RUnlock()
		return entry.info, nil
	}
	c.mu.RUnlock()

	reqBody := rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "getAccountInfo",
		Params: []any{
			mint,
			map[string]string{"encoding": "jsonParsed"},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return MintInfo{}, fmt.Errorf("%w: marshal rpc body: %w", domain.ErrUpstream, err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.rpcURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return MintInfo{}, fmt.Errorf("%w: new rpc request: %w", domain.ErrUpstream, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(httpReq)
	if err != nil {
		return MintInfo{}, fmt.Errorf("%w: rpc call: %w", domain.ErrUpstream, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return MintInfo{}, fmt.Errorf("%w: rpc status %d", domain.ErrUpstream, resp.StatusCode)
	}

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return MintInfo{}, fmt.Errorf("%w: decode rpc response: %w", domain.ErrUpstream, err)
	}

	if rpcResp.Error != nil {
		return MintInfo{}, fmt.Errorf("%w: rpc error: %v", domain.ErrUpstream, rpcResp.Error)
	}

	if rpcResp.Result.Value == nil {
		return MintInfo{}, fmt.Errorf("%w: mint account %s not found", domain.ErrUpstream, mint)
	}

	decimals := rpcResp.Result.Value.Data.Parsed.Info.Decimals
	multiplier := 1.0

	nowUnix := c.clock.Now().Unix()
	for _, ext := range rpcResp.Result.Value.Data.Parsed.Info.Extensions {
		if ext.Extension == "scaledUiAmountConfig" && ext.State != nil {
			if tsVal, hasTs := ext.State["newMultiplierEffectiveTimestamp"]; hasTs {
				if ts, ok := parseIntFlexible(tsVal); ok && nowUnix >= ts {
					if newMultVal, hasNew := ext.State["newMultiplier"]; hasNew {
						if m, ok := parseFloatFlexible(newMultVal); ok {
							multiplier = m
							break
						}
					}
				}
			}

			if multVal, hasMult := ext.State["multiplier"]; hasMult {
				if m, ok := parseFloatFlexible(multVal); ok {
					multiplier = m
					break
				}
			}
		}
	}

	info := MintInfo{
		Decimals:   decimals,
		Multiplier: multiplier,
	}

	c.mu.Lock()
	c.cache[mint] = cacheEntry{
		info:     info,
		cachedAt: c.clock.Now(),
	}
	c.mu.Unlock()

	return info, nil
}

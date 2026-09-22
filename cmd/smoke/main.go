package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"time"

	"premark/internal/adapter/out/jupiter"
	"premark/internal/adapter/out/prestocksapi"
	"premark/internal/adapter/out/solanarpc"
	"premark/internal/adapter/out/system"
	"premark/internal/config"
	"premark/internal/domain"
)

type mintInfoAdapter struct {
	rpc *solanarpc.Client
}

func (a *mintInfoAdapter) MintInfo(ctx context.Context, mint string) (jupiter.MintInfo, error) {
	info, err := a.rpc.MintInfo(ctx, mint)
	if err != nil {
		return jupiter.MintInfo{}, err
	}
	return jupiter.MintInfo{
		Decimals:   info.Decimals,
		Multiplier: info.Multiplier,
	}, nil
}

func printRawExtensions(ctx context.Context, rpcURL, mint string) {
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getAccountInfo",
		"params": []any{
			mint,
			map[string]string{"encoding": "jsonParsed"},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Raw extension fetch failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var raw map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&raw)

	// Extract extensions
	if res, ok := raw["result"].(map[string]any); ok {
		if val, ok := res["value"].(map[string]any); ok {
			if data, ok := val["data"].(map[string]any); ok {
				if parsed, ok := data["parsed"].(map[string]any); ok {
					if info, ok := parsed["info"].(map[string]any); ok {
						if exts, ok := info["extensions"]; ok {
							extJSON, _ := json.MarshalIndent(exts, "", "  ")
							fmt.Printf("Raw extensions JSON for %s:\n%s\n", mint, string(extJSON))
							return
						}
					}
				}
			}
		}
	}
	fmt.Printf("No extensions found in raw account data for %s\n", mint)
}

func fetchRawJupiterQuote(ctx context.Context, baseURL, usdcMint, tokenMint string, amountMicros int64, apiKey string) (inAmt, outAmt, impactPct string, err error) {
	urlStr := fmt.Sprintf("%s/quote?inputMint=%s&outputMint=%s&amount=%d&slippageBps=50", baseURL, usdcMint, tokenMint, amountMicros)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var parsed struct {
		InAmount       string `json:"inAmount"`
		OutAmount      string `json:"outAmount"`
		PriceImpactPct string `json:"priceImpactPct"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", "", fmt.Errorf("status %d, body: %s", resp.StatusCode, string(body))
	}
	return parsed.InAmount, parsed.OutAmount, parsed.PriceImpactPct, nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config load failed: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	clock := system.NewClock()

	marketSource := prestocksapi.New(cfg.PreStocksAPIURL, http.DefaultClient, clock, 0)
	rpcClient := solanarpc.New(cfg.SolanaRPCURL, http.DefaultClient, clock, 60*time.Second)
	mints := &mintInfoAdapter{rpc: rpcClient}
	quoteProvider := jupiter.New(jupiter.Config{
		BaseURL:          cfg.JupiterBaseURL,
		APIKey:           cfg.JupiterAPIKey,
		USDCMint:         cfg.USDCMint,
		SlippageBps:      50,
		ImpactIsFraction: cfg.JupiterImpactIsFraction,
	}, http.DefaultClient, clock, mints)

	fmt.Println("=== 1. PRESTOCKS API ===")
	snaps, err := marketSource.FetchAll(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch PreStocks: %v\n", err)
		os.Exit(1)
	}

	for _, s := range snaps {
		prem, _ := s.Premium()
		fmt.Printf("Symbol: %-12s TokenPrice: %10.2f MarkPrice: %10.2f PremiumBps: %8.1f Mint: %s\n",
			s.Token.Symbol, s.TokenPrice, s.MarkPrice, prem.Bps(), s.Token.Mint)
	}

	fmt.Println("\n=== 2. SOLANA RPC MINT INFO ===")
	if len(snaps) > 0 {
		printRawExtensions(ctx, cfg.SolanaRPCURL, snaps[0].Token.Mint)
	}

	for _, s := range snaps {
		info, err := rpcClient.MintInfo(ctx, s.Token.Mint)
		if err != nil {
			fmt.Printf("Symbol: %-12s Mint: %s Error: %v\n", s.Token.Symbol, s.Token.Mint, err)
		} else {
			fmt.Printf("Symbol: %-12s Mint: %s Decimals: %d Multiplier: %g\n",
				s.Token.Symbol, s.Token.Mint, info.Decimals, info.Multiplier)
		}
	}

	fmt.Println("\n=== 3. JUPITER QUOTES ===")
	var targetSnap *domain.MarketSnapshot
	for i := range snaps {
		if snaps[i].Token.Symbol == "ANTHROPIC" {
			targetSnap = &snaps[i]
			break
		}
	}
	if targetSnap == nil && len(snaps) > 0 {
		targetSnap = &snaps[0]
	}

	if targetSnap == nil {
		fmt.Println("No tokens to quote.")
		return
	}

	quoteBudgets := []struct {
		dollars float64
		usdc    domain.USDC
	}{
		{10, 10 * domain.USDCUnit},
		{1000, 1000 * domain.USDCUnit},
		{50000, 50000 * domain.USDCUnit},
	}

	for _, b := range quoteBudgets {
		fmt.Printf("\n--- Quoting $%.0f (%s) ---\n", b.dollars, targetSnap.Token.Symbol)
		rawIn, rawOut, rawImpact, rawErr := fetchRawJupiterQuote(ctx, cfg.JupiterBaseURL, cfg.USDCMint, targetSnap.Token.Mint, int64(b.usdc), cfg.JupiterAPIKey)
		if rawErr != nil {
			fmt.Printf("Raw quote error: %v\n", rawErr)
		} else {
			fmt.Printf("Raw Jupiter response: inAmount=%s, outAmount=%s, priceImpactPct=%s\n", rawIn, rawOut, rawImpact)
		}

		q, err := quoteProvider.QuoteBuy(ctx, targetSnap.Token, b.usdc)
		if err != nil {
			fmt.Printf("QuoteBuy failed: %v\n", err)
			continue
		}

		tokenPrice, _ := q.PricePerToken()
		execPremBps := (tokenPrice/targetSnap.MarkPrice - 1.0) * 10000.0

		fmt.Printf("Computed: OutTokens=%g, PricePerToken=%.4f, PriceImpactBps=%.2f, ExecPremiumBps=%.1f\n",
			q.OutTokens, tokenPrice, q.PriceImpactBps, execPremBps)

		fmt.Println("\n=== 4. SANITY VERDICT ===")
		diffPct := math.Abs(tokenPrice/targetSnap.TokenPrice-1.0) * 100.0
		within10 := diffPct <= 10.0
		verdict := "no"
		if within10 {
			verdict = "yes"
		}
		fmt.Printf("Executable price ($%.2f) is within 10%% of API price ($%.2f) [diff=%.2f%%]: %s\n",
			tokenPrice, targetSnap.TokenPrice, diffPct, verdict)
	}
}

package httpapi

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"premark/internal/domain"
	"premark/internal/ports"
)

//go:embed static/index.html
var indexHTML []byte

type Deps struct {
	Rules      ports.RuleManager
	Market     ports.MarketReader
	Signals    ports.SignalReader
	Scanner    ports.ScanRunner
	AdminToken string       // if non-empty, POST /v1/scan requires header X-Admin-Token == AdminToken
	Logger     *slog.Logger // nil = discard
}

func decodeJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1 MiB limit
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("extra data after JSON body")
	}
	// Verify EOF
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("extra data after JSON body")
	}
	return nil
}

func requireOwner(w http.ResponseWriter, r *http.Request) (domain.OwnerID, bool) {
	owner := strings.TrimSpace(r.Header.Get("X-Owner-ID"))
	if owner == "" {
		writeError(w, http.StatusBadRequest, "missing_owner", "missing X-Owner-ID header")
		return "", false
	}
	return domain.OwnerID(owner), true
}

func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(indexHTML)
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
	})

	mux.HandleFunc("GET /v1/tokens", func(w http.ResponseWriter, r *http.Request) {
		views, err := d.Market.ListMarket(r.Context())
		if err != nil {
			writeAppError(w, err)
			return
		}
		dtos := make([]MarketDTO, 0, len(views))
		for _, v := range views {
			dtos = append(dtos, toMarketDTO(v))
		}
		writeJSON(w, http.StatusOK, TokensResponse{Tokens: dtos})
	})

	mux.HandleFunc("GET /v1/tokens/{symbol}", func(w http.ResponseWriter, r *http.Request) {
		symbol := r.PathValue("symbol")
		view, err := d.Market.GetMarket(r.Context(), symbol)
		if err != nil {
			writeAppError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toMarketDTO(view))
	})

	mux.HandleFunc("GET /v1/tokens/{symbol}/history", func(w http.ResponseWriter, r *http.Request) {
		symbol := r.PathValue("symbol")
		hoursStr := r.URL.Query().Get("hours")
		hours := 24
		if hoursStr != "" {
			var err error
			hours, err = strconv.Atoi(hoursStr)
			if err != nil || hours < 1 || hours > 168 {
				writeError(w, http.StatusBadRequest, "invalid_request", "invalid hours: must be integer between 1 and 168")
				return
			}
		}

		snaps, err := d.Market.History(r.Context(), symbol, time.Duration(hours)*time.Hour)
		if err != nil {
			writeAppError(w, err)
			return
		}

		points := make([]PointDTO, 0, len(snaps))
		for _, s := range snaps {
			if pt, ok := toPointDTO(s); ok {
				points = append(points, pt)
			}
		}

		writeJSON(w, http.StatusOK, HistoryResponse{
			Symbol: symbol,
			Points: points,
		})
	})

	mux.HandleFunc("POST /v1/rules", func(w http.ResponseWriter, r *http.Request) {
		owner, ok := requireOwner(w, r)
		if !ok {
			return
		}

		var req CreateRuleRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("invalid json body: %v", err))
			return
		}

		if req.BudgetUSDC == nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "budget_usdc is required")
			return
		}

		budget, err := domain.USDCFromFloat(*req.BudgetUSDC)
		if err != nil {
			writeAppError(w, err)
			return
		}

		p := domain.NewRuleParams{
			Owner:         string(owner),
			Symbol:        req.Symbol,
			MaxPremiumBps: req.MaxPremiumBps,
			Budget:        budget,
		}
		if req.MaxPriceImpactBps != nil {
			p.MaxPriceImpactBps = *req.MaxPriceImpactBps
		}
		if req.CooldownMinutes != nil {
			p.Cooldown = time.Duration(*req.CooldownMinutes) * time.Minute
		}

		rule, err := d.Rules.CreateRule(r.Context(), p)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, toRuleDTO(rule))
	})

	mux.HandleFunc("GET /v1/rules", func(w http.ResponseWriter, r *http.Request) {
		owner, ok := requireOwner(w, r)
		if !ok {
			return
		}

		rules, err := d.Rules.ListRules(r.Context(), owner)
		if err != nil {
			writeAppError(w, err)
			return
		}

		dtos := make([]RuleDTO, 0, len(rules))
		for _, rule := range rules {
			dtos = append(dtos, toRuleDTO(rule))
		}
		writeJSON(w, http.StatusOK, RulesResponse{Rules: dtos})
	})

	mux.HandleFunc("PATCH /v1/rules/{id}", func(w http.ResponseWriter, r *http.Request) {
		owner, ok := requireOwner(w, r)
		if !ok {
			return
		}

		id := r.PathValue("id")
		var req PatchRuleRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("invalid json body: %v", err))
			return
		}

		if req.Enabled == nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "enabled is required")
			return
		}

		rule, err := d.Rules.SetRuleEnabled(r.Context(), owner, domain.RuleID(id), *req.Enabled)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, toRuleDTO(rule))
	})

	mux.HandleFunc("DELETE /v1/rules/{id}", func(w http.ResponseWriter, r *http.Request) {
		owner, ok := requireOwner(w, r)
		if !ok {
			return
		}

		id := r.PathValue("id")
		if err := d.Rules.DeleteRule(r.Context(), owner, domain.RuleID(id)); err != nil {
			writeAppError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /v1/signals", func(w http.ResponseWriter, r *http.Request) {
		owner, ok := requireOwner(w, r)
		if !ok {
			return
		}

		limit := 20
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			var err error
			limit, err = strconv.Atoi(limitStr)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "limit must be an integer")
				return
			}
		}

		signals, err := d.Signals.ListSignals(r.Context(), owner, limit)
		if err != nil {
			writeAppError(w, err)
			return
		}

		dtos := make([]SignalDTO, 0, len(signals))
		for _, s := range signals {
			dtos = append(dtos, toSignalDTO(s))
		}
		writeJSON(w, http.StatusOK, SignalsResponse{Signals: dtos})
	})

	var lastScanMu sync.RWMutex
	var lastScanDTO *ScanDTO

	scanHandler := func(w http.ResponseWriter, r *http.Request) {
		if d.AdminToken != "" {
			adminToken := strings.TrimSpace(r.Header.Get("X-Admin-Token"))
			if adminToken != d.AdminToken {
				writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
				return
			}
		}

		report, err := d.Scanner.Scan(r.Context())
		if err != nil {
			writeAppError(w, err)
			return
		}

		dto := toScanDTO(report)
		lastScanMu.Lock()
		lastScanDTO = &dto
		lastScanMu.Unlock()

		writeJSON(w, http.StatusOK, dto)
	}

	mux.HandleFunc("POST /v1/scan", scanHandler)

	mux.HandleFunc("POST /ui/scan", func(w http.ResponseWriter, r *http.Request) {
		internalReq := r.Clone(r.Context())
		if d.AdminToken != "" {
			internalReq.Header.Set("X-Admin-Token", d.AdminToken)
		}
		scanHandler(w, internalReq)
	})

	mux.HandleFunc("GET /ui/scan/latest", func(w http.ResponseWriter, r *http.Request) {
		lastScanMu.RLock()
		defer lastScanMu.RUnlock()
		if lastScanDTO == nil {
			writeJSON(w, http.StatusOK, map[string]any{"scan": nil})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"scan": lastScanDTO})
	})

	// Wrap with panic recovery and logging middleware
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if d.Logger != nil {
					d.Logger.Error("panic in http handler", "panic", rec)
				}
				writeError(w, http.StatusInternalServerError, "internal", "internal error")
			}
		}()

		if d.Logger != nil {
			d.Logger.Debug("http request", "method", r.Method, "path", r.URL.Path)
		}

		// Check if route matches anything
		_, pattern := mux.Handler(r)
		if pattern == "" {
			writeError(w, http.StatusNotFound, "not_found", "route not found")
			return
		}

		mux.ServeHTTP(w, r)
	})
}

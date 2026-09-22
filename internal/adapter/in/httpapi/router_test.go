package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"premark/internal/adapter/in/httpapi"
	"premark/internal/domain"
	"premark/internal/testsupport"
)

func TestHTTPAPI_Router(t *testing.T) {
	app := testsupport.NewApp(t)
	now := app.Clock.Now()

	// Seed market snapshot
	snap := testsupport.Snap("ANTHROPIC", 950.0, 1000.0, now)
	app.Source.Snapshots = []domain.MarketSnapshot{snap}
	_ = app.Snapshots.SaveAll(context.Background(), []domain.MarketSnapshot{snap})

	t.Run("GET / returns static index HTML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Errorf("expected text/html content type, got %s", rec.Header().Get("Content-Type"))
		}
	})

	t.Run("GET /healthz returns ok status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var resp map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["status"] != "ok" {
			t.Errorf("expected status=ok, got %v", resp)
		}
	})

	t.Run("GET /v1/tokens lists tokens", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/tokens", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var resp httpapi.TokensResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Tokens) != 1 {
			t.Fatalf("expected 1 token, got %d", len(resp.Tokens))
		}
		if resp.Tokens[0].Symbol != "ANTHROPIC" {
			t.Errorf("expected ANTHROPIC, got %s", resp.Tokens[0].Symbol)
		}
		if resp.Tokens[0].Label != "cheap" {
			t.Errorf("expected cheap label, got %s", resp.Tokens[0].Label)
		}
	})

	t.Run("GET /v1/tokens/{symbol}", func(t *testing.T) {
		// Valid symbol
		req := httptest.NewRequest(http.MethodGet, "/v1/tokens/ANTHROPIC", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var dto httpapi.MarketDTO
		_ = json.Unmarshal(rec.Body.Bytes(), &dto)
		if dto.Symbol != "ANTHROPIC" {
			t.Errorf("expected ANTHROPIC, got %s", dto.Symbol)
		}

		// Unknown symbol
		reqMissing := httptest.NewRequest(http.MethodGet, "/v1/tokens/UNKNOWN", nil)
		recMissing := httptest.NewRecorder()
		app.Router.ServeHTTP(recMissing, reqMissing)

		if recMissing.Code != http.StatusNotFound {
			t.Errorf("expected 404 for unknown token, got %d", recMissing.Code)
		}
	})

	t.Run("GET /v1/tokens/{symbol}/history", func(t *testing.T) {
		// Valid request with hours query
		req := httptest.NewRequest(http.MethodGet, "/v1/tokens/ANTHROPIC/history?hours=48", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var hist httpapi.HistoryResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &hist)
		if hist.Symbol != "ANTHROPIC" {
			t.Errorf("expected ANTHROPIC, got %s", hist.Symbol)
		}
		if len(hist.Points) != 1 {
			t.Fatalf("expected 1 history point, got %d", len(hist.Points))
		}

		// Invalid hours
		reqBadHours := httptest.NewRequest(http.MethodGet, "/v1/tokens/ANTHROPIC/history?hours=abc", nil)
		recBadHours := httptest.NewRecorder()
		app.Router.ServeHTTP(recBadHours, reqBadHours)

		if recBadHours.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid hours string, got %d", recBadHours.Code)
		}
	})

	var createdRuleID string

	t.Run("POST /v1/rules creates rule", func(t *testing.T) {
		// Missing X-Owner-ID
		body := `{"symbol":"ANTHROPIC","max_premium_bps":-200,"budget_usdc":100.0}`
		reqNoOwner := httptest.NewRequest(http.MethodPost, "/v1/rules", strings.NewReader(body))
		recNoOwner := httptest.NewRecorder()
		app.Router.ServeHTTP(recNoOwner, reqNoOwner)

		if recNoOwner.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for missing X-Owner-ID, got %d", recNoOwner.Code)
		}

		// Missing budget
		bodyNoBudget := `{"symbol":"ANTHROPIC","max_premium_bps":-200}`
		reqNoBudget := httptest.NewRequest(http.MethodPost, "/v1/rules", strings.NewReader(bodyNoBudget))
		reqNoBudget.Header.Set("X-Owner-ID", "alice")
		recNoBudget := httptest.NewRecorder()
		app.Router.ServeHTTP(recNoBudget, reqNoBudget)

		if recNoBudget.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for missing budget, got %d", recNoBudget.Code)
		}

		// Invalid JSON
		reqBadJSON := httptest.NewRequest(http.MethodPost, "/v1/rules", strings.NewReader("{bad json}"))
		reqBadJSON.Header.Set("X-Owner-ID", "alice")
		recBadJSON := httptest.NewRecorder()
		app.Router.ServeHTTP(recBadJSON, reqBadJSON)

		if recBadJSON.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid JSON, got %d", recBadJSON.Code)
		}

		// Valid creation
		reqValid := httptest.NewRequest(http.MethodPost, "/v1/rules", strings.NewReader(body))
		reqValid.Header.Set("X-Owner-ID", "alice")
		recValid := httptest.NewRecorder()
		app.Router.ServeHTTP(recValid, reqValid)

		if recValid.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d body: %s", recValid.Code, recValid.Body.String())
		}

		var ruleDTO httpapi.RuleDTO
		_ = json.Unmarshal(recValid.Body.Bytes(), &ruleDTO)
		if ruleDTO.ID == "" {
			t.Errorf("expected non-empty rule ID")
		}
		if ruleDTO.OwnerID != "alice" {
			t.Errorf("expected OwnerID=alice, got %s", ruleDTO.OwnerID)
		}
		if ruleDTO.Symbol != "ANTHROPIC" {
			t.Errorf("expected Symbol=ANTHROPIC, got %s", ruleDTO.Symbol)
		}
		if ruleDTO.BudgetUSDC != 100.0 {
			t.Errorf("expected BudgetUSDC=100.0, got %v", ruleDTO.BudgetUSDC)
		}
		if !ruleDTO.Enabled {
			t.Errorf("expected rule to be enabled")
		}
		createdRuleID = ruleDTO.ID
	})

	t.Run("GET /v1/rules lists rules for owner", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/rules", nil)
		req.Header.Set("X-Owner-ID", "alice")
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var rulesResp httpapi.RulesResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &rulesResp)
		if len(rulesResp.Rules) != 1 {
			t.Fatalf("expected 1 rule for alice, got %d", len(rulesResp.Rules))
		}
		if rulesResp.Rules[0].ID != createdRuleID {
			t.Errorf("expected ID %s, got %s", createdRuleID, rulesResp.Rules[0].ID)
		}
	})

	t.Run("PATCH /v1/rules/{id} toggles enabled", func(t *testing.T) {
		// Missing enabled
		reqBad := httptest.NewRequest(http.MethodPatch, "/v1/rules/"+createdRuleID, strings.NewReader(`{}`))
		reqBad.Header.Set("X-Owner-ID", "alice")
		recBad := httptest.NewRecorder()
		app.Router.ServeHTTP(recBad, reqBad)

		if recBad.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for missing enabled field, got %d", recBad.Code)
		}

		// Wrong owner returns 404
		reqBob := httptest.NewRequest(http.MethodPatch, "/v1/rules/"+createdRuleID, strings.NewReader(`{"enabled":false}`))
		reqBob.Header.Set("X-Owner-ID", "bob")
		recBob := httptest.NewRecorder()
		app.Router.ServeHTTP(recBob, reqBob)

		if recBob.Code != http.StatusNotFound {
			t.Errorf("expected 404 for wrong owner, got %d", recBob.Code)
		}

		// Valid patch
		reqPatch := httptest.NewRequest(http.MethodPatch, "/v1/rules/"+createdRuleID, strings.NewReader(`{"enabled":false}`))
		reqPatch.Header.Set("X-Owner-ID", "alice")
		recPatch := httptest.NewRecorder()
		app.Router.ServeHTTP(recPatch, reqPatch)

		if recPatch.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", recPatch.Code)
		}

		var ruleDTO httpapi.RuleDTO
		_ = json.Unmarshal(recPatch.Body.Bytes(), &ruleDTO)
		if ruleDTO.Enabled != false {
			t.Errorf("expected Enabled=false, got true")
		}
	})

	t.Run("DELETE /v1/rules/{id} deletes rule", func(t *testing.T) {
		// Wrong owner
		reqBob := httptest.NewRequest(http.MethodDelete, "/v1/rules/"+createdRuleID, nil)
		reqBob.Header.Set("X-Owner-ID", "bob")
		recBob := httptest.NewRecorder()
		app.Router.ServeHTTP(recBob, reqBob)

		if recBob.Code != http.StatusNotFound {
			t.Errorf("expected 404 for wrong owner delete, got %d", recBob.Code)
		}

		// Valid delete
		reqDel := httptest.NewRequest(http.MethodDelete, "/v1/rules/"+createdRuleID, nil)
		reqDel.Header.Set("X-Owner-ID", "alice")
		recDel := httptest.NewRecorder()
		app.Router.ServeHTTP(recDel, reqDel)

		if recDel.Code != http.StatusNoContent {
			t.Fatalf("expected 204 No Content, got %d", recDel.Code)
		}

		// Delete again returns 404
		recDelAgain := httptest.NewRecorder()
		app.Router.ServeHTTP(recDelAgain, reqDel)
		if recDelAgain.Code != http.StatusNotFound {
			t.Errorf("expected 404 on deleting non-existent rule, got %d", recDelAgain.Code)
		}
	})

	t.Run("GET /v1/signals lists signals", func(t *testing.T) {
		// Missing owner header
		reqNoOwner := httptest.NewRequest(http.MethodGet, "/v1/signals", nil)
		recNoOwner := httptest.NewRecorder()
		app.Router.ServeHTTP(recNoOwner, reqNoOwner)
		if recNoOwner.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for missing owner, got %d", recNoOwner.Code)
		}

		// Invalid limit query param
		reqBadLimit := httptest.NewRequest(http.MethodGet, "/v1/signals?limit=invalid", nil)
		reqBadLimit.Header.Set("X-Owner-ID", "alice")
		recBadLimit := httptest.NewRecorder()
		app.Router.ServeHTTP(recBadLimit, reqBadLimit)
		if recBadLimit.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for bad limit, got %d", recBadLimit.Code)
		}

		// Seed a signal
		sig := domain.Signal{
			ID:             "sig-100",
			RuleID:         "r1",
			Owner:          "alice",
			Symbol:         "ANTHROPIC",
			TokenPrice:     950.0,
			MarkPrice:      1000.0,
			APIPremiumBps:  -500.0,
			ExecPremiumBps: -480.0,
			Quote: domain.Quote{
				InAmount:       100 * domain.USDCUnit,
				OutTokens:      0.105,
				PriceImpactBps: 15.0,
				SwapURL:        "https://jup.ag/swap/USDC-ANTHROPIC",
				QuotedAt:       now,
			},
			CreatedAt: now,
		}
		_ = app.Signals.Save(context.Background(), sig)

		reqValid := httptest.NewRequest(http.MethodGet, "/v1/signals?limit=10", nil)
		reqValid.Header.Set("X-Owner-ID", "alice")
		recValid := httptest.NewRecorder()
		app.Router.ServeHTTP(recValid, reqValid)

		if recValid.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", recValid.Code)
		}

		var sigsResp httpapi.SignalsResponse
		_ = json.Unmarshal(recValid.Body.Bytes(), &sigsResp)
		if len(sigsResp.Signals) != 1 {
			t.Fatalf("expected 1 signal, got %d", len(sigsResp.Signals))
		}
		if sigsResp.Signals[0].ID != "sig-100" {
			t.Errorf("expected sig-100, got %s", sigsResp.Signals[0].ID)
		}
	})

	t.Run("POST /v1/scan executes scan", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/scan", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d body: %s", rec.Code, rec.Body.String())
		}

		var scanDTO httpapi.ScanDTO
		_ = json.Unmarshal(rec.Body.Bytes(), &scanDTO)
		if scanDTO.Snapshots != 1 {
			t.Errorf("expected 1 snapshot scanned, got %d", scanDTO.Snapshots)
		}
	})

	t.Run("POST /v1/scan requires admin token when configured", func(t *testing.T) {
		adminApp := testsupport.NewAppWithAdminToken(t, "super-secret-token")

		// Missing token
		reqNoToken := httptest.NewRequest(http.MethodPost, "/v1/scan", nil)
		recNoToken := httptest.NewRecorder()
		adminApp.Router.ServeHTTP(recNoToken, reqNoToken)
		if recNoToken.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for missing token, got %d", recNoToken.Code)
		}

		// Wrong token
		reqWrongToken := httptest.NewRequest(http.MethodPost, "/v1/scan", nil)
		reqWrongToken.Header.Set("X-Admin-Token", "wrong")
		recWrongToken := httptest.NewRecorder()
		adminApp.Router.ServeHTTP(recWrongToken, reqWrongToken)
		if recWrongToken.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for wrong token, got %d", recWrongToken.Code)
		}

		// Valid token
		reqValidToken := httptest.NewRequest(http.MethodPost, "/v1/scan", nil)
		reqValidToken.Header.Set("X-Admin-Token", "super-secret-token")
		recValidToken := httptest.NewRecorder()
		adminApp.Router.ServeHTTP(recValidToken, reqValidToken)
		if recValidToken.Code != http.StatusOK {
			t.Errorf("expected 200 OK with valid token, got %d", recValidToken.Code)
		}
	})

	t.Run("unknown route returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/unknown/route", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for unknown route, got %d", rec.Code)
		}
	})
}

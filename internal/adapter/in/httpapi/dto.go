package httpapi

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type StatusResponse struct {
	Status string `json:"status"`
}

type MarketDTO struct {
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name"`
	Mint       string  `json:"mint"`
	TokenPrice float64 `json:"token_price"`
	MarkPrice  float64 `json:"mark_price"`
	PremiumBps float64 `json:"premium_bps"`
	Label      string  `json:"label"`
	Supply     float64 `json:"supply"`
	ObservedAt string  `json:"observed_at"`
}

type TokensResponse struct {
	Tokens []MarketDTO `json:"tokens"`
}

type PointDTO struct {
	ObservedAt string  `json:"observed_at"`
	TokenPrice float64 `json:"token_price"`
	MarkPrice  float64 `json:"mark_price"`
	PremiumBps float64 `json:"premium_bps"`
	Supply     float64 `json:"supply"`
}

type HistoryResponse struct {
	Symbol string     `json:"symbol"`
	Points []PointDTO `json:"points"`
}

type CreateRuleRequest struct {
	Symbol            string   `json:"symbol"`
	MaxPremiumBps     int      `json:"max_premium_bps"`
	BudgetUSDC        *float64 `json:"budget_usdc"`
	MaxPriceImpactBps *int     `json:"max_price_impact_bps,omitempty"`
	CooldownMinutes   *int     `json:"cooldown_minutes,omitempty"`
}

type PatchRuleRequest struct {
	Enabled *bool `json:"enabled"`
}

type RuleDTO struct {
	ID                string  `json:"id"`
	OwnerID           string  `json:"owner_id"`
	Symbol            string  `json:"symbol"`
	MaxPremiumBps     int     `json:"max_premium_bps"`
	BudgetUSDC        float64 `json:"budget_usdc"`
	MaxPriceImpactBps int     `json:"max_price_impact_bps"`
	CooldownMinutes   int     `json:"cooldown_minutes"`
	Enabled           bool    `json:"enabled"`
	CreatedAt         string  `json:"created_at"`
	LastTriggeredAt   *string `json:"last_triggered_at"`
}

type RulesResponse struct {
	Rules []RuleDTO `json:"rules"`
}

type SignalDTO struct {
	ID             string  `json:"id"`
	RuleID         string  `json:"rule_id"`
	Symbol         string  `json:"symbol"`
	TokenPrice     float64 `json:"token_price"`
	MarkPrice      float64 `json:"mark_price"`
	APIPremiumBps  float64 `json:"api_premium_bps"`
	ExecPremiumBps float64 `json:"exec_premium_bps"`
	BudgetUSDC     float64 `json:"budget_usdc"`
	ExpectedTokens float64 `json:"expected_tokens"`
	PriceImpactBps float64 `json:"price_impact_bps"`
	SwapURL        string  `json:"swap_url"`
	CreatedAt      string  `json:"created_at"`
}

type SignalsResponse struct {
	Signals []SignalDTO `json:"signals"`
}

type ScanDTO struct {
	Snapshots      int            `json:"snapshots"`
	RulesEvaluated int            `json:"rules_evaluated"`
	SignalsCreated int            `json:"signals_created"`
	Skipped        int            `json:"skipped"`
	Failed         int            `json:"failed"`
	SkipReasons    map[string]int `json:"skip_reasons"`
}

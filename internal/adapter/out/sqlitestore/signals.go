package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.SignalRepository = (*SignalRepository)(nil)

type SignalRepository struct {
	db *sql.DB
}

func NewSignalRepository(db *sql.DB) *SignalRepository {
	return &SignalRepository{db: db}
}

type signalRow struct {
	id             string
	ruleID         string
	owner          string
	symbol         string
	tokenPrice     float64
	markPrice      float64
	apiPremiumBps  float64
	execPremiumBps float64
	qInputMint     string
	qOutputMint    string
	qInMicro       int64
	qOutTokens     float64
	qImpactBps     float64
	qSwapURL       string
	qQuotedMs      int64
	createdMs      int64
}

func (s signalRow) toDomain() domain.Signal {
	return domain.Signal{
		ID:             domain.SignalID(s.id),
		RuleID:         domain.RuleID(s.ruleID),
		Owner:          domain.OwnerID(s.owner),
		Symbol:         domain.Symbol(s.symbol),
		TokenPrice:     s.tokenPrice,
		MarkPrice:      s.markPrice,
		APIPremiumBps:  s.apiPremiumBps,
		ExecPremiumBps: s.execPremiumBps,
		Quote: domain.Quote{
			Symbol:         domain.Symbol(s.symbol),
			InputMint:      s.qInputMint,
			OutputMint:     s.qOutputMint,
			InAmount:       domain.USDC(s.qInMicro),
			OutTokens:      s.qOutTokens,
			PriceImpactBps: s.qImpactBps,
			SwapURL:        s.qSwapURL,
			QuotedAt:       time.UnixMilli(s.qQuotedMs).UTC(),
		},
		CreatedAt: time.UnixMilli(s.createdMs).UTC(),
	}
}

func (repo *SignalRepository) Save(ctx context.Context, s domain.Signal) error {
	query := `
INSERT INTO signals (
  id, rule_id, owner, symbol, token_price, mark_price, api_premium_bps, exec_premium_bps,
  q_input_mint, q_output_mint, q_in_micro, q_out_tokens, q_impact_bps, q_swap_url, q_quoted_ms, created_ms
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  rule_id=excluded.rule_id,
  owner=excluded.owner,
  symbol=excluded.symbol,
  token_price=excluded.token_price,
  mark_price=excluded.mark_price,
  api_premium_bps=excluded.api_premium_bps,
  exec_premium_bps=excluded.exec_premium_bps,
  q_input_mint=excluded.q_input_mint,
  q_output_mint=excluded.q_output_mint,
  q_in_micro=excluded.q_in_micro,
  q_out_tokens=excluded.q_out_tokens,
  q_impact_bps=excluded.q_impact_bps,
  q_swap_url=excluded.q_swap_url,
  q_quoted_ms=excluded.q_quoted_ms,
  created_ms=excluded.created_ms;`

	_, err := repo.db.ExecContext(ctx, query,
		string(s.ID),
		string(s.RuleID),
		string(s.Owner),
		string(s.Symbol),
		s.TokenPrice,
		s.MarkPrice,
		s.APIPremiumBps,
		s.ExecPremiumBps,
		s.Quote.InputMint,
		s.Quote.OutputMint,
		int64(s.Quote.InAmount),
		s.Quote.OutTokens,
		s.Quote.PriceImpactBps,
		s.Quote.SwapURL,
		s.Quote.QuotedAt.UnixMilli(),
		s.CreatedAt.UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("sqlite save signal %s: %w", s.ID, err)
	}
	return nil
}

func (repo *SignalRepository) ListByOwner(ctx context.Context, owner domain.OwnerID, limit int) ([]domain.Signal, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
SELECT id, rule_id, owner, symbol, token_price, mark_price, api_premium_bps, exec_premium_bps,
  q_input_mint, q_output_mint, q_in_micro, q_out_tokens, q_impact_bps, q_swap_url, q_quoted_ms, created_ms
FROM signals
WHERE owner = ?
ORDER BY created_ms DESC, id DESC
LIMIT ?;`

	rows, err := repo.db.QueryContext(ctx, query, string(owner), limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite list signals: %w", err)
	}
	defer rows.Close()

	result := make([]domain.Signal, 0)
	for rows.Next() {
		var row signalRow
		if err := rows.Scan(
			&row.id,
			&row.ruleID,
			&row.owner,
			&row.symbol,
			&row.tokenPrice,
			&row.markPrice,
			&row.apiPremiumBps,
			&row.execPremiumBps,
			&row.qInputMint,
			&row.qOutputMint,
			&row.qInMicro,
			&row.qOutTokens,
			&row.qImpactBps,
			&row.qSwapURL,
			&row.qQuotedMs,
			&row.createdMs,
		); err != nil {
			return nil, fmt.Errorf("scan signal row: %w", err)
		}
		result = append(result, row.toDomain())
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

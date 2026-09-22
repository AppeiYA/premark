package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.RuleRepository = (*RuleRepository)(nil)

type RuleRepository struct {
	db *sql.DB
}

func NewRuleRepository(db *sql.DB) *RuleRepository {
	return &RuleRepository{db: db}
}

type ruleRow struct {
	id              string
	owner           string
	symbol          string
	maxPremiumBps   int
	budgetMicro     int64
	maxImpactBps    int
	cooldownSec     int64
	enabled         int
	createdMs       int64
	lastTriggeredMs int64
}

func (r ruleRow) toDomain() domain.Rule {
	var lastTriggered time.Time
	if r.lastTriggeredMs > 0 {
		lastTriggered = time.UnixMilli(r.lastTriggeredMs).UTC()
	}

	return domain.Rule{
		ID:                domain.RuleID(r.id),
		Owner:             domain.OwnerID(r.owner),
		Symbol:            domain.Symbol(r.symbol),
		MaxPremiumBps:     r.maxPremiumBps,
		Budget:            domain.USDC(r.budgetMicro),
		MaxPriceImpactBps: r.maxImpactBps,
		Cooldown:          time.Duration(r.cooldownSec) * time.Second,
		Enabled:           r.enabled == 1,
		CreatedAt:         time.UnixMilli(r.createdMs).UTC(),
		LastTriggeredAt:   lastTriggered,
	}
}

func (repo *RuleRepository) Save(ctx context.Context, r domain.Rule) error {
	enabledInt := 0
	if r.Enabled {
		enabledInt = 1
	}

	var lastTriggeredMs int64
	if !r.LastTriggeredAt.IsZero() {
		lastTriggeredMs = r.LastTriggeredAt.UnixMilli()
	}

	query := `
INSERT INTO rules (id, owner, symbol, max_premium_bps, budget_micro, max_impact_bps, cooldown_sec, enabled, created_ms, last_triggered_ms)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  owner=excluded.owner,
  symbol=excluded.symbol,
  max_premium_bps=excluded.max_premium_bps,
  budget_micro=excluded.budget_micro,
  max_impact_bps=excluded.max_impact_bps,
  cooldown_sec=excluded.cooldown_sec,
  enabled=excluded.enabled,
  created_ms=excluded.created_ms,
  last_triggered_ms=excluded.last_triggered_ms;`

	_, err := repo.db.ExecContext(ctx, query,
		string(r.ID),
		string(r.Owner),
		string(r.Symbol),
		r.MaxPremiumBps,
		int64(r.Budget),
		r.MaxPriceImpactBps,
		int64(r.Cooldown/time.Second),
		enabledInt,
		r.CreatedAt.UnixMilli(),
		lastTriggeredMs,
	)
	if err != nil {
		return fmt.Errorf("sqlite save rule %s: %w", r.ID, err)
	}
	return nil
}

func (repo *RuleRepository) Get(ctx context.Context, id domain.RuleID) (domain.Rule, error) {
	query := `
SELECT id, owner, symbol, max_premium_bps, budget_micro, max_impact_bps, cooldown_sec, enabled, created_ms, last_triggered_ms
FROM rules
WHERE id = ?;`

	var row ruleRow
	err := repo.db.QueryRowContext(ctx, query, string(id)).Scan(
		&row.id,
		&row.owner,
		&row.symbol,
		&row.maxPremiumBps,
		&row.budgetMicro,
		&row.maxImpactBps,
		&row.cooldownSec,
		&row.enabled,
		&row.createdMs,
		&row.lastTriggeredMs,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Rule{}, fmt.Errorf("%w: rule %s", domain.ErrRuleNotFound, id)
		}
		return domain.Rule{}, fmt.Errorf("sqlite get rule %s: %w", id, err)
	}
	return row.toDomain(), nil
}

func (repo *RuleRepository) ListByOwner(ctx context.Context, owner domain.OwnerID) ([]domain.Rule, error) {
	query := `
SELECT id, owner, symbol, max_premium_bps, budget_micro, max_impact_bps, cooldown_sec, enabled, created_ms, last_triggered_ms
FROM rules
WHERE owner = ?
ORDER BY created_ms ASC, id ASC;`

	rows, err := repo.db.QueryContext(ctx, query, string(owner))
	if err != nil {
		return nil, fmt.Errorf("sqlite list rules by owner: %w", err)
	}
	defer rows.Close()

	rules := make([]domain.Rule, 0)
	for rows.Next() {
		var row ruleRow
		if err := rows.Scan(
			&row.id,
			&row.owner,
			&row.symbol,
			&row.maxPremiumBps,
			&row.budgetMicro,
			&row.maxImpactBps,
			&row.cooldownSec,
			&row.enabled,
			&row.createdMs,
			&row.lastTriggeredMs,
		); err != nil {
			return nil, fmt.Errorf("scan rule row: %w", err)
		}
		rules = append(rules, row.toDomain())
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

func (repo *RuleRepository) ListEnabled(ctx context.Context) ([]domain.Rule, error) {
	query := `
SELECT id, owner, symbol, max_premium_bps, budget_micro, max_impact_bps, cooldown_sec, enabled, created_ms, last_triggered_ms
FROM rules
WHERE enabled = 1
ORDER BY created_ms ASC, id ASC;`

	rows, err := repo.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("sqlite list enabled rules: %w", err)
	}
	defer rows.Close()

	rules := make([]domain.Rule, 0)
	for rows.Next() {
		var row ruleRow
		if err := rows.Scan(
			&row.id,
			&row.owner,
			&row.symbol,
			&row.maxPremiumBps,
			&row.budgetMicro,
			&row.maxImpactBps,
			&row.cooldownSec,
			&row.enabled,
			&row.createdMs,
			&row.lastTriggeredMs,
		); err != nil {
			return nil, fmt.Errorf("scan enabled rule row: %w", err)
		}
		rules = append(rules, row.toDomain())
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

func (repo *RuleRepository) Delete(ctx context.Context, id domain.RuleID) error {
	query := `DELETE FROM rules WHERE id = ?;`
	res, err := repo.db.ExecContext(ctx, query, string(id))
	if err != nil {
		return fmt.Errorf("sqlite delete rule %s: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: rule %s", domain.ErrRuleNotFound, id)
	}
	return nil
}
